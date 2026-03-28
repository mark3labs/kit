package kit

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/message"
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

// GetTreeSession returns the tree session manager, or nil if not configured.
func (m *Kit) GetTreeSession() *TreeManager {
	return m.treeSession
}

// SetTreeSession replaces the tree session on a Kit instance. This is used by
// the CLI when it handles session creation externally (e.g. --resume with a
// TUI picker) and needs to inject the result into a Kit-like workflow.
func (m *Kit) SetTreeSession(ts *TreeManager) {
	m.treeSession = ts
}

// GetSessionPath returns the file path of the active tree session, or empty
// for in-memory sessions or when no tree session is configured.
func (m *Kit) GetSessionPath() string {
	if m.treeSession != nil {
		return m.treeSession.GetFilePath()
	}
	return ""
}

// GetSessionID returns the UUID of the active tree session, or empty when no
// tree session is configured.
func (m *Kit) GetSessionID() string {
	if m.treeSession != nil {
		return m.treeSession.GetSessionID()
	}
	return ""
}

// Branch moves the tree session's leaf pointer to the given entry ID, creating
// a branch point. Subsequent Prompt() calls will extend from the new position.
func (m *Kit) Branch(entryID string) error {
	return m.treeSession.Branch(entryID)
}

// SetSessionName sets a user-defined display name for the active tree session.
func (m *Kit) SetSessionName(name string) error {
	if m.treeSession == nil {
		return fmt.Errorf("session naming requires a tree session")
	}
	_, err := m.treeSession.AppendSessionInfo(name)
	return err
}

// ---------------------------------------------------------------------------
// Tree Navigation Bridge for Extensions (Phase 1)
// ---------------------------------------------------------------------------

// GetTreeNode returns a node by ID with full metadata and children.
// Returns nil if entry not found or no tree session.
func (m *Kit) GetTreeNode(entryID string) *TreeNode {
	if m.treeSession == nil {
		return nil
	}
	entry := m.treeSession.GetEntry(entryID)
	if entry == nil {
		return nil
	}
	return m.entryToTreeNode(entry)
}

// GetCurrentBranch returns the path from root to current leaf as TreeNodes.
func (m *Kit) GetCurrentBranch() []TreeNode {
	if m.treeSession == nil {
		return nil
	}
	branch := m.treeSession.GetBranch("")
	var nodes []TreeNode
	for _, entry := range branch {
		node := m.entryToTreeNode(entry)
		if node != nil {
			nodes = append(nodes, *node)
		}
	}
	return nodes
}

// GetChildren returns direct child IDs of an entry.
func (m *Kit) GetChildren(parentID string) []string {
	if m.treeSession == nil {
		return nil
	}
	return m.treeSession.GetChildren(parentID)
}

// NavigateTo branches/forks the session to the specified entry ID.
// Returns error description or empty string for success.
func (m *Kit) NavigateTo(entryID string) string {
	if m.treeSession == nil {
		return "no tree session available"
	}
	if err := m.treeSession.Branch(entryID); err != nil {
		return err.Error()
	}
	return ""
}

// SummarizeBranch uses LLM to summarize a branch range.
// Returns summary text or error string.
func (m *Kit) SummarizeBranch(fromID, toID string) string {
	if m.treeSession == nil {
		return ""
	}

	// Get the branch and find the range
	branch := m.treeSession.GetBranch("")
	var startIdx, endIdx = -1, -1
	for i, entry := range branch {
		id := m.getEntryID(entry)
		if id == fromID {
			startIdx = i
		}
		if id == toID {
			endIdx = i
		}
	}

	if startIdx < 0 || endIdx < 0 || startIdx > endIdx {
		return ""
	}

	// Build text to summarize
	var content strings.Builder
	for i := startIdx; i <= endIdx; i++ {
		node := m.entryToTreeNode(branch[i])
		if node != nil && node.Content != "" {
			fmt.Fprintf(&content, "[%s] %s\n\n", node.Role, node.Content)
		}
	}

	if content.Len() == 0 {
		return ""
	}

	// Use LLM to summarize
	resp, err := m.ExecuteCompletion(context.Background(), extensions.CompleteRequest{
		Model:  "", // Use current model
		System: "You are a concise summarization assistant. Summarize the conversation in 2-3 sentences.",
		Prompt: content.String(),
	})
	if err != nil {
		return ""
	}
	return resp.Text
}

// CollapseBranch replaces a branch range with a summary entry.
// Returns error description or empty string for success.
func (m *Kit) CollapseBranch(fromID, toID, summary string) string {
	if m.treeSession == nil {
		return "no tree session available"
	}
	_, err := m.treeSession.AppendBranchSummary(fromID, summary)
	if err != nil {
		return err.Error()
	}
	return ""
}

// entryToTreeNode converts a session entry to a TreeNode.
func (m *Kit) entryToTreeNode(entry any) *TreeNode {
	switch e := entry.(type) {
	case *session.MessageEntry:
		msg, err := e.ToMessage()
		if err != nil {
			return nil
		}
		var content strings.Builder
		for _, p := range msg.Parts {
			switch pt := p.(type) {
			case message.TextContent:
				content.WriteString(pt.Text)
			case message.ReasoningContent:
				content.WriteString(pt.Thinking)
			case message.ToolCall:
				fmt.Fprintf(&content, "[tool_call: %s]", pt.Name)
			case message.ToolResult:
				fmt.Fprintf(&content, "[tool_result: %s]", pt.Content)
			}
		}
		return &TreeNode{
			ID:        e.ID,
			ParentID:  e.ParentID,
			Type:      "message",
			Role:      string(msg.Role),
			Content:   content.String(),
			Model:     msg.Model,
			Provider:  msg.Provider,
			Timestamp: e.Timestamp.Format(time.RFC3339),
			Children:  m.treeSession.GetChildren(e.ID),
		}
	case *session.BranchSummaryEntry:
		return &TreeNode{
			ID:        e.ID,
			ParentID:  e.ParentID,
			Type:      "branch_summary",
			Content:   e.Summary,
			Timestamp: e.Timestamp.Format(time.RFC3339),
			Children:  m.treeSession.GetChildren(e.ID),
		}
	case *session.ModelChangeEntry:
		return &TreeNode{
			ID:        e.ID,
			ParentID:  e.ParentID,
			Type:      "model_change",
			Content:   fmt.Sprintf("Model changed to %s/%s", e.Provider, e.ModelID),
			Model:     e.Provider + "/" + e.ModelID,
			Provider:  e.Provider,
			Timestamp: e.Timestamp.Format(time.RFC3339),
			Children:  m.treeSession.GetChildren(e.ID),
		}
	case *session.ExtensionDataEntry:
		return &TreeNode{
			ID:        e.ID,
			ParentID:  e.ParentID,
			Type:      "extension_data",
			Content:   fmt.Sprintf("Extension data: %s", e.ExtType),
			Timestamp: e.Timestamp.Format(time.RFC3339),
			Children:  m.treeSession.GetChildren(e.ID),
		}
	default:
		return nil
	}
}

// getEntryID extracts the ID from a session entry.
func (m *Kit) getEntryID(entry any) string {
	switch e := entry.(type) {
	case *session.MessageEntry:
		return e.ID
	case *session.BranchSummaryEntry:
		return e.ID
	case *session.ModelChangeEntry:
		return e.ID
	case *session.ExtensionDataEntry:
		return e.ID
	default:
		return ""
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
