package agent

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
)

// scriptedToolModel is a fake fantasy.LanguageModel. It asks for the "work"
// tool on the first toolSteps calls and then answers with text. It records
// the prompt of every call so tests can check what the LLM saw.
type scriptedToolModel struct {
	toolSteps int

	mu      sync.Mutex
	prompts [][]fantasy.Message
}

func (m *scriptedToolModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *scriptedToolModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *scriptedToolModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *scriptedToolModel) Provider() string { return "fake" }
func (m *scriptedToolModel) Model() string    { return "fake" }

func (m *scriptedToolModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	n := len(m.prompts)
	m.prompts = append(m.prompts, append([]fantasy.Message(nil), call.Prompt...))
	m.mu.Unlock()

	return iter.Seq[fantasy.StreamPart](func(yield func(fantasy.StreamPart) bool) {
		if n < m.toolSteps {
			if !yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolCall,
				ID:            fmt.Sprintf("call-%d", n),
				ToolCallName:  "work",
				ToolCallInput: "{}",
			}) {
				return
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
			return
		}
		parts := []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeTextStart, ID: "text"},
			{Type: fantasy.StreamPartTypeTextDelta, ID: "text", Delta: "done"},
			{Type: fantasy.StreamPartTypeTextEnd, ID: "text"},
			{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
		}
		for _, p := range parts {
			if !yield(p) {
				return
			}
		}
	}), nil
}

func (m *scriptedToolModel) recordedPrompts() [][]fantasy.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prompts
}

// countText returns how many user messages in msgs contain text.
func countText(msgs []fantasy.Message, text string) int {
	n := 0
	for _, msg := range msgs {
		if msg.Role != fantasy.MessageRoleUser {
			continue
		}
		for _, part := range msg.Content {
			if tp, ok := part.(fantasy.TextPart); ok && strings.Contains(tp.Text, text) {
				n++
			}
		}
	}
	return n
}

// indexOfText returns the index of the first user message in msgs that
// contains text, or -1.
func indexOfText(msgs []fantasy.Message, text string) int {
	for i, msg := range msgs {
		if msg.Role != fantasy.MessageRoleUser {
			continue
		}
		for _, part := range msg.Content {
			if tp, ok := part.(fantasy.TextPart); ok && strings.Contains(tp.Text, text) {
				return i
			}
		}
	}
	return -1
}

