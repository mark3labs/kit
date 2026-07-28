package ui

import (
	"math"
	"testing"

	"github.com/mark3labs/kit/internal/models"
)

func TestUsageTracker_OAuthCosts(t *testing.T) {
	// Create a mock model info with costs
	modelInfo := &models.ModelInfo{
		ID:   "claude-3-5-sonnet-20241022",
		Name: "Claude 3.5 Sonnet v2",
		Cost: models.Cost{
			Input:  3.0,
			Output: 15.0,
		},
	}

	// Test with regular API key (costs should be calculated)
	regularTracker := NewUsageTracker(modelInfo, "anthropic", 80, false)
	regularTracker.UpdateUsage(1000, 500, 0, 0) // 1000 input, 500 output tokens

	stats := regularTracker.GetTurnStats()
	if stats == nil {
		t.Fatal("Expected stats to be non-nil")
		return
	}

	// Check that costs are calculated for regular API key
	expectedInputCost := float64(1000) * 3.0 / 1000000          // $0.003
	expectedOutputCost := float64(500) * 15.0 / 1000000         // $0.0075
	expectedTotalCost := expectedInputCost + expectedOutputCost // $0.0105

	if stats.InputCost != expectedInputCost {
		t.Errorf("Expected input cost %f, got %f", expectedInputCost, stats.InputCost)
	}
	if stats.OutputCost != expectedOutputCost {
		t.Errorf("Expected output cost %f, got %f", expectedOutputCost, stats.OutputCost)
	}
	if stats.TotalCost != expectedTotalCost {
		t.Errorf("Expected total cost %f, got %f", expectedTotalCost, stats.TotalCost)
	}

	// Test with OAuth credentials (costs should be $0)
	oauthTracker := NewUsageTracker(modelInfo, "anthropic", 80, true)
	oauthTracker.UpdateUsage(1000, 500, 0, 0) // Same token usage

	oauthStats := oauthTracker.GetTurnStats()
	if oauthStats == nil {
		t.Fatal("Expected OAuth stats to be non-nil")
		return
	}

	// Check that all costs are $0 for OAuth
	if oauthStats.InputCost != 0.0 {
		t.Errorf("Expected OAuth input cost to be $0, got %f", oauthStats.InputCost)
	}
	if oauthStats.OutputCost != 0.0 {
		t.Errorf("Expected OAuth output cost to be $0, got %f", oauthStats.OutputCost)
	}
	if oauthStats.TotalCost != 0.0 {
		t.Errorf("Expected OAuth total cost to be $0, got %f", oauthStats.TotalCost)
	}

	// Verify token counts are still tracked correctly for OAuth
	if oauthStats.InputTokens != 1000 {
		t.Errorf("Expected OAuth input tokens to be 1000, got %d", oauthStats.InputTokens)
	}
	if oauthStats.OutputTokens != 500 {
		t.Errorf("Expected OAuth output tokens to be 500, got %d", oauthStats.OutputTokens)
	}
}

func TestUsageTracker_OAuthSessionStats(t *testing.T) {
	// Create a mock model info with costs
	modelInfo := &models.ModelInfo{
		ID:   "claude-3-5-sonnet-20241022",
		Name: "Claude 3.5 Sonnet v2",
		Cost: models.Cost{
			Input:  3.0,
			Output: 15.0,
		},
	}

	// Test OAuth session stats accumulation
	oauthTracker := NewUsageTracker(modelInfo, "anthropic", 80, true)

	// Make multiple requests
	oauthTracker.UpdateUsage(1000, 500, 0, 0)
	oauthTracker.UpdateUsage(2000, 1000, 0, 0)

	sessionStats := oauthTracker.GetSessionStats()

	// Check that tokens are accumulated correctly
	if sessionStats.TotalInputTokens != 3000 {
		t.Errorf("Expected total input tokens to be 3000, got %d", sessionStats.TotalInputTokens)
	}
	if sessionStats.TotalOutputTokens != 1500 {
		t.Errorf("Expected total output tokens to be 1500, got %d", sessionStats.TotalOutputTokens)
	}

	// Check that total cost remains $0 for OAuth
	if sessionStats.TotalCost != 0.0 {
		t.Errorf("Expected OAuth session total cost to be $0, got %f", sessionStats.TotalCost)
	}

	// Check request count
	if sessionStats.RequestCount != 2 {
		t.Errorf("Expected request count to be 2, got %d", sessionStats.RequestCount)
	}
}

