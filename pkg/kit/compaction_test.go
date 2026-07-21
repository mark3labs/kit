package kit_test

import (
	"context"
	"testing"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// TestAdjustPostCompactionTokens verifies the post-compaction token baseline
// adjustment (issue #80). Previously the API-reported count was zeroed after
// compaction, forcing ShouldCompact()/GetContextStats() onto a text-only
// heuristic that ignores system prompt, tool schemas, and tool traffic —
// auto-compaction then never re-fired and the next API call could overflow
// the real context window.
func TestAdjustPostCompactionTokens(t *testing.T) {
	tests := []struct {
		name            string
		lastInputTokens int
		originalTokens  int
		compactedTokens int
		want            int
	}{
		{
			// Issue #80 scenario: API reports 22k real tokens, heuristic
			// only sees 8k of message text. Compaction shrinks the heuristic
			// estimate to 3k. Baseline must keep the ~14k of invisible
			// overhead: 22k − (8k − 3k) = 17k, not 0.
			name:            "preserves overhead from API baseline",
			lastInputTokens: 22000,
			originalTokens:  8000,
			compactedTokens: 3000,
			want:            17000,
		},
		{
			// No API turn yet — keep 0 so callers fall back to the heuristic.
			name:            "zero baseline stays zero",
			lastInputTokens: 0,
			originalTokens:  8000,
			compactedTokens: 3000,
			want:            0,
		},
		{
			// Heuristic reduction larger than the API baseline — clamp to
			// the heuristic estimate of the kept messages, never below.
			name:            "clamps to compacted estimate",
			lastInputTokens: 4000,
			originalTokens:  10000,
			compactedTokens: 2500,
			want:            2500,
		},
		{
			// Degenerate: compaction "grew" the estimate (summary longer
			// than removed text). Reduction clamps to 0; baseline unchanged.
			name:            "negative reduction leaves baseline unchanged",
			lastInputTokens: 22000,
			originalTokens:  3000,
			compactedTokens: 5000,
			want:            22000,
		},
		{
			// Reduction exactly consumes the baseline but the kept messages
			// still estimate higher — clamp wins.
			name:            "clamp when adjusted below kept estimate",
			lastInputTokens: 10000,
			originalTokens:  9000,
			compactedTokens: 4000,
			want:            5000,
		},
		{
			name:            "negative baseline treated as zero",
			lastInputTokens: -5,
			originalTokens:  100,
			compactedTokens: 50,
			want:            0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kit.AdjustPostCompactionTokensForTest(tt.lastInputTokens, tt.originalTokens, tt.compactedTokens)
			if got != tt.want {
				t.Errorf("adjustPostCompactionTokens(%d, %d, %d) = %d, want %d",
					tt.lastInputTokens, tt.originalTokens, tt.compactedTokens, got, tt.want)
			}
		})
	}
}

// TestPostCompactionTokenBaselinePreserved exercises the real compaction
// persistence path and asserts the API-reported baseline survives compaction
// instead of being reset to zero (issue #80).
func TestPostCompactionTokenBaselinePreserved(t *testing.T) {
	requireAnthropicAuth(t)

	ctx := context.Background()

	host, err := kit.New(ctx, &kit.Options{Quiet: true, NoSession: true})
	if err != nil {
		t.Fatalf("Failed to create Kit: %v", err)
	}
	defer func() { _ = host.Close() }()

	// Simulate a completed API turn reporting 22k real tokens.
	host.SetLastInputTokensForTest(22000)

	// Persist a compaction whose heuristic saw 8k → 3k.
	if err := host.PersistAndEmitCompactionForTest("summary", "", 8000, 3000, 5); err != nil {
		t.Fatalf("persistAndEmitCompaction failed: %v", err)
	}

	if got, want := host.LastInputTokensForTest(), 17000; got != want {
		t.Errorf("post-compaction baseline = %d, want %d (must not be reset to 0)", got, want)
	}

	// GetContextStats must report the adjusted baseline, not the heuristic.
	if got := host.GetContextStats().EstimatedTokens; got != 17000 {
		t.Errorf("GetContextStats().EstimatedTokens = %d, want 17000", got)
	}
}
