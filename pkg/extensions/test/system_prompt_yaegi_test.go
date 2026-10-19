package test

import (
	"testing"

	"github.com/mark3labs/kit/pkg/extensions"
)

// TestGetSystemPromptCrossesYaegiBoundary verifies that an interpreted
// handler receives both values of ctx.GetSystemPrompt, as configured with
// MockContext.SetSystemPrompt.
func TestGetSystemPromptCrossesYaegiBoundary(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		text, source := ctx.GetSystemPrompt()
		ctx.Print(source + "|" + text)
	})
}
`

	harness := New(t)
	harness.Context().SetSystemPrompt("You are a test persona.", "/etc/kit/prompt.md")
	harness.LoadString(src, "sysprompt.go")

	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "t"}); err != nil {
		t.Fatalf("Emit(SessionStart) error = %v", err)
	}

	prints := harness.Context().GetPrints()
	if want := "/etc/kit/prompt.md|You are a test persona."; len(prints) != 1 || prints[0] != want {
		t.Errorf("prints = %q; want [%q]", prints, want)
	}
}
