package ui

import (
	"fmt"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"image/color"

	"github.com/mark3labs/kit/internal/models"
)

// UsageStats encapsulates detailed token usage and cost breakdown for a set of
// LLM request/response cycles, including input, output, and cache token counts
// along with their associated costs. Used both for per-turn accumulation and
// as a generic usage container.
type UsageStats struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	InputCost        float64
	OutputCost       float64
	CacheReadCost    float64
	CacheWriteCost   float64
	TotalCost        float64
}

// SessionStats aggregates token usage and cost information across all requests
// in a session, providing totals and request counts for usage analysis and
// cost tracking.
type SessionStats struct {
	TotalInputTokens      int
	TotalOutputTokens     int
	TotalCacheReadTokens  int
	TotalCacheWriteTokens int
	TotalCost             float64
	RequestCount          int
}

// UsageTracker monitors and accumulates token usage statistics and associated costs
// for LLM interactions throughout a session. It provides real-time usage information
// and supports both estimated and actual token counts. OAuth users see $0 costs.
type UsageTracker struct {
	mu           sync.RWMutex
	modelInfo    *models.ModelInfo
	provider     string
	sessionStats SessionStats

	// turnStats accumulates usage across every LLM request in the current
	// turn. A single turn issues one request per tool-loop iteration, so this
	// must sum them: reporting only the final request would under-count a
	// multi-step turn's tokens and cost. Reset by StartTurn.
	turnStats *UsageStats

	contextTokens int // approximate current context window utilization (last API call)
	width         int
	isOAuth       bool // Whether OAuth credentials are being used (costs should be $0)

	// usageUnreported is true when the last turn's provider did not report
	// token usage at all (e.g. OpenAI-compatible proxies that omit the
	// `usage` field from streaming chunks). When true, RenderUsageInfo shows
	// a muted warning instead of a misleading "Tokens: 0 | Cost: $0.0000".
	// Managed by the app layer at end-of-turn via SetUsageUnreported.
	usageUnreported bool
}

// NewUsageTracker creates and initializes a new UsageTracker for the specified model.
// The tracker uses model-specific pricing information to calculate costs, unless OAuth
// credentials are being used (in which case costs are shown as $0). Width determines
// the display formatting.
func NewUsageTracker(modelInfo *models.ModelInfo, provider string, width int, isOAuth bool) *UsageTracker {
	return &UsageTracker{
		modelInfo: modelInfo,
		provider:  provider,
		width:     width,
		isOAuth:   isOAuth,
	}
}

// estimateTokens provides a rough estimate of the number of tokens in the given text.
// Uses a simple heuristic of ~4 characters per token.
func estimateTokens(text string) int {
	return len(text) / 4
}

// UpdateUsage records new token usage data and calculates associated costs based on
// the model's pricing. Updates both the current turn's statistics and cumulative
// session totals. For OAuth users, costs are recorded as $0 while still tracking
// token counts.
//
// Turn statistics accumulate across calls; callers signal a turn boundary with
// StartTurn.
func (ut *UsageTracker) UpdateUsage(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	// Calculate costs based on model pricing
	// For OAuth credentials, costs are $0 for usage tracking purposes
	var inputCost, outputCost, cacheReadCost, cacheWriteCost, totalCost float64

	if !ut.isOAuth {
		inputCost = float64(inputTokens) * ut.modelInfo.Cost.Input / 1000000 // Cost is per million tokens
		outputCost = float64(outputTokens) * ut.modelInfo.Cost.Output / 1000000

		if ut.modelInfo.Cost.CacheRead != nil {
			cacheReadCost = float64(cacheReadTokens) * (*ut.modelInfo.Cost.CacheRead) / 1000000
		}
		if ut.modelInfo.Cost.CacheWrite != nil {
			cacheWriteCost = float64(cacheWriteTokens) * (*ut.modelInfo.Cost.CacheWrite) / 1000000
		}

		totalCost = inputCost + outputCost + cacheReadCost + cacheWriteCost
	}
	// If OAuth, all costs remain 0.0

	// Accumulate into the current turn. Multi-step turns issue one request per
	// tool-loop iteration, so every step must be added rather than overwrite
	// the previous one.
	if ut.turnStats == nil {
		ut.turnStats = &UsageStats{}
	}
	ut.turnStats.InputTokens += inputTokens
	ut.turnStats.OutputTokens += outputTokens
	ut.turnStats.CacheReadTokens += cacheReadTokens
	ut.turnStats.CacheWriteTokens += cacheWriteTokens
	ut.turnStats.InputCost += inputCost
	ut.turnStats.OutputCost += outputCost
	ut.turnStats.CacheReadCost += cacheReadCost
	ut.turnStats.CacheWriteCost += cacheWriteCost
	ut.turnStats.TotalCost += totalCost

	// Update session stats
	ut.sessionStats.TotalInputTokens += inputTokens
	ut.sessionStats.TotalOutputTokens += outputTokens
	ut.sessionStats.TotalCacheReadTokens += cacheReadTokens
	ut.sessionStats.TotalCacheWriteTokens += cacheWriteTokens
	ut.sessionStats.TotalCost += totalCost
	ut.sessionStats.RequestCount++
}

