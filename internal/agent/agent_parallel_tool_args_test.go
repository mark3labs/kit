package agent

import (
	"context"
	"sync"
	"testing"

	"charm.land/fantasy"
)

// fakeParallelAgent simulates a provider that emits two parallel tool_use
// blocks in a single step. It invokes the streaming callbacks in the order:
//
//	OnToolCall(A) -> OnToolCall(B) -> OnToolResult(A) -> OnToolResult(B)
//
// Before the fix in #33 the agent-layer wrapper recorded a single
// `currentToolArgs` variable that was clobbered by the second OnToolCall, so
// both OnToolResult callbacks received B's args instead of their own.
type fakeParallelAgent struct {
	calls   []fantasy.ToolCallContent
	results []fantasy.ToolResultContent
}

func (f *fakeParallelAgent) Generate(_ context.Context, _ fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}

func (f *fakeParallelAgent) Stream(_ context.Context, opts fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	for _, tc := range f.calls {
		if opts.OnToolCall != nil {
			if err := opts.OnToolCall(tc); err != nil {
				return nil, err
			}
		}
	}
	for _, tr := range f.results {
		if opts.OnToolResult != nil {
			if err := opts.OnToolResult(tr); err != nil {
				return nil, err
			}
		}
	}
	return &fantasy.AgentResult{}, nil
}

// TestGenerateWithCallbacks_ParallelToolArgs is the regression test for #33.
// It drives the streaming-callback wiring inside GenerateWithCallbacks with a
// fake fantasy.Agent that emits two parallel tool calls before either result.
// Each OnToolResult must receive the args of its own tool call (matched by
// ToolCallID), not the args of the last OnToolCall in the step.
func TestGenerateWithCallbacks_ParallelToolArgs(t *testing.T) {
	t.Parallel()

	argsA := `{"name":"scheduled_jobs"}`
	argsB := `{"name":"gmail_trigger"}`

	fake := &fakeParallelAgent{
		calls: []fantasy.ToolCallContent{
			{ToolCallID: "kit-A", ToolName: "load_skill", Input: argsA},
			{ToolCallID: "kit-B", ToolName: "load_skill", Input: argsB},
		},
		results: []fantasy.ToolResultContent{
			{ToolCallID: "kit-A", ToolName: "load_skill", Result: fantasy.ToolResultOutputContentText{Text: "ok-A"}},
			{ToolCallID: "kit-B", ToolName: "load_skill", Result: fantasy.ToolResultOutputContentText{Text: "ok-B"}},
		},
	}

	a := &Agent{
		fantasyAgent:     fake,
		streamingEnabled: false, // exercise the "hasCallbacks" branch
	}

	var mu sync.Mutex
	resultArgs := map[string]string{}
	executionArgs := map[string]string{} // captured when running == false

	cb := GenerateCallbacks{
		OnToolExecution: func(id, _, args string, running bool) {
			if running {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			executionArgs[id] = args
		},
		OnToolResult: func(id, _, args, _, _ string, _ bool) {
			mu.Lock()
			defer mu.Unlock()
			resultArgs[id] = args
		},
	}

	if _, err := a.GenerateWithCallbacks(context.Background(), nil, cb); err != nil {
		t.Fatalf("GenerateWithCallbacks returned error: %v", err)
	}

	if got, want := resultArgs["kit-A"], argsA; got != want {
		t.Errorf("OnToolResult for kit-A: args = %q, want %q", got, want)
	}
	if got, want := resultArgs["kit-B"], argsB; got != want {
		t.Errorf("OnToolResult for kit-B: args = %q, want %q", got, want)
	}
	if got, want := executionArgs["kit-A"], argsA; got != want {
		t.Errorf("OnToolExecution(finish) for kit-A: args = %q, want %q", got, want)
	}
	if got, want := executionArgs["kit-B"], argsB; got != want {
		t.Errorf("OnToolExecution(finish) for kit-B: args = %q, want %q", got, want)
	}
}
