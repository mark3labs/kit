//go:build ignore

package main

import (
	"encoding/json"
	"strings"

	"kit/ext"
)

// Init intercepts potentially dangerous bash commands and asks the user for
// confirmation before allowing execution. Inspired by Pi's permission-gate.ts.
//
// Dangerous patterns: rm -rf, sudo, chmod 777, mkfs, dd, > /dev/
//
// Usage: kit -e examples/extensions/permission-gate.go
func Init(api ext.API) {
	// Patterns that require user confirmation.
	dangerousPatterns := []string{
		"rm -rf",
		"rm -r /",
		"sudo ",
		"chmod 777",
		"chmod -R 777",
		"mkfs",
		"dd if=",
		"> /dev/",
		":(){ :|:& };:",
	}

	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if tc.ToolName != "Bash" {
			return nil
		}

		// Extract the command from the tool input JSON.
		var input struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
			return nil
		}
		cmd := strings.ToLower(input.Command)

		// Check for dangerous patterns.
		for _, pattern := range dangerousPatterns {
			if strings.Contains(cmd, strings.ToLower(pattern)) {
				result := ctx.PromptConfirm(ext.PromptConfirmConfig{
					Message: "Dangerous command detected: " + input.Command + "\n\nAllow execution?",
				})
				if result.Cancelled || !result.Value {
					return &ext.ToolCallResult{
						Block:  true,
						Reason: "User denied execution of dangerous command: " + input.Command,
					}
				}
				return nil // user approved
			}
		}

		return nil
	})
}
