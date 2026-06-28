package agent

import (
	"testing"

	"charm.land/fantasy"
)

// TestDedupeToolsByName verifies duplicate tool names are collapsed to the
// first occurrence while order is otherwise preserved.
func TestDedupeToolsByName(t *testing.T) {
	in := []fantasy.AgentTool{
		newTestTool("read"),
		newTestTool("echo__greet"),
		newTestTool("write"),
		newTestTool("echo__greet"), // duplicate (e.g. re-loaded MCP tool)
		newTestTool("read"),        // duplicate
	}

	out := dedupeToolsByName(in)

	if len(out) != 3 {
		t.Fatalf("expected 3 unique tools, got %d", len(out))
	}
	want := []string{"read", "echo__greet", "write"}
	for i, name := range want {
		if out[i].Info().Name != name {
			t.Errorf("position %d: expected %q, got %q", i, name, out[i].Info().Name)
		}
	}
}

// TestComposeAllTools_DedupesInheritedMCPTools reproduces the subagent
// scenario: a child agent inherits the parent's already-prefixed MCP tools as
// coreTools AND re-loads the same MCP tools (here modeled via extraTools).
// Without dedup the composed list would carry duplicate names and providers
// like Anthropic reject the turn ("every tool name must be unique").
func TestComposeAllTools_DedupesInheritedMCPTools(t *testing.T) {
	a := &Agent{
		// Inherited from parent: core tools + a prefixed MCP tool.
		coreTools: []fantasy.AgentTool{
			newTestTool("read"),
			newTestTool("echo__greet"),
		},
		// Re-loaded duplicate of the same prefixed MCP tool.
		extraTools: []fantasy.AgentTool{
			newTestTool("echo__greet"),
		},
	}

	tools := a.composeAllTools()

	seen := make(map[string]int)
	for _, tool := range tools {
		seen[tool.Info().Name]++
	}
	for name, n := range seen {
		if n > 1 {
			t.Errorf("tool %q appears %d times; expected unique", name, n)
		}
	}
	if seen["echo__greet"] != 1 {
		t.Errorf("expected echo__greet exactly once, got %d", seen["echo__greet"])
	}
}
