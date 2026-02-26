package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/message"
)

// TreeNode represents a node in the session tree for display purposes.
// It mirrors pi's SessionTreeNode design.
type TreeNode struct {
	Entry    any         // the underlying entry (*MessageEntry, *ModelChangeEntry, etc.)
	ID       string      // entry ID
	ParentID string      // parent entry ID
	Children []*TreeNode // child nodes
}

// TreeManager manages a tree-structured JSONL session. It is the replacement
// for the linear session.Manager, following pi's design decisions:
//
//   - JSONL append-only format (one JSON object per line)
//   - Tree structure via id/parent_id on every entry
//   - Leaf pointer tracking current position
//   - Context building walks from leaf to root
//   - Auto-discovery by working directory
type TreeManager struct {
	mu sync.RWMutex

	// header is the session header (first line of the JSONL file).
	header SessionHeader

	// entries is the ordered list of all entries (excluding header).
	entries []any

	// index maps entry ID to the entry for O(1) lookup.
	index map[string]any

	// childIndex maps parent ID to child entry IDs for tree traversal.
	childIndex map[string][]string

	// labels maps entry ID to user-defined label string.
	labels map[string]string

	// leafID is the current position in the tree. Empty string means
	// the session is at the root (before any entries).
	leafID string

	// sessionName is the latest user-defined display name.
	sessionName string

	// filePath is the JSONL file path. Empty for in-memory sessions.
	filePath string

	// file is the open file handle for appending entries. Nil for in-memory.
	file *os.File
}

// --- Constructors ---

// CreateTreeSession creates a new tree session persisted at the default
// location for the given working directory.
func CreateTreeSession(cwd string) (*TreeManager, error) {
	sessionDir := DefaultSessionDir(cwd)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	now := time.Now().UTC()
	fileName := fmt.Sprintf("%s_%s.jsonl",
		now.Format("2006-01-02T15-04-05-000Z"),
		GenerateSessionID()[:12],
	)
	filePath := filepath.Join(sessionDir, fileName)

	header := SessionHeader{
		Type:      EntryTypeSession,
		Version:   CurrentVersion,
		ID:        GenerateSessionID(),
		Timestamp: now,
		Cwd:       cwd,
	}

	tm := &TreeManager{
		header:     header,
		entries:    make([]any, 0),
		index:      make(map[string]any),
		childIndex: make(map[string][]string),
		labels:     make(map[string]string),
		filePath:   filePath,
	}

	// Create the file and write the header.
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create session file: %w", err)
	}
	tm.file = f

	if err := tm.writeEntry(&header); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to write session header: %w", err)
	}

	return tm, nil
}

// OpenTreeSession opens an existing JSONL session file.
func OpenTreeSession(path string) (*TreeManager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	tm := &TreeManager{
		entries:    make([]any, 0),
		index:      make(map[string]any),
		childIndex: make(map[string][]string),
		labels:     make(map[string]string),
		filePath:   path,
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		lineNum++

		entry, err := UnmarshalEntry([]byte(line))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		if lineNum == 1 {
			h, ok := entry.(*SessionHeader)
			if !ok {
				return nil, fmt.Errorf("first line must be a session header, got %T", entry)
			}
			tm.header = *h
			continue
		}

		tm.addEntryToIndex(entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan session file: %w", err)
	}

	// Set leaf to the last entry.
	if len(tm.entries) > 0 {
		tm.leafID = tm.entryID(tm.entries[len(tm.entries)-1])
	}

	// Open file for appending.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file for append: %w", err)
	}
	tm.file = f

	return tm, nil
}

// ContinueRecent finds the most recently modified session for the given cwd,
// or creates a new one if none exists.
func ContinueRecent(cwd string) (*TreeManager, error) {
	sessions, err := ListSessions(cwd)
	if err != nil || len(sessions) == 0 {
		return CreateTreeSession(cwd)
	}
	// sessions are sorted by modified time (newest first).
	return OpenTreeSession(sessions[0].Path)
}

