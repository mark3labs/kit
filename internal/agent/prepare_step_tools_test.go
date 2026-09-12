package agent

import (
	"testing"

	"charm.land/fantasy"
)

// applyPrepareStepUpdate mirrors the Phase-2 application in the PrepareStep
// closure in GenerateWithCallbacks. Keeping it in a helper lets the semantics
// be tested without standing up a provider; the closure and this helper must
// stay in step.
func applyPrepareStepUpdate(result *fantasy.PrepareStepResult, update *PrepareStepUpdate) {
	if update == nil {
		return
	}
	if update.Messages != nil {
		result.Messages = update.Messages
	}
	if update.ToolChoice != nil {
		result.ToolChoice = update.ToolChoice
	}
	if update.Tools != nil {
		result.Tools = update.Tools
	}
}

func liveTools() []fantasy.AgentTool {
	return []fantasy.AgentTool{
		fantasy.NewAgentTool[struct{}]("alpha", "", nil),
		fantasy.NewAgentTool[struct{}]("beta", "", nil),
	}
}

func toolNames(tools []fantasy.AgentTool) []string {
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		out = append(out, t.Info().Name)
	}
	return out
}

// TestPrepareStepUpdateNilToolsKeepsLiveSet is the default path: a hook that
// does not set Tools must leave the live tool set alone, so runtime AddTools
// and RemoveTools still reach the model mid-turn.
func TestPrepareStepUpdateNilToolsKeepsLiveSet(t *testing.T) {
	t.Parallel()

	result := fantasy.PrepareStepResult{Tools: liveTools()}
	applyPrepareStepUpdate(&result, &PrepareStepUpdate{})

	if got := toolNames(result.Tools); len(got) != 2 {
		t.Fatalf("tools = %v, want the live set untouched", got)
	}
}

// TestPrepareStepUpdateReplacesTools covers per-step filtering, the use case
// the PrepareStepHook godoc advertises.
func TestPrepareStepUpdateReplacesTools(t *testing.T) {
	t.Parallel()

	narrowed := []fantasy.AgentTool{fantasy.NewAgentTool[struct{}]("alpha", "", nil)}
	result := fantasy.PrepareStepResult{Tools: liveTools()}
	applyPrepareStepUpdate(&result, &PrepareStepUpdate{Tools: narrowed})

	got := toolNames(result.Tools)
	if len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("tools = %v, want [alpha]", got)
	}
}

// TestPrepareStepUpdateEmptyToolsDisablesTools guards the nil-versus-empty
// distinction. An empty non-nil slice must reach fantasy as an empty tool set,
// which forces a text response and lets an embedder end a turn deterministically.
// Collapsing empty to nil here would silently restore every tool.
func TestPrepareStepUpdateEmptyToolsDisablesTools(t *testing.T) {
	t.Parallel()

	result := fantasy.PrepareStepResult{Tools: liveTools()}
	applyPrepareStepUpdate(&result, &PrepareStepUpdate{Tools: []fantasy.AgentTool{}})

	if result.Tools == nil {
		t.Fatal("empty Tools collapsed to nil: fantasy would restore the full tool set")
	}
	if len(result.Tools) != 0 {
		t.Fatalf("tools = %v, want an empty set", toolNames(result.Tools))
	}
}

// TestPrepareStepUpdateFieldsAreIndependent proves one override does not
// disturb the others.
func TestPrepareStepUpdateFieldsAreIndependent(t *testing.T) {
	t.Parallel()

	choice := fantasy.ToolChoiceNone
	msgs := []fantasy.Message{fantasy.NewUserMessage("hi")}

	result := fantasy.PrepareStepResult{
		Tools:    liveTools(),
		Messages: []fantasy.Message{fantasy.NewUserMessage("original")},
	}
	applyPrepareStepUpdate(&result, &PrepareStepUpdate{
		Messages:   msgs,
		ToolChoice: &choice,
	})

	if len(result.Tools) != 2 {
		t.Fatalf("tools = %v, want untouched by a message-only update", toolNames(result.Tools))
	}
	if result.ToolChoice == nil || *result.ToolChoice != fantasy.ToolChoiceNone {
		t.Fatalf("ToolChoice = %v, want none", result.ToolChoice)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("messages = %d, want the replacement", len(result.Messages))
	}
}
