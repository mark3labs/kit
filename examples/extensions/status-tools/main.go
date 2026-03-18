//go:build ignore

package main

import (
	"fmt"
	"time"

	"kit/ext"
)

// Init registers the status tools extension.
// This extension provides multiple status-related utilities as a
// multi-file extension example.
func Init(api ext.API) {
	// Register a status bar widget that shows time
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for range ticker.C {
				ctx.SetStatus("clock", time.Now().Format("15:04:05"), 5)
			}
		}()
	})

	// Register a /status command
	api.RegisterCommand(ext.CommandDef{
		Name:        "status",
		Description: "Show system status information",
		Execute: func(args string, ctx ext.Context) (string, error) {
			stats := ctx.GetContextStats()
			info := fmt.Sprintf(
				"Model: %s\nTokens: %d/%d (%.1f%%)\nMessages: %d",
				ctx.Model,
				stats.EstimatedTokens,
				stats.ContextLimit,
				stats.UsagePercent*100,
				stats.MessageCount,
			)
			ctx.PrintBlock(ext.PrintBlockOpts{
				Text:        info,
				BorderColor: "#89b4fa",
				Subtitle:    "System Status",
			})
			return "", nil
		},
	})
}