// EstimateAndUpdateUsage estimates token counts from raw text strings and updates
// the usage statistics. This method is used when actual token counts are not available
// from the API response. The estimated values also serve as the context utilization
// approximation since they represent a single API call.
func (ut *UsageTracker) EstimateAndUpdateUsage(inputText, outputText string) {
	inputTokens := estimateTokens(inputText)
	outputTokens := estimateTokens(outputText)
	ut.UpdateUsage(inputTokens, outputTokens, 0, 0)
	// For estimated usage the values represent a single call, so they are a
	// reasonable proxy for the current context window fill level.
	ut.mu.Lock()
	ut.contextTokens = inputTokens + outputTokens
	ut.mu.Unlock()
}

// SetContextTokens records the approximate current context window utilization.
//
// The value should include ALL token categories from the last API call:
//
//	InputTokens + CacheReadTokens + CacheCreationTokens + OutputTokens
//
// With Anthropic prompt caching, InputTokens can be near-zero while
// CacheReadTokens holds the bulk of the context. All four must be summed
// to get the true context window fill level.
//
// OutputTokens is included because the assistant's output becomes part of
// the context on the next turn.
//
// Use FinalResponse.Usage (last step only) rather than aggregate TotalUsage,
// because TotalUsage sums across all tool-calling steps and overstates the
// actual window fill level.
//
// The value is set unconditionally (not max-only) so that context shrinks
// correctly after compaction.
func (ut *UsageTracker) SetContextTokens(tokens int) {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.contextTokens = tokens
}

// RenderUsageInfo generates a formatted string displaying current usage statistics
// including token counts, context utilization percentage, and costs. The display
// adapts colors based on usage levels and formats large numbers with K/M suffixes
// for readability.
func (ut *UsageTracker) RenderUsageInfo() string {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	theme := GetTheme()
	baseStyle := lipgloss.NewStyle()

	// If the active provider did not report token usage on the last turn
	// (common with OpenAI-compatible proxies that omit the `usage` field
	// from the final streaming chunk), show a muted warning instead of a
	// misleading "Tokens: 0 | Cost: $0.0000". This keeps the status bar
	// honest about why metrics are missing rather than looking broken.
	if ut.usageUnreported {
		warnIcon := baseStyle.Foreground(theme.Warning).Render("⚠")
		warnText := baseStyle.Foreground(theme.Muted).Render(" usage not reported by provider")
		return warnIcon + warnText
	}

	// Display the current context window token count (from the last API call),
	// not the cumulative session total. This keeps the number consistent with
	// the percentage and answers "how full is my context right now?".
	displayTokens := ut.contextTokens

	// Format tokens with K/M suffix for better readability
	var tokenStr string
	if displayTokens >= 1000000 {
		tokenStr = fmt.Sprintf("%.1fM", float64(displayTokens)/1000000)
	} else if displayTokens >= 1000 {
		tokenStr = fmt.Sprintf("%.1fK", float64(displayTokens)/1000)
	} else {
		tokenStr = fmt.Sprintf("%d", displayTokens)
	}

	// Calculate context window utilization percentage from the same value.
	var percentageStr string
	var percentageColor color.Color
	if ut.modelInfo.Limit.Context > 0 && displayTokens > 0 {
		percentage := float64(displayTokens) / float64(ut.modelInfo.Limit.Context) * 100

		// Color code based on usage percentage
		if percentage >= 80 {
			percentageColor = theme.Error // Red
		} else if percentage >= 60 {
			percentageColor = theme.Warning // Orange
		} else {
			percentageColor = theme.Success // Green
		}

		percentageStr = baseStyle.
			Foreground(percentageColor).
			Render(fmt.Sprintf(" %.0f%%", percentage))
	}

	// Format cost. Two decimal places is the right resolution for a status
	// bar; four implied a precision nobody reads at a glance. Sub-cent
	// sessions collapse to a bare "$0" rather than a row of zeroes.
	var costStr string
	switch {
	case ut.isOAuth:
		costStr = ""
	case ut.sessionStats.TotalCost <= 0:
		costStr = baseStyle.Foreground(theme.Muted).Render("$0")
	case ut.sessionStats.TotalCost < 0.01:
		costStr = baseStyle.Foreground(theme.Muted).Render("<$0.01")
	default:
		costStr = baseStyle.
			Foreground(theme.Muted).
			Render(fmt.Sprintf("$%.2f", ut.sessionStats.TotalCost))
	}

	// The token count is self-describing — a "Tokens:" label costs nine
	// columns to say what the number already says. Values are joined with the
	// same middle dot the rest of the status bar uses.
	tokensValue := baseStyle.
		Foreground(theme.Muted).
		Render(tokenStr)

	parts := []string{tokensValue + percentageStr}
	if costStr != "" {
		parts = append(parts, costStr)
	}
	return strings.Join(parts, baseStyle.Foreground(theme.VeryMuted).Render(" · "))
}

