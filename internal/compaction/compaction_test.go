package compaction

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"
)

func makeTextMessage(role fantasy.MessageRole, text string) fantasy.Message {
	return fantasy.Message{
		Role:    role,
		Content: []fantasy.MessagePart{fantasy.TextPart{Text: text}},
	}
}

// makeTextMessageN creates a message whose text is exactly n characters long
// (≈ n/4 estimated tokens).
func makeTextMessageN(role fantasy.MessageRole, n int) fantasy.Message {
	return makeTextMessage(role, strings.Repeat("a", n))
}

// ---------------------------------------------------------------------------
// Token estimation
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// ShouldCompact (Pi-style: contextTokens > contextWindow - reserveTokens)
// ---------------------------------------------------------------------------

func TestShouldCompact(t *testing.T) {
	// Create messages that total ~100 tokens (400 chars).
	msgs := []fantasy.Message{makeTextMessageN(fantasy.MessageRoleUser, 400)}

	tests := []struct {
		name          string
		contextWindow int
		reserveTokens int
		want          bool
	}{
		{"above threshold", 110, 16, true},       // 100 > 110-16=94 → true
		{"below threshold", 200, 16, false},      // 100 > 200-16=184 → false
		{"zero window", 0, 16, false},            // no window
		{"zero reserve", 200, 0, false},          // no reserve
		{"exactly at threshold", 116, 16, false}, // 100 > 116-16=100 → false (not >)
		{"one over", 115, 16, true},              // 100 > 115-16=99 → true
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldCompact(msgs, tt.contextWindow, tt.reserveTokens)
			if got != tt.want {
				t.Errorf("ShouldCompact() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FindCutPoint (token-based, Pi-style)
// ---------------------------------------------------------------------------

func TestFindCutPoint_TokenBased(t *testing.T) {
	// Each message is 400 chars = ~100 tokens.
	msgs := make([]fantasy.Message, 10)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = makeTextMessageN(fantasy.MessageRoleUser, 400)
		} else {
			msgs[i] = makeTextMessageN(fantasy.MessageRoleAssistant, 400)
		}
	}

	tests := []struct {
		name             string
		keepRecentTokens int
		want             int // expected cut point
	}{
		// keepRecentTokens=250 → walk back: msg[9]=100, msg[8]=200 ≤ 250,
		// msg[7]=300 > 250 → cut = 8.
		{"keep 250 tokens", 250, 8},
		// keepRecentTokens=500 → walk back 5 msgs = 500 ≤ 500,
		// 6th msg = 600 > 500 → cut = 5.
		{"keep 500 tokens", 500, 5},
		// keepRecentTokens=1000 → all 10 msgs = 1000, not exceeded → cut = 0.
		{"keep all", 1000, 0},
		// keepRecentTokens=50 → msg[9] alone = 100 > 50 → cut = 10,
		// exceeds len → clamped to 9. 9 ≥ 2 → valid.
		{"keep very few", 50, 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindCutPoint(msgs, tt.keepRecentTokens)
			if got != tt.want {
				t.Errorf("FindCutPoint() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFindCutPoint_TooFewMessages(t *testing.T) {
	msgs := []fantasy.Message{
		makeTextMessageN(fantasy.MessageRoleUser, 400),
	}
	got := FindCutPoint(msgs, 50)
	if got != 0 {
		t.Errorf("FindCutPoint(1 msg) = %d, want 0", got)
	}
}

func TestFindCutPoint_SkipsToolResults(t *testing.T) {
	// [user, assistant, tool, user, assistant]
	// Each 400 chars = 100 tokens. keepRecentTokens=150 → walk back:
	//   msg[4] (assistant) = 100 ≤ 150
	//   msg[3] (user) = 200 > 150 → raw cut at 4, but check validity.
	//   msg[4] is assistant → valid cut point. Cut = 4.
	msgs := []fantasy.Message{
		makeTextMessageN(fantasy.MessageRoleUser, 400),
		makeTextMessageN(fantasy.MessageRoleAssistant, 400),
		makeTextMessageN(fantasy.MessageRoleTool, 400),
		makeTextMessageN(fantasy.MessageRoleUser, 400),
		makeTextMessageN(fantasy.MessageRoleAssistant, 400),
	}
	got := FindCutPoint(msgs, 150)
	if got != 4 {
		t.Errorf("FindCutPoint() = %d, want 4", got)
	}

	// Now test where the raw cut lands on a tool result.
	// [user, assistant, tool, tool, user]
	// keepRecentTokens=50 → walk back: msg[4]=100 > 50 → raw cut at 5? No,
	// i=4, accumulated=100 > 50, cut = i+1 = 5 → that's len(msgs), so no
	// valid split. Actually let me think again...
	// i starts at 4 (last), accumulated += 100 = 100 > 50 → cut = 5.
	// cut=5 >= len(msgs)=5 → return 0. Correct.

	// Try keepRecentTokens=150 → walk back:
	//   msg[4] (user) = 100 ≤ 150
	//   msg[3] (tool) = 200 > 150 → cut at 4, msg[4] is user → valid.
	msgs2 := []fantasy.Message{
		makeTextMessageN(fantasy.MessageRoleUser, 400),
		makeTextMessageN(fantasy.MessageRoleAssistant, 400),
		makeTextMessageN(fantasy.MessageRoleTool, 400),
		makeTextMessageN(fantasy.MessageRoleTool, 400),
		makeTextMessageN(fantasy.MessageRoleUser, 400),
	}
	got2 := FindCutPoint(msgs2, 150)
	if got2 != 4 {
		t.Errorf("FindCutPoint(tool results) = %d, want 4", got2)
	}

	// Where raw cut lands ON a tool message and must scan forward.
	// [user(0), assistant(1), tool(2), tool(3), user(4), assistant(5)]
	// keepRecentTokens=250 → walk back:
	//   msg[5] = 100 ≤ 250
	//   msg[4] = 200 ≤ 250
	//   msg[3] = 300 > 250 → cut at 4, msg[4] is user → valid.
	msgs3 := []fantasy.Message{
		makeTextMessageN(fantasy.MessageRoleUser, 400),
		makeTextMessageN(fantasy.MessageRoleAssistant, 400),
		makeTextMessageN(fantasy.MessageRoleTool, 400),
		makeTextMessageN(fantasy.MessageRoleTool, 400),
		makeTextMessageN(fantasy.MessageRoleUser, 400),
		makeTextMessageN(fantasy.MessageRoleAssistant, 400),
	}
	got3 := FindCutPoint(msgs3, 250)
	if got3 != 4 {
		t.Errorf("FindCutPoint(scan forward) = %d, want 4", got3)
	}
}

// ---------------------------------------------------------------------------
// CompactionOptions defaults
// ---------------------------------------------------------------------------

func TestCompactionOptions_Defaults(t *testing.T) {
	opts := CompactionOptions{}
	opts.defaults()

	if opts.ReserveTokens != 16384 {
		t.Errorf("ReserveTokens = %d, want 16384", opts.ReserveTokens)
	}
	if opts.KeepRecentTokens != 20000 {
		t.Errorf("KeepRecentTokens = %d, want 20000", opts.KeepRecentTokens)
	}
}

func TestCompactionOptions_DefaultsPreservesExisting(t *testing.T) {
	opts := CompactionOptions{ReserveTokens: 8192, KeepRecentTokens: 10000}
	opts.defaults()

	if opts.ReserveTokens != 8192 {
		t.Errorf("ReserveTokens = %d, want 8192", opts.ReserveTokens)
	}
	if opts.KeepRecentTokens != 10000 {
		t.Errorf("KeepRecentTokens = %d, want 10000", opts.KeepRecentTokens)
	}
}

// ---------------------------------------------------------------------------
// Compact (integration — too few messages)
// ---------------------------------------------------------------------------

func TestCompact_TooFewMessages(t *testing.T) {
	msgs := []fantasy.Message{
		makeTextMessageN(fantasy.MessageRoleUser, 400),
	}

	result, newMsgs, err := Compact(context.TODO(), nil, msgs, CompactionOptions{}, "")
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

func TestCompact_WithinBudget(t *testing.T) {
	// 2 messages, each 100 tokens, keepRecentTokens=20000 → all fit.
	msgs := []fantasy.Message{
		makeTextMessageN(fantasy.MessageRoleUser, 400),
		makeTextMessageN(fantasy.MessageRoleAssistant, 400),
	}

	result, newMsgs, err := Compact(context.TODO(), nil, msgs, CompactionOptions{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when all messages fit within budget")
	}
	if len(newMsgs) != len(msgs) {
		t.Errorf("messages changed: got %d, want %d", len(newMsgs), len(msgs))
	}
}
