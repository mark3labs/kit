package compaction

import (
	"context"
	"testing"

	"charm.land/fantasy"
)

func makeTextMessage(role fantasy.MessageRole, text string) fantasy.Message {
	return fantasy.Message{
		Role:    role,
		Content: []fantasy.MessagePart{fantasy.TextPart{Text: text}},
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"hi", 0},          // 2 chars / 4 = 0
		{"hello", 1},       // 5 / 4 = 1
		{"hello world", 2}, // 11 / 4 = 2
	}
	for _, tt := range tests {
		got := EstimateTokens(tt.text)
		if got != tt.want {
			t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestEstimateMessageTokens(t *testing.T) {
	msgs := []fantasy.Message{
		makeTextMessage(fantasy.MessageRoleUser, "Hello, how are you?"),  // 19 / 4 = 4
		makeTextMessage(fantasy.MessageRoleAssistant, "I'm doing great"), // 15 / 4 = 3
	}
	got := EstimateMessageTokens(msgs)
	want := 4 + 3
	if got != want {
		t.Errorf("EstimateMessageTokens = %d, want %d", got, want)
	}
}

func TestEstimateMessageTokens_Empty(t *testing.T) {
	got := EstimateMessageTokens(nil)
	if got != 0 {
		t.Errorf("EstimateMessageTokens(nil) = %d, want 0", got)
	}
}

func TestShouldCompact(t *testing.T) {
	// Create messages that total ~100 tokens (400 chars).
	longText := make([]byte, 400)
	for i := range longText {
		longText[i] = 'a'
	}
	msgs := []fantasy.Message{makeTextMessage(fantasy.MessageRoleUser, string(longText))}

	tests := []struct {
		name         string
		contextLimit int
		threshold    float64
		want         bool
	}{
		{"above threshold", 120, 0.8, true},      // 100 >= 120*0.8=96
		{"below threshold", 200, 0.8, false},     // 100 < 200*0.8=160
		{"zero limit", 0, 0.8, false},            // no limit
		{"zero threshold", 200, 0.0, false},      // no threshold
		{"exactly at threshold", 125, 0.8, true}, // 100 >= 125*0.8=100
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldCompact(msgs, tt.contextLimit, tt.threshold)
			if got != tt.want {
				t.Errorf("ShouldCompact() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindCutPoint(t *testing.T) {
	msgs := make([]fantasy.Message, 20)
	for i := range msgs {
		msgs[i] = makeTextMessage(fantasy.MessageRoleUser, "msg")
	}

	tests := []struct {
		name           string
		preserveRecent int
		want           int
	}{
		{"preserve 10", 10, 10},
		{"preserve 5", 5, 15},
		{"preserve all", 20, 0},
		{"preserve more than total", 25, 0},
		{"preserve 0 uses default 10", 0, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindCutPoint(msgs, tt.preserveRecent)
			if got != tt.want {
				t.Errorf("FindCutPoint() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCompactionOptions_Defaults(t *testing.T) {
	opts := CompactionOptions{}
	opts.defaults()

	if opts.ThresholdPct != 0.8 {
		t.Errorf("ThresholdPct = %f, want 0.8", opts.ThresholdPct)
	}
	if opts.PreserveRecent != 10 {
		t.Errorf("PreserveRecent = %d, want 10", opts.PreserveRecent)
	}
}

func TestCompactionOptions_DefaultsPreservesExisting(t *testing.T) {
	opts := CompactionOptions{ThresholdPct: 0.9, PreserveRecent: 5}
	opts.defaults()

	if opts.ThresholdPct != 0.9 {
		t.Errorf("ThresholdPct = %f, want 0.9", opts.ThresholdPct)
	}
	if opts.PreserveRecent != 5 {
		t.Errorf("PreserveRecent = %d, want 5", opts.PreserveRecent)
	}
}

func TestCompact_TooFewMessages(t *testing.T) {
	msgs := make([]fantasy.Message, 5)
	for i := range msgs {
		msgs[i] = makeTextMessage(fantasy.MessageRoleUser, "short")
	}

	// Default preserveRecent = 10, so 5 messages is too few.
	result, newMsgs, err := Compact(context.TODO(), nil, msgs, CompactionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for too-few messages")
	}
	if len(newMsgs) != len(msgs) {
		t.Errorf("messages changed: got %d, want %d", len(newMsgs), len(msgs))
	}
}
