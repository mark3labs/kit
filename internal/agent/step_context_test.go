package agent

import (
	"context"
	"fmt"
	"testing"

	"charm.land/fantasy"
)

func TestStepContextIsolationAndResume(t *testing.T) {
	type key int
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var steps []int
	var contexts []context.Context
	calls := 0
	model := &retryTestModel{stream: func(ctx context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
		calls++
		contexts = append(contexts, ctx)
		want := 0
		if calls > 1 {
			want = 1
		}
		if ctx.Value(key(0)) != want {
			t.Errorf("call %d context step = %v", calls, ctx.Value(key(0)))
		}
		switch calls {
		case 1:
			return toolStep("one"), nil
		case 2:
			return partialFailure("partial", overloadError()), nil
		default:
			return answerStep("done"), nil
		}
	}}
	tool := fantasy.NewAgentTool("work", "test", func(ctx context.Context, _ struct{}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
		if ctx.Value(key(0)) != 0 {
			t.Errorf("tool context = %v", ctx.Value(key(0)))
		}
		return fantasy.NewTextResponse("ok"), nil
	})
	a := &Agent{model: model, retryPolicy: resumeTestPolicy(1), coreTools: []fantasy.AgentTool{tool}, streamingEnabled: true}
	a.rebuildFantasyAgent()
	_, err := a.GenerateWithCallbacks(ctx, []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{
		OnStepContext: func(base context.Context, step int, model string) context.Context {
			if base.Value(key(0)) != nil {
				t.Error("step inherited previous decoration")
			}
			steps = append(steps, step)
			return context.WithValue(base, key(0), step)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(steps) != "[0 1 1]" {
		t.Fatalf("steps = %v", steps)
	}
	cancel()
	for _, ctx := range contexts {
		if ctx.Err() != context.Canceled {
			t.Errorf("decorated context not canceled: %v", ctx.Err())
		}
	}
}
