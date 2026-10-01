package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/codemode"
	"github.com/mark3labs/kit/internal/core"
)

// codeModeStreamAgent is a fake fantasy agent. On its single step it records
// the tools PrepareStep yields and runs the code mode tool with script.
type codeModeStreamAgent struct {
	script    string
	stepTools map[string]struct{}
	resp      fantasy.ToolResponse
	runErr    error
}

func (f *codeModeStreamAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}

func (f *codeModeStreamAgent) Stream(ctx context.Context, opts fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	_, prepared, err := opts.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{StepNumber: 0})
	if err != nil {
		return nil, err
	}
	f.stepTools = toolNamesFromStep(prepared.Tools)
	for _, t := range prepared.Tools {
		if t.Info().Name == core.CodeModeToolName {
			input, _ := json.Marshal(map[string]string{"code": f.script})
			f.resp, f.runErr = t.Run(ctx, fantasy.ToolCall{ID: "call_1", Name: t.Info().Name, Input: string(input)})
		}
	}
	return &fantasy.AgentResult{}, nil
}

// TestCodeModeHidesScriptOnlyToolsAndCallsThemThroughWrappers checks the
// agent integration end to end: a tool with codemode exposure is removed
// from the model's tool list, the code mode tool can still call it, and the
// call passes through the agent's tool wrapper (where extension and SDK
// hooks live).
func TestCodeModeHidesScriptOnlyToolsAndCallsThemThroughWrappers(t *testing.T) {
	t.Parallel()

	cm := core.NewCodeModeTool(core.WithCodeMode(core.CodeModeConfig{
		Policy: codemode.Policy{Rules: []codemode.ExposureRule{{Pattern: "github__*", Exposure: codemode.ExposureCodeMode}}},
	}))
	var wrapped atomic.Int32
	wrapper := func(tools []fantasy.AgentTool) []fantasy.AgentTool {
		out := make([]fantasy.AgentTool, len(tools))
		for i, t := range tools {
			out[i] = &countingTool{AgentTool: t, n: &wrapped}
		}
		return out
	}

	fake := &codeModeStreamAgent{script: `
const a = await tools.github.list_issues({})
const b = await tools.read({})
return a + "+" + b`}
	a := &Agent{
		fantasyAgent:     fake,
		streamingEnabled: true,
		coreTools:        []fantasy.AgentTool{newTestTool("read"), cm},
		extraTools:       []fantasy.AgentTool{newTestTool("github__list_issues")},
		toolWrapper:      wrapper,
	}

	msgs := []fantasy.Message{fantasy.NewUserMessage("go")}
	if _, err := a.GenerateWithCallbacks(context.Background(), msgs, GenerateCallbacks{}); err != nil {
		t.Fatalf("GenerateWithCallbacks: %v", err)
	}

	if _, ok := fake.stepTools["github__list_issues"]; ok {
		t.Error("script-only tool was sent to the model")
	}
	for _, name := range []string{"read", core.CodeModeToolName} {
		if _, ok := fake.stepTools[name]; !ok {
			t.Errorf("tool %q missing from the model's tool list", name)
		}
	}
	if fake.runErr != nil || fake.resp.IsError {
		t.Fatalf("code mode run failed: err=%v resp=%s", fake.runErr, fake.resp.Content)
	}
	if !strings.Contains(fake.resp.Content, "ok+ok") {
		t.Fatalf("unexpected result: %s", fake.resp.Content)
	}
	// One wrapped run for the code mode tool itself, plus the two nested
	// calls.
	if got := wrapped.Load(); got != 3 {
		t.Fatalf("wrapped runs = %d, want 3 (nested calls must use the wrapped tools)", got)
	}
	// The description lists the hidden tool with its full signature.
	if desc := cm.Info().Description; !strings.Contains(desc, "tools.github.list_issues(args:") {
		t.Fatalf("catalog missing hidden tool:\n%s", desc)
	}
}

type countingTool struct {
	fantasy.AgentTool
	n *atomic.Int32
}

func (c *countingTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	c.n.Add(1)
	return c.AgentTool.Run(ctx, call)
}

// TestComposeModelToolsWithoutObserver keeps the default path untouched.
func TestComposeModelToolsWithoutObserver(t *testing.T) {
	t.Parallel()
	a := &Agent{coreTools: []fantasy.AgentTool{newTestTool("read"), newTestTool("github__x")}}
	if got := len(a.composeModelTools()); got != 2 {
		t.Fatalf("tools = %d, want 2", got)
	}
}
