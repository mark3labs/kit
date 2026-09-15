package extensions_test

import (
	"testing"

	"github.com/mark3labs/kit/pkg/extensions"
	"github.com/mark3labs/kit/pkg/extensions/test"
)

// TestPublicTypesUsableFromHarness verifies that an external consumer can
// construct events and inspect results using only the public pkg/extensions
// surface — no internal/ import required.
func TestPublicTypesUsableFromHarness(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if tc.ToolName == "blocked" {
			return &ext.ToolCallResult{Block: true, Reason: "nope"}
		}
		return nil
	})
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.Print("started")
	})
}
`
	harness := test.New(t)
	loaded := harness.LoadString(src, "public-api.go")
	if loaded == nil {
		t.Fatal("expected loaded extension")
	}
	if loaded.Path != "public-api.go" {
		t.Fatalf("loaded path = %q, want public-api.go", loaded.Path)
	}

	if !harness.HasHandlers(extensions.ToolCall) {
		t.Fatal("expected ToolCall handlers")
	}
	if !harness.HasHandlers(extensions.SessionStart) {
		t.Fatal("expected SessionStart handlers")
	}
	test.AssertHasHandlers(t, harness, extensions.ToolCall)

	var event extensions.Event = extensions.SessionStartEvent{SessionID: "s1"}
	if event.Type() != extensions.SessionStart {
		t.Fatalf("event type = %q, want %q", event.Type(), extensions.SessionStart)
	}

	result, err := harness.Emit(event)
	if err != nil {
		t.Fatalf("Emit(SessionStartEvent): %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil SessionStart result, got %#v", result)
	}
	test.AssertPrinted(t, harness, "started")

	var tc extensions.Event = extensions.ToolCallEvent{
		ToolName: "blocked",
		Input:    `{"x":1}`,
	}
	result, err = harness.Emit(tc)
	if err != nil {
		t.Fatalf("Emit(ToolCallEvent): %v", err)
	}
	test.AssertBlocked(t, result, "nope")

	tcr := test.GetToolCallResult(result)
	if tcr == nil || !tcr.Block || tcr.Reason != "nope" {
		t.Fatalf("GetToolCallResult = %#v", tcr)
	}

	jsonResult, err := harness.EmitJSON("allowed", `{}`)
	if err != nil {
		t.Fatalf("EmitJSON: %v", err)
	}
	if jsonResult != nil {
		t.Fatalf("expected nil EmitJSON result, got %#v", jsonResult)
	}

	if harness.Runner() == nil {
		t.Fatal("expected non-nil Runner")
	}

	all := extensions.AllEventTypes()
	if len(all) == 0 {
		t.Fatal("AllEventTypes returned empty")
	}
	if !extensions.ToolCall.IsValid() {
		t.Fatal("ToolCall should be a valid EventType")
	}
}

// TestPublicTypeAliasesShareIdentity ensures the public aliases are the same
// underlying types the harness returns, so type assertions in external tests
// succeed without converting through internal packages.
func TestPublicTypeAliasesShareIdentity(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnToolCall(func(tc ext.ToolCallEvent, _ ext.Context) *ext.ToolCallResult {
		return &ext.ToolCallResult{Block: true, Reason: "x"}
	})
	api.RegisterTool(ext.ToolDef{Name: "t", Description: "d", Parameters: "{}"})
	api.RegisterCommand(ext.CommandDef{Name: "c", Description: "d"})
}
`
	harness := test.New(t)
	ext := harness.LoadString(src, "alias.go")

	var _ *extensions.LoadedExtension = ext
	var _ *extensions.Runner = harness.Runner()
	var _ []extensions.ToolDef = harness.RegisteredTools()
	var _ []extensions.CommandDef = harness.RegisteredCommands()

	result, err := harness.Emit(extensions.ToolCallEvent{ToolName: "t", Input: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	var _ extensions.Result = result
	if _, ok := result.(extensions.ToolCallResult); !ok {
		t.Fatalf("result type %T is not extensions.ToolCallResult", result)
	}
}
