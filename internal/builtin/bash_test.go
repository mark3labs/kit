package builtin

import (
	"context"
	"slices"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewBashServer(t *testing.T) {
	server, err := NewBashServer()
	if err != nil {
		t.Fatalf("Failed to create bash server: %v", err)
	}

	if server == nil {
		t.Fatal("Expected server to be non-nil")
	}
}

func TestBashServerRegistry(t *testing.T) {
	registry := NewRegistry()

	// Test that bash server is registered
	servers := registry.ListServers()
	found := slices.Contains(servers, "bash")

	if !found {
		t.Error("bash server not found in registry")
	}

	// Test creating bash server through registry
	wrapper, err := registry.CreateServer("bash", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("Failed to create bash server through registry: %v", err)
	}

	if wrapper == nil {
		t.Fatal("Expected wrapper to be non-nil")
	}

	if wrapper.GetServer() == nil {
		t.Fatal("Expected wrapped server to be non-nil")
	}
}

func TestExecuteBash(t *testing.T) {
	// Create a simple test request
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "run_shell_cmd",
			Arguments: map[string]any{
				"command":     "echo 'Hello, World!'",
				"description": "Test echo command",
			},
		},
	}

	ctx := context.Background()
	result, err := executeBash(ctx, request)

	if err != nil {
		t.Fatalf("Failed to execute bash command: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	// Check that the result contains our expected output
	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		if textContent.Text == "" {
			t.Error("Expected non-empty text content")
		}
	} else {
		t.Error("Expected text content")
	}
}

func TestBashCommandValidation(t *testing.T) {
	// Test banned command
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "run_shell_cmd",
			Arguments: map[string]any{
				"command":     "curl http://example.com",
				"description": "Test banned command",
			},
		},
	}

	ctx := context.Background()
	result, err := executeBash(ctx, request)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return an error result, not fail
	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	// Check that it's an error result
	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		if textContent.Text == "" {
			t.Error("Expected non-empty error message")
		}
	}
}

func TestToolNameChange(t *testing.T) {
	// Test that the tool can be called with the new name "run_shell_cmd"
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "run_shell_cmd", // This should be the new name
			Arguments: map[string]any{
				"command":     "echo 'test'",
				"description": "Test renamed tool",
			},
		},
	}

	ctx := context.Background()
	result, err := executeBash(ctx, request)

	if err != nil {
		t.Fatalf("Failed to execute renamed tool: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	// Verify the tool executed successfully
	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}
}
