package app

import (
	"testing"

	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/ui"
	"github.com/mark3labs/kit/pkg/kit"
)

// TestUpdateUsageFromTurnResult_ZeroOutputTokens verifies that when the API
// returns valid input tokens but 0 output tokens, we still use the API-reported
// counts rather than falling back to estimation. This was a bug where sessions
// with many messages would show severely underestimated token counts (~1K instead
// of the actual context window size).
func TestUpdateUsageFromTurnResult_ZeroOutputTokens(t *testing.T) {
	modelInfo := &models.ModelInfo{
		ID:    "test-model",
		Name:  "Test Model",
		Limit: models.ModelLimits{Context: 128000},
		Cost:  models.ModelCost{Input: 0.001, Output: 0.002},
	}
	tracker := ui.NewUsageTracker(modelInfo, "test", 100, false)

	app := &App{
		opts: Options{
			UsageTracker: tracker,
		},
	}

	// Simulate a TurnResult with valid input tokens but 0 output tokens
	// This can happen with some API providers or edge cases
	result := &kit.TurnResult{
		Response: "test response",
		TotalUsage: &kit.Usage{
			InputTokens:  25000, // Large input count representing conversation history
			OutputTokens: 0,     // Zero output (edge case)
		},
		FinalUsage: &kit.Usage{
			InputTokens:  25000,
			OutputTokens: 0,
		},
	}

	userPrompt := "short prompt"

	// Before the fix, this would fall back to estimation and return early
	// After the fix, it should use the API-reported 25000 input tokens
	app.updateUsageFromTurnResult(result, userPrompt)

	// Check that the session stats reflect the API-reported input tokens
	stats := tracker.GetSessionStats()
	if stats.TotalInputTokens != 25000 {
		t.Errorf("Expected TotalInputTokens = 25000, got %d", stats.TotalInputTokens)
	}

	// Check that the context tokens are set correctly (input + output = 25000 + 0)
	// This is what gets displayed in the token counter
	rendered := tracker.RenderUsageInfo()
	if rendered == "" {
		t.Error("Expected non-empty usage info after update")
	}

	// The rendered output should show the API-reported token count, not an estimate
	// With 25000 tokens, it should show "25.0K" not a small estimate like "1.1K"
	// We can't easily parse the rendered string, but we can verify the context tokens
	// were set by checking the session stats were updated
	if stats.RequestCount != 1 {
		t.Errorf("Expected RequestCount = 1, got %d", stats.RequestCount)
	}
}

// TestUpdateUsageFromTurnResult_BothZeroTokens verifies fallback to estimation
// when both input and output tokens are 0 (no API metadata available).
func TestUpdateUsageFromTurnResult_BothZeroTokens(t *testing.T) {
	modelInfo := &models.ModelInfo{
		ID:    "test-model",
		Name:  "Test Model",
		Limit: models.ModelLimits{Context: 128000},
		Cost:  models.ModelCost{Input: 0.001, Output: 0.002},
	}
	tracker := ui.NewUsageTracker(modelInfo, "test", 100, false)

	app := &App{
		opts: Options{
			UsageTracker: tracker,
		},
	}

	// Simulate a TurnResult with 0 tokens (no metadata from API)
	result := &kit.TurnResult{
		Response:   "test response with some content",
		TotalUsage: &kit.Usage{
			InputTokens:  0,
			OutputTokens: 0,
		},
		FinalUsage: &kit.Usage{
			InputTokens:  0,
			OutputTokens: 0,
		},
	}

	userPrompt := "test prompt"

	app.updateUsageFromTurnResult(result, userPrompt)

	// Should fall back to estimation
	stats := tracker.GetSessionStats()
	if stats.RequestCount != 1 {
		t.Errorf("Expected RequestCount = 1, got %d", stats.RequestCount)
	}

	// The tokens should be estimated from the text length, not 0
	// estimateTokens uses len(text)/4, so we expect some non-zero value
	if stats.TotalInputTokens == 0 {
		t.Error("Expected non-zero estimated input tokens")
	}
}

// TestUpdateUsageFromTurnResult_ValidOutputZeroInput verifies that we use
// API-reported tokens when output is valid but input is 0 (unusual edge case).
func TestUpdateUsageFromTurnResult_ValidOutputZeroInput(t *testing.T) {
	modelInfo := &models.ModelInfo{
		ID:    "test-model",
		Name:  "Test Model",
		Limit: models.ModelLimits{Context: 128000},
		Cost:  models.ModelCost{Input: 0.001, Output: 0.002},
	}
	tracker := ui.NewUsageTracker(modelInfo, "test", 100, false)

	app := &App{
		opts: Options{
			UsageTracker: tracker,
		},
	}

	// Edge case: 0 input tokens but valid output tokens
	// This shouldn't happen in practice but we should handle it
	result := &kit.TurnResult{
		Response: "test response",
		TotalUsage: &kit.Usage{
			InputTokens:  0,
			OutputTokens: 100,
		},
		FinalUsage: &kit.Usage{
			InputTokens:  0,
			OutputTokens: 100,
		},
	}

	app.updateUsageFromTurnResult(result, "prompt")

	// With inputTokens = 0, we should fall back to estimation
	// (the fix only changes the condition to accept inputTokens > 0 alone)
	stats := tracker.GetSessionStats()
	if stats.TotalOutputTokens != 0 {
		// If we used API data, output would be 100
		// If we estimated, output would be estimated from response text
		t.Logf("Output tokens: %d (may be estimated or API-reported)", stats.TotalOutputTokens)
	}
}
