package kit

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/session"
)

// --- Package-level session operations (no Kit instance required) ---

// ListSessions finds all sessions for the given working directory, sorted by
// modification time (newest first). If dir is empty, the current working
// directory is used.
func ListSessions(dir string) ([]SessionInfo, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}
	return session.ListSessions(dir)
}

// ListAllSessions finds all sessions across all working directories, sorted
// by modification time (newest first).
func ListAllSessions() ([]SessionInfo, error) {
	return session.ListAllSessions()
}

// DeleteSession removes a session file from disk.
func DeleteSession(path string) error {
	return session.DeleteSession(path)
}

// OpenTreeSession opens an existing JSONL session file. This is a package-level
// function (no Kit instance required) used by the CLI for session switching.
func OpenTreeSession(path string) (*TreeManager, error) {
	return session.OpenTreeSession(path)
}

// --- Instance methods on Kit ---

// GetSessionManager returns the session manager, or nil if not configured.
func (m *Kit) GetSessionManager() SessionManager {
	return m.session
}

// GetTreeSession returns the tree session manager, or nil if not configured.
// Deprecated: Use GetSessionManager instead.
func (m *Kit) GetTreeSession() *TreeManager {
	// Try to unwrap the adapter if using default implementation
	if adapter, ok := m.session.(*treeManagerAdapter); ok {
		return adapter.inner
	}
	return nil
}

// SetSessionManager replaces the session manager on a Kit instance.
func (m *Kit) SetSessionManager(sm SessionManager) {
	m.session = sm
}

// SetTreeSession replaces the tree session on a Kit instance. This is used by
// the CLI when it handles session creation externally (e.g. --resume with a
// TUI picker) and needs to inject the result into a Kit-like workflow.
// Deprecated: Use SetSessionManager instead.
func (m *Kit) SetTreeSession(ts *TreeManager) {
	m.session = NewTreeManagerAdapter(ts)
}

// GetSessionPath returns the file path of the active session, or empty
// for in-memory sessions or when no file-based session is configured.
func (m *Kit) GetSessionPath() string {
	// Only file-based sessions have a path
	// Try to get it from the underlying TreeManager if using default adapter
	if m.session == nil {
		return ""
	}
	// Check if it's the default adapter
	if adapter, ok := m.session.(*treeManagerAdapter); ok {
		return adapter.inner.GetFilePath()
	}
	return ""
}

// GetSessionID returns the UUID of the active session, or empty when no
// session is configured.
func (m *Kit) GetSessionID() string {
	if m.session == nil {
		return ""
	}
	return m.session.GetSessionID()
}

// Branch moves the session's leaf pointer to the given entry ID, creating
// a branch point. Subsequent Prompt() calls will extend from the new position.
func (m *Kit) Branch(entryID string) error {
	if m.session == nil {
		return fmt.Errorf("no session available")
	}
	return m.session.Branch(entryID)
}

// SetSessionName sets a user-defined display name for the active session.
func (m *Kit) SetSessionName(name string) error {
	if m.session == nil {
		return fmt.Errorf("session naming requires a session")
	}
	return m.session.SetSessionName(name)
}

// ---------------------------------------------------------------------------
// Tree Navigation Bridge for Extensions (Phase 1)
// ---------------------------------------------------------------------------

// GetTreeNode returns a node by ID with full metadata and children.
// Returns nil if entry not found or no session.
func (m *Kit) GetTreeNode(entryID string) *TreeNode {
	if m.session == nil {
		return nil
	}
	entry := m.session.GetEntry(entryID)
	if entry == nil {
		return nil
	}
	return m.branchEntryToTreeNode(entry)
}

// GetCurrentBranch returns the path from root to current leaf as TreeNodes.
func (m *Kit) GetCurrentBranch() []TreeNode {
	if m.session == nil {
		return nil
	}
	branch := m.session.GetCurrentBranch()
	var nodes []TreeNode
	for _, entry := range branch {
		node := m.branchEntryToTreeNode(&entry)
		if node != nil {
			nodes = append(nodes, *node)
		}
	}
	return nodes
}

// GetChildren returns direct child IDs of an entry.
func (m *Kit) GetChildren(parentID string) []string {
	if m.session == nil {
		return nil
	}
	return m.session.GetChildren(parentID)
}

// NavigateTo branches/forks the session to the specified entry ID.
// Returns an error if the session is unavailable or the entry ID is not found.
func (m *Kit) NavigateTo(entryID string) error {
	if m.session == nil {
		return fmt.Errorf("no session available")
	}
	return m.session.Branch(entryID)
}

