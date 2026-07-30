package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/session"
)

// ErrNoSession is returned by session mutation methods when no tree session
// is active. Callers that only need to know whether a session exists should
// prefer the ok result of SessionSnapshot.
var ErrNoSession = errors.New("no tree session active")

// EntryKind classifies a session tree entry without exposing the concrete
// persistence types from internal/session.
//
// Presentation layers switch on EntryKind instead of type-switching on
// *session.MessageEntry and friends, so the on-disk entry schema can evolve
// without breaking every consumer. Unrecognised entries carry
// EntryKindUnknown; consumers should always handle it.
type EntryKind string

const (
	// EntryKindUnknown is used for entries the app layer does not recognise.
	EntryKindUnknown EntryKind = ""
	// EntryKindMessage is a conversation message (user, assistant or tool).
	EntryKindMessage EntryKind = "message"
	// EntryKindModelChange records a provider/model switch.
	EntryKindModelChange EntryKind = "model_change"
	// EntryKindBranchSummary carries a summary of an abandoned branch.
	EntryKindBranchSummary EntryKind = "branch_summary"
	// EntryKindLabel bookmarks another entry with a user-defined label.
	EntryKindLabel EntryKind = "label"
	// EntryKindSessionInfo records the session's display name.
	EntryKindSessionInfo EntryKind = "session_info"
	// EntryKindExtensionData holds extension-defined persisted state.
	EntryKindExtensionData EntryKind = "extension_data"
	// EntryKindCompaction records an LLM-generated summary of older messages.
	EntryKindCompaction EntryKind = "compaction"
	// EntryKindSystemPrompt captures the system prompt used for a session.
	EntryKindSystemPrompt EntryKind = "system_prompt"
)

// TreeNodeView is an immutable value projection of a single node in the
// session entry tree. It carries everything a presentation layer needs to
// filter, search and render the node without reaching back into the session
// store or knowing the entry schema.
type TreeNodeView struct {
	// ID is the entry ID.
	ID string
	// ParentID is the parent entry ID, empty for root entries.
	ParentID string
	// Kind classifies the entry.
	Kind EntryKind
	// Role is the message role ("user", "assistant", "tool") for
	// EntryKindMessage nodes and empty for every other kind.
	Role string
	// Text is the kind-appropriate plain-text payload: message text for
	// messages, "provider/model" for model changes, the summary for branch
	// summaries and compactions, the label text for labels and the session
	// name for session info. Empty for kinds with no textual payload.
	//
	// Text is never truncated or collapsed — formatting is the caller's job.
	Text string
	// Label is the user-defined label bookmarking this entry, or empty.
	Label string
	// Children are the node's child entries, in insertion order.
	Children []TreeNodeView
}

// IsUserMessage reports whether the node is a message authored by the user.
func (n TreeNodeView) IsUserMessage() bool {
	return n.Kind == EntryKindMessage && n.Role == "user"
}

// SessionSnapshot is a point-in-time value view of the active tree session's
// metadata. It is a copy: mutating it has no effect on the session, and it
// does not go stale in a way that can corrupt state — callers re-read it
// whenever they need current values.
type SessionSnapshot struct {
	// ID is the session UUID.
	ID string
	// Name is the user-defined display name, empty if unnamed.
	Name string
	// FilePath is the JSONL file backing the session, empty when in-memory.
	FilePath string
	// Cwd is the working directory the session was created in.
	Cwd string
	// Created is the session creation timestamp.
	Created time.Time
	// LeafID is the current position in the entry tree.
	LeafID string
	// EntryCount is the total number of entries in the session.
	EntryCount int
	// MessageCount is the number of message entries in the session.
	MessageCount int
	// Persisted reports whether the session is backed by a file on disk.
	Persisted bool
}

// SessionSnapshot returns a value snapshot of the active tree session's
// metadata. ok is false when no tree session is configured, in which case the
// returned snapshot is the zero value.
func (a *App) SessionSnapshot() (SessionSnapshot, bool) {
	tm := a.opts.TreeSession
	if tm == nil {
		return SessionSnapshot{}, false
	}
	header := tm.GetHeader()
	return SessionSnapshot{
		ID:           header.ID,
		Name:         tm.GetSessionName(),
		FilePath:     tm.GetFilePath(),
		Cwd:          header.Cwd,
		Created:      header.Timestamp,
		LeafID:       tm.GetLeafID(),
		EntryCount:   tm.EntryCount(),
		MessageCount: tm.MessageCount(),
		Persisted:    tm.IsPersisted(),
	}, true
}

