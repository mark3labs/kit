package compaction

import (
	"context"
	"errors"
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
		got := estimateTokens(tt.text)
		if got != tt.want {
			t.Errorf("estimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
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

func TestEstimateMessageTokens_AllPartTypes(t *testing.T) {
	// Tool-heavy message traffic must be counted (issue #83): tool-call
	// JSON arguments, tool results, reasoning, and file parts all
	// contribute to the estimate.
	toolInput := `{"path":"` + strings.Repeat("a", 394) + `"}` // 404 chars
	msgs := []fantasy.Message{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.ReasoningPart{Text: strings.Repeat("r", 400)}, // 100 tokens
				fantasy.ToolCallPart{
					ToolCallID: "1",
					ToolName:   "read",    // 4 chars → 1 token
					Input:      toolInput, // 404 chars → 101 tokens
				},
			},
		},
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "1",
					Output:     fantasy.ToolResultOutputContentText{Text: strings.Repeat("o", 800)}, // 200 tokens
				},
			},
		},
	}

	got := EstimateMessageTokens(msgs)
	want := 100 + 1 + 101 + 200
	if got != want {
		t.Errorf("EstimateMessageTokens = %d, want %d", got, want)
	}
}

func TestEstimateMessageTokens_ToolResultVariants(t *testing.T) {
	errMsg := fantasy.Message{
		Role: fantasy.MessageRoleTool,
		Content: []fantasy.MessagePart{
			fantasy.ToolResultPart{
				Output: fantasy.ToolResultOutputContentError{Error: errors.New(strings.Repeat("e", 400))},
			},
		},
	}
	if got := EstimateMessageTokens([]fantasy.Message{errMsg}); got != 100 {
		t.Errorf("error tool result tokens = %d, want 100", got)
	}

	mediaMsg := fantasy.Message{
		Role: fantasy.MessageRoleTool,
		Content: []fantasy.MessagePart{
			fantasy.ToolResultPart{
				Output: fantasy.ToolResultOutputContentMedia{
					Data: strings.Repeat("D", 400), // base64 payload → 100 tokens
					Text: strings.Repeat("t", 40),  // 10 tokens
				},
			},
		},
	}
	if got := EstimateMessageTokens([]fantasy.Message{mediaMsg}); got != 110 {
		t.Errorf("media tool result tokens = %d, want 110", got)
	}

	nilErrMsg := fantasy.Message{
		Role: fantasy.MessageRoleTool,
		Content: []fantasy.MessagePart{
			fantasy.ToolResultPart{Output: fantasy.ToolResultOutputContentError{}},
		},
	}
	if got := EstimateMessageTokens([]fantasy.Message{nilErrMsg}); got != 0 {
		t.Errorf("nil error tool result tokens = %d, want 0", got)
	}
}

func TestEstimateMessageTokens_FilePart(t *testing.T) {
	msg := fantasy.Message{
		Role: fantasy.MessageRoleUser,
		Content: []fantasy.MessagePart{
			fantasy.FilePart{
				Filename:  strings.Repeat("f", 8), // 2 tokens
				Data:      make([]byte, 300),      // 300/3 = 100 tokens
				MediaType: "image/png",
			},
		},
	}
	if got := EstimateMessageTokens([]fantasy.Message{msg}); got != 102 {
		t.Errorf("file part tokens = %d, want 102", got)
	}
}

// ---------------------------------------------------------------------------
// ShouldCompact (contextTokens > contextWindow - reserveTokens)
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
// FindCutPoint (token-based)
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

func TestCompactionOptions_AdaptiveDefaults(t *testing.T) {
	// Small-context, small-output model: both budgets scale down.
	opts := CompactionOptions{ContextWindow: 32768, MaxOutputTokens: 4096}
	opts.ApplyDefaults()

	if opts.ReserveTokens != 4096 {
		t.Errorf("ReserveTokens = %d, want 4096 (min(16384, maxOutput))", opts.ReserveTokens)
	}
	wantKeep := (32768 - 4096) / 4 // 7168
	if opts.KeepRecentTokens != wantKeep {
		t.Errorf("KeepRecentTokens = %d, want %d (usable/4)", opts.KeepRecentTokens, wantKeep)
	}
}

func TestAdaptiveReserveTokens(t *testing.T) {
	tests := []struct {
		name      string
		maxOutput int
		want      int
	}{
		{"unknown limit", 0, DefaultReserveTokens},
		{"negative limit", -1, DefaultReserveTokens},
		{"small output model", 4096, 4096},
		{"large output model", 65536, DefaultReserveTokens},
		{"exactly at default", 16384, 16384},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AdaptiveReserveTokens(tt.maxOutput); got != tt.want {
				t.Errorf("AdaptiveReserveTokens(%d) = %d, want %d", tt.maxOutput, got, tt.want)
			}
		})
	}
}

