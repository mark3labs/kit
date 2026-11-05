package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/core"
)

// sheetPlain renders the open sheet as plain text, or "" when none is open.
func sheetPlain(m *AppModel) string {
	if m.sheet == nil {
		return ""
	}
	return xansi.Strip(m.renderSheet())
}

// TestInfoCommandsOpenSheets checks that each info command opens a sheet
// with its content and prints nothing to the scrollback.
func TestInfoCommandsOpenSheets(t *testing.T) {
	cases := []struct {
		cmd  string
		want string
	}{
		{"/tools", "bash_exec"},
		{"/servers", "filesystem"},
		{"/session", "stub-session"},
		{"/shortcuts", "Open the palette"},
	}
	for _, c := range cases {
		t.Run(c.cmd, func(t *testing.T) {
			m, _, _ := newTestAppModel(&stubAppController{hasSession: true})
			m.toolNames = []string{"bash_exec", "read_file"}
			m.serverNames = []string{"filesystem"}
			m.getShortcutList = func() []ShortcutInfo {
				return []ShortcutInfo{{Key: "ctrl+p", Description: "Open the palette", Source: "/x/palette.go"}}
			}
			before := len(m.messages)
			m = sendMsg(m, core.SubmitMsg{Text: c.cmd})
			if m.sheet == nil {
				t.Fatalf("%s did not open a sheet", c.cmd)
			}
			if text := sheetPlain(m); !strings.Contains(text, c.want) {
				t.Errorf("%s sheet is missing %q:\n%s", c.cmd, c.want, text)
			}
			if len(m.messages) != before {
				t.Errorf("%s printed to the scrollback", c.cmd)
			}
		})
	}
}

// TestInfoSheetsLinesHaveEqualWidth checks that the box border stays
// straight for every info sheet at small and large sizes.
func TestInfoSheetsLinesHaveEqualWidth(t *testing.T) {
	long := strings.Repeat("a-very-long-tool-name-", 8)
	for _, cmd := range []string{"/tools", "/servers", "/session", "/shortcuts"} {
		for _, size := range [][2]int{{40, 20}, {80, 24}, {200, 60}} {
			m, _, _ := newTestAppModel(&stubAppController{hasSession: true})
			m.width, m.height = size[0], size[1]
			m.toolNames = []string{"bash", long, "fs_read", "fs_write", "fetch"}
			m.serverNames = []string{long}
			m.getShortcutList = func() []ShortcutInfo {
				return []ShortcutInfo{{Key: "ctrl+t", Description: "Open\tthe \x1b[1mterminal\x1b[0m\t\t", Source: "/x/term.go"}}
			}
			m = sendMsg(m, core.SubmitMsg{Text: cmd})
			lines := strings.Split(m.renderSheet(), "\n")
			want := lipgloss.Width(lines[0])
			for i, l := range lines {
				if got := lipgloss.Width(l); got != want {
					t.Errorf("%s %dx%d: line %d width %d, want %d: %q", cmd, size[0], size[1], i, got, want, xansi.Strip(l))
				}
			}
			if want > size[0] || len(lines) > size[1] {
				t.Errorf("%s %dx%d: box %dx%d does not fit", cmd, size[0], size[1], want, len(lines))
			}
		}
	}
}

// TestSheetDragSelectCopies checks that a drag over the body selects the
// text under it, copies it on release, and shows the count in the footer.
func TestSheetDragSelectCopies(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.toolNames = []string{"alpha_tool"}
	m.openToolsSheet()
	_ = m.renderSheet()
	sh := m.sheet

	// The first body line holds "alpha_tool" at the left of the body.
	x, y := sh.bodyX, sh.bodyY
	if handled, _ := m.updateSheet(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}); !handled {
		t.Fatal("click not handled")
	}
	m.updateSheet(tea.MouseMotionMsg{X: x + 5, Y: y, Button: tea.MouseLeft})
	handled, cmd := m.updateSheet(tea.MouseReleaseMsg{X: x + 5, Y: y, Button: tea.MouseLeft})
	if !handled || cmd == nil {
		t.Fatal("release did not copy")
	}
	if got := sh.selectedText(); got != "alpha" {
		t.Errorf("selected %q, want %q", got, "alpha")
	}
	if !strings.Contains(sheetPlain(m), "Copied 5 chars") {
		t.Error("footer does not report the copy")
	}

	// Triple click selects the whole line, without its padding.
	now := time.Now().Add(time.Second)
	for range 3 {
		sh.mouseDown(x+1, y, now)
	}
	if got := sh.mouseUp(); got != "alpha_tool" {
		t.Errorf("triple click selected %q, want %q", got, "alpha_tool")
	}

	// A click outside the body clears the selection.
	m.updateSheet(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	m.updateSheet(tea.MouseReleaseMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if sh.sel.HasSelection() {
		t.Error("click outside the body kept the selection")
	}
}

// TestHelpSheetDragScrolls checks that a drag past the bottom edge scrolls
// the help sheet and extends the selection onto the new lines.
func TestHelpSheetDragScrolls(t *testing.T) {
	m := helpTestModel(80, 24)
	_ = m.renderSheet()
	sh := m.sheet
	if sh.maxOff == 0 {
		t.Fatal("expected the help sheet to scroll at 80x24")
	}
	m.updateSheet(tea.MouseClickMsg{X: sh.bodyX, Y: sh.bodyY, Button: tea.MouseLeft})
	below := sh.bodyY + sh.pageSize + 2
	for range 3 {
		m.updateSheet(tea.MouseMotionMsg{X: sh.bodyX + 10, Y: below, Button: tea.MouseLeft})
	}
	if sh.scrollOff != 3 {
		t.Errorf("scrollOff = %d after 3 drags past the edge, want 3", sh.scrollOff)
	}
	if sh.sel.DragLineIdx != sh.scrollOff+sh.pageSize-1 {
		t.Errorf("drag line %d, want the last visible line %d", sh.sel.DragLineIdx, sh.scrollOff+sh.pageSize-1)
	}
	if text := sh.mouseUp(); !strings.HasPrefix(text, "COMMANDS") {
		t.Errorf("selection %q does not start at the top of the body", text)
	}
}
