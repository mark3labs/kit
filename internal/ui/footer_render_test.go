package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/models"
)

func TestFooterRender_FormatShortModelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"anthropic/claude-3-7-sonnet", "sonnet 3.7"},
		{"claude-3-5-sonnet-20241022", "sonnet 3.5"},
		{"claude-4-5-sonnet", "sonnet 4.5"},
		{"google/gemini-3.6-flash", "gemini 3.6 flash"},
		{"openai/gpt-4o", "gpt 4o"},
		{"copilot", "copilot"},
	}

	for _, tt := range tests {
		got := formatShortModelName(tt.input)
		if got != tt.expected {
			t.Errorf("formatShortModelName(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFooterRender_StatusBarOutput(t *testing.T) {
	cacheRead := 3.0
	cacheWrite := 3.75
	modelInfo := &models.ModelInfo{
		ID:   "claude-3-7-sonnet-20250219",
		Name: "Claude 3.7 Sonnet",
		Cost: models.Cost{
			Input:      3.0,
			Output:     15.0,
			CacheRead:  &cacheRead,
			CacheWrite: &cacheWrite,
		},
		Limit: models.Limit{
			Context: 200000,
			Output:  8192,
		},
	}

	tracker := NewUsageTracker(modelInfo, "anthropic", 200, false)
	// Simulate usage: 500 input tokens, 55200 cache read tokens, 1000 output tokens
	tracker.UpdateUsage(500, 1000, 55200, 0)
	tracker.SetContextTokens(55500)

	app := &AppModel{
		modelName:         "claude-4-5-sonnet",
		providerName:      "anthropic",
		usageTracker:      tracker,
		width:             200,
		lastTurnDuration:  26 * time.Second,
		totalTurnDuration: 26 * time.Second,
	}

	rendered := stripAnsi(app.renderStatusBar())

	expectedElements := []string{
		"[KIT]",
		"sonnet 4.5",
		"ctx 55.5k",
		"[███░░░░░░░] 28%",
		"🗃 55.2k (99%)",
		"💰",
		"🪙",
		"🦕 ~",
		"⏱ 26s / 26s",
	}

	for _, elem := range expectedElements {
		if !strings.Contains(rendered, elem) {
			t.Errorf("Expected rendered status bar to contain %q, got:\n%s", elem, rendered)
		}
	}
}

func TestFooterRender_ReasoningEffortAndFallback(t *testing.T) {
	modelInfo := &models.ModelInfo{
		ID:   "gemini-3.6-flash",
		Name: "Gemini 3.6 Flash",
		Limit: models.Limit{
			Context: 1000000,
			Output:  8192,
		},
	}
	if modelInfo.Limit.Context != 1000000 {
		t.Errorf("Expected 1M context limit for gemini, got %d", modelInfo.Limit.Context)
	}

	tracker := NewUsageTracker(modelInfo, "google", 200, false)
	tracker.UpdateUsage(150000, 2000, 0, 0)
	tracker.SetContextTokens(152000)

	app := &AppModel{
		modelName:         "google/gemini-3.6-flash",
		providerName:      "google",
		thinkingLevel:     "high",
		isReasoningModel:  true,
		usageTracker:      tracker,
		width:             200,
		lastTurnDuration:  10 * time.Second,
		totalTurnDuration: 10 * time.Second,
	}

	rendered := stripAnsi(app.renderStatusBar())

	expectedElements := []string{
		"[KIT]",
		"gemini 3.6 flash (effort: high)",
		"ctx 152.0k",
		"[██░░░░░░░░] 15%",
		"💰 $",
		"⏱ 10s / 10s",
	}

	for _, elem := range expectedElements {
		if !strings.Contains(rendered, elem) {
			t.Errorf("Expected rendered status bar to contain %q, got:\n%s", elem, rendered)
		}
	}
}

func TestFooterRender_MultiTurnCostBreakdown(t *testing.T) {
	modelInfo := &models.ModelInfo{
		ID:   "claude-3-7-sonnet-20250219",
		Name: "Claude 3.7 Sonnet",
		Cost: models.Cost{
			Input:  3.0,
			Output: 15.0,
		},
		Limit: models.Limit{
			Context: 200000,
			Output:  8192,
		},
	}

	tracker := NewUsageTracker(modelInfo, "anthropic", 200, false)

	// Turn 1: 10,000 input tokens, 1,000 output tokens -> Cost = $0.045
	tracker.StartTransmission()
	tracker.UpdateUsage(10000, 1000, 0, 0)

	sessCost1, txCost1, _ := tracker.GetCostBreakdown()
	if sessCost1 != 0.045 || txCost1 != 0.045 {
		t.Fatalf("Turn 1 expected sess=0.045, tx=0.045; got sess=%f, tx=%f", sessCost1, txCost1)
	}

	// Turn 2: 20,000 input tokens, 1,000 output tokens -> Cost = $0.075
	tracker.StartTransmission()
	tracker.UpdateUsage(20000, 1000, 0, 0)

	sessCost2, txCost2, _ := tracker.GetCostBreakdown()
	if sessCost2 != 0.120 || txCost2 != 0.075 {
		t.Fatalf("Turn 2 expected sess=0.120, tx=0.075; got sess=%f, tx=%f", sessCost2, txCost2)
	}

	app := &AppModel{
		modelName:    "claude-3-7-sonnet",
		providerName: "anthropic",
		usageTracker: tracker,
		width:        200,
	}

	rendered := stripAnsi(app.renderStatusBar())

	if !strings.Contains(rendered, "💰 $0.12000") {
		t.Errorf("Expected status bar total cost 💰 $0.12000, got rendered:\n%s", rendered)
	}
	if !strings.Contains(rendered, "🪙 $0.07500") {
		t.Errorf("Expected status bar turn cost 🪙 $0.07500, got rendered:\n%s", rendered)
	}
}
