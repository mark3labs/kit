package core

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"charm.land/fantasy"
)

// Todo status values.
const (
	TodoStatusPending    = "pending"
	TodoStatusInProgress = "in_progress"
	TodoStatusCompleted  = "completed"
	TodoStatusCancelled  = "cancelled"
)

// Todo priority values.
const (
	TodoPriorityLow    = "low"
	TodoPriorityMedium = "medium"
	TodoPriorityHigh   = "high"
)

// TodoItem represents a single task in a structured checklist.
type TodoItem struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority,omitempty"`
}

// TodoStore provides storage and retrieval for todo items.
type TodoStore interface {
	GetTodos(ctx context.Context) ([]TodoItem, error)
	SaveTodos(ctx context.Context, todos []TodoItem) error
}

type memoryTodoStore struct {
	mu    sync.RWMutex
	todos []TodoItem
}

// NewMemoryTodoStore returns a thread-safe in-memory TodoStore.
func NewMemoryTodoStore() TodoStore {
	return &memoryTodoStore{}
}

func (s *memoryTodoStore) GetTodos(ctx context.Context) ([]TodoItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]TodoItem, len(s.todos))
	copy(res, s.todos)
	return res, nil
}

func (s *memoryTodoStore) SaveTodos(ctx context.Context, todos []TodoItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.todos = make([]TodoItem, len(todos))
	copy(s.todos, todos)
	return nil
}

type todoStoreKey struct{}

// ContextWithTodoStore stores the TodoStore in ctx for use by the todo tool.
func ContextWithTodoStore(ctx context.Context, store TodoStore) context.Context {
	return context.WithValue(ctx, todoStoreKey{}, store)
}

// TodoStoreFromContext returns the TodoStore stored in ctx, or nil if none is set.
func TodoStoreFromContext(ctx context.Context) TodoStore {
	if s, ok := ctx.Value(todoStoreKey{}).(TodoStore); ok {
		return s
	}
	return nil
}

type todoArgs struct {
	Action string      `json:"action,omitempty"`
	Todos  *[]TodoItem `json:"todos,omitempty"`
}

// NewTodoTool creates the todo core tool for managing structured task checklists.
func NewTodoTool(opts ...ToolOption) fantasy.AgentTool {
	cfg := ApplyOptions(opts)
	defaultStore := cfg.TodoStore
	if defaultStore == nil {
		defaultStore = NewMemoryTodoStore()
	}

	return &coreTool{
		info: fantasy.ToolInfo{
			Name: "todo",
			Description: `Track and manage structured task checklists for complex multi-step workflows.

Use this tool to plan multi-step tasks, track progress, and maintain focus across long workflows.
- To view existing tasks: call with no arguments, {"action": "read"}, or {"todos": []}.
- To update or replace tasks: call with {"todos": [...]}. Each task must have:
  - id: unique string identifier (e.g. "1", "2")
  - content: clear description of the step
  - status: "pending", "in_progress", "completed", or "cancelled"
  - priority: optional "high", "medium", or "low" (default: "medium")
- To clear all tasks: call with {"action": "clear"}.

Tasks are persisted in the session store and survive context compaction and session restarts.`,
			Parameters: map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"read", "write", "clear"},
					"description": "Optional action. If non-empty 'todos' is provided, 'write' is assumed. 'clear' removes all tasks. Defaults to 'read'.",
				},
				"todos": map[string]any{
					"type":        "array",
					"description": "The updated list of todo items.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id": map[string]any{
								"type":        "string",
								"description": "Unique identifier for the task.",
							},
							"content": map[string]any{
								"type":        "string",
								"description": "Detailed description of the task.",
							},
							"status": map[string]any{
								"type":        "string",
								"enum":        []string{"pending", "in_progress", "completed", "cancelled"},
								"description": "Current status of the task.",
							},
							"priority": map[string]any{
								"type":        "string",
								"enum":        []string{"high", "medium", "low"},
								"description": "Priority level of the task.",
							},
						},
						"required": []string{"content", "status"},
					},
				},
			},
			Required: []string{},
			Parallel: false,
		},
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return executeTodo(ctx, call, defaultStore)
		},
	}
}

