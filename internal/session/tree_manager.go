package session

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/log"

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

	// writer is a buffered writer wrapping file. Writes go through this
	// buffer and are flushed to disk at explicit sync points (after each
	// public Append* call, in Close, etc.) to reduce syscall overhead.
	writer *bufio.Writer

	// lockRelease drops the process-level exclusive lock on filePath. It is
	// nil for in-memory sessions and after Close. The lock makes "one
	// process owns one session file" true: a second kit process that tries
	// to open the same transcript fails instead of interleaving appends.
	lockRelease func()

	// persistFailed marks a session whose file could not be rolled back to
	// a consistent state after a failed append. The in-memory tree and the
	// file can no longer be kept in agreement, so every append path refuses
	// to persist rather than silently fork the transcript.
	persistFailed bool

	// cleanPath is filepath.Clean(filePath), the lock table's key. It is
	// empty for in-memory sessions.
	cleanPath string

	// syncHook, when set, replaces file.Sync in AppendStep. Test-only: it
	// lets a test fail the step after the bytes reached the file, which is
	// the case the rollback path exists for.
	syncHook func() error

	// llmCache holds the decoded LLM messages of each MessageEntry. It has
	// its own mutex because it is filled lazily by readers holding only
	// mu.RLock. See llmMessagesLocked.
	llmCacheMu sync.Mutex
	llmCache   map[*MessageEntry][]fantasy.Message
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
		cleanPath:  filepath.Clean(filePath),
	}

	// Create the file and write the header.
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create session file: %w", err)
	}
	tm.file = f
	tm.writer = bufio.NewWriter(f)

	if err := tm.writeEntry(&header); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to write session header: %w", err)
	}
	if err := tm.flushLocked(); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to flush session header: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("sync new session file: %w", err)
	}
	if err := syncSessionDir(sessionDir); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("sync new session directory: %w", err)
	}

	// Claim the file before handing the session out, so a parallel run
	// cannot open it while this one is still empty.
	release, err := acquireSessionLock(filePath)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	tm.lockRelease = release

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
	newTm.writer = bufio.NewWriter(f)

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
				Entry:    cloneEntry(e.Entry, newID, prevNewID),
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
				Entry:    cloneEntry(e.Entry, newID, prevNewID),
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
				Entry:    cloneEntry(e.Entry, newID, prevNewID),
				TargetID: newTargetID,
				Label:    e.Label,
			}

		case *SessionInfoEntry:
			newEntry = &SessionInfoEntry{
				Entry: cloneEntry(e.Entry, newID, prevNewID),
				Name:  e.Name,
			}
			newTm.sessionName = e.Name

		case *ExtensionDataEntry:
			newEntry = &ExtensionDataEntry{
				Entry:   cloneEntry(e.Entry, newID, prevNewID),
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
				Entry:   cloneEntry(e.Entry, newID, prevNewID),
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
				Entry:            cloneEntry(e.Entry, newID, prevNewID),
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

	// Flush all buffered writes from the fork in a single syscall.
	if err := newTm.flushLocked(); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to flush forked session: %w", err)
	}

	// Set the leaf to the last entry in the new session.
	newTm.leafID = prevNewID

	return newTm, nil
}

// cloneEntry builds the base envelope for an entry copied into a forked
// session: it keeps the original type and timestamp but assigns the fresh ID
// and the remapped parent so the copy chains sequentially in the new tree.
// Every per-type case in ForkBranch goes through this helper, so a new base
// field on Entry needs exactly one edit here.
func cloneEntry(old Entry, newID, parentID string) Entry {
	return Entry{
		Type:      old.Type,
		ID:        newID,
		ParentID:  parentID,
		Timestamp: old.Timestamp,
	}
}