// InMemoryTreeSession creates a tree session that is not persisted to disk.
func InMemoryTreeSession(cwd string) *TreeManager {
	return &TreeManager{
		header: SessionHeader{
			Type:      EntryTypeSession,
			Version:   CurrentVersion,
			ID:        GenerateSessionID(),
			Timestamp: time.Now().UTC(),
			Cwd:       cwd,
		},
		entries:    make([]any, 0),
		index:      make(map[string]any),
		childIndex: make(map[string][]string),
		labels:     make(map[string]string),
	}
}

// --- Append operations (all return entry ID) ---

// AppendMessage adds a message entry to the tree and persists it.
func (tm *TreeManager) AppendMessage(msg message.Message) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry, err := NewMessageEntry(tm.leafID, msg)
	if err != nil {
		return "", err
	}

	if err := tm.appendAndPersist(entry); err != nil {
		return "", err
	}

	tm.leafID = entry.ID
	return entry.ID, nil
}

// AppendFantasyMessage converts a fantasy.Message and appends it.
func (tm *TreeManager) AppendFantasyMessage(msg fantasy.Message) (string, error) {
	return tm.AppendMessage(message.FromFantasyMessage(msg))
}

// AppendModelChange records a model/provider change.
func (tm *TreeManager) AppendModelChange(provider, modelID string) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry := NewModelChangeEntry(tm.leafID, provider, modelID)
	if err := tm.appendAndPersist(entry); err != nil {
		return "", err
	}

	tm.leafID = entry.ID
	return entry.ID, nil
}

// AppendBranchSummary adds a summary of an abandoned branch.
func (tm *TreeManager) AppendBranchSummary(fromID, summary string) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry := NewBranchSummaryEntry(tm.leafID, fromID, summary)
	if err := tm.appendAndPersist(entry); err != nil {
		return "", err
	}

	tm.leafID = entry.ID
	return entry.ID, nil
}

// AppendLabel sets a label on a target entry.
func (tm *TreeManager) AppendLabel(targetID, label string) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry := NewLabelEntry(tm.leafID, targetID, label)
	if err := tm.appendAndPersist(entry); err != nil {
		return "", err
	}

	tm.labels[targetID] = label
	tm.leafID = entry.ID
	return entry.ID, nil
}

// AppendSessionInfo sets a display name for the session.
func (tm *TreeManager) AppendSessionInfo(name string) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry := NewSessionInfoEntry(tm.leafID, name)
	if err := tm.appendAndPersist(entry); err != nil {
		return "", err
	}

	tm.sessionName = name
	tm.leafID = entry.ID
	return entry.ID, nil
}

// --- Tree navigation ---

// Branch moves the leaf pointer to the given entry ID, creating a branch
// point. Subsequent appends will extend from this new position.
func (tm *TreeManager) Branch(entryID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if entryID == "" {
		tm.leafID = ""
		return nil
	}

	if _, ok := tm.index[entryID]; !ok {
		return fmt.Errorf("entry %q not found", entryID)
	}
	tm.leafID = entryID
	return nil
}

// ResetLeaf moves the leaf pointer to before the first entry (empty conversation).
func (tm *TreeManager) ResetLeaf() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.leafID = ""
}

// GetLeafID returns the current leaf position.
func (tm *TreeManager) GetLeafID() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.leafID
}

// GetEntry returns the entry with the given ID, or nil if not found.
func (tm *TreeManager) GetEntry(id string) any {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.index[id]
}

// GetEntries returns all entries (excluding the session header).
func (tm *TreeManager) GetEntries() []any {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	cp := make([]any, len(tm.entries))
	copy(cp, tm.entries)
	return cp
}

// GetChildren returns direct child entry IDs for a given parent.
func (tm *TreeManager) GetChildren(parentID string) []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	cp := make([]string, len(tm.childIndex[parentID]))
	copy(cp, tm.childIndex[parentID])
	return cp
}

