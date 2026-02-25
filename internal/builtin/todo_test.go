package builtin

import (
	"context"
	"slices"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewTodoServer(t *testing.T) {
	server, err := NewTodoServer()
	if err != nil {
		t.Fatalf("Failed to create todo server: %v", err)
	}

	if server == nil {
		t.Fatal("Expected server to be non-nil")
	}
}

func TestTodoServerRegistry(t *testing.T) {
	registry := NewRegistry()

	// Test that todo server is registered
	servers := registry.ListServers()
	found := slices.Contains(servers, "todo")

	if !found {
		t.Error("todo server not found in registry")
	}

	// Test creating todo server through registry
	wrapper, err := registry.CreateServer("todo", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("Failed to create todo server through registry: %v", err)
	}

	if wrapper == nil {
		t.Fatal("Expected wrapper to be non-nil")
	}

	if wrapper.GetServer() == nil {
		t.Fatal("Expected wrapped server to be non-nil")
	}
}

func TestTodoWrite(t *testing.T) {
	server := &TodoServer{
		todos: make([]TodoInfo, 0),
	}

	// Create a test request with valid todos
	todos := []TodoInfo{
		{
			Content:  "Test task 1",
			Status:   "pending",
			Priority: "high",
			ID:       "1",
		},
		{
			Content:  "Test task 2",
			Status:   "in_progress",
			Priority: "medium",
			ID:       "2",
		},
	}

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "todowrite",
			Arguments: map[string]any{
				"todos": todos,
			},
		},
	}

	ctx := context.Background()
	result, err := server.executeTodoWrite(ctx, request)

	if err != nil {
		t.Fatalf("Failed to execute todowrite: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	// Check that todos were stored
	storedTodos := server.getTodos()
	if len(storedTodos) != 2 {
		t.Errorf("Expected 2 todos, got %d", len(storedTodos))
	}

	// Verify the content is in readable format
	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		expectedOutput := "\n\n[ ] Test task 1\n[~] Test task 2"
		if textContent.Text != expectedOutput {
			t.Errorf("Expected formatted output:\n%s\nGot:\n%s", expectedOutput, textContent.Text)
		}
	} else {
		t.Error("Expected text content")
	}
}

func TestTodoRead(t *testing.T) {
	server := &TodoServer{
		todos: []TodoInfo{
			{
				Content:  "Existing task",
				Status:   "pending",
				Priority: "low",
				ID:       "existing-1",
			},
		},
	}

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "todoread",
		},
	}

	ctx := context.Background()
	result, err := server.executeTodoRead(ctx, request)

	if err != nil {
		t.Fatalf("Failed to execute todoread: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	// Verify the content is in readable format
	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		expectedOutput := "\n\n[ ] Existing task"
		if textContent.Text != expectedOutput {
			t.Errorf("Expected formatted output:\n%s\nGot:\n%s", expectedOutput, textContent.Text)
		}
	} else {
		t.Error("Expected text content")
	}
}

func TestTodoValidation(t *testing.T) {
	server := &TodoServer{
		todos: make([]TodoInfo, 0),
	}

	// Test invalid status
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "todowrite",
			Arguments: map[string]any{
				"todos": []TodoInfo{
					{
						Content:  "Test task",
						Status:   "invalid_status",
						Priority: "high",
						ID:       "1",
					},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := server.executeTodoWrite(ctx, request)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return an error result
	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		if textContent.Text == "" {
			t.Error("Expected non-empty error message")
		}
	}
}

func TestTodoEmptyContent(t *testing.T) {
	server := &TodoServer{
		todos: make([]TodoInfo, 0),
	}

	// Test empty content
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "todowrite",
			Arguments: map[string]any{
				"todos": []TodoInfo{
					{
						Content:  "",
						Status:   "pending",
						Priority: "high",
						ID:       "1",
					},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := server.executeTodoWrite(ctx, request)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return an error result
	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		if textContent.Text == "" {
			t.Error("Expected non-empty error message")
		}
	}
}

func TestTodoActiveCounting(t *testing.T) {
	server := &TodoServer{
		todos: make([]TodoInfo, 0),
	}

	// Create todos with different statuses
	todos := []TodoInfo{
		{Content: "Task 1", Status: "pending", Priority: "high", ID: "1"},
		{Content: "Task 2", Status: "in_progress", Priority: "medium", ID: "2"},
		{Content: "Task 3", Status: "completed", Priority: "low", ID: "3"},
		{Content: "Task 4", Status: "pending", Priority: "high", ID: "4"},
	}

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "todowrite",
			Arguments: map[string]any{
				"todos": todos,
			},
		},
	}

	ctx := context.Background()
	result, err := server.executeTodoWrite(ctx, request)

	if err != nil {
		t.Fatalf("Failed to execute todowrite: %v", err)
	}

	// Check that metadata contains todos
	if result.Meta == nil {
		t.Fatal("Expected metadata to be non-nil")
	}

	metaTodos, ok := result.Meta.AdditionalFields["todos"].([]TodoInfo)
	if !ok {
		t.Fatal("Expected todos in metadata")
	}

	if len(metaTodos) != 4 {
		t.Errorf("Expected 4 todos in metadata, got %d", len(metaTodos))
	}

	// Verify the content is in readable format
	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		expectedOutput := "\n\n[ ] Task 1\n[~] Task 2\n[X] Task 3\n[ ] Task 4"
		if textContent.Text != expectedOutput {
			t.Errorf("Expected formatted output:\n%s\nGot:\n%s", expectedOutput, textContent.Text)
		}
	} else {
		t.Error("Expected text content")
	}
}
