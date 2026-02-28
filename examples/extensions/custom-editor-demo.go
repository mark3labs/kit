//go:build ignore

package main

import (
	"fmt"
	"strings"

	"kit/ext"
)

// vimActive tracks whether the vim interceptor is installed at all.
// normalMode tracks whether we are in normal mode (true) or insert mode (false).
var vimActive bool
var normalMode bool

// Init demonstrates the editor interceptor system. Extensions can intercept
// key events before they reach the built-in editor and wrap the editor's
// rendered output. This example implements a simple vim-like modal editor
// with normal/insert mode switching.
//
// Slash commands:
//   - /vim       — toggle vim mode on/off
//   - /vim-info  — show current editor mode
func Init(api ext.API) {
	// /vim — toggle the vim interceptor on/off.
	api.RegisterCommand(ext.CommandDef{
		Name:        "vim",
		Description: "Toggle vim-like modal editing",
		Execute: func(args string, ctx ext.Context) (string, error) {
			if vimActive {
				// Turn off vim mode entirely.
				vimActive = false
				normalMode = false
				ctx.ResetEditor()
				return "Vim mode OFF. Default editor restored.", nil
			}
			// Turn on vim mode, start in normal mode.
			vimActive = true
			normalMode = true
			ctx.SetEditor(ext.EditorConfig{
				HandleKey: func(key string, currentText string) ext.EditorKeyAction {
					return handleVimKey(key, currentText)
				},
				Render: func(width int, defaultContent string) string {
					return renderVimMode(width, defaultContent)
				},
			})
			return "Vim mode ON (NORMAL). Press 'i' to insert, Esc to return to normal, h/j/k/l to navigate.", nil
		},
	})

	// /vim-info — show the current editor mode.
	api.RegisterCommand(ext.CommandDef{
		Name:        "vim-info",
		Description: "Show current vim mode",
		Execute: func(args string, ctx ext.Context) (string, error) {
			if !vimActive {
				return "Vim mode is OFF (default editor).", nil
			}
			if normalMode {
				return "Vim mode ON — NORMAL mode", nil
			}
			return "Vim mode ON — INSERT mode (Esc to return to normal)", nil
		},
	})
}

// handleVimKey processes keys for both normal and insert modes.
// The interceptor stays active in both modes so Esc can switch back.
func handleVimKey(key string, currentText string) ext.EditorKeyAction {
	if !normalMode {
		// ── Insert mode: pass everything through except Esc ──
		if key == "esc" {
			normalMode = true
			return ext.EditorKeyAction{Type: ext.EditorKeyConsumed}
		}
		return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
	}

	// ── Normal mode ──
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
		normalMode = false
		return ext.EditorKeyAction{Type: ext.EditorKeyConsumed}

	// Editing shortcuts.
	case "x":
		return ext.EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "delete"}
	case "0":
		return ext.EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "home"}
	case "$":
		return ext.EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "end"}

	// Submission.
	case "enter":
		if strings.TrimSpace(currentText) != "" {
			return ext.EditorKeyAction{Type: ext.EditorKeySubmit}
		}
		return ext.EditorKeyAction{Type: ext.EditorKeyConsumed}

	// Block most printable keys in normal mode.
	default:
		// Let control sequences and special keys through (e.g., ctrl+c).
		if len(key) > 1 && key != "space" {
			return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
		}
		return ext.EditorKeyAction{Type: ext.EditorKeyConsumed}
	}
}

// renderVimMode wraps the default editor rendering with a mode indicator.
func renderVimMode(width int, defaultContent string) string {
	mode := "-- NORMAL --"
	if !normalMode {
		mode = "-- INSERT --"
	}

	indicator := fmt.Sprintf("  %s", mode)
	padding := width - len(indicator)
	if padding > 0 {
		indicator += strings.Repeat(" ", padding)
	}

	return indicator + "\n" + defaultContent
}
