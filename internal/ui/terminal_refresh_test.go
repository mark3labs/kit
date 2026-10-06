package ui

import (
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/mark3labs/kit/internal/ui/style"
)

func TestRemoteCapabilitiesRefreshOnWindowSize(t *testing.T) {
	saved := style.CaptureStylingState()
	t.Cleanup(func() { saved.Restore() })
	style.SetTerminalCapabilities(true, colorprofile.TrueColor)
	calls := 0
	m := NewAppModel(&stubAppController{}, AppModelOptions{
		ReadTerminalCapabilities: func() error {
			calls++
			style.SetTerminalCapabilities(false, colorprofile.ANSI)
			return nil
		},
	})
	// The same dimensions must still read the file. This also covers modal
	// paths that do not reach the main window-size handler.
	for _, state := range []appState{stateInput, stateThemeSelector} {
		m.state = state
		before := style.ThemeGeneration()
		_, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		if calls == 1 && cmd == nil {
			t.Fatal("capability change did not request repaint")
		}
		if style.IsDarkBackground() || style.ColorProfile() != colorprofile.ANSI {
			t.Fatal("terminal capabilities were not applied")
		}
		if style.ThemeGeneration() <= before {
			t.Fatal("theme caches were not invalidated")
		}
	}
	if calls != 2 {
		t.Fatalf("read calls = %d, want 2", calls)
	}
}

func TestRefreshReadsRemoteCapabilitiesAndInvalidatesCaches(t *testing.T) {
	saved := style.CaptureStylingState()
	t.Cleanup(func() { saved.Restore() })
	style.SetTerminalCapabilities(true, colorprofile.ANSI256)
	calls := 0
	m := NewAppModel(&stubAppController{}, AppModelOptions{
		ReadTerminalCapabilities: func() error { calls++; return nil },
	})
	before := style.ThemeGeneration()
	if cmd := m.handleRefreshCommand(); cmd == nil {
		t.Fatal("refresh did not request repaint")
	}
	if calls != 1 {
		t.Fatalf("read calls = %d, want 1", calls)
	}
	if style.ThemeGeneration() <= before {
		t.Fatal("unchanged capabilities did not invalidate caches")
	}
}

func TestLocalBackgroundReplyRefreshesTheme(t *testing.T) {
	saved := style.CaptureStylingState()
	t.Cleanup(func() { saved.Restore() })
	style.SetTerminalCapabilities(true, colorprofile.TrueColor)
	m := NewAppModel(&stubAppController{}, AppModelOptions{})
	before := style.ThemeGeneration()
	_, cmd := m.Update(tea.BackgroundColorMsg{Color: color.White})
	if style.IsDarkBackground() {
		t.Fatal("light background reply was ignored")
	}
	if style.ColorProfile() != colorprofile.TrueColor {
		t.Fatal("background reply changed profile")
	}
	if style.ThemeGeneration() <= before || cmd == nil {
		t.Fatal("background reply did not refresh and repaint")
	}
}

func TestRemoteIgnoresPTYCapabilityReplies(t *testing.T) {
	saved := style.CaptureStylingState()
	t.Cleanup(func() { saved.Restore() })
	style.SetTerminalCapabilities(true, colorprofile.ANSI)
	m := NewAppModel(&stubAppController{}, AppModelOptions{ReadTerminalCapabilities: func() error { return nil }})
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	if !style.IsDarkBackground() || style.ColorProfile() != colorprofile.ANSI {
		t.Fatal("PTY replies replaced remote capabilities")
	}
}