// GetBranch returns the path of entries from the given entry to the root,
// ordered from root to the entry. If fromID is empty, uses the current leaf.
func (tm *TreeManager) GetBranch(fromID string) []any {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if fromID == "" {
		fromID = tm.leafID
	}
	if fromID == "" {
		return nil
	}

	var path []any
	current := fromID
	for current != "" {
		entry, ok := tm.index[current]
		if !ok {
			break
		}
		path = append(path, entry)
		current = tm.entryParentID(entry)
	}

	// Reverse to get root-to-leaf order.
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// GetLabel returns the label for an entry, or empty string if none.
func (tm *TreeManager) GetLabel(id string) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.labels[id]
}

// GetTree builds the full tree structure from root entries.
func (tm *TreeManager) GetTree() []*TreeNode {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Find root entries (entries with empty ParentID).
	var roots []*TreeNode
	rootIDs := tm.childIndex[""]

	for _, id := range rootIDs {
		node := tm.buildTreeNode(id)
		if node != nil {
			roots = append(roots, node)
		}
	}
	return roots
}

// --- Context building ---

// BuildContext walks from the current leaf to the root and returns the
// conversation messages suitable for sending to the LLM. Branch summaries
// are converted to user messages to provide context from abandoned branches.
// Also returns the latest model/provider settings encountered on the path.
func (tm *TreeManager) BuildContext() (messages []fantasy.Message, provider string, modelID string) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.leafID == "" {
		return nil, "", ""
	}

	// Walk from leaf to root collecting entries.
	branch := tm.getBranchLocked(tm.leafID)

	for _, entry := range branch {
		switch e := entry.(type) {
		case *MessageEntry:
			msg, err := e.ToMessage()
			if err != nil {
				continue // skip malformed entries
			}
			msgs := msg.ToFantasyMessages()
			messages = append(messages, msgs...)

		case *BranchSummaryEntry:
			// Convert branch summary to a user message for context.
			if e.Summary != "" {
				messages = append(messages, fantasy.Message{
					Role: fantasy.MessageRoleUser,
					Content: []fantasy.MessagePart{
						fantasy.TextPart{
							Text: fmt.Sprintf("[Branch context: %s]", e.Summary),
						},
					},
				})
			}

		case *ModelChangeEntry:
			provider = e.Provider
			modelID = e.ModelID
		}
	}

	return messages, provider, modelID
}

// --- Session info ---

// GetSessionID returns the session UUID.
func (tm *TreeManager) GetSessionID() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.header.ID
}

// GetSessionName returns the user-defined display name, or empty string.
func (tm *TreeManager) GetSessionName() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.sessionName
}

// GetCwd returns the working directory this session was created in.
func (tm *TreeManager) GetCwd() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.header.Cwd
}

// GetFilePath returns the JSONL file path, or empty for in-memory sessions.
func (tm *TreeManager) GetFilePath() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.filePath
}

// GetHeader returns a copy of the session header.
func (tm *TreeManager) GetHeader() SessionHeader {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.header
}

// IsPersisted returns true if this session writes to disk.
func (tm *TreeManager) IsPersisted() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.filePath != ""
}

// EntryCount returns the number of entries (excluding header).
func (tm *TreeManager) EntryCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.entries)
}

// MessageCount returns the number of message entries.
func (tm *TreeManager) MessageCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	count := 0
	for _, e := range tm.entries {
		if _, ok := e.(*MessageEntry); ok {
			count++
		}
	}
	return count
}

// Close closes the underlying file handle.
func (tm *TreeManager) Close() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.file != nil {
		err := tm.file.Close()
		tm.file = nil
		return err
	}
	return nil
}

// --- Legacy bridge ---

// AddFantasyMessages appends multiple fantasy messages as entries. This is
// used when syncing from the agent's ConversationMessages after a step.
func (tm *TreeManager) AddFantasyMessages(msgs []fantasy.Message) error {
	for _, msg := range msgs {
		if _, err := tm.AppendFantasyMessage(msg); err != nil {
			return err
		}
	}
	return nil
}