// SetParentLink records a parent session reference (and, optionally, the
// originating subagent task) in the session header. It is used when a
// subagent-backed session is created from a session-backed parent so viewers
// can navigate the parent/child session tree.
//
// For persisted sessions the header rewrite is atomic: the updated header
// plus all existing entries are written to a temp file which then replaces
// the original via rename, so a partial write can never corrupt the session.
// The session file lock is held for the whole rewrite except the rename
// itself (see below), and every error path either re-claims it or reports
// losing it, so the session never keeps appending unlocked. Sessions are
// typically freshly created when this is called, so the entry list is small
// (usually empty). For in-memory sessions the header is updated in memory
// only.
func (tm *TreeManager) SetParentLink(parentSessionPath, parentSessionID, subagentTask string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.filePath != "" {
		done, err := tm.reserveRewrite()
		if err != nil {
			return err
		}
		defer done()
		if pathMu := tm.appendPathMu(); pathMu != nil {
			pathMu.Lock()
			defer pathMu.Unlock()
		}
	}

	tm.header.ParentSession = parentSessionPath
	tm.header.ParentSessionID = parentSessionID
	if subagentTask != "" {
		tm.header.SubagentTask = subagentTask
	}

	if tm.file == nil {
		return nil // in-memory session: header updated in place
	}

	// The file lock stays held for everything up to the rename. The rename
	// itself is the one step that cannot hold it: on Windows, MoveFileEx
	// fails with a sharing violation while any handle is open on the source
	// or the replaced target (os.OpenFile handles never carry
	// FILE_SHARE_DELETE), so both the lock handle and the append handle must
	// be closed first. The lock is therefore dropped only here and
	// re-claimed before any append handle is opened again — on every error
	// path below the session either keeps its lock or reports that it lost
	// it and refuses to continue appending.

	// Flush anything buffered so the on-disk file is complete before rewrite.
	if err := tm.flushLocked(); err != nil {
		return fmt.Errorf("failed to flush session before header rewrite: %w", err)
	}

	// Write the updated header plus all existing entries to a temp file
	// first. The original file is never truncated, so a failure at any point
	// before the final rename leaves it fully intact.
	tmpPath := tm.filePath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp session file: %w", err)
	}
	w := bufio.NewWriter(tmpFile)
	writeLine := func(v any) error {
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal entry: %w", err)
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		return w.WriteByte('\n')
	}
	rewriteErr := writeLine(&tm.header)
	for _, entry := range tm.entries {
		if rewriteErr != nil {
			break
		}
		rewriteErr = writeLine(entry)
	}
	if rewriteErr == nil {
		rewriteErr = w.Flush()
	}
	if rewriteErr == nil {
		// Force data to stable storage before the rename so a host crash
		// cannot leave a renamed-but-empty file on some filesystems.
		rewriteErr = tmpFile.Sync()
	}
	if closeErr := tmpFile.Close(); rewriteErr == nil && closeErr != nil {
		rewriteErr = closeErr
	}
	if rewriteErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp session file: %w", rewriteErr)
	}

	// Close the current handle before the rename (required on Windows). The
	// lock handle is separate and unaffected. A close failure leaves the
	// original file and its lock intact; the append handle may be in an
	// unknown state, so reopen it best effort before reporting.
	if err := tm.file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		tm.file = nil
		tm.writer = nil
		if f, reopenErr := os.OpenFile(tm.filePath, os.O_WRONLY|os.O_APPEND, 0644); reopenErr == nil {
			tm.file = f
			tm.writer = bufio.NewWriter(f)
		}
		return fmt.Errorf("failed to close session file for header rewrite: %w", err)
	}
	tm.file = nil
	tm.writer = nil

	// From here the session is unlocked. No append handle exists and none is
	// opened until the lock is back, so the session itself cannot write into
	// the window; the re-claims below fail loudly if another process took
	// the lock in the meantime.
	if tm.lockRelease != nil {
		tm.lockRelease()
		tm.lockRelease = nil
	}

	if err := os.Rename(tmpPath, tm.filePath); err != nil {
		_ = os.Remove(tmpPath)
		// The original inode still backs the path, so it can be locked again.
		// Reclaim the lock BEFORE any append handle is reopened: if the lock
		// is gone, the session must not continue appending, and it must not
		// keep a live handle that invites exactly that.
		release, lockErr := acquireSessionLockForRewrite(tm.filePath, true)
		if lockErr != nil {
			tm.persistFailed = true
			return fmt.Errorf("failed to replace session file: %w (and the session file lock could not be re-claimed: %v; the session refuses further appends)", err, lockErr)
		}
		tm.lockRelease = release
		if f, reopenErr := os.OpenFile(tm.filePath, os.O_WRONLY|os.O_APPEND, 0644); reopenErr == nil {
			tm.file = f
			tm.writer = bufio.NewWriter(f)
		}
		return fmt.Errorf("failed to replace session file: %w", err)
	}

	// The path now points at the rewritten file. Re-claim the lock before
	// anything can append to it. A same-process opener that joined during
	// the window already re-created the table entry; joining that entry is
	// exactly right, because its lock guards the inode the path now has.
	release, err := acquireSessionLockForRewrite(tm.filePath, true)
	if err != nil {
		tm.persistFailed = true
		return fmt.Errorf("session file lock could not be re-claimed after the header rewrite (%v); the session refuses further appends", err)
	}
	tm.lockRelease = release

	f, err := os.OpenFile(tm.filePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to reopen session file after header rewrite: %w", err)
	}
	tm.file = f
	tm.writer = bufio.NewWriter(f)
	if err := syncSessionDir(filepath.Dir(tm.filePath)); err != nil {
		tm.persistFailed = true
		return fmt.Errorf("sync session directory after rewrite: %w", err)
	}
	return nil
}

