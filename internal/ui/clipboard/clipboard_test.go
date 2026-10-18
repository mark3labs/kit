package clipboard

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// fakeTools makes a directory with empty executables of the given names the
// whole PATH.
func fakeTools(t *testing.T, names ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

// primaryArgs picks the tool for the primary selection in the order that
// atotto/clipboard uses for the clipboard.
func TestPrimaryArgs(t *testing.T) {
	if !hasPrimary() {
		t.Skip("no primary selection on this platform")
	}
	tests := []struct {
		name             string
		tools            []string
		wayland, display string
		want             []string
	}{
		{"wayland", []string{"wl-copy", "xclip"}, "wayland-0", ":0", []string{"wl-copy", "--primary"}},
		{"wayland without wl-copy", []string{"xclip"}, "wayland-0", ":0", []string{"xclip", "-in", "-selection", "primary"}},
		{"xclip before xsel", []string{"xclip", "xsel"}, "", ":0", []string{"xclip", "-in", "-selection", "primary"}},
		{"xsel", []string{"xsel"}, "", ":0", []string{"xsel", "--input", "--primary"}},
		{"no display", []string{"xclip", "xsel"}, "", "", nil},
		{"no tool", nil, "", ":0", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeTools(t, tc.tools...)
			t.Setenv("WAYLAND_DISPLAY", tc.wayland)
			t.Setenv("DISPLAY", tc.display)
			if got := primaryArgs(); !slices.Equal(got, tc.want) {
				t.Errorf("primaryArgs() = %q; want %q", got, tc.want)
			}
		})
	}
}

// On X11 and Wayland systems, wl-copy, xclip and xsel need a display.
// Without one, as over SSH, their failure is no fault and is not reported.
// termux-clipboard-set and clip.exe (WSL) need no display.
func TestHasSystemClipboard(t *testing.T) {
	if !hasPrimary() {
		t.Skip("no display server on this platform")
	}
	tests := []struct {
		name             string
		tools            []string
		wayland, display string
		want             bool
	}{
		{"wayland", nil, "wayland-0", "", true},
		{"x11", nil, "", ":0", true},
		{"no display", []string{"xclip", "xsel"}, "", "", false},
		{"wsl", []string{"clip.exe"}, "", "", true},
		{"termux", []string{"termux-clipboard-set"}, "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeTools(t, tc.tools...)
			t.Setenv("WAYLAND_DISPLAY", tc.wayland)
			t.Setenv("DISPLAY", tc.display)
			if got := hasSystemClipboard(); got != tc.want {
				t.Errorf("hasSystemClipboard() = %v; want %v", got, tc.want)
			}
		})
	}
}

// A missing tool in an X11 or Wayland session is reported with the tool to
// install for that display, not with atotto/clipboard's text, which also
// names the Termux:API add-on. Other failures keep their own error.
func TestFailure(t *testing.T) {
	if !hasPrimary() {
		t.Skip("no X11 or Wayland on this platform")
	}
	missing := errors.New("no clipboard utilities available")
	exited := errors.New("exit status 1")
	tests := []struct {
		name             string
		err              error
		noTool           bool
		wayland, display string
		want             string
	}{
		{"wayland", missing, true, "wayland-0", ":0", "no clipboard tool found; install wl-clipboard"},
		{"x11", missing, true, "", ":1", "no clipboard tool found; install xclip or xsel"},
		{"no display", missing, true, "", "", missing.Error()},
		{"tool failed", exited, false, "", ":1", exited.Error()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("WAYLAND_DISPLAY", tc.wayland)
			t.Setenv("DISPLAY", tc.display)
			if got := failure(tc.err, tc.noTool).Error(); got != tc.want {
				t.Errorf("failure() = %q; want %q", got, tc.want)
			}
		})
	}
}