func executeTodo(ctx context.Context, call fantasy.ToolCall, defaultStore TodoStore) (fantasy.ToolResponse, error) {
	if err := ctx.Err(); err != nil {
		return fantasy.ToolResponse{}, err
	}

	store := TodoStoreFromContext(ctx)
	if store == nil {
		store = defaultStore
	}

	var args todoArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	if args.Action == "clear" {
		if err := store.SaveTodos(ctx, []TodoItem{}); err != nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to clear todos: %v", err)), nil
		}
		return fantasy.NewTextResponse("Todos cleared."), nil
	}

	// If non-empty todos are provided, treat as write action.
	if args.Todos != nil && len(*args.Todos) > 0 {
		items := *args.Todos

		// Reserve all non-empty explicit IDs and reject duplicate explicit IDs.
		usedIDs := make(map[string]bool, len(items))
		for i, item := range items {
			id := strings.TrimSpace(item.ID)
			if id != "" {
				if usedIDs[id] {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("item %d: duplicate todo id %q", i+1, id)), nil
				}
				usedIDs[id] = true
			}
		}

		nextID := 1
		normalized := make([]TodoItem, len(items))
		for i, item := range items {
			content := strings.TrimSpace(item.Content)
			if content == "" {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("item %d: content cannot be empty", i+1)), nil
			}

			status := strings.ToLower(strings.TrimSpace(item.Status))
			switch status {
			case "done":
				status = TodoStatusCompleted
			case "in-progress":
				status = TodoStatusInProgress
			case TodoStatusPending, TodoStatusInProgress, TodoStatusCompleted, TodoStatusCancelled:
				// valid
			case "":
				status = TodoStatusPending
			default:
				return fantasy.NewTextErrorResponse(fmt.Sprintf("item %d: invalid status %q (must be pending, in_progress, completed, or cancelled)", i+1, item.Status)), nil
			}

			priority := strings.ToLower(strings.TrimSpace(item.Priority))
			switch priority {
			case TodoPriorityHigh, TodoPriorityMedium, TodoPriorityLow:
				// valid
			default:
				priority = TodoPriorityMedium
			}

			id := strings.TrimSpace(item.ID)
			if id == "" {
				for {
					cand := fmt.Sprintf("%d", nextID)
					nextID++
					if !usedIDs[cand] {
						id = cand
						usedIDs[id] = true
						break
					}
				}
			}

			normalized[i] = TodoItem{
				ID:       id,
				Content:  content,
				Status:   status,
				Priority: priority,
			}
		}

		if err := store.SaveTodos(ctx, normalized); err != nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to save todos: %v", err)), nil
		}

		return fantasy.NewTextResponse(FormatTodoList(normalized)), nil
	}

	if args.Action == "write" {
		return fantasy.NewTextErrorResponse("action 'write' requires 'todos' list"), nil
	}

	// Read action
	todos, err := store.GetTodos(ctx)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to read todos: %v", err)), nil
	}

	if len(todos) == 0 {
		return fantasy.NewTextResponse("No todos found. Call todo with a list of tasks in \"todos\" to create some."), nil
	}

	return fantasy.NewTextResponse(FormatTodoList(todos)), nil
}

// FormatTodoList renders a list of TodoItems into a formatted human-readable checklist.
func FormatTodoList(todos []TodoItem) string {
	if len(todos) == 0 {
		return "No todos found."
	}

	var completed, inProgress, cancelled int
	for _, t := range todos {
		switch t.Status {
		case TodoStatusCompleted:
			completed++
		case TodoStatusInProgress:
			inProgress++
		case TodoStatusCancelled:
			cancelled++
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Todos (%d/%d completed", completed, len(todos))
	if inProgress > 0 {
		fmt.Fprintf(&sb, ", %d in progress", inProgress)
	}
	if cancelled > 0 {
		fmt.Fprintf(&sb, ", %d cancelled", cancelled)
	}
	sb.WriteString("):\n")

	for _, t := range todos {
		var marker string
		switch t.Status {
		case TodoStatusCompleted:
			marker = "[x]"
		case TodoStatusInProgress:
			marker = "[~]"
		case TodoStatusCancelled:
			marker = "[-]"
		default:
			marker = "[ ]"
		}

		prio := ""
		if t.Priority != "" && t.Priority != TodoPriorityMedium {
			prio = fmt.Sprintf(" [%s]", t.Priority)
		}
		fmt.Fprintf(&sb, "%s %s: %s%s\n", marker, t.ID, t.Content, prio)
	}

	return strings.TrimRight(sb.String(), "\n")
}
