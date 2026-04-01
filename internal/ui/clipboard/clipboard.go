package clipboard

import (
	"fmt"
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
	// atotto/clipboard supports Linux (with xclip or xsel), macOS, Windows
	switch runtime.GOOS {
	case "darwin", "windows":
		return true
	case "linux":
		// Check if xclip or xsel is available
		// This is a best-effort check
		return true
	default:
		return false
	}
}

// CopySelection represents a text selection with start/end positions.
type CopySelection struct {
	StartItemIdx int  // Index of item where selection starts
	StartLine    int  // Line within item where selection starts
	StartCol     int  // Column where selection starts
	EndItemIdx   int  // Index of item where selection ends
	EndLine      int  // Line within item where selection ends
	EndCol       int  // Column where selection ends
	Active       bool // Whether selection is currently active
}

// IsEmpty returns true if the selection has no content.
func (s CopySelection) IsEmpty() bool {
	return !s.Active || (s.StartItemIdx == s.EndItemIdx && s.StartLine == s.EndLine && s.StartCol == s.EndCol)
}

// String returns a string representation for debugging.
func (s CopySelection) String() string {
	return fmt.Sprintf("Selection{item:%d-%d, line:%d-%d, col:%d-%d, active:%v}",
		s.StartItemIdx, s.EndItemIdx, s.StartLine, s.EndLine, s.StartCol, s.EndCol, s.Active)
}
