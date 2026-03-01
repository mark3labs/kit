//go:build ignore

package main

import (
	"fmt"
	"math"
	"strings"

	"kit/ext"
)

// Init demonstrates a minimal-chrome extension — a port of Pi's minimal.ts.
// Hides the startup banner, status bar, separator, and input hint, replacing
// them with a compact footer showing model name and a context usage bar:
//
//	claude-sonnet-4-5-20250929  [###-------] 30%  (3.9K/200K tokens)
//
// Usage: kit -e examples/extensions/minimal.go
func Init(api ext.API) {
	// updateFooter builds the footer text from current context stats.
	updateFooter := func(ctx ext.Context) {
		stats := ctx.GetContextStats()
		pct := stats.UsagePercent * 100
		if pct > 100 {
			pct = 100
		}
		filled := int(math.Round(pct)) / 10
		bar := strings.Repeat("#", filled) + strings.Repeat("-", 10-filled)

		// Format token counts like the built-in status bar (e.g. "3.9K/200K").
		fmtTokens := func(n int) string {
			if n >= 1000 {
				return fmt.Sprintf("%.1fK", float64(n)/1000)
			}
			return fmt.Sprintf("%d", n)
		}

		text := fmt.Sprintf("%s  [%s] %d%%", ctx.Model, bar, int(math.Round(pct)))
		if stats.ContextLimit > 0 {
			text += fmt.Sprintf("  (%s/%s tokens)",
				fmtTokens(stats.EstimatedTokens), fmtTokens(stats.ContextLimit))
		}

		ctx.SetFooter(ext.HeaderFooterConfig{
			Content: ext.WidgetContent{Text: text},
			Style:   ext.WidgetStyle{BorderColor: "#585b70"},
		})
	}

	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		// Strip built-in chrome for a minimal look.
		ctx.SetUIVisibility(ext.UIVisibility{
			HideStartupMessage: true,
			HideStatusBar:      true,
			HideSeparator:      true,
			HideInputHint:      true,
		})

		updateFooter(ctx)
	})

	// Refresh after each agent turn — context usage changes here.
	api.OnAgentEnd(func(_ ext.AgentEndEvent, ctx ext.Context) {
		updateFooter(ctx)
	})

	api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, ctx ext.Context) {
		ctx.RemoveFooter()
	})
}
