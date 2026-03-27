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