// GetSessionStats returns a copy of the cumulative session statistics including
// total token counts, costs, and request count. The returned copy is safe to use
// without additional synchronization.
func (ut *UsageTracker) GetSessionStats() SessionStats {
	ut.mu.RLock()
	defer ut.mu.RUnlock()
	return ut.sessionStats
}

// GetTurnStats returns a copy of the usage statistics accumulated during the
// current (or most recently completed) turn, or nil if no usage has been
// recorded. A turn spans every LLM request in one tool-calling loop, so the
// returned totals cover all steps. The copy is safe to use without additional
// synchronization.
func (ut *UsageTracker) GetTurnStats() *UsageStats {
	ut.mu.RLock()
	defer ut.mu.RUnlock()
	if ut.turnStats == nil {
		return nil
	}
	stats := *ut.turnStats
	return &stats
}

// IsOAuth reports whether the tracker is running against an OAuth credential
// (e.g. a Claude subscription) rather than a per-token billed API key. Under
// OAuth all recorded costs are 0 by design, so callers rendering cost need
// this flag to distinguish "not billed" from "nothing spent yet".
func (ut *UsageTracker) IsOAuth() bool {
	ut.mu.RLock()
	defer ut.mu.RUnlock()
	return ut.isOAuth
}

// StartTurn marks the beginning of a new turn, clearing usage accumulated for
// the previous one while preserving session totals. Called by the app layer
// before dispatching a prompt to the SDK.
func (ut *UsageTracker) StartTurn() {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.turnStats = nil
	ut.usageUnreported = false
}

// Reset clears all accumulated usage statistics, resetting both session totals
// and current-turn information to their initial empty state. This is typically
// used when starting a new conversation or clearing usage history.
func (ut *UsageTracker) Reset() {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.sessionStats = SessionStats{}
	ut.turnStats = nil
	ut.contextTokens = 0
	ut.usageUnreported = false // new conversation: don't presume the provider is silent
}

// SetWidth updates the terminal width used for formatting usage information display.
// This should be called when the terminal is resized to ensure proper text wrapping
// and alignment.
func (ut *UsageTracker) SetWidth(width int) {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.width = width
}

// UpdateModelInfo updates the model information and OAuth status when the model
// is switched mid-session. This ensures token costs and context limits are
// calculated correctly for the new model.
func (ut *UsageTracker) UpdateModelInfo(modelInfo *models.ModelInfo, provider string, isOAuth bool) {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.modelInfo = modelInfo
	ut.provider = provider
	ut.isOAuth = isOAuth
	// A model switch invalidates the previous provider's "unreported" state;
	// the next turn re-derives it via SetUsageUnreported.
	ut.usageUnreported = false
}

// SetUsageUnreported records whether the active provider failed to report
// token usage on the most recent turn. When set to true, RenderUsageInfo
// displays a muted "⚠ usage not reported by provider" notice instead of a
// bare zero. The app layer calls this once per turn from
// updateUsageFromTurnResult, passing false when any real usage was observed
// (via StepUsageEvent callbacks or TurnResult.TotalUsage) and true otherwise.
func (ut *UsageTracker) SetUsageUnreported(unreported bool) {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.usageUnreported = unreported
}
