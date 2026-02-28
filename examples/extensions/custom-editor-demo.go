//go:build ignore

package main

import (
	"fmt"
	"strings"

	"kit/ext"
)

// normalMode tracks whether the vim-like normal mode is active.
// When false, all keys pass through to the default editor (insert mode).
var normalMode bool

// savedCtx holds the extension context for use in the HandleKey callback.
var savedCtx ext.Context

// Init demonstrates the editor interceptor system. Extensions can intercept
// key events before they reach the built-in editor and wrap the editor's
// rendered output. This example implements a simple vim-like modal editor
// with normal/insert mode switching.
//
// Slash commands:
//   - /vim       — toggle vim mode (normal ↔ insert)
//   - /vim-info  — show current editor mode
func Init(api ext.API) {
	// /vim — toggle vim-like normal/insert mode.
	api.RegisterCommand(ext.CommandDef{
		Name:        "vim",
		Description: "Toggle vim-like normal/insert mode",
		Execute: func(args string, ctx ext.Context) (string, error) {
			savedCtx = ctx
			if normalMode {
				// Switch to insert mode (remove interceptor).
				normalMode = false
				ctx.ResetEditor()
				return "Switched to INSERT mode (default editor).", nil
			}
			// Switch to normal mode (install interceptor).
			normalMode = true
			ctx.SetEditor(ext.EditorConfig{
				HandleKey: func(key string, currentText string) ext.EditorKeyAction {
					return handleVimKey(key, currentText)
				},
				Render: func(width int, defaultContent string) string {
					return renderVimMode(width, defaultContent)
				},
			})
			return "Switched to NORMAL mode. Press 'i' to insert, 'h/j/k/l' to navigate.", nil
		},
	})

	// /vim-info — show the current editor mode.
	api.RegisterCommand(ext.CommandDef{
		Name:        "vim-info",
		Description: "Show current vim mode",
		Execute: func(args string, ctx ext.Context) (string, error) {
			if normalMode {
				return "Current mode: NORMAL (vim interceptor active)", nil
			}
			return "Current mode: INSERT (default editor)", nil
		},
	})
}

// handleVimKey processes keys in vim normal mode.
func handleVimKey(key string, currentText string) ext.EditorKeyAction {
	switch key {
	// Navigation: remap hjkl to arrow keys.
	case "h":
		return ext.EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "left"}
	case "j":
		return ext.EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "down"}
	case "k":
		return ext.EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "up"}
	case "l":
		return ext.EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "right"}

	// Mode switching.
	case "i":
		// Enter insert mode.
		normalMode = false
		if savedCtx.ResetEditor != nil {
			savedCtx.ResetEditor()
		}
		return ext.EditorKeyAction{Type: ext.EditorKeyConsumed}

	// Editing shortcuts.
	case "x":
		// Delete character under cursor (remap to delete key).
		return ext.EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "delete"}
	case "0":
		// Jump to beginning of line.
		return ext.EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "home"}
	case "$":
		// Jump to end of line.
		return ext.EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "end"}

	// Submission.
	case "enter":
		// In normal mode, Enter submits the current text.
		if strings.TrimSpace(currentText) != "" {
			return ext.EditorKeyAction{Type: ext.EditorKeySubmit}
		}
		return ext.EditorKeyAction{Type: ext.EditorKeyConsumed}

	// Block most printable keys in normal mode.
	default:
		// Let control sequences and special keys through (e.g., ctrl+c, esc).
		if len(key) > 1 && key != "space" {
			return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
		}
		// Consume single printable characters — don't insert in normal mode.
		return ext.EditorKeyAction{Type: ext.EditorKeyConsumed}
	}
}

// renderVimMode wraps the default editor rendering with a mode indicator.
func renderVimMode(width int, defaultContent string) string {
	mode := "-- NORMAL --"
	if !normalMode {
		mode = "-- INSERT --"
	}

	// Build a mode indicator line.
	indicator := fmt.Sprintf("  %s", mode)

	// Pad to fill width.
	padding := width - len(indicator)
	if padding > 0 {
		indicator += strings.Repeat(" ", padding)
	}

	return indicator + "\n" + defaultContent
}