// OpenTreeSession opens an existing JSONL session file.
//
// The session file is locked for exclusive use before anything is read, so
// two processes cannot load the same transcript and then append to it
// concurrently. Reopening finishes with a repair pass: tool calls left
// unanswered by a stopped process get synthetic results, so the session is
// resumable (see repair.go).
func OpenTreeSession(path string) (*TreeManager, error) {
	// Claim the file before parsing. The handle opened for the lock is
	// separate from the append handle opened below, and both outlive this
	// function: Close releases the lock.
	release, err := acquireSessionLock(path)
	if err != nil {
		return nil, err
	}
	tm, err := openTreeSessionLocked(path)
	if err != nil {
		release()
		return nil, err
	}
	tm.lockRelease = release
	return tm, nil
}

// openTreeSessionLocked is the body of OpenTreeSession, called with the
// session file already locked.
func openTreeSessionLocked(path string) (*TreeManager, error) {
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
		cleanPath:  filepath.Clean(path),
	}

	// Split lines straight out of the file buffer. The previous
	// string(data) + bufio.Reader + []byte(line) path copied every byte up to
	// three more times, which on a 35MB session with inline images was a
	// large share of the open time. json.Unmarshal copies what it keeps
	// (including json.RawMessage), so entries do not pin this buffer.
	lineNum := 0
	// tornOffset records where a torn final line starts, so the fragment can
	// be cut at exactly that byte. Truncating to the last newline is not
	// enough: a malformed line that ends with a newline (a torn fragment an
	// older write appended onto, or any complete but corrupt final entry)
	// would otherwise survive the trim and become a middle line — a state
	// the parser rejects — on the next append.
	tornOffset := int64(-1)
	for rest := data; len(rest) > 0; {
		lineStart := int64(len(data) - len(rest))
		var line []byte
		line, rest, _ = bytes.Cut(rest, []byte{'\n'})
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		lineNum++

		entry, err := UnmarshalEntry(line)
		if err != nil {
			// A line written by a newer kit version is complete, valid JSON:
			// it is a forward-compatibility case, never a torn write, and it
			// must not be dropped.
			if errors.Is(err, ErrUnknownEntryType) {
				return nil, fmt.Errorf("line %d: %w", lineNum, err)
			}
			if len(rest) == 0 {
				// A torn final write: the process died mid-append, so the
				// last line is a JSON prefix. Everything before it is a
				// complete transcript; drop the fragment instead of failing
				// the whole session. A malformed line that is NOT the last
				// one is real corruption and stays an error.
				if lineNum == 1 {
					// The header itself did not survive; without it the file
					// has no session ID, cwd or version, and appends would
					// build a transcript with no head line. Keep the failure
					// loud: the header is one line, cheap to recover by
					// starting a fresh session and forking nothing.
					return nil, fmt.Errorf("line 1: session header is torn (%w); the file cannot be opened", err)
				}
				tornOffset = lineStart
				log.Warn("session: dropping a torn final line left by a stopped process",
					"path", path, "line", lineNum, "error", err)
				continue
			}
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

	// Validate tree integrity and log diagnostics
	tm.LogTreeDiagnostics()

	if tornOffset >= 0 {
		// The dropped fragment must not resurface on the next append, and a
		// repair entry appended later must not grow a JSON prefix into a
		// corrupt transcript. Failing the open is deliberate: appending to a
		// file that still holds the fragment would corrupt the session on
		// the next open, which is exactly the state this repair exists to
		// prevent.
		if err := os.Truncate(path, tornOffset); err != nil {
			return nil, fmt.Errorf("failed to trim torn final line: %w", err)
		}
	}

	// Open file for appending.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file for append: %w", err)
	}
	tm.file = f
	tm.writer = bufio.NewWriter(f)

	// Close the crash window: a step written as two appends (assistant tool
	// calls, then their results) may have stopped in between. Without this
	// the session opens holding tool calls with no results, which providers
	// reject. The repair appends through the handle opened above, so the
	// synthetic results are persisted with the rest of the session.
	if _, err := tm.repairInterruptedToolCalls(); err != nil {
		log.Warn("session: could not repair interrupted tool calls", "path", path, "error", err)
	}

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

	// Validate parent chain before appending to detect/prevent cycles
	// that could be caused by external file corruption or race conditions.
	if err := tm.validateParentChainLocked(tm.leafID, ""); err != nil {
		return "", fmt.Errorf("parent chain validation failed: %w", err)
	}

	entry, err := NewMessageEntry(tm.leafID, msg)
	if err != nil {
		return "", err
	}

	if err := tm.appendAndPersist(entry); err != nil {
		return "", err
	}
	if err := tm.flushLocked(); err != nil {
		return "", fmt.Errorf("failed to flush message: %w", err)
	}

	tm.leafID = entry.ID
	return entry.ID, nil
}