// TestUsageTracker_TurnStatsAccumulateAcrossSteps guards the multi-step turn
// accounting: a turn issues one LLM request per tool-loop iteration, so the
// per-turn stats must sum every step rather than reflect only the last one.
func TestUsageTracker_TurnStatsAccumulateAcrossSteps(t *testing.T) {
	cacheRead := 0.3
	cacheWrite := 3.75
	modelInfo := &models.ModelInfo{
		ID:   "claude-3-5-sonnet-20241022",
		Name: "Claude 3.5 Sonnet v2",
		Cost: models.Cost{
			Input:      3.0,
			Output:     15.0,
			CacheRead:  &cacheRead,
			CacheWrite: &cacheWrite,
		},
	}

	tracker := NewUsageTracker(modelInfo, "anthropic", 80, false)
	tracker.StartTurn()

	// Three steps of a single tool-calling turn.
	tracker.UpdateUsage(1000, 100, 10, 20)
	tracker.UpdateUsage(2000, 200, 30, 40)
	tracker.UpdateUsage(3000, 300, 50, 60)

	stats := tracker.GetTurnStats()
	if stats == nil {
		t.Fatal("GetTurnStats() = nil; want accumulated stats")
	}

	if stats.InputTokens != 6000 {
		t.Errorf("turn InputTokens = %d; want 6000 (sum of all steps)", stats.InputTokens)
	}
	if stats.OutputTokens != 600 {
		t.Errorf("turn OutputTokens = %d; want 600", stats.OutputTokens)
	}
	if stats.CacheReadTokens != 90 {
		t.Errorf("turn CacheReadTokens = %d; want 90", stats.CacheReadTokens)
	}
	if stats.CacheWriteTokens != 120 {
		t.Errorf("turn CacheWriteTokens = %d; want 120", stats.CacheWriteTokens)
	}

	wantCost := float64(6000)*3.0/1e6 + float64(600)*15.0/1e6 +
		float64(90)*cacheRead/1e6 + float64(120)*cacheWrite/1e6
	if math.Abs(stats.TotalCost-wantCost) > 1e-12 {
		t.Errorf("turn TotalCost = %v; want %v", stats.TotalCost, wantCost)
	}

	// Turn stats must match session totals when only one turn has occurred.
	session := tracker.GetSessionStats()
	if session.TotalInputTokens != stats.InputTokens {
		t.Errorf("session input %d != turn input %d after a single turn",
			session.TotalInputTokens, stats.InputTokens)
	}
	if math.Abs(session.TotalCost-stats.TotalCost) > 1e-12 {
		t.Errorf("session cost %v != turn cost %v after a single turn",
			session.TotalCost, stats.TotalCost)
	}

	// A new turn resets the accumulator but preserves session totals.
	tracker.StartTurn()
	if got := tracker.GetTurnStats(); got != nil {
		t.Errorf("GetTurnStats() after StartTurn = %+v; want nil", got)
	}

	tracker.UpdateUsage(500, 50, 0, 0)
	stats2 := tracker.GetTurnStats()
	if stats2.InputTokens != 500 {
		t.Errorf("second turn InputTokens = %d; want 500 (previous turn must not leak)", stats2.InputTokens)
	}

	session = tracker.GetSessionStats()
	if session.TotalInputTokens != 6500 {
		t.Errorf("session TotalInputTokens = %d; want 6500 (StartTurn must not clear session totals)",
			session.TotalInputTokens)
	}
	if session.RequestCount != 4 {
		t.Errorf("session RequestCount = %d; want 4", session.RequestCount)
	}
}