// TestSteerMessagePersistsAcrossSteps is the regression test for steer
// messages that were visible to the LLM for one step only. The LLM library
// rebuilds each step's prompt from the original prompt plus the step
// responses, so a message added in PrepareStep disappeared at the next step
// and was never saved. The agent then continued its old task and ignored
// the steer message.
func TestSteerMessagePersistsAcrossSteps(t *testing.T) {
	t.Parallel()

	const steerText = "STEER: stop and summarize"
	steerCh := make(chan SteerMessage, 4)

	type emptyInput struct{}
	var injectOnce sync.Once
	work := fantasy.NewAgentTool("work", "does work",
		func(_ context.Context, _ emptyInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// The user steers while the first tool call runs.
			injectOnce.Do(func() {
				steerCh <- SteerMessage{Text: steerText}
			})
			return fantasy.NewTextResponse("ok"), nil
		},
	)

	model := &scriptedToolModel{toolSteps: 2}
	a := &Agent{
		fantasyAgent:     fantasy.NewAgent(model, fantasy.WithTools(work)),
		streamingEnabled: true,
		coreTools:        []fantasy.AgentTool{work},
	}

	var consumed int
	var persisted []fantasy.Message
	ctx := ContextWithSteerCh(context.Background(), steerCh)
	ctx = ContextWithSteerConsumed(ctx, func(n int) { consumed += n })

	input := []fantasy.Message{fantasy.NewUserMessage("go")}
	res, err := a.GenerateWithCallbacks(ctx, input, GenerateCallbacks{
		OnStreamingResponse: func(string) {},
		OnStepMessages: func(msgs []fantasy.Message) {
			persisted = append(persisted, msgs...)
		},
	})
	if err != nil {
		t.Fatalf("GenerateWithCallbacks returned error: %v", err)
	}

	if consumed != 1 {
		t.Errorf("expected 1 consumed steer message, got %d", consumed)
	}

	// Call 0 runs before the steer. Calls 1 and 2 must both contain the
	// steer message exactly once, at the same position (after the first
	// tool result).
	prompts := model.recordedPrompts()
	if len(prompts) != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", len(prompts))
	}
	if n := countText(prompts[0], steerText); n != 0 {
		t.Errorf("call 0: steer message must not be present yet, found %d", n)
	}
	pos := -1
	for i := 1; i < len(prompts); i++ {
		if n := countText(prompts[i], steerText); n != 1 {
			t.Fatalf("call %d: expected steer message exactly once, found %d", i, n)
		}
		got := indexOfText(prompts[i], steerText)
		if pos == -1 {
			pos = got
		} else if got != pos {
			t.Errorf("call %d: steer message moved from index %d to %d", i, pos, got)
		}
		if prev := prompts[i][got-1]; prev.Role != fantasy.MessageRoleTool {
			t.Errorf("call %d: steer message must follow the tool result, follows %q", i, prev.Role)
		}
	}

	// The steer message must be saved incrementally, in conversation order.
	if n := countText(persisted, steerText); n != 1 {
		t.Fatalf("expected steer message persisted once, found %d", n)
	}
	steerIdx := indexOfText(persisted, steerText)
	if steerIdx < 2 || persisted[steerIdx-1].Role != fantasy.MessageRoleTool {
		t.Errorf("persisted steer message must follow the first tool result, got index %d", steerIdx)
	}

	// The returned conversation must contain the steer message and agree
	// with PersistedMessageCount so the caller does not save it twice.
	if n := countText(res.ConversationMessages, steerText); n != 1 {
		t.Fatalf("expected steer message once in ConversationMessages, found %d", n)
	}
	if len(res.Messages) != len(res.ConversationMessages) {
		t.Errorf("Messages (%d) and ConversationMessages (%d) differ in length",
			len(res.Messages), len(res.ConversationMessages))
	}
	newMessages := res.ConversationMessages[len(input):]
	if res.PersistedMessageCount != len(persisted) {
		t.Errorf("PersistedMessageCount = %d, want %d", res.PersistedMessageCount, len(persisted))
	}
	if res.PersistedMessageCount != len(newMessages) {
		t.Errorf("PersistedMessageCount = %d, but %d new messages were returned",
			res.PersistedMessageCount, len(newMessages))
	}
}

func TestWithSteerInjections(t *testing.T) {
	t.Parallel()

	msg := func(s string) fantasy.Message { return fantasy.NewUserMessage(s) }
	text := func(msgs []fantasy.Message) []string {
		out := make([]string, len(msgs))
		for i, m := range msgs {
			out[i] = m.Content[0].(fantasy.TextPart).Text
		}
		return out
	}

	base := []fantasy.Message{msg("a"), msg("b"), msg("c"), msg("d")}

	tests := []struct {
		name       string
		injections []steerInjection
		want       []string
	}{
		{"none", nil, []string{"a", "b", "c", "d"}},
		{"middle", []steerInjection{{pos: 2, msg: msg("s1")}}, []string{"a", "b", "s1", "c", "d"}},
		{
			"same position keeps order",
			[]steerInjection{{pos: 2, msg: msg("s1")}, {pos: 2, msg: msg("s2")}},
			[]string{"a", "b", "s1", "s2", "c", "d"},
		},
		{
			"several positions",
			[]steerInjection{{pos: 1, msg: msg("s1")}, {pos: 3, msg: msg("s2")}},
			[]string{"a", "s1", "b", "c", "s2", "d"},
		},
		{"end", []steerInjection{{pos: 4, msg: msg("s1")}}, []string{"a", "b", "c", "d", "s1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := text(withSteerInjections(base, tt.injections))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
