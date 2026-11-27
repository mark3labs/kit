package kit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/kit/internal/core"
)

// sessionTodoStore adapts SessionManager's ExtensionData storage to core.TodoStore.
type sessionTodoStore struct {
	session SessionManager
}

func newSessionTodoStore(sm SessionManager) core.TodoStore {
	return &sessionTodoStore{session: sm}
}

func (s *sessionTodoStore) GetTodos(ctx context.Context) ([]core.TodoItem, error) {
	if s.session == nil {
		return nil, nil
	}
	entries := s.session.GetExtensionData("todo")
	if len(entries) == 0 {
		return nil, nil
	}
	// Use the latest entry on the branch
	latest := entries[len(entries)-1]
	var items []core.TodoItem
	if err := json.Unmarshal([]byte(latest.Data), &items); err != nil {
		return nil, fmt.Errorf("unmarshal session todos: %w", err)
	}
	return items, nil
}

func (s *sessionTodoStore) SaveTodos(ctx context.Context, todos []core.TodoItem) error {
	if s.session == nil {
		return nil
	}
	data, err := json.Marshal(todos)
	if err != nil {
		return fmt.Errorf("marshal session todos: %w", err)
	}
	_, err = s.session.AppendExtensionData("todo", string(data))
	if err != nil {
		return fmt.Errorf("append session todo extension data: %w", err)
	}
	return nil
}
