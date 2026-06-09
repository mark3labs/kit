//go:build ignore

package main

import (
	"fmt"
	"strconv"

	"kit/ext"
)

// Init demonstrates the three primitives added in issue #53:
//
//  1. api.OnLLMUsage(...) — per-LLM-call usage callback with token + cost
//     deltas. Use this for budget enforcement that reacts between calls
//     within a single agent turn, rather than only at turn boundaries.
//
//  2. ctx.SetState / ctx.GetState / ctx.DeleteState / ctx.ListState —
//     last-write-wins, session-scoped key-value store backed by a sidecar
//     file. Use this for snapshot state (current value of X) instead of
//     ctx.AppendEntry, which is append-only and bloats branch reads.
//
//  3. ext.AgentEndEvent.ToolCallCount / .ToolNames / .LLMCallCount /
//     .InputTokensDelta / .OutputTokensDelta / .CostDelta / .DurationMs —
//     per-turn aggregates so observer extensions don't need to maintain
//     parallel bookkeeping.
//
// Together these support a simple soft-budget cap: warn when the
// cumulative cost in this session exceeds a threshold, and print a
// per-turn report on AgentEnd.
//
// Usage: kit -e examples/extensions/usage-budget.go
func Init(api ext.API) {
	const warnAtKey = "usage-budget:warn-at-usd"

	// 1. Print per-LLM-call usage with provider, model, and cost.
	api.OnLLMUsage(func(e ext.LLMUsageEvent, ctx ext.Context) {
		ctx.Print(fmt.Sprintf(
			"[usage] step=%d %s/%s tokens=↑%d ↓%d cache=↑%d/↓%d cost=$%.4f (%s)",
			e.StepNumber, e.Provider, e.Model,
			e.InputTokens, e.OutputTokens,
			e.CacheWriteTokens, e.CacheReadTokens,
			e.Cost, e.FinishReason,
		))

		// 2. Persist running total in last-write-wins state.
		current := 0.0
		if raw, ok := ctx.GetState("usage-budget:total-cost"); ok {
			current, _ = strconv.ParseFloat(raw, 64)
		}
		current += e.Cost
		ctx.SetState("usage-budget:total-cost", strconv.FormatFloat(current, 'f', 6, 64))

		// Soft warn-at threshold (configurable via state).
		warnAt := 0.50
		if raw, ok := ctx.GetState(warnAtKey); ok {
			if v, err := strconv.ParseFloat(raw, 64); err == nil {
				warnAt = v
			}
		}
		if current > warnAt {
			ctx.PrintError(fmt.Sprintf(
				"[usage] session cost $%.4f exceeds soft cap $%.2f",
				current, warnAt,
			))
		}
	})

	// 3. Print a per-turn summary using the enriched AgentEndEvent.
	api.OnAgentEnd(func(e ext.AgentEndEvent, ctx ext.Context) {
		ctx.Print(fmt.Sprintf(
			"[turn] stop=%s tools=%d llm-calls=%d tokens=↑%d ↓%d cost=$%.4f duration=%dms",
			e.StopReason, e.ToolCallCount, e.LLMCallCount,
			e.InputTokensDelta, e.OutputTokensDelta, e.CostDelta, e.DurationMs,
		))
		if len(e.ToolNames) > 0 {
			ctx.Print(fmt.Sprintf("[turn] tool order: %v", e.ToolNames))
		}
	})

	// Bootstrap default soft cap once per session.
	api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
		if _, ok := ctx.GetState(warnAtKey); !ok {
			ctx.SetState(warnAtKey, "0.50")
		}
	})
}
