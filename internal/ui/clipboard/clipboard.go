package clipboard

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// NativeCopyFailedMsg reports that the system clipboard could not be
// written, for example because no clipboard tool is installed. The text
// still went to the terminal as an OSC 52 sequence, which only some
// terminals support.
type NativeCopyFailedMsg struct {
	Err error
}

// CopyToClipboard writes text to both the system clipboard and via OSC 52.
// Returns a tea.Cmd that can be used in Bubble Tea's Update flow. When the
// system clipboard cannot be written, the command yields a
// NativeCopyFailedMsg, except where no system clipboard exists (see
// hasSystemClipboard).
func CopyToClipboard(text string) tea.Cmd {
	if text == "" {
		return nil
	}

	return tea.Sequence(
		// Method 1: OSC 52 escape sequence (works in modern terminals)
		tea.SetClipboard(text),

		// Method 2: Native system clipboard (atotto/clipboard)
		writeNative(text),
	)
}

// CopySelection copies text that the user selected with the mouse. It does
// what CopyToClipboard does and, on X11 and Wayland, also sets the primary
// selection, so a middle click pastes the text as it does after a selection
// in the terminal itself.
func CopySelection(text string) tea.Cmd {
	if text == "" {
		return nil
	}
	if !hasPrimary() {
		return CopyToClipboard(text)
	}

	return tea.Sequence(
		tea.SetClipboard(text),
		tea.SetPrimaryClipboard(text),
		writeNative(text),
		writePrimary(text),
	)
}

// writeNative writes the system clipboard with atotto/clipboard.
func writeNative(text string) tea.Cmd {
	return func() tea.Msg {
		if err := clipboard.WriteAll(text); err != nil && hasSystemClipboard() {
			return NativeCopyFailedMsg{Err: failure(err, clipboard.Unsupported)}
		}
		return nil
	}
}

// failure returns the error to report for a failed write. When no tool was
// found (noTool), atotto/clipboard's own error also names the Termux:API
// add-on for Android. An X11 or Wayland session gets the tool to install
// for its display instead.
func failure(err error, noTool bool) error {
	if !noTool || !hasPrimary() {
		return err
	}
	switch {
	case os.Getenv("WAYLAND_DISPLAY") != "":
		return errors.New("no clipboard tool found; install wl-clipboard")
	case os.Getenv("DISPLAY") != "":
		return errors.New("no clipboard tool found; install xclip or xsel")
	}
	return err
}

// hasSystemClipboard reports whether a failed write to the system clipboard
// is a fault worth reporting. On X11 and Wayland systems, wl-copy, xclip and
// xsel need a display. Without one, for example in an SSH session without X
// forwarding, they cannot work, and only OSC 52 can copy. The other tools
// that atotto/clipboard uses there, termux-clipboard-set and clip.exe on
// WSL, need no display.
func hasSystemClipboard() bool {
	if !hasPrimary() || os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("DISPLAY") != "" {
		return true
	}
	for _, tool := range []string{"termux-clipboard-set", "clip.exe"} {
		if _, err := exec.LookPath(tool); err == nil {
			return true
		}
	}
	return false
}

// writePrimary writes the primary selection with the first tool that
// primaryArgs finds. It reports no error: writeNative already reports a
// missing clipboard tool.
func writePrimary(text string) tea.Cmd {
	return func() tea.Msg {
		args := primaryArgs()
		if args == nil {
			return nil
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		_ = cmd.Run()
		return nil
	}
}

// hasPrimary reports whether the platform can have a primary selection.
// These are the systems with X11 or Wayland, where atotto/clipboard uses
// wl-copy, xclip or xsel.
func hasPrimary() bool {
	switch runtime.GOOS {
	case "linux", "freebsd", "netbsd", "openbsd", "dragonfly", "solaris", "illumos":
		return true
	}
	return false
}

// primaryArgs returns the command line that writes the primary selection,
// or nil when no tool is available. It tries the tools in the order that
// atotto/clipboard tries them for the clipboard: wl-copy on Wayland, then
// xclip, then xsel. xclip and xsel need an X display.
func primaryArgs() []string {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("wl-copy"); err == nil {
			return []string{"wl-copy", "--primary"}
		}
	}
	if os.Getenv("DISPLAY") == "" {
		return nil
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		return []string{"xclip", "-in", "-selection", "primary"}
	}
	if _, err := exec.LookPath("xsel"); err == nil {
		return []string{"xsel", "--input", "--primary"}
	}
	return nil
}