// AppendLLMMessage converts an LLM message and appends it.
func (tm *TreeManager) AppendLLMMessage(msg fantasy.Message) (string, error) {
	return tm.AppendMessage(message.FromLLMMessage(msg))
}

// AppendStep persists one agent step — the group of messages a single model
// request produced — with one buffered write, one flush, and one fsync.
//
// A tool-calling step produces an assistant message carrying tool calls and
// a tool-role message carrying the matching results. Persisting them as one
// unit closes the crash window that would otherwise leave storage with a
// tool call and no result (providers reject that transcript, which makes the
// session unresumable). Kit calls this instead of looping over
// [TreeManager.AppendMessage] whenever the messages belong together; see
// also repairInterruptedToolCalls, which covers steps written by older
// versions that persisted per message.
//
// Either every message is appended or none is: serialization happens for all
// messages before the first byte is written, and a failure anywhere in the
// write, flush, or sync is rolled back — the file is truncated to its
// pre-append offset and the buffered tail discarded, while the indices and
// the leaf pointer are only advanced after the whole step reached storage.
// If the rollback itself fails, the manager refuses all further appends: the
// file and the in-memory tree can no longer be kept in agreement, and silent
// divergence would fork the transcript.
//
// ctx is accepted for signature compatibility with the durable-backend
// contract and is not used for cancellation: a finished step must survive
// the turn that produced it, even when the turn was cancelled (see the
// Cancellation section on kit's StepAppender).
func (tm *TreeManager) AppendStep(ctx context.Context, msgs []fantasy.Message) ([]string, error) {
	if len(msgs) == 0 {
		return nil, nil
	}
	_ = ctx // steps survive their turn; cancellation must not drop them

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.persistFailed {
		return nil, fmt.Errorf("session file could not be rolled back after an earlier failed write; appends are refused")
	}

	// Validate the chain the step will extend once, before any work.
	if err := tm.validateParentChainLocked(tm.leafID, ""); err != nil {
		return nil, fmt.Errorf("parent chain validation failed: %w", err)
	}

	// Phase 1 — build and serialize every entry. Nothing is written and no
	// index changes until every message has converted cleanly, so a bad
	// message cannot leave a half-written step behind.
	entries := make([]*MessageEntry, len(msgs))
	lines := make([][]byte, len(msgs))
	parent := tm.leafID
	for i, msg := range msgs {
		entry, err := NewMessageEntry(parent, message.FromLLMMessage(msg))
		if err != nil {
			return nil, err
		}
		line, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal step message %d: %w", i, err)
		}
		entries[i] = entry
		lines[i] = line
		parent = entry.ID
	}

	if tm.writer == nil {
		if tm.filePath != "" {
			// A persisted session lost its file handle (closed, or a failed
			// rewrite that could not relock): the memory-only path would
			// silently drop the step from the transcript.
			return nil, fmt.Errorf("session file is not open; refusing to append to %q", tm.filePath)
		}
		// In-memory session: index updates are the whole persistence.
		ids := make([]string, len(entries))
		for i, entry := range entries {
			tm.addEntryToIndex(entry)
			ids[i] = entry.ID
		}
		tm.leafID = entries[len(entries)-1].ID
		return ids, nil
	}

	// Phases 2-4 — serialize against every other manager of the same file,
	// so no other handle can append (or roll back) between this step's seek
	// and its commit-or-rollback. The mutex is a leaf lock: nothing inside
	// these brackets takes another lock or calls into another manager.
	if pathMu := tm.appendPathMu(); pathMu != nil {
		pathMu.Lock()
		defer pathMu.Unlock()
	}

	// Phase 2 — record where the step starts, then write the serialized
	// lines. The rollback point is the file's END, not this handle's offset:
	// the per-file append mutex keeps every other handle out until the step
	// is committed or rolled back, so the end cannot move underneath it.
	// The buffer is empty here: every other append path flushes before
	// returning, and a failed step resets the writer (see rollback).
	startOffset, err := tm.file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to record the append position: %w", err)
	}
	for i, line := range lines {
		if _, err := tm.writer.Write(line); err != nil {
			return nil, tm.rollbackStep(startOffset, fmt.Errorf("failed to write step message %d: %w", i, err))
		}
		if err := tm.writer.WriteByte('\n'); err != nil {
			return nil, tm.rollbackStep(startOffset, fmt.Errorf("failed to write step message %d: %w", i, err))
		}
	}

	// Phase 3 — flush and fsync, so both lines of a tool-calling step reach
	// storage together.
	if err := tm.flushLocked(); err != nil {
		return nil, tm.rollbackStep(startOffset, fmt.Errorf("failed to flush step: %w", err))
	}
	syncErr := tm.file.Sync()
	if tm.syncHook != nil {
		syncErr = tm.syncHook()
	}
	if syncErr != nil {
		return nil, tm.rollbackStep(startOffset, fmt.Errorf("failed to sync step: %w", syncErr))
	}

	// Phase 4 — the step is on stable storage; now the in-memory view.
	ids := make([]string, len(entries))
	for i, entry := range entries {
		tm.addEntryToIndex(entry)
		ids[i] = entry.ID
	}
	tm.leafID = entries[len(entries)-1].ID
	return ids, nil
}

