package ui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/style"
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

// fgSeq returns the prefix lipgloss writes to set a colour. It deliberately
// stops before the terminator: lipgloss merges a foreground and a background
// into one sequence, so the prefix is matched rather than the whole escape.
func fgSeq(t *testing.T, c color.Color) string {
	t.Helper()
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("\x1b[38;2;%d;%d;%d", r>>8, g>>8, b>>8)
}

// rowContaining returns the rendered line holding needle, with its escape
// sequences intact.
func rowContaining(out, needle string) string {
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(xansi.Strip(line), needle) {
			return line
		}
	}
	return ""
}

// A row the program left plain must still be drawn in the theme's text colour,
// not the terminal default. Dropping the foreground was a regression once the
// shell started forcing colour: the codes are what separate "this row is
// uncoloured" from "this row is whatever the terminal defaults to".
func TestStreamingBashOutputPlainRowUsesThemeColour(t *testing.T) {
	item := NewStreamingBashOutputItem("id", "echo hi")
	item.AppendStdout("plain text")
	item.MarkComplete()

	row := rowContaining(item.Render(80), "plain text")
	if row == "" {
		t.Fatal("the row was not found")
	}
	if want := fgSeq(t, style.GetTheme().Text); !strings.Contains(row, want) {
		t.Errorf("the theme text colour is missing from a plain row: %q", row)
	}
}

func TestStreamingBashOutputStderrRowUsesErrorColour(t *testing.T) {
	item := NewStreamingBashOutputItem("id", "boom")
	item.AppendStderr("it went wrong")
	item.MarkComplete()

	row := rowContaining(item.Render(80), "it went wrong")
	if row == "" {
		t.Fatal("the row was not found")
	}
	if want := fgSeq(t, style.GetTheme().Error); !strings.Contains(row, want) {
		t.Errorf("a plain stderr row is not in the error colour: %q", row)
	}
}

// A row the program coloured must not be given a foreground here, or a reset
// inside it clears this one and the rest of the row loses its colour.
func TestStreamingBashOutputColouredRowKeepsProgramColour(t *testing.T) {
	item := NewStreamingBashOutputItem("id", "colour")
	item.AppendStdout("\x1b[32mgreen\x1b[0m tail")
	item.MarkComplete()

	row := rowContaining(item.Render(80), "green")
	if row == "" {
		t.Fatal("the row was not found")
	}
	if strings.Contains(row, fgSeq(t, style.GetTheme().Text)) {
		t.Errorf("a foreground was imposed on a coloured row: %q", row)
	}
	// The fill has to be re-armed after the program's reset, or the padding to
	// the right of the row loses the panel background.
	if idx := strings.Index(row, "tail"); idx >= 0 && !strings.Contains(row[idx:], "\x1b[48;") {
		t.Errorf("the fill was not re-armed after the program's reset: %q", row)
	}
}
