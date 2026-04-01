package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/message"
)

// TreeNode represents a node in the session tree for display purposes.

type TreeNode struct {
	Entry    any         // the underlying entry (*MessageEntry, *ModelChangeEntry, etc.)
	ID       string      // entry ID
	ParentID string      // parent entry ID
	Children []*TreeNode // child nodes
}

// TreeManager manages a tree-structured JSONL session. It is the replacement
// for the linear session.Manager:
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
		_ = f.Close()
		return nil, fmt.Errorf("failed to write session header: %w", err)
	}

	return tm, nil
}

// ForkToNewSession creates a new session file containing the history up to and
// including the target entry ID. This matches Pi's /fork behavior: it creates
// a completely new session file with a parent_session reference, copying all
// entries from the root to the target point.
func (tm *TreeManager) ForkToNewSession(cwd string, targetID string) (*TreeManager, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Get the branch from root to target (root-to-leaf order).
	branch := tm.getBranchLocked(targetID)
	if len(branch) == 0 {
		return nil, fmt.Errorf("target entry %q not found", targetID)
	}

	// Create a new session file.
	newTm, err := CreateTreeSession(cwd)
	if err != nil {
		return nil, err
	}

	// Set the parent session reference in the header.
	newTm.header.ParentSession = tm.filePath
	newTm.header.ParentSessionID = tm.header.ID

	// Rewrite the header with the parent reference.
	// We need to close and recreate the file to rewrite the header.
	if err := newTm.file.Close(); err != nil {
		return nil, fmt.Errorf("failed to close new session file: %w", err)
	}

	// Recreate the file and write the updated header.
	f, err := os.Create(newTm.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to recreate session file: %w", err)
	}
	newTm.file = f

	if err := newTm.writeEntry(&newTm.header); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to write session header: %w", err)
	}

	// Copy entries from the branch to the new session.
	// We need to remap IDs since the new session is independent.
	idMap := make(map[string]string) // old ID -> new ID
	var prevNewID string

	for _, entry := range branch {
		oldID := tm.EntryID(entry)
		newID := GenerateEntryID()
		idMap[oldID] = newID

		// Create a copy of the entry with the new ID and remapped parent.
		var newEntry any
		switch e := entry.(type) {
		case *MessageEntry:
			newEntry = &MessageEntry{
				Entry: Entry{
					Type:      EntryTypeMessage,
					ID:        newID,
					ParentID:  prevNewID, // Chain sequentially in new session
					Timestamp: e.Timestamp,
				},
				Role:     e.Role,
				Parts:    e.Parts,
				Model:    e.Model,
				Provider: e.Provider,
			}
			// Copy label if present.
			if label, ok := tm.labels[oldID]; ok {
				newTm.labels[newID] = label
			}

		case *ModelChangeEntry:
			newEntry = &ModelChangeEntry{
				Entry: Entry{
					Type:      EntryTypeModelChange,
					ID:        newID,
					ParentID:  prevNewID,
					Timestamp: e.Timestamp,
				},
				Provider: e.Provider,
				ModelID:  e.ModelID,
			}

		case *LabelEntry:
			// Remap the target ID if it's in our copied branch.
			newTargetID := e.TargetID
			if mapped, ok := idMap[e.TargetID]; ok {
				newTargetID = mapped
			}
			newEntry = &LabelEntry{
				Entry: Entry{
					Type:      EntryTypeLabel,
					ID:        newID,
					ParentID:  prevNewID,
					Timestamp: e.Timestamp,
				},
				TargetID: newTargetID,
				Label:    e.Label,
			}

		case *SessionInfoEntry:
			newEntry = &SessionInfoEntry{
				Entry: Entry{
					Type:      EntryTypeSessionInfo,
					ID:        newID,
					ParentID:  prevNewID,
					Timestamp: e.Timestamp,
				},
				Name: e.Name,
			}
			newTm.sessionName = e.Name

		case *ExtensionDataEntry:
			newEntry = &ExtensionDataEntry{
				Entry: Entry{
					Type:      EntryTypeExtensionData,
					ID:        newID,
					ParentID:  prevNewID,
					Timestamp: e.Timestamp,
				},
				ExtType: e.ExtType,
				Data:    e.Data,
			}

		case *BranchSummaryEntry:
			// Remap the from ID if it's in our copied branch.
			newFromID := e.FromID
			if mapped, ok := idMap[e.FromID]; ok {
				newFromID = mapped
			}
			newEntry = &BranchSummaryEntry{
				Entry: Entry{
					Type:      EntryTypeBranchSummary,
					ID:        newID,
					ParentID:  prevNewID,
					Timestamp: e.Timestamp,
				},
				FromID:  newFromID,
				Summary: e.Summary,
			}

		case *CompactionEntry:
			// Remap the first kept entry ID if it's in our copied branch.
			newFirstKeptID := e.FirstKeptEntryID
			if mapped, ok := idMap[e.FirstKeptEntryID]; ok {
				newFirstKeptID = mapped
			}
			newEntry = &CompactionEntry{
				Entry: Entry{
					Type:      EntryTypeCompaction,
					ID:        newID,
					ParentID:  prevNewID,
					Timestamp: e.Timestamp,
				},
				Summary:          e.Summary,
				FirstKeptEntryID: newFirstKeptID,
				TokensBefore:     e.TokensBefore,
				TokensAfter:      e.TokensAfter,
				MessagesRemoved:  e.MessagesRemoved,
				ReadFiles:        e.ReadFiles,
				ModifiedFiles:    e.ModifiedFiles,
			}
		}

		if newEntry != nil {
			if err := newTm.appendAndPersist(newEntry); err != nil {
				_ = f.Close()
				return nil, fmt.Errorf("failed to copy entry: %w", err)
			}
			prevNewID = newID
		}
	}

	// Set the leaf to the last entry in the new session.
	newTm.leafID = prevNewID

	return newTm, nil
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

	reader := bufio.NewReader(strings.NewReader(string(data)))
	lineNum := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Process the last line if it's not empty
				if strings.TrimSpace(line) != "" {
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
					} else {
						tm.addEntryToIndex(entry)
					}
				}
				break
			}
			return nil, fmt.Errorf("failed to read session file: %w", err)
		}

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

	// Set leaf to the last entry.
	if len(tm.entries) > 0 {
		tm.leafID = tm.EntryID(tm.entries[len(tm.entries)-1])
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

// AppendLLMMessage converts an LLM message and appends it.
func (tm *TreeManager) AppendLLMMessage(msg fantasy.Message) (string, error) {
	return tm.AppendMessage(message.FromLLMMessage(msg))
}

// Deprecated: Use AppendLLMMessage instead.
func (tm *TreeManager) AppendFantasyMessage(msg fantasy.Message) (string, error) {
	return tm.AppendLLMMessage(msg)
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

// AppendExtensionData adds an extension data entry to the tree and persists it.
// Extensions use this to store custom state that survives across session restarts.
func (tm *TreeManager) AppendExtensionData(extType, data string) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry := NewExtensionDataEntry(tm.leafID, extType, data)
	if err := tm.appendAndPersist(entry); err != nil {
		return "", err
	}

	tm.leafID = entry.ID
	return entry.ID, nil
}

// AppendCompaction adds a compaction entry to the tree. The entry records
// the summary and the ID of the first entry that should be preserved in the
// LLM context. Messages before that entry are replaced by the summary.
func (tm *TreeManager) AppendCompaction(summary, firstKeptEntryID string, tokensBefore, tokensAfter, messagesRemoved int, readFiles, modifiedFiles []string) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry := NewCompactionEntry(tm.leafID, summary, firstKeptEntryID, tokensBefore, tokensAfter, messagesRemoved, readFiles, modifiedFiles)
	if err := tm.appendAndPersist(entry); err != nil {
		return "", err
	}

	tm.leafID = entry.ID
	return entry.ID, nil
}

