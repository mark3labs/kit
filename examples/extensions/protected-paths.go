//go:build ignore

package main

import (
	"encoding/json"
	"strings"

	"kit/ext"
)

// Init blocks tool calls that attempt to write, edit, or delete files in
// protected paths.
//
// Protected: .env*, .git/, secrets/, credentials*, *.pem, *.key
//
// Usage: kit -e examples/extensions/protected-paths.go
func Init(api ext.API) {
	// Tools that modify files.
	writeTools := map[string]bool{
		"write": true,
		"edit":  true,
		"shell": true,
		"bash":  true,
	}

	// Path patterns to protect (checked against the path field and shell commands).
	protectedPatterns := []string{
		".env",
		".git/",
		"secrets/",
		"credentials",
		".pem",
		".key",
		"id_rsa",
		"id_ed25519",
	}

	// Bash commands that could modify protected files.
	bashWritePatterns := []string{
		"rm ", "mv ", "cp ", "> ",
		"cat >", "echo >", "tee ",
		"chmod ", "chown ",
	}

	isProtected := func(path string) bool {
		lower := strings.ToLower(path)
		for _, p := range protectedPatterns {
			if strings.Contains(lower, p) {
				return true
			}
		}
		return false
	}

	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if !writeTools[tc.ToolName] {
			return nil
		}

		// For write and edit: check the path field.
		if tc.ToolName == "write" || tc.ToolName == "edit" {
			var input map[string]any
			if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
				return nil
			}
			filePath, _ := input["path"].(string)
			if isProtected(filePath) {
				return &ext.ToolCallResult{
					Block:  true,
					Reason: "Blocked: writing to protected path: " + filePath,
				}
			}
			return nil
		}

		// For the shell tool: check if the command references protected paths.
		if tc.ToolName == "shell" || tc.ToolName == "bash" {
			var input struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
				return nil
			}

			// Only check bash commands that look like file mutations.
			isMutation := false
			for _, pat := range bashWritePatterns {
				if strings.Contains(input.Command, pat) {
					isMutation = true
					break
				}
			}
			if !isMutation {
				return nil
			}

			// Check if any protected pattern appears in the command.
			for _, p := range protectedPatterns {
				if strings.Contains(input.Command, p) {
					return &ext.ToolCallResult{
						Block:  true,
						Reason: "Blocked: bash command references protected path (" + p + "): " + input.Command,
					}
				}
			}
		}

		return nil
	})
}
