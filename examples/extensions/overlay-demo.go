//go:build ignore

package main

import (
	"fmt"
	"strings"

	"kit/ext"
)

// Init demonstrates the overlay dialog system. Extensions can show modal
// overlay dialogs that block until the user dismisses them or selects an
// action. Four slash commands illustrate different overlay use cases.
func Init(api ext.API) {
	// /overlay-info — simple information dialog (no actions, dismissed with Enter or ESC).
	api.RegisterCommand(ext.CommandDef{
		Name:        "overlay-info",
		Description: "Show an info overlay dialog",
		Execute: func(args string, ctx ext.Context) (string, error) {
			content := "This is a simple informational overlay.\n\n" +
				"Overlays are modal dialogs that appear over the TUI.\n" +
				"They can display plain text or markdown content.\n\n" +
				"Press Enter or ESC to dismiss."

			result := ctx.ShowOverlay(ext.OverlayConfig{
				Title:   "Information",
				Content: ext.WidgetContent{Text: content},
				Style:   ext.OverlayStyle{BorderColor: "#89b4fa"},
			})

			if result.Cancelled {
				return "Info dialog cancelled.", nil
			}
			return "Info dialog dismissed.", nil
		},
	})

	// /overlay-actions — overlay with action buttons.
	api.RegisterCommand(ext.CommandDef{
		Name:        "overlay-actions",
		Description: "Show an overlay with action buttons",
		Execute: func(args string, ctx ext.Context) (string, error) {
			result := ctx.ShowOverlay(ext.OverlayConfig{
				Title: "Deploy to Production?",
				Content: ext.WidgetContent{
					Text: "You are about to deploy the following changes:\n\n" +
						"  - Updated API handlers (3 files)\n" +
						"  - New database migration (v42)\n" +
						"  - Config change: increased rate limit\n\n" +
						"All tests are passing. Last deploy: 2 hours ago.",
				},
				Style:   ext.OverlayStyle{BorderColor: "#f38ba8"},
				Width:   65,
				Actions: []string{"Deploy", "Cancel", "Show Diff"},
			})

			if result.Cancelled {
				return "Deployment cancelled (ESC).", nil
			}
			return fmt.Sprintf("Selected action: %q (index %d)", result.Action, result.Index), nil
		},
	})

	// /overlay-markdown — overlay with markdown content.
	api.RegisterCommand(ext.CommandDef{
		Name:        "overlay-md",
		Description: "Show an overlay with markdown content",
		Execute: func(args string, ctx ext.Context) (string, error) {
			md := "## Build Report\n\n" +
				"| Component | Status | Duration |\n" +
				"|-----------|--------|----------|\n" +
				"| Frontend  | Pass   | 12.3s    |\n" +
				"| Backend   | Pass   | 8.7s     |\n" +
				"| E2E Tests | Pass   | 45.1s    |\n\n" +
				"**Total time:** 66.1s\n\n" +
				"All checks passed. Ready to merge."

			result := ctx.ShowOverlay(ext.OverlayConfig{
				Title:   "Build Report",
				Content: ext.WidgetContent{Text: md, Markdown: true},
				Style:   ext.OverlayStyle{BorderColor: "#a6e3a1"},
				Width:   70,
				Actions: []string{"Merge", "Close"},
			})

			if result.Cancelled {
				return "Build report closed.", nil
			}
			return fmt.Sprintf("Build report action: %q", result.Action), nil
		},
	})

	// /overlay-scroll — overlay with long scrollable content.
	api.RegisterCommand(ext.CommandDef{
		Name:        "overlay-scroll",
		Description: "Show an overlay with scrollable content",
		Execute: func(args string, ctx ext.Context) (string, error) {
			var lines []string
			lines = append(lines, "This overlay has a lot of content to demonstrate scrolling.")
			lines = append(lines, "Use j/k or arrow keys to scroll through the content.")
			lines = append(lines, "")
			for i := 1; i <= 50; i++ {
				lines = append(lines, fmt.Sprintf("  Line %02d: The quick brown fox jumps over the lazy dog.", i))
			}
			lines = append(lines, "")
			lines = append(lines, "End of content. Press Enter to dismiss or ESC to cancel.")

			result := ctx.ShowOverlay(ext.OverlayConfig{
				Title:     "Log Output (50 lines)",
				Content:   ext.WidgetContent{Text: strings.Join(lines, "\n")},
				Style:     ext.OverlayStyle{BorderColor: "#fab387"},
				MaxHeight: 20,
				Actions:   []string{"OK", "Copy to Clipboard"},
			})

			if result.Cancelled {
				return "Log viewer cancelled.", nil
			}
			return fmt.Sprintf("Log viewer action: %q", result.Action), nil
		},
	})
}
