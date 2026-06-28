package kit

import (
	"context"
	"testing"
)

// fakeNamedTool builds a minimal Tool with the given name for unit tests that
// only inspect tool names.
func fakeNamedTool(name string) Tool {
	return NewRawTool(name, "test tool "+name, nil,
		func(_ context.Context, _ map[string]any) (ToolOutput, error) {
			return ToolOutput{}, nil
		})
}

func TestToolsIncludeMCP(t *testing.T) {
	tests := []struct {
		name     string
		tools    []Tool
		mcpNames []string
		want     bool
	}{
		{
			name:     "inherited tools contain an MCP tool",
			tools:    []Tool{fakeNamedTool("read"), fakeNamedTool("echo__greet")},
			mcpNames: []string{"echo__greet", "echo__echo"},
			want:     true,
		},
		{
			name:     "core-only tools, MCP loaded on parent",
			tools:    []Tool{fakeNamedTool("read"), fakeNamedTool("write")},
			mcpNames: []string{"echo__greet"},
			want:     false,
		},
		{
			name:     "no MCP tools on parent",
			tools:    []Tool{fakeNamedTool("read"), fakeNamedTool("echo__greet")},
			mcpNames: nil,
			want:     false,
		},
		{
			name:     "empty tool set",
			tools:    nil,
			mcpNames: []string{"echo__greet"},
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := toolsIncludeMCP(tc.tools, tc.mcpNames); got != tc.want {
				t.Errorf("toolsIncludeMCP() = %v, want %v", got, tc.want)
			}
		})
	}
}
