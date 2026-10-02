package ui

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
)

// A real command in a real repository. This is the case the colour work exists
// for: git colours its output whenever it believes a terminal is attached, and
// that output is what the user wants to read.
func TestRenderBashBodyKeepsGitColour(t *testing.T) {
	result := "\x1b[32m+ added line\x1b[0m\n\x1b[31m- removed line\x1b[0m\n"

	out := renderBashBody("", result, 80, 10)
	plain := xansi.Strip(out)

	if !strings.Contains(plain, "+ added line") || !strings.Contains(plain, "- removed line") {
		t.Fatalf("text lost: %q", plain)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("colour did not survive into the tool body: %q", out)
	}
	if strings.Contains(out, "\x1b[32m") || strings.Contains(out, "\x1b[31m") {
		t.Errorf("base colours were not remapped: %q", out)
	}
}

// A program may end a line with a reset. The panel fill has to come back after
// it, or the right-hand end of the row loses its background.
func TestRenderBashBodyReArmsPanelFill(t *testing.T) {
	out := renderBashBody("", "\x1b[32mgreen\x1b[0m tail\n", 80, 10)

	row := ""
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(xansi.Strip(line), "green") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatalf("row not found in %q", xansi.Strip(out))
	}
	text := strings.Index(row, "green")
	if text < 0 {
		t.Fatalf("text missing: %q", row)
	}
	if !strings.Contains(row[text:], "\x1b[48;") {
		t.Errorf("panel fill was not re-armed after the program's reset: %q", row)
	}
}

// Cursor control from a progress bar must never reach the frame.
func TestRenderBashBodyStripsCursorControl(t *testing.T) {
	out := renderBashBody("", "\x1b[2K\x1b[1Gbuilding...\x1b[K\ndone\n", 80, 10)

	for _, seq := range []string{"\x1b[2K", "\x1b[1G", "\x1b[K"} {
		if strings.Contains(out, seq) {
			t.Errorf("cursor control %q survived: %q", seq, out)
		}
	}
	if plain := xansi.Strip(out); !strings.Contains(plain, "done") {
		t.Errorf("text lost: %q", plain)
	}
}

// Every tool body panel is drawn by the same helper, so the geometry contract
// has to hold for the panel change as well.
func TestToolPanelFillKeepsPanelWidth(t *testing.T) {
	panel := newToolPanel(80)
	for _, line := range []string{"short", strings.Repeat("W", 300), "\x1b[31m" + strings.Repeat("C", 40) + "\x1b[0m"} {
		row := panel.line(line, false)
		if w := xansi.StringWidth(row); w != panel.width {
			t.Errorf("row is %d columns, want %d: %q", w, panel.width, line)
		}
	}
}

func TestToolPanelBlankFillsWidth(t *testing.T) {
	panel := newToolPanel(80)
	if w := xansi.StringWidth(panel.blank()); w != panel.width {
		t.Errorf("blank row is %d columns, want %d", w, panel.width)
	}
}
