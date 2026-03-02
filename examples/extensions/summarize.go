//go:build ignore

package main

import (
	"fmt"
	"strings"

	"kit/ext"
)

// Init adds a /summarize command that generates a concise summary of the
// current conversation using a direct LLM completion. Demonstrates the
// ctx.Complete API (Gap 17). Inspired by Pi's summarize.ts.
//
// The summary is displayed in a styled block and can optionally be saved
// to the session via AppendEntry for later retrieval.
//
// Usage: kit -e examples/extensions/summarize.go
func Init(api ext.API) {
	api.RegisterCommand(ext.CommandDef{
		Name:        "summarize",
		Description: "Summarize the current conversation",
		Execute: func(args string, ctx ext.Context) (string, error) {
			msgs := ctx.GetMessages()
			if len(msgs) == 0 {
				ctx.PrintInfo("Nothing to summarize — no messages yet.")
				return "", nil
			}

			// Build a text representation of the conversation.
			var parts []string
			for _, m := range msgs {
				content := m.Content
				if len(content) > 2000 {
					content = content[:1997] + "..."
				}
				parts = append(parts, fmt.Sprintf("[%s]: %s", m.Role, content))
			}
			conversation := strings.Join(parts, "\n\n")

			ctx.PrintInfo("Generating summary...")

			resp, err := ctx.Complete(ext.CompleteRequest{
				System: `You are a concise summarization assistant. Summarize the conversation below in 3-5 bullet points. Focus on:
- What was discussed or requested
- Key decisions or outcomes
- Any pending action items

Be concise. Use plain text, no markdown headers.`,
				Prompt: conversation,
			})
			if err != nil {
				ctx.PrintError("Summary failed: " + err.Error())
				return "", nil
			}

			summary := strings.TrimSpace(resp.Text)

			ctx.PrintBlock(ext.PrintBlockOpts{
				Text:        summary,
				BorderColor: "#89b4fa",
				Subtitle:    fmt.Sprintf("Summary (%d messages, %d tokens used)", len(msgs), resp.InputTokens+resp.OutputTokens),
			})

			// Persist the summary in the session for later retrieval.
			ctx.AppendEntry("summary", summary)

			return "", nil
		},
	})

	// /summaries — list all saved summaries.
	api.RegisterCommand(ext.CommandDef{
		Name:        "summaries",
		Description: "List saved conversation summaries",
		Execute: func(args string, ctx ext.Context) (string, error) {
			entries := ctx.GetEntries("summary")
			if len(entries) == 0 {
				ctx.PrintInfo("No summaries saved yet. Use /summarize to create one.")
				return "", nil
			}
			for i, e := range entries {
				ctx.PrintBlock(ext.PrintBlockOpts{
					Text:        e.Data,
					BorderColor: "#89b4fa",
					Subtitle:    fmt.Sprintf("Summary #%d (%s)", i+1, e.Timestamp[:19]),
				})
			}
			return "", nil
		},
	})
}