// SessionTree returns the session's entry tree projected into value nodes,
// rooted at the entries with no parent. Returns nil when no tree session is
// active.
//
// The projection is a deep copy taken under the session's read lock, so the
// result is safe to hold and walk while the session keeps mutating.
func (a *App) SessionTree() []TreeNodeView {
	tm := a.opts.TreeSession
	if tm == nil {
		return nil
	}
	roots := tm.GetTree()
	if len(roots) == 0 {
		return nil
	}
	out := make([]TreeNodeView, 0, len(roots))
	for _, root := range roots {
		out = append(out, treeNodeView(tm, root))
	}
	return out
}

// treeNodeView recursively projects a session tree node into a TreeNodeView.
func treeNodeView(tm *session.TreeManager, node *session.TreeNode) TreeNodeView {
	view := TreeNodeView{
		ID:       node.ID,
		ParentID: node.ParentID,
		Label:    tm.GetLabel(node.ID),
	}
	view.Kind, view.Role, view.Text = describeEntry(node.Entry)

	if len(node.Children) > 0 {
		view.Children = make([]TreeNodeView, 0, len(node.Children))
		for _, child := range node.Children {
			view.Children = append(view.Children, treeNodeView(tm, child))
		}
	}
	return view
}

// describeEntry maps a concrete session entry to its kind, role and textual
// payload. This is the single place in the codebase that needs to know the
// full set of entry types for presentation purposes.
func describeEntry(entry any) (kind EntryKind, role, text string) {
	switch e := entry.(type) {
	case *session.MessageEntry:
		return EntryKindMessage, e.Role, e.Text()
	case *session.ModelChangeEntry:
		return EntryKindModelChange, "", e.Provider + "/" + e.ModelID
	case *session.BranchSummaryEntry:
		return EntryKindBranchSummary, "", e.Summary
	case *session.CompactionEntry:
		return EntryKindCompaction, "", e.Summary
	case *session.LabelEntry:
		return EntryKindLabel, "", e.Label
	case *session.SessionInfoEntry:
		return EntryKindSessionInfo, "", e.Name
	case *session.ExtensionDataEntry:
		return EntryKindExtensionData, "", ""
	case *session.SystemPromptEntry:
		return EntryKindSystemPrompt, "", e.Content
	default:
		return EntryKindUnknown, "", ""
	}
}

// SessionHistory returns the conversation messages on the session's current
// branch, oldest first. Entries that are not messages, and messages that fail
// to decode, are skipped. Returns nil when no tree session is active.
//
// This is what the UI replays to rebuild the transcript after resuming or
// forking a session.
func (a *App) SessionHistory() []message.Message {
	tm := a.opts.TreeSession
	if tm == nil {
		return nil
	}
	branch := tm.GetBranch("")
	if len(branch) == 0 {
		return nil
	}
	out := make([]message.Message, 0, len(branch))
	for _, entry := range branch {
		me, ok := entry.(*session.MessageEntry)
		if !ok {
			continue
		}
		msg, err := me.ToMessage()
		if err != nil {
			continue
		}
		out = append(out, msg)
	}
	return out
}

// SetSessionName sets the session's display name, persisting it as a
// session_info entry. Returns ErrNoSession when no tree session is active.
func (a *App) SetSessionName(name string) error {
	tm := a.opts.TreeSession
	if tm == nil {
		return ErrNoSession
	}
	if _, err := tm.AppendSessionInfo(name); err != nil {
		return fmt.Errorf("append session info: %w", err)
	}
	return nil
}

// NewSession replaces the active tree session with a brand new one created in
// cwd, closing and flushing the old session. The in-memory message store is
// reset to the (empty) new session.
//
// Returns ErrNoSession when no tree session is active — callers that want to
// support the session-less mode should check SessionSnapshot first and clear
// their own state instead.
func (a *App) NewSession(cwd string) error {
	if a.opts.TreeSession == nil {
		return ErrNoSession
	}
	ts, err := session.CreateTreeSession(cwd)
	if err != nil {
		return fmt.Errorf("create tree session: %w", err)
	}
	a.SwitchTreeSession(ts)
	return nil
}

// ForkSession creates a new session in cwd containing the history up to and
// including targetID, then switches to it. The original session is left
// untouched on disk. Returns ErrNoSession when no tree session is active.
func (a *App) ForkSession(cwd, targetID string) error {
	tm := a.opts.TreeSession
	if tm == nil {
		return ErrNoSession
	}
	ts, err := tm.ForkToNewSession(cwd, targetID)
	if err != nil {
		return fmt.Errorf("fork session from entry %q: %w", targetID, err)
	}
	a.SwitchTreeSession(ts)
	return nil
}
