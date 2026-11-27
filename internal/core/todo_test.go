package core

import (
	"context"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
)

func runTodo(t *testing.T, tool fantasy.AgentTool, ctx context.Context, input string) fantasy.ToolResponse {
	t.Helper()
	resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "call_todo", Name: "todo", Input: input})
	if err != nil {
		t.Fatalf("unexpected error running todo tool: %v", err)
	}
	return resp
}

func TestTodoTool_EmptyRead(t *testing.T) {
	tool := NewTodoTool()
	resp := runTodo(t, tool, context.Background(), `{}`)
	if resp.IsError {
		t.Fatalf("expected success, got error: %s", resp.Content)
	}
	if !strings.Contains(resp.Content, "No todos found") {
		t.Errorf("expected 'No todos found', got %q", resp.Content)
	}
}

func TestTodoTool_WriteAndRead(t *testing.T) {
	tool := NewTodoTool()
	ctx := context.Background()

	input := `{
		"todos": [
			{"id": "1", "content": "Explore codebase", "status": "completed", "priority": "high"},
			{"id": "2", "content": "Implement core tool", "status": "in_progress", "priority": "medium"},
			{"id": "3", "content": "Write unit tests", "status": "pending", "priority": "low"}
		]
	}`

	resp := runTodo(t, tool, ctx, input)
	if resp.IsError {
		t.Fatalf("expected success, got error: %s", resp.Content)
	}

	expectedParts := []string{
		"Todos (1/3 completed, 1 in progress):",
		"[x] 1: Explore codebase [high]",
		"[~] 2: Implement core tool",
		"[ ] 3: Write unit tests [low]",
	}
	for _, part := range expectedParts {
		if !strings.Contains(resp.Content, part) {
			t.Errorf("expected response to contain %q, got:\n%s", part, resp.Content)
		}
	}

	// Read back with empty input
	readResp := runTodo(t, tool, ctx, `{}`)
	if readResp.IsError {
		t.Fatalf("read failed: %s", readResp.Content)
	}
	if readResp.Content != resp.Content {
		t.Errorf("read response does not match write response.\nGot:\n%s\nWant:\n%s", readResp.Content, resp.Content)
	}

	// Read back with explicit action: read
	readResp2 := runTodo(t, tool, ctx, `{"action":"read"}`)
	if readResp2.Content != resp.Content {
		t.Errorf("action:read response does not match.\nGot:\n%s\nWant:\n%s", readResp2.Content, resp.Content)
	}
}

func TestTodoTool_ClearTodos(t *testing.T) {
	tool := NewTodoTool()
	ctx := context.Background()

	// Add a todo
	_ = runTodo(t, tool, ctx, `{"todos":[{"content":"step 1","status":"pending"}]}`)

	// Clear todos
	clearResp := runTodo(t, tool, ctx, `{"todos":[]}`)
	if clearResp.IsError {
		t.Fatalf("clear failed: %s", clearResp.Content)
	}
	if !strings.Contains(clearResp.Content, "Todos cleared") {
		t.Errorf("expected 'Todos cleared', got %q", clearResp.Content)
	}

	// Verify empty read
	readResp := runTodo(t, tool, ctx, `{}`)
	if !strings.Contains(readResp.Content, "No todos found") {
		t.Errorf("expected 'No todos found', got %q", readResp.Content)
	}
}

func TestTodoTool_ValidationAndNormalization(t *testing.T) {
	tool := NewTodoTool()
	ctx := context.Background()

	tests := []struct {
		name      string
		input     string
		wantError bool
		contains  string
	}{
		{
			name:      "empty content",
			input:     `{"todos":[{"content":"","status":"pending"}]}`,
			wantError: true,
			contains:  "content cannot be empty",
		},
		{
			name:      "invalid status",
			input:     `{"todos":[{"content":"task","status":"unknown"}]}`,
			wantError: true,
			contains:  "invalid status",
		},
		{
			name:      "action write without todos",
			input:     `{"action":"write"}`,
			wantError: true,
			contains:  "action 'write' requires 'todos' list",
		},
		{
			name:      "normalize done and in-progress, auto-assign ID",
			input:     `{"todos":[{"content":"task 1","status":"done"},{"content":"task 2","status":"in-progress"}]}`,
			wantError: false,
			contains:  "[x] 1: task 1\n[~] 2: task 2",
		},
		{
			name:      "cancelled status",
			input:     `{"todos":[{"id":"1","content":"cancelled task","status":"cancelled"}]}`,
			wantError: false,
			contains:  "[-] 1: cancelled task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := runTodo(t, tool, ctx, tt.input)
			if tt.wantError {
				if !resp.IsError {
					t.Fatalf("expected error response, got success: %s", resp.Content)
				}
				if !strings.Contains(resp.Content, tt.contains) {
					t.Errorf("expected error to contain %q, got %q", tt.contains, resp.Content)
				}
			} else {
				if resp.IsError {
					t.Fatalf("unexpected error: %s", resp.Content)
				}
				if !strings.Contains(resp.Content, tt.contains) {
					t.Errorf("expected response to contain %q, got:\n%s", tt.contains, resp.Content)
				}
			}
		})
	}
}

func TestTodoTool_ContextStoreOverride(t *testing.T) {
	defaultStore := NewMemoryTodoStore()
	_ = defaultStore.SaveTodos(context.Background(), []TodoItem{
		{ID: "default", Content: "from default store", Status: TodoStatusPending},
	})

	customStore := NewMemoryTodoStore()
	_ = customStore.SaveTodos(context.Background(), []TodoItem{
		{ID: "custom", Content: "from custom store", Status: TodoStatusCompleted},
	})

	tool := NewTodoTool(WithTodoStore(defaultStore))

	// Without context override: reads default store
	resp1 := runTodo(t, tool, context.Background(), `{}`)
	if !strings.Contains(resp1.Content, "from default store") {
		t.Errorf("expected default store content, got %s", resp1.Content)
	}

	// With context override: reads custom store
	ctxWithCustom := ContextWithTodoStore(context.Background(), customStore)
	resp2 := runTodo(t, tool, ctxWithCustom, `{}`)
	if !strings.Contains(resp2.Content, "from custom store") {
		t.Errorf("expected custom store content, got %s", resp2.Content)
	}
}

func TestMemoryTodoStore_Concurrency(t *testing.T) {
	store := NewMemoryTodoStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			_ = store.SaveTodos(ctx, []TodoItem{
				{ID: "1", Content: "concurrent task", Status: TodoStatusInProgress},
			})
		}(i)
		go func() {
			defer wg.Done()
			_, _ = store.GetTodos(ctx)
		}()
	}
	wg.Wait()
}
