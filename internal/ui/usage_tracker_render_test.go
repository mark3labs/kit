package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/models"
)

// stripAnsi removes ANSI escape codes from a string for test comparisons.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func TestUsageTracker_RenderUsageInfo_OAuth(t *testing.T) {
	// Create a mock model info with costs and context limit
	modelInfo := &models.ModelInfo{
		ID:   "claude-3-5-sonnet-20241022",
		Name: "Claude 3.5 Sonnet v2",
		Cost: models.Cost{
			Input:  3.0,
			Output: 15.0,
		},
		Limit: models.Limit{
			Context: 200000,
			Output:  8192,
		},
	}

	// Test OAuth rendering (should show $0.00)
	oauthTracker := NewUsageTracker(modelInfo, "anthropic", 80, true)
	oauthTracker.UpdateUsage(1500, 500, 0, 0) // 2000 total tokens (session/billing)
	oauthTracker.SetContextTokens(1500 + 500) // context window utilization

	rendered := stripAnsi(oauthTracker.RenderUsageInfo())

	// Should show tokens and percentage, but cost should show "$0.00"
	if !strings.Contains(rendered, "Tokens: 2.0K") {
		t.Errorf("Expected rendered output to contain 'Tokens: 2.0K', got: %s", rendered)
	}
	if !strings.Contains(rendered, "(1%)") { // 2000/200000 = 1%
		t.Errorf("Expected rendered output to contain percentage, got: %s", rendered)
	}
	if !strings.Contains(rendered, "Cost: $0.00") {
		t.Errorf("Expected rendered output to contain 'Cost: $0.00', got: %s", rendered)
	}

	// Test regular API key rendering (should show actual cost)
	regularTracker := NewUsageTracker(modelInfo, "anthropic", 80, false)
	regularTracker.UpdateUsage(1500, 500, 0, 0) // Same token usage
	regularTracker.SetContextTokens(1500 + 500) // context window utilization

	regularRendered := stripAnsi(regularTracker.RenderUsageInfo())

	// Should show tokens and actual cost
	if !strings.Contains(regularRendered, "Tokens: 2.0K") {
		t.Errorf("Expected regular rendered output to contain 'Tokens: 2.0K', got: %s", regularRendered)
	}
	if strings.Contains(regularRendered, "Cost: $0.00") {
		t.Errorf("Expected regular rendered output to NOT show $0.00, got: %s", regularRendered)
	}
	// Should show actual calculated cost (1500*3 + 500*15)/1000000 = 0.0120
	if !strings.Contains(regularRendered, "Cost: $0.0120") { // Now showing 4 decimal places
		t.Errorf("Expected regular rendered output to show actual cost, got: %s", regularRendered)
	}
}

func TestUsageTracker_RenderUsageInfo_StartupState(t *testing.T) {
	// Create a mock model info with costs and context limit
	modelInfo := &models.ModelInfo{
		ID:   "claude-3-5-sonnet-20241022",
		Name: "Claude 3.5 Sonnet v2",
		Cost: models.Cost{
			Input:  3.0,
			Output: 15.0,
		},
		Limit: models.Limit{
			Context: 200000,
			Output:  8192,
		},
	}

	// Test startup state (no requests made yet) - Regular API key
	regularTracker := NewUsageTracker(modelInfo, "anthropic", 80, false)
	rendered := stripAnsi(regularTracker.RenderUsageInfo())

	// Should NOT return empty string on startup
	if rendered == "" {
		t.Errorf("Expected non-empty output on startup, got empty string")
	}

	// Should show 0 tokens
	if !strings.Contains(rendered, "Tokens: 0") {
		t.Errorf("Expected 'Tokens: 0' on startup, got: %s", rendered)
	}

	// Should NOT show percentage when tokens are 0
	if strings.Contains(rendered, "(%") {
		t.Errorf("Expected no percentage on startup with 0 tokens, got: %s", rendered)
	}

	// Should show $0.0000 cost for regular API key
	if !strings.Contains(rendered, "Cost: $0.0000") {
		t.Errorf("Expected 'Cost: $0.0000' on startup, got: %s", rendered)
	}

	// Test startup state (no requests made yet) - OAuth
	oauthTracker := NewUsageTracker(modelInfo, "anthropic", 80, true)
	oauthRendered := stripAnsi(oauthTracker.RenderUsageInfo())

	// Should NOT return empty string on startup
	if oauthRendered == "" {
		t.Errorf("Expected non-empty output on startup for OAuth, got empty string")
	}

	// Should show 0 tokens for OAuth
	if !strings.Contains(oauthRendered, "Tokens: 0") {
		t.Errorf("Expected 'Tokens: 0' on startup for OAuth, got: %s", oauthRendered)
	}

	// Should show $0.00 cost for OAuth
	if !strings.Contains(oauthRendered, "Cost: $0.00") {
		t.Errorf("Expected 'Cost: $0.00' on startup for OAuth, got: %s", oauthRendered)
	}
}