// rollbackStep undoes a partially persisted step: the file is cut back to
// startOffset and the writer is reset, which discards whatever the buffer
// still holds and clears its error state. The in-memory tree needs no
// repair — indices and leaf only advance after a step is fully persisted.
// If the file cannot be cut back, the manager is marked unusable: future
// appends would build on a file whose contents cannot be matched to the
// tree. Returns the original error, wrapped, for the caller to pass on.
func (tm *TreeManager) rollbackStep(startOffset int64, cause error) error {
	tm.writer.Reset(tm.file)
	if err := tm.file.Truncate(startOffset); err != nil {
		tm.persistFailed = true
		return fmt.Errorf("%w (rollback failed: %v; the session refuses further appends)", cause, err)
	}
	// Truncate does not reset the offset on newly created handles, which
	// do not use O_APPEND. Reset it to prevent a hole on the next write.
	if _, err := tm.file.Seek(startOffset, io.SeekStart); err != nil {
		tm.persistFailed = true
		return fmt.Errorf("%w (rollback seek failed: %v)", cause, err)
	}
	if err := tm.file.Sync(); err != nil {
		tm.persistFailed = true
		return fmt.Errorf("%w (rollback sync failed: %v)", cause, err)
	}
	return cause
}

// AppendModelChange records a model/provider change.
func (tm *TreeManager) AppendModelChange(provider, modelID string) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry := NewModelChangeEntry(tm.leafID, provider, modelID)
	if err := tm.appendAndPersist(entry); err != nil {
		return "", err
	}
	if err := tm.flushLocked(); err != nil {
		return "", fmt.Errorf("failed to flush model change: %w", err)
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
	if err := tm.flushLocked(); err != nil {
		return "", fmt.Errorf("failed to flush branch summary: %w", err)
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
	if err := tm.flushLocked(); err != nil {
		return "", fmt.Errorf("failed to flush label: %w", err)
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
	if err := tm.flushLocked(); err != nil {
		return "", fmt.Errorf("failed to flush session info: %w", err)
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
	if err := tm.flushLocked(); err != nil {
		return "", fmt.Errorf("failed to flush extension data: %w", err)
	}

	tm.leafID = entry.ID
	return entry.ID, nil
}

// AppendCompaction adds a compaction entry to the tree. The entry records
// the summary and the ID of the first entry that should be preserved in the
// LLM context. Messages before that entry are replaced by the summary.
//
// The compaction entry becomes a new "root" for the post-compaction branch
// with no parent (empty ParentID). This breaks the parent chain so that old
// compacted messages are no longer traversed when building context. The kept
// messages are explicitly collected via FirstKeptEntryID in BuildContext.
func (tm *TreeManager) AppendCompaction(summary, firstKeptEntryID string, tokensBefore, tokensAfter, messagesRemoved int, readFiles, modifiedFiles []string) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Validate that firstKeptEntryID exists if provided
	if firstKeptEntryID != "" {
		if _, ok := tm.index[firstKeptEntryID]; !ok {
			return "", fmt.Errorf("first kept entry %q does not exist", firstKeptEntryID)
		}
	}

	// The compaction entry has no parent, making it a new "root" for the
	// post-compaction branch. This ensures old compacted messages are not
	// traversed when walking from the current leaf.
	entry := NewCompactionEntry("", summary, firstKeptEntryID, tokensBefore, tokensAfter, messagesRemoved, readFiles, modifiedFiles)
	if err := tm.appendAndPersist(entry); err != nil {
		return "", err
	}
	if err := tm.flushLocked(); err != nil {
		return "", fmt.Errorf("failed to flush compaction: %w", err)
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
//
// Each returned message is a copy (see cloneLLMMessage), so callers,
// including SDK ContextPrepare hooks, may modify the result in place without
// corrupting the decoded-message cache behind it.
func (tm *TreeManager) BuildContext() (messages []fantasy.Message, provider string, modelID string) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	provider, modelID = tm.walkContextLocked(func(_ string, msgs []fantasy.Message) {
		for _, msg := range msgs {
			messages = append(messages, cloneLLMMessage(msg))
		}
	})
	return messages, provider, modelID
}

// cloneLLMMessage returns a copy of msg that shares no mutable memory with
// it: the Content slice, every ProviderOptions map, and FilePart.Data are
// copied. All other fields are values or immutable strings.
//
// The values inside a ProviderOptions map are copied shallowly. The cache
// never holds any: ToLLMMessages does not set provider options.
func cloneLLMMessage(msg fantasy.Message) fantasy.Message {
	msg.ProviderOptions = maps.Clone(msg.ProviderOptions)
	if msg.Content == nil {
		return msg
	}
	content := make([]fantasy.MessagePart, len(msg.Content))
	for i, part := range msg.Content {
		switch p := part.(type) {
		case fantasy.TextPart:
			p.ProviderOptions = maps.Clone(p.ProviderOptions)
			part = p
		case fantasy.ReasoningPart:
			p.ProviderOptions = maps.Clone(p.ProviderOptions)
			part = p
		case fantasy.FilePart:
			p.ProviderOptions = maps.Clone(p.ProviderOptions)
			p.Data = bytes.Clone(p.Data)
			part = p
		case fantasy.ToolCallPart:
			p.ProviderOptions = maps.Clone(p.ProviderOptions)
			part = p
		case fantasy.ToolResultPart:
			p.ProviderOptions = maps.Clone(p.ProviderOptions)
			part = p
		}
		content[i] = part
	}
	msg.Content = content
	return msg
}

// walkContextLocked is the single definition of which session entries make
// up the LLM context, and in which order. BuildContext and
// GetContextEntryIDs both use it, so a compaction cut-point index always
// maps back to the entry that produced the message at that index.
//
// visit is called once per contributing entry, in context order, with the
// entry ID ("" for the synthetic compaction-summary message) and the
// messages that entry contributes. The msgs slice may be shared with the
// decoded-message cache: visit must not modify it or its elements' Content.
//
// If there is a compaction, the summary comes first, then the preserved
// "kept" messages (chronologically before the compaction), then the
// post-compaction messages (chronologically after).
//
// Order matters: the kept messages must come BEFORE the post-compaction
// branch so the LLM sees the conversation in chronological order. If the
// kept messages were appended last, the latest user message in the current
// branch would be followed by an older kept user message, breaking the
// strict user/assistant alternation that providers expect and causing the
// model to respond as if the previous turn never happened.
//
// Returns the latest model/provider settings encountered. Caller must hold
// at least tm.mu.RLock.
func (tm *TreeManager) walkContextLocked(visit func(entryID string, msgs []fantasy.Message)) (provider string, modelID string) {
	if tm.leafID == "" {
		return "", ""
	}

	// Walk from leaf to root collecting entries.
	branch := tm.getBranchLocked(tm.leafID)

	// Find the last compaction entry on this branch — it determines
	// which older messages are replaced by the summary.
	var lastCompaction *CompactionEntry
	compactionIndex := -1
	for i, b := range slices.Backward(branch) {
		if c, ok := b.(*CompactionEntry); ok {
			lastCompaction = c
			compactionIndex = i
			break
		}
	}

	visitEntry := func(entry any) {
		switch e := entry.(type) {
		case *MessageEntry:
			if msgs := tm.llmMessagesLocked(e); len(msgs) > 0 {
				visit(e.ID, msgs)
			}
		case *BranchSummaryEntry:
			// Convert branch summary to a user message for context.
			if e.Summary != "" {
				visit(e.ID, []fantasy.Message{{
					Role: fantasy.MessageRoleUser,
					Content: []fantasy.MessagePart{
						fantasy.TextPart{Text: fmt.Sprintf("[Branch context: %s]", e.Summary)},
					},
				}})
			}
		case *ModelChangeEntry:
			provider = e.Provider
			modelID = e.ModelID
		}
		// CompactionEntry: an older compaction contributes nothing; the
		// latest one's summary is injected separately below.
	}

	// No compaction - process the entire branch normally.
	if lastCompaction == nil {
		for _, entry := range branch {
			visitEntry(entry)
		}
		return provider, modelID
	}

	visit("", []fantasy.Message{{
		Role: fantasy.MessageRoleSystem,
		Content: []fantasy.MessagePart{
			fantasy.TextPart{
				Text: fmt.Sprintf("[Conversation summary — earlier messages were compacted]\n\n%s", lastCompaction.Summary),
			},
		},
	}})

	// Step 1: the kept messages starting from FirstKeptEntryID. These are
	// not on the current branch (the compaction entry is a new root with no
	// parent), so iterate tm.entries in append order and stop at the
	// compaction entry itself; messages after it come from the branch walk.
	if lastCompaction.FirstKeptEntryID != "" {
		found := false
		for _, entry := range tm.entries {
			entryID := tm.EntryID(entry)
			if !found {
				if entryID != lastCompaction.FirstKeptEntryID {
					continue
				}
				found = true
			}
			if entryID == lastCompaction.ID {
				break
			}
			visitEntry(entry)
		}
	}

	// Step 2: entries on the current branch after the compaction entry
	// (post-compaction messages).
	for _, entry := range branch[compactionIndex+1:] {
		visitEntry(entry)
	}
	return provider, modelID
}

// llmMessagesLocked returns the LLM messages for a message entry, decoding
// its stored parts only on first use.
//
// Decoding is the dominant cost of building the context (JSON parts, often
// with base64 image data): ~100ms per call on a 35MB session. Entries are
// immutable once appended and never removed, so the result is cached for
// the TreeManager's lifetime, keyed by entry pointer. Entries that fail to
// decode are cached as nil and skipped, as before.
//
// Safe under tm.mu.RLock: concurrent readers are serialised by llmCacheMu.
// Two readers may decode the same entry at once; both results are equal
// and the later store wins, which is harmless.
func (tm *TreeManager) llmMessagesLocked(e *MessageEntry) []fantasy.Message {
	tm.llmCacheMu.Lock()
	msgs, ok := tm.llmCache[e]
	tm.llmCacheMu.Unlock()
	if ok {
		return msgs
	}

	if msg, err := e.ToMessage(); err == nil {
		msgs = msg.ToLLMMessages()
	}

	tm.llmCacheMu.Lock()
	if tm.llmCache == nil {
		tm.llmCache = make(map[*MessageEntry][]fantasy.Message)
	}
	tm.llmCache[e] = msgs
	tm.llmCacheMu.Unlock()
	return msgs
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

// Flush writes any buffered data to the underlying file.
func (tm *TreeManager) Flush() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.flushLocked()
}

// flushLocked writes buffered data to disk. Caller must hold the lock.
func (tm *TreeManager) flushLocked() error {
	if tm.writer != nil {
		return tm.writer.Flush()
	}
	return nil
}

// Close flushes any buffered writes, closes the underlying file handle, and
// releases the session file lock.
func (tm *TreeManager) Close() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.file != nil {
		// Flush buffered data before closing.
		if tm.writer != nil {
			_ = tm.writer.Flush()
			tm.writer = nil
		}
		// Push what is in the page cache to storage: after Close returns, a
		// power cut must not take the session's tail with it.
		_ = tm.file.Sync()
		err := tm.file.Close()
		tm.file = nil
		if tm.lockRelease != nil {
			tm.lockRelease()
			tm.lockRelease = nil
		}
		return err
	}
	if tm.lockRelease != nil {
		tm.lockRelease()
		tm.lockRelease = nil
	}
	return nil
}

// GetContextEntryIDs returns the entry IDs corresponding to the fantasy
// messages returned by BuildContext, in the same order. Each entry ID maps
// to the session entry that produced the fantasy message at the same index.
// This is used by compaction to map a cut point index back to an entry ID.
//
// A MessageEntry may produce more than one message; each of them gets the
// entry's ID. The returned slice has the same length as the messages slice
// from BuildContext. The compaction summary system message has no entry, so
// its position holds the empty string "".
func (tm *TreeManager) GetContextEntryIDs() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var ids []string
	tm.walkContextLocked(func(entryID string, msgs []fantasy.Message) {
		for range msgs {
			ids = append(ids, entryID)
		}
	})
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
	for _, b := range slices.Backward(branch) {
		if c, ok := b.(*CompactionEntry); ok {
			return c
		}
	}
	return nil
}

// GetLLMMessages builds the context and returns just the messages.
// This satisfies the same conceptual role as the old Manager.GetMessages().
func (tm *TreeManager) GetLLMMessages() []fantasy.Message {
	msgs, _, _ := tm.BuildContext()
	return msgs
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
	if tm.persistFailed {
		return fmt.Errorf("session file could not be rolled back after an earlier failed write; appends are refused")
	}
	if tm.filePath != "" && tm.file == nil {
		// A persisted session lost its file handle (closed, or a failed
		// rewrite that could not relock). Writing memory-only would drop the
		// entry from the transcript the next open reads, so refuse instead.
		return fmt.Errorf("session file is not open; refusing to append to %q", tm.filePath)
	}
	// Serialize against other managers of the same file (same leaf-lock rule
	// as AppendStep: syscalls only inside the mutex).
	if pathMu := tm.appendPathMu(); pathMu != nil {
		pathMu.Lock()
		defer pathMu.Unlock()
	}
	tm.addEntryToIndex(entry)
	if tm.file != nil {
		// Newly created handles do not use O_APPEND. Another manager can
		// have extended the file since this handle's last write.
		if _, err := tm.file.Seek(0, io.SeekEnd); err != nil {
			return fmt.Errorf("seek session end: %w", err)
		}
		if err := tm.writeEntry(entry); err != nil {
			return err
		}
		// Flush before releasing the shared append lock. A later rollback
		// must not truncate bytes that another writer still has buffered.
		return tm.flushLocked()
	}
	return nil
}

// writeEntry serializes an entry and appends it to the buffered writer.
// The data is not flushed to disk until flushLocked is called.
func (tm *TreeManager) writeEntry(entry any) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}
	if tm.writer != nil {
		if _, err := tm.writer.Write(data); err != nil {
			return err
		}
		return tm.writer.WriteByte('\n')
	}
	// Fallback for direct file writes (shouldn't happen in normal flow).
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
// It includes a depth limit to prevent infinite recursion in case of
// corrupted parent-child relationships.
func (tm *TreeManager) buildTreeNode(id string) *TreeNode {
	return tm.buildTreeNodeDepth(id, 0, make(map[string]bool))
}

