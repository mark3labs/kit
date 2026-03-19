//go:build ignore

package main

import (
	"os/exec"
	"strings"

	"kit/ext"
)

// Init automatically commits staged changes when the session shuts down,
// using the last assistant message as the commit message.
//
// Only commits if:
//   - There are staged changes (git diff --cached is non-empty)
//   - There is at least one assistant message to use as commit message
//
// The commit message is derived from the last assistant response, trimmed
// to the first paragraph (max 72 chars for the subject line).
//
// Usage: kit -e examples/extensions/auto-commit.go
func Init(api ext.API) {
	api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, ctx ext.Context) {
		// Check for staged changes.
		err := exec.Command("git", "diff", "--cached", "--quiet").Run()
		if err == nil {
			return // exit code 0 means no staged changes
		}

		// Get the last assistant message.
		msgs := ctx.GetMessages()
		var lastAssistant string
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == "assistant" {
				lastAssistant = msgs[i].Content
				break
			}
		}
		if lastAssistant == "" {
			return
		}

		// Build commit message: first paragraph, subject line max 72 chars.
		subject := firstParagraph(lastAssistant)
		if len(subject) > 72 {
			subject = subject[:69] + "..."
		}

		// Commit.
		cmd := exec.Command("git", "commit", "-m", subject)
		output, err := cmd.CombinedOutput()
		if err != nil {
			ctx.PrintError("Auto-commit failed: " + string(output))
			return
		}
		ctx.PrintInfo("Auto-committed: " + subject)
	})
}

// firstParagraph returns the first non-empty paragraph of text.
func firstParagraph(text string) string {
	text = strings.TrimSpace(text)
	// Split on double newlines (paragraph breaks).
	parts := strings.SplitN(text, "\n\n", 2)
	line := strings.TrimSpace(parts[0])
	// Collapse to single line.
	line = strings.ReplaceAll(line, "\n", " ")
	return line
}
