package test

import (
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