// buildTreeNodeDepth is the internal implementation with depth tracking.
func (tm *TreeManager) buildTreeNodeDepth(id string, depth int, visited map[string]bool) *TreeNode {
	const maxDepth = 1000
	if depth > maxDepth {
		// Cycle or extremely deep tree detected, stop recursing
		return nil
	}
	if visited[id] {
		// Cycle detected, stop recursing
		return nil
	}

	entry, ok := tm.index[id]
	if !ok {
		return nil
	}

	visited[id] = true
	defer delete(visited, id)

	node := &TreeNode{
		Entry:    entry,
		ID:       id,
		ParentID: tm.entryParentID(entry),
	}

	for _, childID := range tm.childIndex[id] {
		child := tm.buildTreeNodeDepth(childID, depth+1, visited)
		if child != nil {
			node.Children = append(node.Children, child)
		}
	}

	return node
}

// --- Path conventions ---

// BareSessionKey is the sentinel passed in place of a working directory when
// Kit runs in bare mode. Bare sessions are not tied to a working directory —
// that is the point of the mode — so they all share one bucket and
// `--continue` resumes the last bare conversation from anywhere.
//
// [DefaultSessionDir] maps this sentinel to a directory that sits beside the
// cwd-keyed namespace rather than inside it, so no working directory can ever
// resolve to the bare bucket. See the comment there.
const BareSessionKey = "__bare__"

