package kit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/kit/internal/core"
)

// sessionTodoStore adapts SessionManager's ExtensionData storage to core.TodoStore,
// optionally bound to an owner branch to prevent writes from detached jobs to an
// inactive branch after navigation.
type sessionTodoStore struct {
	session       SessionManager
	ownerBranchID string
}

func newSessionTodoStore(sm SessionManager, ownerBranchID string) core.TodoStore {
	return &sessionTodoStore{
		session:       sm,
		ownerBranchID: ownerBranchID,
	}
}

func (s *sessionTodoStore) isBranchActive() bool {
	if s.ownerBranchID == "" || s.session == nil {
		return true
	}
	currentBranch := s.session.GetCurrentBranch()
	for _, entry := range currentBranch {
		if entry.ID == s.ownerBranchID {
			return true
		}
	}

	// If ownerBranchID is not directly on the current branch, check if the
	// current branch is a post-compaction continuation of the owner branch.
	compaction := s.session.GetLastCompaction()
	if compaction == nil {
		return false
	}

	visited := make(map[string]bool)
	currID := compaction.PreCompactionLeafID
	if currID == "" {
		currID = compaction.FirstKeptEntryID
	}

	for currID != "" && !visited[currID] {
		if currID == s.ownerBranchID {
			return true
		}
		visited[currID] = true
		entry := s.session.GetEntry(currID)
		if entry == nil {
			break
		}
		if entry.Type == EntryTypeCompaction && entry.ParentID == "" && entry.PreCompactionLeafID != "" {
			currID = entry.PreCompactionLeafID
		} else {
			currID = entry.ParentID
		}
	}

	return false
}

func (s *sessionTodoStore) GetTodos(ctx context.Context) ([]core.TodoItem, error) {
	if s.session == nil {
		return nil, nil
	}
	if !s.isBranchActive() {
		return nil, fmt.Errorf("cannot read todos: session navigated away from owner branch %q", s.ownerBranchID)
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
	if !s.isBranchActive() {
		return fmt.Errorf("cannot save todos: session navigated away from owner branch %q", s.ownerBranchID)
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
