package test

import (
	"errors"
	"testing"

	"github.com/mark3labs/kit/pkg/extensions"
)

// TestPermissionGateExample checks that the permission-gate example asks
// before a dangerous command under both names of the shell tool, blocks the
// command when the user says no, and does not ask for other commands.
func TestPermissionGateExample(t *testing.T) {
	for _, tool := range []string{"shell", "bash"} {
		t.Run(tool, func(t *testing.T) {
			harness := New(t)
			harness.LoadFile("../../../examples/extensions/permission-gate.go")
			dangerous := extensions.ToolCallEvent{ToolName: tool, Input: `{"command":"rm -rf /tmp/kit-example"}`}

			// The mock context answers no to a prompt.
			result, err := harness.Emit(dangerous)
			if err != nil {
				t.Fatal(err)
			}
			AssertBlocked(t, result, "User denied execution of dangerous command: rm -rf /tmp/kit-example")

			harness.Context().PromptConfirmResult.Value = true
			result, err = harness.Emit(dangerous)
			if err != nil {
				t.Fatal(err)
			}
			AssertNotBlocked(t, result)

			// With the answer set back to no, a prompt would block the call.
			harness.Context().PromptConfirmResult.Value = false
			result, err = harness.Emit(extensions.ToolCallEvent{ToolName: tool, Input: `{"command":"ls -l"}`})
			if err != nil {
				t.Fatal(err)
			}
			AssertNotBlocked(t, result)
		})
	}
}

// TestProtectedPathsExample checks that the protected-paths example blocks
// the write, edit and shell tools on a protected path, and lets other calls
// through.
func TestProtectedPathsExample(t *testing.T) {
	harness := New(t)
	harness.LoadFile("../../../examples/extensions/protected-paths.go")

	for _, call := range []extensions.ToolCallEvent{
		{ToolName: "write", Input: `{"path":".env","content":"x"}`},
		{ToolName: "edit", Input: `{"path":"secrets/token","edits":[{"old_text":"a","new_text":"b"}]}`},
		{ToolName: "shell", Input: `{"command":"echo x > .env"}`},
		{ToolName: "bash", Input: `{"command":"rm .git/config"}`},
	} {
		result, err := harness.Emit(call)
		if err != nil {
			t.Fatal(err)
		}
		if tcr, ok := result.(extensions.ToolCallResult); !ok || !tcr.Block {
			t.Errorf("%s %s: result = %+v; want it blocked", call.ToolName, call.Input, result)
		}
	}

	for _, call := range []extensions.ToolCallEvent{
		{ToolName: "write", Input: `{"path":"main.go","content":"x"}`},
		{ToolName: "shell", Input: `{"command":"cat .env"}`},
		{ToolName: "read", Input: `{"path":".env"}`},
	} {
		result, err := harness.Emit(call)
		if err != nil {
			t.Fatal(err)
		}
		if tcr, ok := result.(extensions.ToolCallResult); ok && tcr.Block {
			t.Errorf("%s %s: blocked with %q; want it allowed", call.ToolName, call.Input, tcr.Reason)
		}
	}
}

// TestDecideGateExample checks that the decide-gate example lets a command
// through when the decision model rates it safe, asks for a command rated
// destructive, and asks for every command when no decision-model is set.
func TestDecideGateExample(t *testing.T) {
	harness := New(t)
	harness.LoadFile("../../../examples/extensions/decide-gate.go")
	mock := harness.Context()
	call := extensions.ToolCallEvent{ToolName: "shell", Input: `{"command":"rm -rf build"}`}

	// No decision-model: the mock prompt answers no, so the call is blocked.
	result, err := harness.Emit(call)
	if err != nil {
		t.Fatal(err)
	}
	AssertBlocked(t, result, "User denied execution of command: rm -rf build")

	mock.DecisionModel = "openrouter/typesafe/jev-1.14"
	mock.DecideResult = extensions.DecisionResult{Answers: []extensions.DecisionAnswer{
		{ID: "destructive", Type: extensions.DecisionBoolean, Probability: 0.1},
	}}
	result, err = harness.Emit(call)
	if err != nil {
		t.Fatal(err)
	}
	AssertNotBlocked(t, result)

	mock.DecideResult.Answers[0].Probability = 0.9
	result, err = harness.Emit(call)
	if err != nil {
		t.Fatal(err)
	}
	AssertBlocked(t, result, "User denied execution of command: rm -rf build")

	// Fail closed: a missing answer, a wrong answer type or a failed call
	// asks the user, and the mock prompt answers no.
	unusable := []struct {
		name   string
		result extensions.DecisionResult
		err    error
	}{
		{name: "missing answer"},
		{name: "wrong type", result: extensions.DecisionResult{Answers: []extensions.DecisionAnswer{
			{ID: "destructive", Type: extensions.DecisionScore},
		}}},
		{name: "call failed", err: errors.New("provider unavailable")},
	}
	for _, c := range unusable {
		mock.DecideResult, mock.DecideErr = c.result, c.err
		result, err = harness.Emit(call)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		AssertBlocked(t, result, "User denied execution of command: rm -rf build")
	}

	if n := len(mock.DecideRequests); n != 5 {
		t.Fatalf("got %d Decide requests, want 5", n)
	}
	if got := mock.DecideRequests[0].State.Text; got != "rm -rf build" {
		t.Errorf("State.Text = %q", got)
	}
}
