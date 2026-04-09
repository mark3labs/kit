package kit_test

import (
	"testing"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// TestMCPServerStatus_TypeSurface verifies the MCPServerStatus type is
// accessible and has the expected fields.
func TestMCPServerStatus_TypeSurface(t *testing.T) {
	s := kit.MCPServerStatus{
		Name:      "test-server",
		ToolCount: 5,
	}
	if s.Name != "test-server" {
		t.Errorf("Expected Name 'test-server', got %q", s.Name)
	}
	if s.ToolCount != 5 {
		t.Errorf("Expected ToolCount 5, got %d", s.ToolCount)
	}
}

// TestMCPServerConfig_ForDynamicAdd verifies that MCPServerConfig can be
// constructed with the expected fields for dynamic server management.
func TestMCPServerConfig_ForDynamicAdd(t *testing.T) {
	// Stdio server config.
	stdio := kit.MCPServerConfig{
		Command:     []string{"npx", "-y", "@modelcontextprotocol/server-github"},
		Environment: map[string]string{"GITHUB_TOKEN": "test-token"},
	}
	if len(stdio.Command) != 3 {
		t.Errorf("Expected 3 command parts, got %d", len(stdio.Command))
	}
	if stdio.Environment["GITHUB_TOKEN"] != "test-token" {
		t.Error("Expected GITHUB_TOKEN in environment")
	}

	// Remote server config.
	remote := kit.MCPServerConfig{
		URL:     "https://mcp.example.com/sse",
		Headers: []string{"Authorization: Bearer test"},
	}
	if remote.URL != "https://mcp.example.com/sse" {
		t.Errorf("Unexpected URL: %s", remote.URL)
	}

	// Config with tool filtering.
	filtered := kit.MCPServerConfig{
		Command:      []string{"some-server"},
		AllowedTools: []string{"read", "write"},
	}
	if len(filtered.AllowedTools) != 2 {
		t.Errorf("Expected 2 allowed tools, got %d", len(filtered.AllowedTools))
	}
}
