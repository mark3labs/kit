package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/mark3labs/kit/internal/daemon"
	"github.com/mark3labs/kit/internal/ui"
	"github.com/mark3labs/kit/internal/ui/style"
)

func TestSessionTerminalCapabilitiesPreferFile(t *testing.T) {
	saved := style.CaptureStylingState()
	t.Cleanup(saved.Restore)
	path := filepath.Join(t.TempDir(), "terminal.json")
	t.Setenv(daemon.RemoteTerminalFileEnv, path)
	t.Setenv(daemon.RemoteBackgroundEnv, "#000000")
	if err := os.WriteFile(path, []byte(`{"background":"#ffffff","color_profile":3}`), 0600); err != nil {
		t.Fatal(err)
	}
	adoptSessionTerminalCapabilities()
	if ui.IsDarkBackground() || ui.ColorProfile() != colorprofile.ANSI {
		t.Fatal("startup did not prefer capability file")
	}
	if err := os.WriteFile(path, []byte(`{"background":"#000000","color_profile":5}`), 0600); err != nil {
		t.Fatal(err)
	}
	read := sessionTerminalCapabilityReader()
	if read == nil {
		t.Fatal("remote callback is nil")
	}
	if err := read(); err != nil {
		t.Fatal(err)
	}
	if !ui.IsDarkBackground() || ui.ColorProfile() != colorprofile.TrueColor {
		t.Fatal("live callback did not apply new capabilities")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := read(); err == nil {
		t.Fatal("missing file did not return an error")
	}
	if !ui.IsDarkBackground() || ui.ColorProfile() != colorprofile.TrueColor {
		t.Fatal("missing file changed capabilities")
	}
}

func TestSessionTerminalCapabilitiesPreserveMissingFields(t *testing.T) {
	saved := style.CaptureStylingState()
	t.Cleanup(saved.Restore)
	ui.SetTerminalCapabilities(false, colorprofile.ANSI256)
	path := filepath.Join(t.TempDir(), "terminal.json")
	t.Setenv(daemon.RemoteTerminalFileEnv, path)
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := readSessionTerminalCapabilities(); err != nil {
		t.Fatal(err)
	}
	if ui.IsDarkBackground() || ui.ColorProfile() != colorprofile.ANSI256 {
		t.Fatal("missing fields changed capabilities")
	}
	t.Setenv(daemon.RemoteTerminalFileEnv, "")
	if sessionTerminalCapabilityReader() != nil {
		t.Fatal("local terminal has remote callback")
	}
}
