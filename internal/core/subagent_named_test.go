package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
)

func TestSubagentDescription_NoAgents(t *testing.T) {
	if got := subagentDescription(nil); got != subagentBaseDescription {
		t.Errorf("description without agents should equal the base description")
	}
}

func TestSubagentDescription_WithAgents(t *testing.T) {
	desc := subagentDescription([]NamedAgentSpec{
		{Name: "explore", Description: "Read-only exploration", Tools: []string{"read", "grep"}},
		{Name: "general", Description: "General-purpose agent"},
	})

	if !strings.HasPrefix(desc, subagentBaseDescription) {
		t.Error("description should start with the base description")
	}
	if !strings.Contains(desc, "Available named agents") {
		t.Error("description should advertise named agents")
	}
	if !strings.Contains(desc, "- explore: Read-only exploration (tools: read, grep)") {
		t.Errorf("missing explore line in:\n%s", desc)
	}
	if !strings.Contains(desc, "- general: General-purpose agent (tools: all tools)") {
		t.Errorf("missing general line in:\n%s", desc)
	}
}

func TestNewSubagentTool_AdvertisesNamedAgents(t *testing.T) {
	tool := NewSubagentTool(WithNamedAgents(NamedAgentSpec{
		Name: "reviewer", Description: "Reviews code", Tools: []string{"read"},
	}))

	info := tool.Info()
	if !strings.Contains(info.Description, "- reviewer: Reviews code (tools: read)") {
		t.Errorf("tool description missing named agent:\n%s", info.Description)
	}
	if _, ok := info.Parameters["agent"]; !ok {
		t.Error("subagent tool should expose an 'agent' parameter")
	}
}

func TestNewSubagentTool_DefaultHasAgentParam(t *testing.T) {
	info := NewSubagentTool().Info()
	if info.Description != subagentBaseDescription {
		t.Error("default tool description should be the base description")
	}
	if _, ok := info.Parameters["agent"]; !ok {
		t.Error("subagent tool should expose an 'agent' parameter even without named agents")
	}
}

func TestExecuteSubagent_PassesAgentToSpawner(t *testing.T) {
	var captured SubagentSpawnRequest
	spawner := SubagentSpawnFunc(func(ctx context.Context, req SubagentSpawnRequest) (*SubagentSpawnResult, error) {
		captured = req
		return &SubagentSpawnResult{Response: "done"}, nil
	})
	ctx := WithSubagentSpawner(context.Background(), spawner)

	resp, err := executeSubagent(ctx, fantasy.ToolCall{
		ID:    "tc42",
		Input: `{"task":"look around","agent":"explore","timeout_seconds":60}`,
	})
	if err != nil {
		t.Fatalf("executeSubagent failed: %v", err)
	}
	if resp.IsError {
		t.Fatalf("unexpected error response: %s", resp.Content)
	}

	if captured.ToolCallID != "tc42" {
		t.Errorf("ToolCallID = %q, want tc42", captured.ToolCallID)
	}
	if captured.Prompt != "look around" {
		t.Errorf("Prompt = %q", captured.Prompt)
	}
	if captured.Agent != "explore" {
		t.Errorf("Agent = %q, want explore", captured.Agent)
	}
	if captured.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s", captured.Timeout)
	}
}

func TestExecuteSubagent_UnsetTimeoutIsZero(t *testing.T) {
	// A zero timeout signals "unset" so downstream resolution can apply a
	// named agent's preset timeout or the default.
	var captured SubagentSpawnRequest
	spawner := SubagentSpawnFunc(func(ctx context.Context, req SubagentSpawnRequest) (*SubagentSpawnResult, error) {
		captured = req
		return &SubagentSpawnResult{Response: "done"}, nil
	})
	ctx := WithSubagentSpawner(context.Background(), spawner)

	if _, err := executeSubagent(ctx, fantasy.ToolCall{ID: "tc1", Input: `{"task":"t"}`}); err != nil {
		t.Fatalf("executeSubagent failed: %v", err)
	}
	if captured.Timeout != 0 {
		t.Errorf("Timeout = %v, want 0 (unset)", captured.Timeout)
	}
	if captured.Agent != "" {
		t.Errorf("Agent = %q, want empty", captured.Agent)
	}
}
