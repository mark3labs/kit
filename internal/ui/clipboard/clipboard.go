package clipboard

import (
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