// bareSessionDirName is the on-disk home for bare sessions. It deliberately
// lives next to "sessions", not under it: every cwd-derived key is joined
// beneath "sessions", so keeping bare outside that subtree makes a collision
// structurally impossible rather than merely unlikely.
const bareSessionDirName = "bare-sessions"

// DefaultSessionDir returns the default session storage directory for a cwd.
// Convention: ~/.kit/sessions/<encoded-cwd>, where path separators are
// encoded as "--" with no leading or trailing dashes — e.g.
// /home/user/proj becomes home--user--proj. See encodeCwdForDir for the
// full encoding rules (including Windows path handling).
//
// The bare sentinel is special-cased to ~/.kit/bare-sessions. The encoding is
// lossy — "/__bare__" and "__bare__" both encode to "__bare__" — so routing
// bare sessions through it would let a project directory named /__bare__ share
// the bare bucket and have `--continue` resume the wrong conversation.
// Matching on the raw sentinel before encoding keeps the two namespaces apart:
// a real /__bare__ directory still gets ~/.kit/sessions/__bare__.
func DefaultSessionDir(cwd string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	if cwd == BareSessionKey {
		return filepath.Join(home, ".kit", bareSessionDirName)
	}
	return filepath.Join(home, ".kit", "sessions", encodeCwdForDir(cwd))
}

// encodeCwdForDir converts a working-directory path into a single, filesystem-
// safe directory name. Path separators are replaced with double dashes and
// characters that are illegal in Windows directory names — most importantly
// the colon that follows the drive letter (e.g. `C:\foo` → `C--foo`) — are
// stripped. The result is identical to the previous Unix-only encoding for
// paths that do not contain such characters, so existing session directories
// are preserved.
func encodeCwdForDir(cwd string) string {
	// Convert both `/` and `\` to double dashes so encoding is stable across
	// platforms and remains correct on Windows where `filepath.Separator`
	// would otherwise miss forward-slash style paths.
	safeCwd := strings.ReplaceAll(cwd, "\\", "--")
	safeCwd = strings.ReplaceAll(safeCwd, "/", "--")
	// Remove leading separator replacement.
	safeCwd = strings.TrimPrefix(safeCwd, "--")
	// Strip characters that are illegal in directory names on Windows
	// (`< > : " | ? *`). On Unix these characters are legal but rare in
	// practice; stripping them keeps the encoding portable.
	replacer := strings.NewReplacer(
		":", "",
		"<", "",
		">", "",
		"\"", "",
		"|", "",
		"?", "",
		"*", "",
	)
	return replacer.Replace(safeCwd)
}
