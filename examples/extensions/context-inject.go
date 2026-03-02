// context-inject.go — Injects context from a local file into every LLM turn.
//
// Reads a context file (default: .kit/context.md) and prepends it as a system
// message to every LLM context window via OnContextPrepare. This is useful for
// injecting project-specific knowledge, coding standards, or RAG results that
// should always be visible to the model — without cluttering the session history.
//
// The injected message does NOT persist in the session tree (it's ephemeral,
// added at query time only). This means:
//   - Changing the context file immediately affects future turns
//   - No session bloat from repeated context injection
//   - The model always sees the latest version of the context
//
// Configuration:
//
//	KIT_OPT_CONTEXT_FILE  — path to context file (default: .kit/context.md)
//
// Usage:
//
//	kit -e examples/extensions/context-inject.go
//	echo "Always use error wrapping with fmt.Errorf" > .kit/context.md
package main

import (
	"fmt"
	"os"
	"strings"

	ext "kit/ext"
)

func Init(api ext.API) {
	api.RegisterOption(ext.OptionDef{
		Name:        "context-file",
		Description: "Path to the context file to inject into every turn",
		Default:     ".kit/context.md",
	})

	api.OnContextPrepare(func(e ext.ContextPrepareEvent, ctx ext.Context) *ext.ContextPrepareResult {
		path := ctx.GetOption("context-file")
		if path == "" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			// File doesn't exist or can't be read — skip silently.
			return nil
		}

		content := strings.TrimSpace(string(data))
		if content == "" {
			return nil
		}

		// Prepend a system message with the context file contents.
		injected := ext.ContextMessage{
			Index:   -1,
			Role:    "system",
			Content: fmt.Sprintf("[Project Context from %s]\n\n%s", path, content),
		}

		msgs := make([]ext.ContextMessage, 0, len(e.Messages)+1)
		msgs = append(msgs, injected)
		msgs = append(msgs, e.Messages...)

		return &ext.ContextPrepareResult{Messages: msgs}
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "context",
		Description: "Show or edit the injected context file path",
		Execute: func(args string, ctx ext.Context) (string, error) {
			path := ctx.GetOption("context-file")
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Sprintf("Context file: %s (not found or unreadable)", path), nil
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			preview := strings.Join(lines, "\n")
			if len(lines) > 10 {
				preview = strings.Join(lines[:10], "\n") + "\n..."
			}
			return fmt.Sprintf("Context file: %s (%d lines)\n\n%s", path, len(lines), preview), nil
		},
	})
}
