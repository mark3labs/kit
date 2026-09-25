package kit

import (
	"context"
	"fmt"
	"iter"
	"sync"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
)

// scriptedToolModel is an in-process language model that calls one tool per
// step, in the order given by script. When the script is exhausted it
// answers with plain text. It counts every model call.
type scriptedToolModel struct {
	echoModel
	script []string

	mu    sync.Mutex
	calls int
}

func (m *scriptedToolModel) next() (int, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := m.calls
	m.calls++
	if i < len(m.script) {
		return i, m.script[i]
	}
	return i, ""
}

func (m *scriptedToolModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func (m *scriptedToolModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	i, tool := m.next()
	if tool == "" {
		return &fantasy.Response{
			Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "ok"}},
			FinishReason: fantasy.FinishReasonStop,
		}, nil
	}
	return &fantasy.Response{
		Content: fantasy.ResponseContent{fantasy.ToolCallContent{
			ToolCallID: fmt.Sprintf("call-%d", i),
			ToolName:   tool,
			Input:      "{}",
		}},
		FinishReason: fantasy.FinishReasonToolCalls,
	}, nil
}

func (m *scriptedToolModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	i, tool := m.next()
	var parts []fantasy.StreamPart
	if tool == "" {
		parts = []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeTextStart, ID: "t"},
			{Type: fantasy.StreamPartTypeTextDelta, ID: "t", Delta: "ok"},
			{Type: fantasy.StreamPartTypeTextEnd, ID: "t"},
			{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
		}
	} else {
		id := fmt.Sprintf("call-%d", i)
		parts = []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeToolInputStart, ID: id, ToolCallName: tool},
			{Type: fantasy.StreamPartTypeToolInputDelta, ID: id, Delta: "{}"},
			{Type: fantasy.StreamPartTypeToolInputEnd, ID: id},
			{Type: fantasy.StreamPartTypeToolCall, ID: id, ToolCallName: tool, ToolCallInput: "{}"},
			{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls},
		}
	}
	return iter.Seq[fantasy.StreamPart](func(yield func(fantasy.StreamPart) bool) {
		for _, p := range parts {
			if !yield(p) {
				return
			}
		}
	}), nil
}

// TestHaltEndsTheLoop is a regression test for issue #147: a tool that
// returns ToolOutput.Halt must end the agent loop. The model must not get
// another step, so a tool it would call in that step must not run.
func TestHaltEndsTheLoop(t *testing.T) {
	var ran atomic.Int32
	halt := NewTool("halt", "Halt.", func(context.Context, struct{}) (ToolOutput, error) {
		return ToolOutput{Content: "halted", Halt: true, FinalValue: "v"}, nil
	})
	act := NewTool("act", "Side effect.", func(context.Context, struct{}) (ToolOutput, error) {
		ran.Add(1)
		return ToolOutput{Content: "done"}, nil
	})

	model := &scriptedToolModel{script: []string{"halt", "act"}}
	model.provider, model.model = "script", "m"
	factory := func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
		return &ProviderResult{Model: model}, nil
	}

	k := newProviderTestKit(t, &Options{
		Model:      "script/m",
		Providers:  map[string]ProviderFactory{"script": factory},
		ExtraTools: []Tool{halt, act},
	})

	res, err := k.PromptResult(context.Background(), "go")
	if err != nil {
		t.Fatalf("PromptResult: %v", err)
	}
	if res.HaltedByTool != "halt" {
		t.Fatalf("HaltedByTool = %q, want %q", res.HaltedByTool, "halt")
	}
	if got, ok := res.FinalValue.(string); !ok || got != "v" {
		t.Fatalf("FinalValue = %#v, want %q", res.FinalValue, "v")
	}
	if n := ran.Load(); n != 0 {
		t.Fatalf("tool %q ran %d time(s) after Halt: the loop did not terminate", "act", n)
	}
	if n := model.callCount(); n != 1 {
		t.Fatalf("model called %d time(s), want 1", n)
	}
}
