//go:build ignore

package main

import (
	"fmt"

	"kit/ext"
)

// Init registers a before-compact hook that notifies the user when
// compaction is about to happen and optionally blocks automatic compaction.
//
// When automatic compaction is triggered (via --auto-compact), the extension
// asks for user confirmation. Manual /compact commands are always allowed.
//
// This demonstrates the OnBeforeCompact event which allows extensions to
// inspect context usage stats and gate the compaction process.
//
// Usage: kit -e examples/extensions/compact-notify.go --auto-compact
func Init(api ext.API) {
	api.OnBeforeCompact(func(e ext.BeforeCompactEvent, ctx ext.Context) *ext.BeforeCompactResult {
		pct := int(e.UsagePercent * 100)
		summary := fmt.Sprintf("Context: %dk/%dk tokens (%d%%), %d messages",
			e.EstimatedTokens/1000, e.ContextLimit/1000, pct, e.MessageCount)

		if e.IsAutomatic {
			// Auto-compaction: ask user first.
			ctx.PrintBlock(ext.PrintBlockOpts{
				Text:        "Auto-compaction triggered.\n" + summary,
				BorderColor: "#f9e2af",
				Subtitle:    "compact-notify",
			})

			result := ctx.PromptConfirm(ext.PromptConfirmConfig{
				Message:      "Allow automatic compaction?",
				DefaultValue: true,
			})
			if result.Cancelled || !result.Value {
				return &ext.BeforeCompactResult{
					Cancel: true,
					Reason: "Auto-compaction skipped by user.",
				}
			}
		} else {
			// Manual /compact: just notify.
			ctx.PrintBlock(ext.PrintBlockOpts{
				Text:        "Compacting conversation...\n" + summary,
				BorderColor: "#89b4fa",
				Subtitle:    "compact-notify",
			})
		}

		return nil // allow compaction
	})
}
