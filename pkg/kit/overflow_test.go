package kit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/fantasy"
)

// ---------------------------------------------------------------------------
// isContextOverflow
// ---------------------------------------------------------------------------

func TestIsContextOverflow(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"raw provider text", errors.New("error: context_length_exceeded for this model"), true},
		{"context window phrase", errors.New("the prompt is too long for the context window"), true},
		{"pre-wrapped sentinel", fmt.Errorf("%w: upstream detail", ErrContextOverflow), true},
		{"rate limit", errors.New("HTTP status 429: rate limit exceeded"), false},
		{"generic", errors.New("connection reset by peer"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isContextOverflow(tc.err); got != tc.want {
				t.Errorf("isContextOverflow(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// stripMediaParts
// ---------------------------------------------------------------------------

func TestStripMediaParts_ReplacesFileParts(t *testing.T) {
	msgs := []fantasy.Message{
		fantasy.NewSystemMessage("system prompt"),
		{
			Role: fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: "look at this"},
				fantasy.FilePart{Filename: "diagram.png", MediaType: "image/png", Data: []byte{1, 2, 3}},
			},
		},
	}

	out := stripMediaParts(msgs)

	if len(out) != 2 {
		t.Fatalf("got %d messages, want 2", len(out))
	}
	// Untouched message shares content.
	if len(out[0].Content) != 1 {
		t.Errorf("system message should be unchanged")
	}
	// File part replaced by placeholder text.
	if len(out[1].Content) != 2 {
		t.Fatalf("user message has %d parts, want 2", len(out[1].Content))
	}
	tp, ok := out[1].Content[1].(fantasy.TextPart)
	if !ok {
		t.Fatalf("second part is %T, want TextPart", out[1].Content[1])
	}
	if !strings.Contains(tp.Text, "diagram.png") || !strings.Contains(tp.Text, "removed after context overflow") {
		t.Errorf("placeholder text = %q", tp.Text)
	}
	// Original input must not be mutated.
	if _, ok := msgs[1].Content[1].(fantasy.FilePart); !ok {
		t.Error("input messages were mutated")
	}
}

func TestStripMediaParts_PlaceholderNameFallbacks(t *testing.T) {
	msgs := []fantasy.Message{
		{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{
			fantasy.FilePart{MediaType: "image/jpeg"}, // no filename
			fantasy.FilePart{},                        // no filename, no media type
		}},
	}

	out := stripMediaParts(msgs)

	t0 := out[0].Content[0].(fantasy.TextPart)
	if !strings.Contains(t0.Text, "image/jpeg") {
		t.Errorf("media-type fallback missing: %q", t0.Text)
	}
	t1 := out[0].Content[1].(fantasy.TextPart)
	if !strings.Contains(t1.Text, "attachment") {
		t.Errorf("generic fallback missing: %q", t1.Text)
	}
}

func TestStripMediaParts_NoFilesPassthrough(t *testing.T) {
	msgs := []fantasy.Message{
		fantasy.NewUserMessage("plain text"),
		fantasy.NewSystemMessage("system"),
	}
	out := stripMediaParts(msgs)
	for i := range out {
		if len(out[i].Content) != len(msgs[i].Content) {
			t.Errorf("message %d content length changed", i)
		}
	}
}

// ---------------------------------------------------------------------------
// prepareOverflowRetry
// ---------------------------------------------------------------------------

// newOverflowTestKit builds a minimal Kit with an in-memory session, an event
// bus, and empty hook registries — enough to exercise the reactive
// compaction path without a provider (compaction itself is short-circuited
// by a BeforeCompact hook supplying a custom summary).
func newOverflowTestKit(t *testing.T) *Kit {
	t.Helper()
	tree, err := InitTreeSession(&Options{NoSession: true})
	if err != nil {
		t.Fatalf("InitTreeSession: %v", err)
	}
	return &Kit{
		session:        NewTreeManagerAdapter(tree),
		events:         newEventBus(),
		beforeCompact:  newHookRegistry[BeforeCompactHook, BeforeCompactResult](),
		contextPrepare: newHookRegistry[ContextPrepareHook, ContextPrepareResult](),
	}
}

func TestPrepareOverflowRetry_CompactsAndStripsMedia(t *testing.T) {
	k := newOverflowTestKit(t)

	// Supply a custom summary so compaction needs no LLM call.
	k.OnBeforeCompact(HookPriorityNormal, func(h BeforeCompactHook) *BeforeCompactResult {
		if !h.IsAutomatic {
			t.Error("reactive compaction should run as automatic")
		}
		return &BeforeCompactResult{Summary: "compacted summary of earlier work"}
	})

	// Seed a conversation with a media attachment in the latest user turn.
	_, _ = k.session.AppendMessage(fantasy.NewUserMessage("first question"))
	_, _ = k.session.AppendMessage(fantasy.Message{
		Role:    fantasy.MessageRoleAssistant,
		Content: []fantasy.MessagePart{fantasy.TextPart{Text: "first answer"}},
	})
	_, _ = k.session.AppendMessage(fantasy.Message{
		Role: fantasy.MessageRoleUser,
		Content: []fantasy.MessagePart{
			fantasy.TextPart{Text: "analyze this image"},
			fantasy.FilePart{Filename: "big.png", MediaType: "image/png", Data: make([]byte, 64)},
		},
	})

	messages, ok := k.prepareOverflowRetry(context.Background())
	if !ok {
		t.Fatal("prepareOverflowRetry should succeed")
	}
	if len(messages) == 0 {
		t.Fatal("replay context is empty")
	}

	sawSummary := false
	for _, msg := range messages {
		for _, part := range msg.Content {
			if _, isFile := part.(fantasy.FilePart); isFile {
				t.Error("replay context still contains a media part")
			}
			if tp, isText := part.(fantasy.TextPart); isText && strings.Contains(tp.Text, "compacted summary of earlier work") {
				sawSummary = true
			}
		}
	}
	if !sawSummary {
		t.Error("replay context does not include the compaction summary")
	}
}

func TestPrepareOverflowRetry_CompactionFailureReturnsFalse(t *testing.T) {
	k := newOverflowTestKit(t)

	// Fewer than 2 messages — compactImpl refuses, so recovery must report
	// failure and let the caller surface the original provider error.
	_, _ = k.session.AppendMessage(fantasy.NewUserMessage("only message"))

	if _, ok := k.prepareOverflowRetry(context.Background()); ok {
		t.Fatal("prepareOverflowRetry should fail when compaction is impossible")
	}
}

func TestPrepareOverflowRetry_CancelledCompactionReturnsFalse(t *testing.T) {
	k := newOverflowTestKit(t)
	k.OnBeforeCompact(HookPriorityNormal, func(BeforeCompactHook) *BeforeCompactResult {
		return &BeforeCompactResult{Cancel: true, Reason: "no compaction in tests"}
	})

	_, _ = k.session.AppendMessage(fantasy.NewUserMessage("q"))
	_, _ = k.session.AppendMessage(fantasy.Message{
		Role:    fantasy.MessageRoleAssistant,
		Content: []fantasy.MessagePart{fantasy.TextPart{Text: "a"}},
	})

	if _, ok := k.prepareOverflowRetry(context.Background()); ok {
		t.Fatal("cancelled compaction should abort the retry")
	}
}

func TestPrepareOverflowRetry_RunsContextPrepareHooks(t *testing.T) {
	k := newOverflowTestKit(t)
	k.OnBeforeCompact(HookPriorityNormal, func(BeforeCompactHook) *BeforeCompactResult {
		return &BeforeCompactResult{Summary: "s"}
	})
	hookRan := false
	k.contextPrepare.register(HookPriorityNormal, func(h ContextPrepareHook) *ContextPrepareResult {
		hookRan = true
		return nil
	})

	_, _ = k.session.AppendMessage(fantasy.NewUserMessage("q"))
	_, _ = k.session.AppendMessage(fantasy.Message{
		Role:    fantasy.MessageRoleAssistant,
		Content: []fantasy.MessagePart{fantasy.TextPart{Text: "a"}},
	})

	if _, ok := k.prepareOverflowRetry(context.Background()); !ok {
		t.Fatal("prepareOverflowRetry should succeed")
	}
	if !hookRan {
		t.Error("ContextPrepare hooks must run on the replayed context")
	}
}
