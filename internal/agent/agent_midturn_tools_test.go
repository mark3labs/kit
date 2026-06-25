package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
)

// toolNamesFromStep extracts tool names from a PrepareStep result for assertions.
func toolNamesFromStep(tools []fantasy.AgentTool) map[string]struct{} {
	out := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		out[t.Info().Name] = struct{}{}
	}
	return out
}

// newTestTool builds a minimal AgentTool with the given name for use in tests.
func newTestTool(name string) fantasy.AgentTool {
	type emptyInput struct{}
	return fantasy.NewAgentTool(name, "test tool "+name,
		func(_ context.Context, _ emptyInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)
}

// midTurnToolAgent is a fake fantasy.Agent that simulates the multi-step
// agentic loop fantasy runs inside a single Stream call. Between step 0 and
// step 1 it invokes mutate(), emulating a tool handler that calls AddTools/
// RemoveTools mid-turn. It records the tool list each step's PrepareStep
// callback yields so the test can assert that mid-turn changes are observed.
type midTurnToolAgent struct {
	mutate     func()
	stepTools  []map[string]struct{}
	prepareErr error
}

func (f *midTurnToolAgent) Generate(_ context.Context, _ fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}

func (f *midTurnToolAgent) Stream(ctx context.Context, opts fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	// Step 0: capture the tools fantasy would use for the first step.
	for step := range 2 {
		if step == 1 && f.mutate != nil {
			// Emulate a tool handler that mutates the live tool set
			// mid-turn (e.g. enable_toolset calling host.AddTools).
			f.mutate()
		}
		if opts.PrepareStep != nil {
			_, prepared, err := opts.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{
				StepNumber: step,
				Model:      nil,
				Messages:   nil,
			})
			if err != nil {
				f.prepareErr = err
				return nil, err
			}
			f.stepTools = append(f.stepTools, toolNamesFromStep(prepared.Tools))
		}
	}
	return &fantasy.AgentResult{}, nil
}

// TestPrepareStepReflectsMidTurnTools is the regression test for #76.
// AddTools/RemoveTools must take effect at the next LLM step of the current
// turn. Because the whole agentic loop runs inside a single fantasy Stream
// call, this only works if Kit's PrepareStep callback re-reads the live tool
// set each step and populates PrepareStepResult.Tools. Before the fix the
// per-step tool list was the snapshot captured when Stream began, so a tool
// added mid-turn never appeared until the next turn.
func TestPrepareStepReflectsMidTurnTools(t *testing.T) {
	t.Parallel()

	core := newTestTool("read")
	loadMore := newTestTool("load_more")
	foo := newTestTool("foo")

	fake := &midTurnToolAgent{}

	a := &Agent{
		fantasyAgent:     fake,
		streamingEnabled: true,
		coreTools:        []fantasy.AgentTool{core},
		extraTools:       []fantasy.AgentTool{loadMore},
	}

	// Mid-turn, add a brand new tool "foo" (additive to load_more).
	fake.mutate = func() {
		a.SetExtraTools([]fantasy.AgentTool{loadMore, foo})
	}

	msgs := []fantasy.Message{fantasy.NewUserMessage("go")}
	if _, err := a.GenerateWithCallbacks(context.Background(), msgs, GenerateCallbacks{}); err != nil {
		t.Fatalf("GenerateWithCallbacks returned error: %v", err)
	}

	if len(fake.stepTools) != 2 {
		t.Fatalf("expected 2 prepared steps, got %d", len(fake.stepTools))
	}

	// Step 0: foo must NOT be present yet; load_more and read are.
	step0 := fake.stepTools[0]
	if _, ok := step0["read"]; !ok {
		t.Errorf("step 0: expected core tool 'read' to be present")
	}
	if _, ok := step0["load_more"]; !ok {
		t.Errorf("step 0: expected 'load_more' to be present")
	}
	if _, ok := step0["foo"]; ok {
		t.Errorf("step 0: 'foo' should not be present before it was added")
	}

	// Step 1: after the mid-turn AddTools, foo MUST be visible to the step.
	step1 := fake.stepTools[1]
	if _, ok := step1["foo"]; !ok {
		t.Errorf("step 1: expected mid-turn-added 'foo' to be present (regression #76)")
	}
	if _, ok := step1["read"]; !ok {
		t.Errorf("step 1: expected core tool 'read' to remain present")
	}
	if _, ok := step1["load_more"]; !ok {
		t.Errorf("step 1: expected 'load_more' to remain present")
	}
}

// TestPrepareStepReflectsMidTurnToolRemoval verifies the RemoveTools side of
// the contract: a tool removed mid-turn disappears from the next step.
func TestPrepareStepReflectsMidTurnToolRemoval(t *testing.T) {
	t.Parallel()

	core := newTestTool("read")
	loadMore := newTestTool("load_more")
	temp := newTestTool("temp")

	fake := &midTurnToolAgent{}

	a := &Agent{
		fantasyAgent:     fake,
		streamingEnabled: true,
		coreTools:        []fantasy.AgentTool{core},
		extraTools:       []fantasy.AgentTool{loadMore, temp},
	}

	// Mid-turn, remove "temp".
	fake.mutate = func() {
		a.SetExtraTools([]fantasy.AgentTool{loadMore})
	}

	msgs := []fantasy.Message{fantasy.NewUserMessage("go")}
	if _, err := a.GenerateWithCallbacks(context.Background(), msgs, GenerateCallbacks{}); err != nil {
		t.Fatalf("GenerateWithCallbacks returned error: %v", err)
	}

	if len(fake.stepTools) != 2 {
		t.Fatalf("expected 2 prepared steps, got %d", len(fake.stepTools))
	}

	if _, ok := fake.stepTools[0]["temp"]; !ok {
		t.Errorf("step 0: expected 'temp' to be present before removal")
	}
	if _, ok := fake.stepTools[1]["temp"]; ok {
		t.Errorf("step 1: 'temp' should be gone after mid-turn RemoveTools (regression #76)")
	}
}
