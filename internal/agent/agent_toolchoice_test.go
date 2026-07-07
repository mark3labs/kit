package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
)

// toolChoiceAgent is a fake fantasy.Agent that simulates a two-step agentic
// loop and records the ToolChoice each step's PrepareStep callback yields, so
// tests can assert per-step tool-choice overrides are threaded through.
type toolChoiceAgent struct {
	stepChoices []*fantasy.ToolChoice
}

func (f *toolChoiceAgent) Generate(_ context.Context, _ fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}

func (f *toolChoiceAgent) Stream(ctx context.Context, opts fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	for step := range 2 {
		if opts.PrepareStep != nil {
			_, prepared, err := opts.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{
				StepNumber: step,
				Model:      nil,
				Messages:   nil,
			})
			if err != nil {
				return nil, err
			}
			f.stepChoices = append(f.stepChoices, prepared.ToolChoice)
		}
	}
	return &fantasy.AgentResult{}, nil
}

// TestPrepareStepToolChoiceForcing verifies that a PrepareStepHandler can
// force a specific tool choice on one step and release it on the next. This
// is the pattern used to guarantee a capture/structured-output tool is
// invoked: force SpecificToolChoice until the call is observed, then flip
// back to nil so the turn can end.
func TestPrepareStepToolChoiceForcing(t *testing.T) {
	t.Parallel()

	fake := &toolChoiceAgent{}
	a := &Agent{
		fantasyAgent:     fake,
		streamingEnabled: true,
	}

	forced := fantasy.SpecificToolChoice("record_result")
	cb := GenerateCallbacks{
		OnPrepareStep: func(stepNumber int, messages []fantasy.Message) *PrepareStepUpdate {
			if stepNumber == 0 {
				return &PrepareStepUpdate{ToolChoice: &forced}
			}
			return nil
		},
	}

	msgs := []fantasy.Message{fantasy.NewUserMessage("go")}
	if _, err := a.GenerateWithCallbacks(context.Background(), msgs, cb); err != nil {
		t.Fatalf("GenerateWithCallbacks returned error: %v", err)
	}

	if len(fake.stepChoices) != 2 {
		t.Fatalf("expected 2 prepared steps, got %d", len(fake.stepChoices))
	}
	if fake.stepChoices[0] == nil {
		t.Fatal("step 0: expected forced ToolChoice, got nil")
	}
	if *fake.stepChoices[0] != forced {
		t.Errorf("step 0: expected ToolChoice %q, got %q", forced, *fake.stepChoices[0])
	}
	if fake.stepChoices[1] != nil {
		t.Errorf("step 1: expected nil ToolChoice after release, got %q", *fake.stepChoices[1])
	}
}

// TestPrepareStepToolChoiceWithoutHandler verifies that when no
// PrepareStepHandler is registered, no ToolChoice override is sent — fantasy
// falls back to its default (auto).
func TestPrepareStepToolChoiceWithoutHandler(t *testing.T) {
	t.Parallel()

	fake := &toolChoiceAgent{}
	a := &Agent{
		fantasyAgent:     fake,
		streamingEnabled: true,
	}

	msgs := []fantasy.Message{fantasy.NewUserMessage("go")}
	if _, err := a.GenerateWithCallbacks(context.Background(), msgs, GenerateCallbacks{}); err != nil {
		t.Fatalf("GenerateWithCallbacks returned error: %v", err)
	}

	for i, tc := range fake.stepChoices {
		if tc != nil {
			t.Errorf("step %d: expected nil ToolChoice without handler, got %q", i, *tc)
		}
	}
}
