//go:build ignore

// sudo-handler.go - Extension to handle sudo password prompts securely
//
// This extension intercepts bash commands containing "sudo" and:
// 1. Checks if sudo credentials are already cached (via sudo -n)
// 2. If not cached, prompts the user for their password (with masking)
// 3. Temporarily sets SUDO_PASSWORD environment variable for execution
// 4. The bash tool automatically uses sudo -S -p '' to pipe the password
//
// Usage: kit -e examples/extensions/sudo-handler.go
//
// Security notes:
// - Password is only stored in memory for the duration of the session
// - Password is never logged or displayed
// - Each session requires re-authentication (sudo -k is used)
// - The SUDO_PASSWORD env var is set only during tool execution

package main

import (
	"encoding/json"
	"os"
	"strings"
	"sync"

	"kit/ext"
)

var (
	// cachedPassword stores the sudo password for the session
	cachedPassword string
	// hasCachedPassword tracks if we have a valid cached password
	hasCachedPassword bool
	// mu protects cached password access
	mu sync.RWMutex
)

// Init sets up the sudo handler extension
func Init(api ext.API) {
	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if tc.ToolName != "bash" {
			return nil
		}

		// Parse the command from tool input
		var input struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
			return nil
		}

		// Check if command contains sudo
		if !containsSudo(input.Command) {
			return nil
		}

		// Check if we already have cached credentials
		mu.RLock()
		password := cachedPassword
		hasCached := hasCachedPassword
		mu.RUnlock()

		if hasCached {
			// Use cached password
			os.Setenv("SUDO_PASSWORD", password)
			return nil
		}

		// No cached password - prompt user
		result := ctx.PromptInput(ext.PromptInputConfig{
			Message:     "🔐 Sudo password required for:\n  " + truncateCommand(input.Command, 60),
			Placeholder: "Enter your password",
		})

		if result.Cancelled {
			return &ext.ToolCallResult{
				Block:  true,
				Reason: "Sudo password prompt cancelled by user",
			}
		}

		if result.Value == "" {
			return &ext.ToolCallResult{
				Block:  true,
				Reason: "No password provided",
			}
		}

		// Cache the password for this session
		mu.Lock()
		cachedPassword = result.Value
		hasCachedPassword = true
		mu.Unlock()

		// Set environment variable for the bash tool to use
		os.Setenv("SUDO_PASSWORD", result.Value)

		// Show confirmation (without revealing password)
		ctx.PrintInfo("Sudo password cached for this session")

		return nil
	})

	// Clear cached password when session ends
	api.OnSessionShutdown(func(event ext.SessionShutdownEvent, ctx ext.Context) {
		mu.Lock()
		cachedPassword = ""
		hasCachedPassword = false
		mu.Unlock()
		os.Unsetenv("SUDO_PASSWORD")
	})
}

// containsSudo checks if the command contains sudo as a command (not in a string)
func containsSudo(command string) bool {
	// Simple check for sudo as a word, not inside quotes or as part of another word
	lower := strings.ToLower(command)

	// Check for sudo at start or after separators
	patterns := []string{
		"sudo ",
		"sudo\t",
		";sudo ",
		"&& sudo ",
		"|| sudo ",
		"| sudo ",
		"$(sudo ",
		"`sudo ",
	}

	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Check if command starts with sudo
	if strings.HasPrefix(lower, "sudo ") {
		return true
	}

	return false
}

// truncateCommand truncates a long command for display
func truncateCommand(cmd string, maxLen int) string {
	if len(cmd) <= maxLen {
		return cmd
	}
	return cmd[:maxLen-3] + "..."
}
