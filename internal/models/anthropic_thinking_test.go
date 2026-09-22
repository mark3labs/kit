package models

import (
	"testing"

	"charm.land/fantasy/providers/anthropic"
)

// TestBuildAnthropicProviderOptions_EffortModels verifies that effort-graded
// Anthropic models (e.g. claude-opus-5) are configured with
// output_config.effort rather than thinking.budget_tokens. claude-opus-5
// publishes no budget_tokens option, so sending a budget is rejected by the
// API — this is the regression under test.
func TestBuildAnthropicProviderOptions_EffortModels(t *testing.T) {
	cases := []struct {
		model      string
		level      ThinkingLevel
		wantEffort anthropic.Effort
	}{
		{"claude-opus-5", ThinkingHigh, anthropic.EffortHigh},
		{"claude-opus-5", ThinkingMedium, anthropic.EffortMedium},
		{"claude-opus-5", ThinkingLow, anthropic.EffortLow},
		// "none"/"minimal" have no effort name; they coerce to the lowest
		// supported graded level (low) rather than being dropped.
		{"claude-opus-5", ThinkingNone, anthropic.EffortLow},
		{"claude-opus-5", ThinkingMinimal, anthropic.EffortLow},
		{"claude-opus-4-5", ThinkingHigh, anthropic.EffortHigh},
		{"claude-sonnet-4-6", ThinkingMedium, anthropic.EffortMedium},
	}

	for _, tc := range cases {
		t.Run(string(tc.model)+"/"+string(tc.level), func(t *testing.T) {
			cfg := &ProviderConfig{ThinkingLevel: tc.level}
			opts := buildAnthropicProviderOptions(cfg, tc.model)
			if opts == nil {
				t.Fatalf("expected non-nil options for %s @ %s", tc.model, tc.level)
			}
			po, ok := opts[anthropic.Name].(*anthropic.ProviderOptions)
			if !ok || po == nil {
				t.Fatalf("expected *anthropic.ProviderOptions under %q", anthropic.Name)
			}
			if po.Thinking != nil {
				t.Errorf("effort model must not carry thinking.budget_tokens, got %+v", po.Thinking)
			}
			if po.Effort == nil {
				t.Fatalf("effort model must carry output_config.effort")
			}
			if *po.Effort != tc.wantEffort {
				t.Errorf("Effort = %q, want %q", *po.Effort, tc.wantEffort)
			}
		})
	}
}

// TestBuildAnthropicProviderOptions_BudgetModels verifies that classic,
// budget-only Anthropic models keep sending thinking.budget_tokens and never
// switch to the effort field.
func TestBuildAnthropicProviderOptions_BudgetModels(t *testing.T) {
	for _, model := range []string{"claude-sonnet-4-5", "claude-haiku-4-5"} {
		t.Run(model, func(t *testing.T) {
			cfg := &ProviderConfig{ThinkingLevel: ThinkingHigh}
			opts := buildAnthropicProviderOptions(cfg, model)
			if opts == nil {
				t.Fatalf("expected non-nil options for %s", model)
			}
			po, ok := opts[anthropic.Name].(*anthropic.ProviderOptions)
			if !ok || po == nil {
				t.Fatalf("expected *anthropic.ProviderOptions under %q", anthropic.Name)
			}
			if po.Effort != nil {
				t.Errorf("budget model must not carry output_config.effort, got %q", *po.Effort)
			}
			if po.Thinking == nil {
				t.Fatalf("budget model must carry thinking.budget_tokens")
			}
			if po.Thinking.BudgetTokens == 0 {
				t.Errorf("budget must be non-zero")
			}
		})
	}
}

// TestBuildAnthropicProviderOptions_Off verifies thinking-off yields no options
// regardless of model family.
func TestBuildAnthropicProviderOptions_Off(t *testing.T) {
	for _, model := range []string{"claude-opus-5", "claude-sonnet-4-5"} {
		cfg := &ProviderConfig{ThinkingLevel: ThinkingOff}
		if opts := buildAnthropicProviderOptions(cfg, model); opts != nil {
			t.Errorf("%s: expected nil options when thinking is off, got %+v", model, opts)
		}
	}
}
