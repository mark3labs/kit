package kit

import (
	"testing"
	"time"
)

// TestTurnAggregator_BasicLifecycle exercises the per-turn aggregator:
// start → record several tools and steps → consume → snapshot should reflect
// the accumulated counts and zero out for the next turn.
func TestTurnAggregator_BasicLifecycle(t *testing.T) {
	agg := &turnAggregator{}

	agg.start()
	agg.recordTool("bash")
	agg.recordTool("read")
	agg.recordTool("bash")
	agg.recordStep(LLMUsage{
		InputTokens:         100,
		OutputTokens:        50,
		CacheReadTokens:     10,
		CacheCreationTokens: 5,
	})
	agg.recordStep(LLMUsage{
		InputTokens:  200,
		OutputTokens: 75,
	})

	snap := agg.consume()
	if snap.toolCallCount != 3 {
		t.Errorf("toolCallCount: got %d want 3", snap.toolCallCount)
	}
	wantNames := []string{"bash", "read", "bash"}
	if len(snap.toolNames) != len(wantNames) {
		t.Fatalf("toolNames length: got %d want %d", len(snap.toolNames), len(wantNames))
	}
	for i, n := range wantNames {
		if snap.toolNames[i] != n {
			t.Errorf("toolNames[%d]: got %q want %q", i, snap.toolNames[i], n)
		}
	}
	if snap.llmCallCount != 2 {
		t.Errorf("llmCallCount: got %d want 2", snap.llmCallCount)
	}
	if snap.inputTokens != 300 {
		t.Errorf("inputTokens: got %d want 300", snap.inputTokens)
	}
	if snap.outputTokens != 125 {
		t.Errorf("outputTokens: got %d want 125", snap.outputTokens)
	}
	if snap.cacheReadTokens != 10 {
		t.Errorf("cacheReadTokens: got %d want 10", snap.cacheReadTokens)
	}
	if snap.cacheWriteTokens != 5 {
		t.Errorf("cacheWriteTokens: got %d want 5", snap.cacheWriteTokens)
	}
	if snap.durationMs() < 0 {
		t.Errorf("durationMs should not be negative, got %d", snap.durationMs())
	}
}

func TestTurnAggregator_StartResetsCounters(t *testing.T) {
	agg := &turnAggregator{}
	agg.start()
	agg.recordTool("bash")
	agg.recordStep(LLMUsage{InputTokens: 50})

	// Begin a new turn — previous counters should be cleared.
	agg.start()
	snap := agg.consume()

	if snap.toolCallCount != 0 || snap.llmCallCount != 0 || snap.inputTokens != 0 {
		t.Errorf("expected counters zeroed after start(), got %+v", snap)
	}
	if snap.toolNames != nil {
		t.Errorf("expected toolNames=nil after start(), got %v", snap.toolNames)
	}
}

// TestTurnAggregator_DurationMs verifies the snapshot computes a positive
// duration when consume() runs after start().
func TestTurnAggregator_DurationMs(t *testing.T) {
	agg := &turnAggregator{}
	agg.start()
	time.Sleep(5 * time.Millisecond)
	snap := agg.consume()
	if snap.durationMs() < 1 {
		t.Errorf("expected positive duration, got %d", snap.durationMs())
	}
}

// TestTurnAggregator_ZeroStartSafe ensures a snapshot taken without a prior
// start() doesn't crash and reports zero duration.
func TestTurnAggregator_ZeroStartSafe(t *testing.T) {
	agg := &turnAggregator{}
	snap := agg.consume()
	if snap.durationMs() != 0 {
		t.Errorf("expected zero duration for unstarted aggregator, got %d", snap.durationMs())
	}
}

// TestLLMUsageMeta_NilKit verifies the helper degrades gracefully when given
// a nil Kit instance (zero values, no panic).
func TestLLMUsageMeta_NilKit(t *testing.T) {
	provider, modelID, cost := llmUsageMeta(nil, LLMUsage{InputTokens: 100})
	if provider != "" || modelID != "" || cost != 0 {
		t.Errorf("expected zero values for nil kit, got (%q,%q,%v)", provider, modelID, cost)
	}
}

// TestIsAnthropicOAuth_NonAnthropic verifies the helper short-circuits for any
// provider other than "anthropic" without touching the credential store.
func TestIsAnthropicOAuth_NonAnthropic(t *testing.T) {
	for _, provider := range []string{"openai", "google", "openrouter", ""} {
		if isAnthropicOAuth(nil, provider) {
			t.Errorf("isAnthropicOAuth(nil, %q) = true, want false", provider)
		}
	}
}

func TestExtStateSidecarPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"jsonl", "/tmp/sessions/abc.jsonl", "/tmp/sessions/abc.ext-state.json"},
		{"jsonl with subdir", "/a/b/c.jsonl", "/a/b/c.ext-state.json"},
		{"no extension", "/tmp/session-blob", "/tmp/session-blob.ext-state.json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extStateSidecarPath(tc.in)
			if got != tc.want {
				t.Errorf("extStateSidecarPath(%q): got %q want %q", tc.in, got, tc.want)
			}
		})
	}
}