func TestAdaptiveKeepRecentTokens(t *testing.T) {
	tests := []struct {
		name          string
		contextWindow int
		reserveTokens int
		want          int
	}{
		{"unknown window", 0, 16384, DefaultKeepRecentTokens},
		{"reserve swallows window", 8192, 16384, minKeepRecentTokens},
		{"tiny model floors at minimum", 8192, 4096, minKeepRecentTokens}, // usable/4 = 1024 < 2000
		{"small model", 32768, 4096, (32768 - 4096) / 4},                  // 7168
		{"200k model", 200000, 16384, (200000 - 16384) / 4},               // 45904
		{"1M model scales up", 1000000, 16384, (1000000 - 16384) / 4},     // 245904
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AdaptiveKeepRecentTokens(tt.contextWindow, tt.reserveTokens)
			if got != tt.want {
				t.Errorf("AdaptiveKeepRecentTokens(%d, %d) = %d, want %d",
					tt.contextWindow, tt.reserveTokens, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Compact (integration — too few messages)
// ---------------------------------------------------------------------------

func TestCompact_TooFewMessages(t *testing.T) {
	msgs := []fantasy.Message{
		makeTextMessageN(fantasy.MessageRoleUser, 400),
	}

	result, newMsgs, err := Compact(context.TODO(), nil, msgs, CompactionOptions{}, "", nil, nil)
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

	result, newMsgs, err := Compact(context.TODO(), nil, msgs, CompactionOptions{}, "", nil, nil)
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

// ---------------------------------------------------------------------------
// Tool result truncation
// ---------------------------------------------------------------------------

func TestTruncateToolResult(t *testing.T) {
	// Short text — no truncation.
	short := strings.Repeat("x", 100)
	if got := truncateToolResult(short); got != short {
		t.Errorf("truncated short text unexpectedly")
	}

	// Exactly at limit.
	exact := strings.Repeat("x", maxToolResultChars)
	if got := truncateToolResult(exact); got != exact {
		t.Errorf("truncated text at exact limit")
	}

	// Over limit.
	over := strings.Repeat("x", maxToolResultChars+500)
	got := truncateToolResult(over)
	if len(got) > maxToolResultChars+50 { // allow room for marker
		t.Errorf("truncated text too long: %d chars", len(got))
	}
	if !strings.Contains(got, "500 chars truncated") {
		t.Errorf("truncation marker missing, got: %s", got[maxToolResultChars:])
	}
}

func TestSerializeMessages_TruncatesToolResults(t *testing.T) {
	longResult := strings.Repeat("R", maxToolResultChars+1000)
	msgs := []fantasy.Message{
		makeTextMessage(fantasy.MessageRoleUser, "question"),
		{
			Role:    fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{fantasy.TextPart{Text: longResult}},
		},
	}

	serialized := serializeMessages(msgs)
	if strings.Contains(serialized, longResult) {
		t.Error("tool result was not truncated during serialisation")
	}
	if !strings.Contains(serialized, "chars truncated") {
		t.Error("truncation marker missing in serialised output")
	}
}

func TestSerializeMessages_PreservesNonToolText(t *testing.T) {
	longText := strings.Repeat("T", maxToolResultChars+1000)
	msgs := []fantasy.Message{
		makeTextMessage(fantasy.MessageRoleUser, longText),
	}

	serialized := serializeMessages(msgs)
	if !strings.Contains(serialized, longText) {
		t.Error("non-tool text was unexpectedly truncated")
	}
}

// ---------------------------------------------------------------------------
// Split turn detection
// ---------------------------------------------------------------------------

func TestIsSplitTurn(t *testing.T) {
	msgs := []fantasy.Message{
		makeTextMessageN(fantasy.MessageRoleUser, 400),      // 0: turn 1 user
		makeTextMessageN(fantasy.MessageRoleAssistant, 400), // 1: turn 1 assistant
		makeTextMessageN(fantasy.MessageRoleUser, 400),      // 2: turn 2 user
		makeTextMessageN(fantasy.MessageRoleAssistant, 400), // 3: turn 2 assistant
		makeTextMessageN(fantasy.MessageRoleTool, 400),      // 4: turn 2 tool result
		makeTextMessageN(fantasy.MessageRoleAssistant, 400), // 5: turn 2 assistant
	}

	tests := []struct {
		name     string
		cutPoint int
		want     bool
	}{
		{"at user message (turn boundary)", 2, false},
		{"at assistant mid-turn", 3, true},
		{"at assistant after tool (mid-turn)", 5, true},
		{"at 0 (no cut)", 0, false},
		{"beyond range", 10, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSplitTurn(msgs, tt.cutPoint)
			if got != tt.want {
				t.Errorf("IsSplitTurn(msgs, %d) = %v, want %v", tt.cutPoint, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// File operations extraction
// ---------------------------------------------------------------------------

func TestExtractFileOps(t *testing.T) {
	// Create messages with tool calls.
	msgs := []fantasy.Message{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.ToolCallPart{ToolCallID: "1", ToolName: "read", Input: `{"path":"src/main.go"}`},
				fantasy.ToolCallPart{ToolCallID: "2", ToolName: "write", Input: `{"path":"src/out.go"}`},
				fantasy.ToolCallPart{ToolCallID: "3", ToolName: "edit", Input: `{"path":"src/edit.go"}`},
				fantasy.ToolCallPart{ToolCallID: "4", ToolName: "grep", Input: `{"path":"src/search"}`},
			},
		},
	}

	ops := extractFileOps(msgs)
	if !ops.ReadFiles["src/main.go"] {
		t.Error("read file not tracked: src/main.go")
	}
	if !ops.ReadFiles["src/search"] {
		t.Error("grep path not tracked as read: src/search")
	}
	if !ops.ModifiedFiles["src/out.go"] {
		t.Error("write file not tracked: src/out.go")
	}
	if !ops.ModifiedFiles["src/edit.go"] {
		t.Error("edit file not tracked: src/edit.go")
	}
}

func TestFileOps_MergeSlices(t *testing.T) {
	ops := newFileOps()
	ops.ReadFiles["a.go"] = true
	ops.ModifiedFiles["b.go"] = true

	ops.mergeSlices(
		[]string{"c.go", "a.go"},
		[]string{"d.go"},
	)

	if len(ops.ReadFiles) != 2 { // a.go, c.go
		t.Errorf("ReadFiles len = %d, want 2", len(ops.ReadFiles))
	}
	if len(ops.ModifiedFiles) != 2 { // b.go, d.go
		t.Errorf("ModifiedFiles len = %d, want 2", len(ops.ModifiedFiles))
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]bool{"c": true, "a": true, "b": true}
	got := sortedKeys(m)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("sortedKeys len = %d, want %d", len(got), len(want))
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("sortedKeys[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestSortedKeys_Empty(t *testing.T) {
	got := sortedKeys(nil)
	if got != nil {
		t.Errorf("sortedKeys(nil) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// Anchored summary prompt (issue #83)
// ---------------------------------------------------------------------------

func TestBuildSummaryPrompt_NoAnchor(t *testing.T) {
	prompt := buildSummaryPrompt("[User]:\nhello\n\n", CompactionOptions{}, "", "")
	if strings.Contains(prompt, "<anchored-summary>") {
		t.Error("anchored-summary block present without a previous summary")
	}
	if !strings.Contains(prompt, "## Goal") {
		t.Error("default summary prompt missing")
	}
}

func TestBuildSummaryPrompt_WithAnchor(t *testing.T) {
	prev := "## Goal\nShip the widget feature."
	prompt := buildSummaryPrompt("[User]:\nhello\n\n", CompactionOptions{}, "", prev)

	if !strings.HasPrefix(prompt, "<anchored-summary>\n"+prev+"\n</anchored-summary>") {
		t.Error("previous summary not anchored at the top of the prompt")
	}
	if !strings.Contains(prompt, "Update that anchored summary") {
		t.Error("anchored update instructions missing")
	}
	// The anchor must come before the conversation text.
	if strings.Index(prompt, "</anchored-summary>") > strings.Index(prompt, "[User]:") {
		t.Error("anchored summary should precede the conversation text")
	}
}

func TestBuildSummaryPrompt_WithAnchorAndCustomInstructions(t *testing.T) {
	prompt := buildSummaryPrompt("convo", CompactionOptions{}, "Focus on API design", "prior summary")
	if !strings.Contains(prompt, "Additional instructions: Focus on API design") {
		t.Error("custom instructions missing")
	}
	if !strings.Contains(prompt, "<anchored-summary>") {
		t.Error("anchored summary missing")
	}
}

// ---------------------------------------------------------------------------
// Skill-content protection (issue #65, gap #7)
// ---------------------------------------------------------------------------

func TestIsProtectedMessage(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{`<skill name="foo" location="/x">body</skill>`, true},
		{`<skill_content name="foo">body</skill_content>`, true},
		{"just a normal message", false},
		{"talking about skills in general", false},
	}
	for _, c := range cases {
		msg := makeTextMessage(fantasy.MessageRoleUser, c.text)
		if got := isProtectedMessage(msg); got != c.want {
			t.Errorf("isProtectedMessage(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}