// GetFantasyMessages builds the context and returns just the messages.
// This satisfies the same conceptual role as the old Manager.GetMessages().
func (tm *TreeManager) GetFantasyMessages() []fantasy.Message {
	msgs, _, _ := tm.BuildContext()
	return msgs
}

// --- Internal helpers ---

// addEntryToIndex adds an entry to the in-memory indices.
func (tm *TreeManager) addEntryToIndex(entry any) {
	tm.entries = append(tm.entries, entry)

	id := tm.entryID(entry)
	parentID := tm.entryParentID(entry)

	if id != "" {
		tm.index[id] = entry
		tm.childIndex[parentID] = append(tm.childIndex[parentID], id)
	}

	// Track labels and session names.
	switch e := entry.(type) {
	case *LabelEntry:
		tm.labels[e.TargetID] = e.Label
	case *SessionInfoEntry:
		tm.sessionName = e.Name
	}
}

// appendAndPersist adds an entry to indices and writes it to the JSONL file.
func (tm *TreeManager) appendAndPersist(entry any) error {
	tm.addEntryToIndex(entry)
	if tm.file != nil {
		return tm.writeEntry(entry)
	}
	return nil
}

// writeEntry serializes an entry and appends it as a line to the file.
func (tm *TreeManager) writeEntry(entry any) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}
	data = append(data, '\n')
	_, err = tm.file.Write(data)
	return err
}

// entryID extracts the ID from any entry type.
func (tm *TreeManager) entryID(entry any) string {
	switch e := entry.(type) {
	case *MessageEntry:
		return e.ID
	case *ModelChangeEntry:
		return e.ID
	case *BranchSummaryEntry:
		return e.ID
	case *LabelEntry:
		return e.ID
	case *SessionInfoEntry:
		return e.ID
	default:
		return ""
	}
}

// entryParentID extracts the ParentID from any entry type.
func (tm *TreeManager) entryParentID(entry any) string {
	switch e := entry.(type) {
	case *MessageEntry:
		return e.ParentID
	case *ModelChangeEntry:
		return e.ParentID
	case *BranchSummaryEntry:
		return e.ParentID
	case *LabelEntry:
		return e.ParentID
	case *SessionInfoEntry:
		return e.ParentID
	default:
		return ""
	}
}

// getBranchLocked walks from an entry to the root (must hold at least RLock).
func (tm *TreeManager) getBranchLocked(fromID string) []any {
	var path []any
	visited := make(map[string]bool) // prevent cycles
	current := fromID
	for current != "" {
		if visited[current] {
			break
		}
		visited[current] = true
		entry, ok := tm.index[current]
		if !ok {
			break
		}
		path = append(path, entry)
		current = tm.entryParentID(entry)
	}

	// Reverse to root-first order.
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// buildTreeNode recursively builds a TreeNode from an entry ID.
func (tm *TreeManager) buildTreeNode(id string) *TreeNode {
	entry, ok := tm.index[id]
	if !ok {
		return nil
	}

	node := &TreeNode{
		Entry:    entry,
		ID:       id,
		ParentID: tm.entryParentID(entry),
	}

	for _, childID := range tm.childIndex[id] {
		child := tm.buildTreeNode(childID)
		if child != nil {
			node.Children = append(node.Children, child)
		}
	}

	return node
}

// --- Path conventions ---

// DefaultSessionDir returns the default session storage directory for a cwd.
// Following pi's convention: ~/.kit/sessions/--<cwd-path>--/
func DefaultSessionDir(cwd string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	// Convert path separators to double dashes.
	safeCwd := strings.ReplaceAll(cwd, string(filepath.Separator), "--")
	// Remove leading separator replacement.
	safeCwd = strings.TrimPrefix(safeCwd, "--")
	return filepath.Join(home, ".kit", "sessions", safeCwd)
}
