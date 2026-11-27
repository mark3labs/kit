package kit

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/core"
	"github.com/mark3labs/kit/internal/session"
)

func runKitTodo(t *testing.T, tool Tool, ctx context.Context, input string) fantasy.ToolResponse {
	t.Helper()
	resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "call_todo", Name: "todo", Input: input})
	if err != nil {
		t.Fatalf("unexpected error running todo tool: %v", err)
	}
	return resp
}

func TestSessionTodoStore_SaveAndGet(t *testing.T) {
	tm := session.InMemoryTreeSession(t.TempDir())
	sm := NewTreeManagerAdapter(tm)
	store := newSessionTodoStore(sm)

	ctx := context.Background()

	// Initial get on empty session
	todos, err := store.GetTodos(ctx)
	if err != nil {
		t.Fatalf("GetTodos failed: %v", err)
	}
	if len(todos) != 0 {
		t.Fatalf("expected 0 todos, got %d", len(todos))
	}

	// Save todos
	items := []core.TodoItem{
		{ID: "1", Content: "Task 1", Status: core.TodoStatusCompleted, Priority: core.TodoPriorityHigh},
		{ID: "2", Content: "Task 2", Status: core.TodoStatusInProgress, Priority: core.TodoPriorityMedium},
	}
	if err := store.SaveTodos(ctx, items); err != nil {
		t.Fatalf("SaveTodos failed: %v", err)
	}

	// Read back
	retrieved, err := store.GetTodos(ctx)
	if err != nil {
		t.Fatalf("GetTodos after save failed: %v", err)
	}
	if len(retrieved) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(retrieved))
	}
	if retrieved[0].Content != "Task 1" || retrieved[0].Status != core.TodoStatusCompleted {
		t.Errorf("unexpected first item: %+v", retrieved[0])
	}
	if retrieved[1].Content != "Task 2" || retrieved[1].Status != core.TodoStatusInProgress {
		t.Errorf("unexpected second item: %+v", retrieved[1])
	}
}

func TestTodo_PersistsAcrossSessionRestartAndCompaction(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create a persistent tree session
	tm, err := session.CreateTreeSession(tempDir)
	if err != nil {
		t.Fatalf("CreateTreeSession failed: %v", err)
	}
	sessionPath := tm.GetFilePath()
	sm := NewTreeManagerAdapter(tm)

	todoTool := NewTodoTool()
	ctx := core.ContextWithTodoStore(context.Background(), newSessionTodoStore(sm))

	// Write tasks using the tool
	input := `{"todos":[{"id":"task-1","content":"Design architecture","status":"completed"},{"id":"task-2","content":"Write tests","status":"in_progress"}]}`
	resp := runKitTodo(t, todoTool, ctx, input)
	if resp.IsError {
		t.Fatalf("todo tool write failed: %s", resp.Content)
	}

	if !strings.Contains(resp.Content, "[x] task-1: Design architecture") {
		t.Errorf("expected completed task-1, got: %s", resp.Content)
	}

	// Trigger compaction on this session
	_, err = tm.AppendCompaction("Compacted earlier messages", "", 500, 100, 4, nil, nil)
	if err != nil {
		t.Fatalf("AppendCompaction failed: %v", err)
	}

	// Verify todos still exist immediately after compaction
	compactionRead := runKitTodo(t, todoTool, ctx, `{}`)
	if !strings.Contains(compactionRead.Content, "[x] task-1: Design architecture") {
		t.Errorf("expected todos to survive compaction, got: %s", compactionRead.Content)
	}

	// 2. Close session
	if err := tm.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 3. Re-open session from disk (simulating restart / resume)
	reopenedTM, err := session.OpenTreeSession(sessionPath)
	if err != nil {
		t.Fatalf("OpenTreeSession failed: %v", err)
	}
	defer func() { _ = reopenedTM.Close() }()

	reopenedSM := NewTreeManagerAdapter(reopenedTM)
	reopenedCtx := core.ContextWithTodoStore(context.Background(), newSessionTodoStore(reopenedSM))

	// Read todos with reopened session
	resumedRead := runKitTodo(t, todoTool, reopenedCtx, `{}`)
	if resumedRead.IsError {
		t.Fatalf("resumed read failed: %s", resumedRead.Content)
	}

	if !strings.Contains(resumedRead.Content, "[x] task-1: Design architecture") {
		t.Errorf("expected resumed read to contain task-1, got: %s", resumedRead.Content)
	}
	if !strings.Contains(resumedRead.Content, "[~] task-2: Write tests") {
		t.Errorf("expected resumed read to contain task-2, got: %s", resumedRead.Content)
	}
}

func TestKit_TodoStorePrecedence(t *testing.T) {
	// 1. Kit with explicit CoreToolOptions.WithTodoStore
	explicitStore := core.NewMemoryTodoStore()
	_ = explicitStore.SaveTodos(context.Background(), []core.TodoItem{
		{ID: "explicit-1", Content: "Explicit Store Task", Status: core.TodoStatusPending},
	})

	k1 := &Kit{
		explicitTodoStore: explicitStore,
		session:           NewTreeManagerAdapter(session.InMemoryTreeSession(t.TempDir())),
	}

	// effectiveTodoStore must return explicitStore, not session store
	if got := k1.effectiveTodoStore(); got != explicitStore {
		t.Errorf("effectiveTodoStore with explicit store: got %v, want %v", got, explicitStore)
	}

	// 2. Kit with NO explicit store, but has session
	tm := session.InMemoryTreeSession(t.TempDir())
	sm := NewTreeManagerAdapter(tm)
	k2 := &Kit{
		session: sm,
	}

	gotStore := k2.effectiveTodoStore()
	if gotStore == nil {
		t.Fatal("expected non-nil session todo store when session is present")
	}

	// 3. Kit with NO explicit store and NO session
	k3 := &Kit{}
	if got := k3.effectiveTodoStore(); got != nil {
		t.Errorf("expected nil effectiveTodoStore when no session and no explicit store, got %v", got)
	}

	// 4. Per-turn context override
	perTurnStore := core.NewMemoryTodoStore()
	ctxWithPerTurn := ContextWithTodoStore(context.Background(), perTurnStore)
	if got := TodoStoreFromContext(ctxWithPerTurn); got != perTurnStore {
		t.Errorf("TodoStoreFromContext: got %v, want %v", got, perTurnStore)
	}
}
