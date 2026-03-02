//go:build ignore

// branded-output.go — Custom Message Rendering example extension for Kit.
//
// Demonstrates api.RegisterMessageRenderer() and ctx.RenderMessage() which
// let extensions define reusable visual styles for output. Each renderer has
// a name and a render function that receives content and terminal width.
//
// This extension registers three renderers:
//   "success" — green-bordered block for success messages
//   "warning" — yellow-bordered block for warnings
//   "metric"  — compact key=value display for metrics
//
// Commands:
//   /demo-render   — shows all three renderers in action

package main

import (
	"fmt"
	"strings"
	"time"

	ext "kit/ext"
)

func Init(api ext.API) {
	// Register a "success" renderer — green-accented block.
	api.RegisterMessageRenderer(ext.MessageRendererConfig{
		Name: "success",
		Render: func(content string, width int) string {
			maxW := width - 6
			if maxW < 20 {
				maxW = 20
			}
			bar := strings.Repeat("─", maxW)
			return fmt.Sprintf("  \033[32m┌%s┐\033[0m\n  \033[32m│\033[0m \033[1;32m%s\033[0m\n  \033[32m└%s┘\033[0m",
				bar, content, bar)
		},
	})

	// Register a "warning" renderer — yellow-accented block.
	api.RegisterMessageRenderer(ext.MessageRendererConfig{
		Name: "warning",
		Render: func(content string, width int) string {
			maxW := width - 6
			if maxW < 20 {
				maxW = 20
			}
			bar := strings.Repeat("─", maxW)
			return fmt.Sprintf("  \033[33m┌%s┐\033[0m\n  \033[33m│\033[0m \033[1;33m%s\033[0m\n  \033[33m└%s┘\033[0m",
				bar, content, bar)
		},
	})

	// Register a "metric" renderer — compact label: value format.
	api.RegisterMessageRenderer(ext.MessageRendererConfig{
		Name: "metric",
		Render: func(content string, width int) string {
			return fmt.Sprintf("  \033[36m▸\033[0m %s", content)
		},
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "demo-render",
		Description: "Demonstrate custom message renderers",
		Execute: func(args string, ctx ext.Context) (string, error) {
			ctx.RenderMessage("success", "All 42 tests passed in 3.2s")
			ctx.RenderMessage("warning", "3 deprecation warnings detected")
			ctx.RenderMessage("metric", fmt.Sprintf("build_time=%.1fs  tests=42  coverage=87%%  timestamp=%s",
				3.2, time.Now().Format("15:04:05")))

			return "Rendered three message styles.", nil
		},
	})
}
