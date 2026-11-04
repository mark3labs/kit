package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/commands"
)

// helpTestModel returns a model with the help sheet open, an extension
// command, a skill with a long name, and one extension shortcut.
func helpTestModel(w, h int) *AppModel {
	m, _, _ := newTestAppModel(nil)
	in := newTestInput(nil)
	in.commands = append(append([]commands.SlashCommand{}, commands.SlashCommands...),
		commands.SlashCommand{Name: "/explorer", Description: "Toggle the file explorer", Category: "Extensions"},
		commands.SlashCommand{Name: "/a-really-long-skill-name-that-overflows", Description: "Skill with a very long name and an unbreakable-word-that-is-longer-than-any-column-could-ever-be", Category: "Skills"},
	)
	m.input = in
	m.getShortcutList = func() []ShortcutInfo {
		return []ShortcutInfo{{Key: "ctrl+p", Description: "Open the palette", Source: "/x/palette.go"}}
	}
	m.width, m.height = w, h
	m.openHelpModal()
	return m
}

// TestHelpModalLinesHaveEqualWidth checks that no line grows past the box,
// which would skew the border, at small and large terminal sizes.
func TestHelpModalLinesHaveEqualWidth(t *testing.T) {
	for _, size := range [][2]int{{40, 20}, {80, 24}, {120, 40}, {200, 60}} {
		m := helpTestModel(size[0], size[1])
		box := m.renderHelpModal()
		lines := strings.Split(box, "\n")
		want := lipgloss.Width(lines[0])
		for i, l := range lines {
			if got := lipgloss.Width(l); got != want {
				t.Errorf("%dx%d: line %d width %d, want %d: %q", size[0], size[1], i, got, want, xansi.Strip(l))
			}
		}
		if want > size[0] {
			t.Errorf("%dx%d: box width %d is wider than the terminal", size[0], size[1], want)
		}
		if len(lines) > size[1] {
			t.Errorf("%dx%d: box height %d is taller than the terminal", size[0], size[1], len(lines))
		}
	}
}

// TestHelpModalListsEverything checks that the sheet is built from the live
// sources: every command, every built-in key binding and every extension
// shortcut appears, and long names are not cut.
func TestHelpModalListsEverything(t *testing.T) {
	m := helpTestModel(200, 60)

	var text strings.Builder
	for _, sec := range append(m.helpCommandSections(), m.helpKeySections()...) {
		for _, e := range sec.Entries {
			text.WriteString(e.Key + "\n")
		}
	}
	all := text.String()

	for _, c := range m.input.(*InputComponent).commands {
		if !strings.Contains(all, c.Name+"\n") {
			t.Errorf("command %s missing from help", c.Name)
		}
	}
	for _, sec := range keys.keyHelpSections() {
		for _, e := range sec.Entries {
			if !strings.Contains(all, e.Key+"\n") {
				t.Errorf("key %q missing from help", e.Key)
			}
		}
	}
	if !strings.Contains(all, "ctrl+p\n") {
		t.Error("extension shortcut missing from help")
	}

	// Scroll through the whole sheet and collect what is drawn.
	var drawn strings.Builder
	for range 200 {
		drawn.WriteString(xansi.Strip(m.renderHelpModal()))
		m.updateHelpModal(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if !strings.Contains(drawn.String(), "/a-really-long-skill-name-that-overflows") {
		t.Error("long command name was truncated")
	}
}

// TestHelpModalKeys checks scrolling bounds and that esc closes the sheet.
func TestHelpModalKeys(t *testing.T) {
	m := helpTestModel(80, 24)
	_ = m.renderHelpModal()
	if m.helpModal.maxOff == 0 {
		t.Fatal("expected the sheet to scroll at 80x24")
	}

	m.updateHelpModal(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.helpModal.scrollOff != 0 {
		t.Errorf("scrolled above the top: %d", m.helpModal.scrollOff)
	}
	m.updateHelpModal(tea.KeyPressMsg{Code: tea.KeyEnd})
	if m.helpModal.scrollOff != m.helpModal.maxOff {
		t.Errorf("end: offset %d, want %d", m.helpModal.scrollOff, m.helpModal.maxOff)
	}
	m.updateHelpModal(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.helpModal.scrollOff != m.helpModal.maxOff {
		t.Errorf("scrolled past the end: %d", m.helpModal.scrollOff)
	}
	if !m.modalActive() {
		t.Error("modalActive should be true while help is open")
	}

	m.updateHelpModal(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.helpModal != nil {
		t.Error("esc did not close the help sheet")
	}
}