// GetExtensionData returns all extension data entries matching the given type,
// walking the current branch from root to leaf. If extType is empty, all
// extension data entries on the branch are returned.
func (tm *TreeManager) GetExtensionData(extType string) []*ExtensionDataEntry {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.leafID == "" {
		return nil
	}

	branch := tm.getBranchLocked(tm.leafID)
	var results []*ExtensionDataEntry
	for _, entry := range branch {
		if e, ok := entry.(*ExtensionDataEntry); ok {
			if extType == "" || e.ExtType == extType {
				results = append(results, e)
			}
		}
	}
	return results
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
// conversation messages suitable for sending to the LLM. Compaction entries
// cause older messages to be replaced by the summary. Branch summaries are
// converted to user messages to provide context from abandoned branches.
// Also returns the latest model/provider settings encountered on the path.
func (tm *TreeManager) BuildContext() (messages []fantasy.Message, provider string, modelID string) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.leafID == "" {
		return nil, "", ""
	}

	// Walk from leaf to root collecting entries.
	branch := tm.getBranchLocked(tm.leafID)

	// Find the last compaction entry on this branch — it determines
	// which older messages are replaced by the summary.
	var lastCompaction *CompactionEntry
	for i := len(branch) - 1; i >= 0; i-- {
		if c, ok := branch[i].(*CompactionEntry); ok {
			lastCompaction = c
			break
		}
	}

	// If there is a compaction, inject the summary first.
	if lastCompaction != nil {
		messages = append(messages, fantasy.Message{
			Role: fantasy.MessageRoleSystem,
			Content: []fantasy.MessagePart{
				fantasy.TextPart{
					Text: fmt.Sprintf("[Conversation summary — earlier messages were compacted]\n\n%s", lastCompaction.Summary),
				},
			},
		})
	}

	// Determine whether to skip entries (everything before firstKeptEntryID).
	skipping := lastCompaction != nil
	for _, entry := range branch {
		// Once we reach the first kept entry, stop skipping.
		if skipping {
			entryID := tm.EntryID(entry)
			if entryID == lastCompaction.FirstKeptEntryID {
				skipping = false
			} else {
				continue
			}
		}

		switch e := entry.(type) {
		case *MessageEntry:
			msg, err := e.ToMessage()
			if err != nil {
				continue // skip malformed entries
			}
			msgs := msg.ToLLMMessages()
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

		case *CompactionEntry:
			// Already handled above (the last one on the branch).
			continue
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

// IsEmpty returns true if the session has no messages (only header).
func (tm *TreeManager) IsEmpty() bool {
	return tm.MessageCount() == 0
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

// GetContextEntryIDs returns the entry IDs corresponding to the fantasy
// messages returned by BuildContext, in the same order. Each entry ID maps
// to the session entry that produced the fantasy message at the same index.
// This is used by compaction to map a cut point index back to an entry ID.
//
// Note: A single MessageEntry produces at most one fantasy message. Branch
// summary entries also produce one message each. The returned slice has the
// same length as the messages slice from BuildContext (excluding the
// compaction summary system message, which has no entry ID — it gets the
// empty string "").
func (tm *TreeManager) GetContextEntryIDs() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.leafID == "" {
		return nil
	}

	branch := tm.getBranchLocked(tm.leafID)

	// Find the last compaction entry for skip logic.
	var lastCompaction *CompactionEntry
	for i := len(branch) - 1; i >= 0; i-- {
		if c, ok := branch[i].(*CompactionEntry); ok {
			lastCompaction = c
			break
		}
	}

	var ids []string

	// If there's a compaction summary injected, it has no entry ID.
	if lastCompaction != nil {
		ids = append(ids, "") // placeholder for the summary system message
	}

	skipping := lastCompaction != nil
	for _, entry := range branch {
		if skipping {
			entryID := tm.EntryID(entry)
			if entryID == lastCompaction.FirstKeptEntryID {
				skipping = false
			} else {
				continue
			}
		}

		switch e := entry.(type) {
		case *MessageEntry:
			msg, err := e.ToMessage()
			if err != nil {
				continue
			}
			msgs := msg.ToLLMMessages()
			for range msgs {
				ids = append(ids, e.ID)
			}

		case *BranchSummaryEntry:
			if e.Summary != "" {
				ids = append(ids, e.ID)
			}

		case *CompactionEntry:
			continue
		}
	}

	return ids
}

// GetLastCompaction returns the most recent CompactionEntry on the current
// branch, or nil if none exists. Used to carry forward file tracking.
func (tm *TreeManager) GetLastCompaction() *CompactionEntry {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.leafID == "" {
		return nil
	}

	branch := tm.getBranchLocked(tm.leafID)
	for i := len(branch) - 1; i >= 0; i-- {
		if c, ok := branch[i].(*CompactionEntry); ok {
			return c
		}
	}
	return nil
}

// --- Legacy bridge ---

// AddLLMMessages appends multiple LLM messages as entries. This is
// used when syncing from the agent's ConversationMessages after a step.
func (tm *TreeManager) AddLLMMessages(msgs []fantasy.Message) error {
	for _, msg := range msgs {
		if _, err := tm.AppendLLMMessage(msg); err != nil {
			return err
		}
	}
	return nil
}

// Deprecated: Use AddLLMMessages instead.
func (tm *TreeManager) AddFantasyMessages(msgs []fantasy.Message) error {
	return tm.AddLLMMessages(msgs)
}

// GetLLMMessages builds the context and returns just the messages.
// This satisfies the same conceptual role as the old Manager.GetMessages().
func (tm *TreeManager) GetLLMMessages() []fantasy.Message {
	msgs, _, _ := tm.BuildContext()
	return msgs
}

// Deprecated: Use GetLLMMessages instead.
func (tm *TreeManager) GetFantasyMessages() []fantasy.Message {
	return tm.GetLLMMessages()
}

// --- Internal helpers ---

// addEntryToIndex adds an entry to the in-memory indices.
func (tm *TreeManager) addEntryToIndex(entry any) {
	tm.entries = append(tm.entries, entry)

	id := tm.EntryID(entry)
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

// EntryID extracts the ID from any entry type.
func (tm *TreeManager) EntryID(entry any) string {
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
	case *ExtensionDataEntry:
		return e.ID
	case *CompactionEntry:
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
	case *ExtensionDataEntry:
		return e.ParentID
	case *CompactionEntry:
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
// Convention: ~/.kit/sessions/--<cwd-path>--/
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
