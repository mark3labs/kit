package ui

import "testing"

func TestUsageTrackerActiveOAuthState(t *testing.T) {
	for _, model := range []string{"anthropic/claude-sonnet-4-5", "openai/gpt-5", "copilot/gpt-5.5", "github-copilot/gpt-5.5"} {
		t.Run(model, func(t *testing.T) {
			tracker := CreateUsageTracker(model, true)
			if tracker == nil {
				t.Fatal("missing model metadata")
			}
			if !tracker.IsOAuth() {
				t.Fatal("OAuth state not set")
			}
			tracker.UpdateUsage(1000, 500, 0, 0)
			if tracker.GetTurnStats().TotalCost != 0 {
				t.Fatal("OAuth usage has a cost")
			}
			UpdateUsageTrackerForModel(tracker, model, false)
			if tracker.IsOAuth() {
				t.Fatal("OAuth state not cleared on switch to API key")
			}
			tracker.UpdateUsage(1000, 500, 0, 0)
			if tracker.GetTurnStats().TotalCost <= 0 {
				t.Fatal("API key usage has no cost")
			}
			UpdateUsageTrackerForModel(tracker, model, true)
			if !tracker.IsOAuth() {
				t.Fatal("OAuth state not restored")
			}
		})
	}
}