// TestUsageTracker_RenderUsageInfo_UnreportedWarning verifies that when the
// active provider did not report token usage on the last turn (the
// OpenAI-compatible-proxy / glm-5.2 case), RenderUsageInfo shows a muted
// "⚠ usage not reported by provider" notice instead of a misleading
// "Tokens: 0 | Cost: $0.0000". Clearing the flag must restore the normal
// token/cost display, and Reset must clear it so a fresh session does not
// pre-show the warning before the first turn.
func TestUsageTracker_RenderUsageInfo_UnreportedWarning(t *testing.T) {
	modelInfo := &models.ModelInfo{
		ID:    "glm-5.2",
		Name:  "GLM 5.2",
		Cost:  models.Cost{Input: 1.0, Output: 2.0},
		Limit: models.Limit{Context: 128000, Output: 8192},
	}

	tracker := NewUsageTracker(modelInfo, "opencode", 80, false)

	// Default state: no warning, shows the normal bare-zero render.
	defaultRender := stripAnsi(tracker.RenderUsageInfo())
	if strings.Contains(defaultRender, "usage not reported") {
		t.Fatalf("default state should NOT show unreported warning, got: %s", defaultRender)
	}
	if !strings.Contains(defaultRender, "Tokens: 0") {
		t.Fatalf("default state should show 'Tokens: 0', got: %s", defaultRender)
	}

	// After a turn where the provider reported nothing → warning appears.
	tracker.SetUsageUnreported(true)
	warned := stripAnsi(tracker.RenderUsageInfo())
	if !strings.Contains(warned, "⚠") {
		t.Fatalf("expected warning icon in unreported render, got: %s", warned)
	}
	if !strings.Contains(warned, "usage not reported by provider") {
		t.Fatalf("expected 'usage not reported by provider' text, got: %s", warned)
	}
	// The misleading bare-zero metrics must NOT appear alongside the warning.
	if strings.Contains(warned, "Tokens:") || strings.Contains(warned, "Cost:") {
		t.Fatalf("unreported render must not show token/cost metrics, got: %s", warned)
	}

	// Once the provider reports usage again, the flag clears and normal
	// metrics render (no warning).
	tracker.SetUsageUnreported(false)
	tracker.UpdateUsage(1500, 500, 0, 0)
	tracker.SetContextTokens(2000)
	restored := stripAnsi(tracker.RenderUsageInfo())
	if strings.Contains(restored, "usage not reported") {
		t.Fatalf("restored state should NOT show unreported warning, got: %s", restored)
	}
	if !strings.Contains(restored, "Tokens: 2.0K") {
		t.Fatalf("restored state should show 'Tokens: 2.0K', got: %s", restored)
	}

	// Reset (new conversation) must clear the flag so a fresh session with
	// the same provider doesn't pre-show the warning before the first turn.
	tracker2 := NewUsageTracker(modelInfo, "opencode", 80, false)
	tracker2.SetUsageUnreported(true)
	tracker2.Reset()
	resetRender := stripAnsi(tracker2.RenderUsageInfo())
	if strings.Contains(resetRender, "usage not reported") {
		t.Fatalf("Reset must clear the unreported flag, but warning still shown: %s", resetRender)
	}
}