// SummarizeBranch uses the LLM to summarize the conversation between two
// entry IDs. Returns the summary text, or an error if the range is invalid,
// the session is unavailable, or the LLM call fails.
func (m *Kit) SummarizeBranch(fromID, toID string) (string, error) {
	if m.session == nil {
		return "", fmt.Errorf("no session available")
	}

	// Get the branch and find the range
	branch := m.session.GetCurrentBranch()
	var startIdx, endIdx = -1, -1
	for i, entry := range branch {
		id := entry.ID
		if id == fromID {
			startIdx = i
		}
		if id == toID {
			endIdx = i
		}
	}

	if startIdx < 0 || endIdx < 0 || startIdx > endIdx {
		return "", fmt.Errorf("entry IDs not found or out of order in current branch")
	}

	// Build text to summarize
	var content strings.Builder
	for i := startIdx; i <= endIdx; i++ {
		node := m.branchEntryToTreeNode(&branch[i])
		if node != nil && node.Content != "" {
			fmt.Fprintf(&content, "[%s] %s\n\n", node.Role, node.Content)
		}
	}

	if content.Len() == 0 {
		return "", fmt.Errorf("no content found in the specified range")
	}

	// Use LLM to summarize
	resp, err := m.ExecuteCompletion(context.Background(), extensions.CompleteRequest{
		Model:  "", // Use current model
		System: "You are a concise summarization assistant. Summarize the conversation in 2-3 sentences.",
		Prompt: content.String(),
	})
	if err != nil {
		return "", fmt.Errorf("summarization failed: %w", err)
	}
	return resp.Text, nil
}

// CollapseBranch replaces a branch range with a summary entry.
// Returns an error if the session is unavailable or the operation fails.
func (m *Kit) CollapseBranch(fromID, toID, summary string) error {
	if m.session == nil {
		return fmt.Errorf("no session available")
	}
	// Note: This operation is not directly supported by SessionManager interface
	// as it requires AppendBranchSummary which is TreeManager-specific.
	// For custom SessionManagers, this would need to be implemented differently.
	// For now, we try to use the underlying TreeManager if available.
	if adapter, ok := m.session.(*treeManagerAdapter); ok {
		_, err := adapter.inner.AppendBranchSummary(fromID, summary)
		return err
	}
	return fmt.Errorf("CollapseBranch not supported by custom session manager")
}

// branchEntryToTreeNode converts a BranchEntry to a TreeNode.
func (m *Kit) branchEntryToTreeNode(entry *BranchEntry) *TreeNode {
	if entry == nil {
		return nil
	}

	switch entry.Type {
	case EntryTypeMessage:
		// Build content from RawParts
		var content strings.Builder
		for _, p := range entry.RawParts {
			switch pt := p.(type) {
			case TextContent:
				content.WriteString(pt.Text)
			case ReasoningContent:
				content.WriteString(pt.Thinking)
			case ToolCall:
				fmt.Fprintf(&content, "[tool_call: %s]", pt.Name)
			case ToolResult:
				fmt.Fprintf(&content, "[tool_result: %s]", pt.Content)
			}
		}
		return &TreeNode{
			ID:        entry.ID,
			ParentID:  entry.ParentID,
			Type:      "message",
			Role:      entry.Role,
			Content:   content.String(),
			Model:     entry.Model,
			Provider:  entry.Provider,
			Timestamp: entry.Timestamp.Format(time.RFC3339),
			Children:  m.session.GetChildren(entry.ID),
		}
	case EntryTypeBranchSummary:
		return &TreeNode{
			ID:        entry.ID,
			ParentID:  entry.ParentID,
			Type:      "branch_summary",
			Content:   entry.Content,
			Timestamp: entry.Timestamp.Format(time.RFC3339),
			Children:  m.session.GetChildren(entry.ID),
		}
	case EntryTypeModelChange:
		return &TreeNode{
			ID:        entry.ID,
			ParentID:  entry.ParentID,
			Type:      "model_change",
			Content:   entry.Content,
			Model:     entry.Model,
			Provider:  entry.Provider,
			Timestamp: entry.Timestamp.Format(time.RFC3339),
			Children:  m.session.GetChildren(entry.ID),
		}
	case EntryTypeExtensionData:
		return &TreeNode{
			ID:        entry.ID,
			ParentID:  entry.ParentID,
			Type:      "extension_data",
			Content:   entry.Content,
			Timestamp: entry.Timestamp.Format(time.RFC3339),
			Children:  m.session.GetChildren(entry.ID),
		}
	default:
		return nil
	}
}

// TreeNode represents a node in the session tree for SDK consumers.
type TreeNode struct {
	ID        string
	ParentID  string
	Type      string // "message", "branch_summary", "model_change", "extension_data"
	Role      string // for messages: "user", "assistant", "system", "tool"
	Content   string
	Model     string
	Provider  string
	Timestamp string
	Children  []string
}
