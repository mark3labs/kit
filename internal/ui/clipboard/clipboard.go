package clipboard

import (
	"runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// CopyToClipboard writes text to both the system clipboard and via OSC 52.
// Returns a tea.Cmd that can be used in Bubble Tea's Update flow.
func CopyToClipboard(text string) tea.Cmd {
	if text == "" {
		return nil
	}

	return tea.Sequence(
		// Method 1: OSC 52 escape sequence (works in modern terminals)
		tea.SetClipboard(text),

		// Method 2: Native system clipboard (atotto/clipboard)
		func() tea.Msg {
			// Best effort - ignore errors
			_ = clipboard.WriteAll(text)
			return nil
		},
	)
}

// CopyToClipboardWithMessage writes text to clipboard and returns a toast notification.
func CopyToClipboardWithMessage(text string, message string) tea.Cmd {
	if text == "" {
		return nil
	}

	return tea.Sequence(
		CopyToClipboard(text),
		func() tea.Msg {
			return ToastMsg{Message: message, Type: ToastInfo}
		},
	)
}

// ToastType represents the type of toast notification.
type ToastType int

const (
	ToastInfo ToastType = iota
	ToastSuccess
	ToastWarning
	ToastError
)

// ToastMsg is a message to display a toast notification.
type ToastMsg struct {
	Message string
	Type    ToastType
}

// IsClipboardSupported returns true if the clipboard is supported on this platform.
func IsClipboardSupported() bool {
	switch runtime.GOOS {
	case "darwin", "windows":
		return true
	case "linux":
		return true
	default:
		return false
	}
}
