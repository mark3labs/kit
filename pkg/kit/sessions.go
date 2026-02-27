package kit

import (
	"fmt"
	"os"

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
