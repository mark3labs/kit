package kit

import (
	"errors"
	"time"
)

// ErrBranchSummaryNotSupported is returned by SessionManager implementations
// that do not support collapsing a branch range into a summary entry.
var ErrBranchSummaryNotSupported = errors.New("session manager does not support branch summaries")

// ErrNoSession is returned by session-dependent operations ([Kit.Branch],
// [Kit.NavigateTo], [Kit.SummarizeBranch], [Kit.CollapseBranch],
// [Kit.SetSessionName], and the extension session API) when the Kit was
// created without a session manager. Callers can detect this condition with
// errors.Is.
var ErrNoSession = errors.New("no session available")

// SessionManager defines the contract for conversation storage backends.
// Implementations can use files (default), databases, cloud storage, etc.
//
// Implementations must be safe for concurrent use. During generation,
// AppendMessage is called incrementally from the agent's step-completion
// callback while read methods (GetMessages, GetCurrentBranch, etc.) may be
// called concurrently from the UI or extension goroutines.
//
// # Stability
//
// This interface is frozen for the v0.x line. New capability is added through
// optional interfaces that Kit type-asserts for — see [StepAppender] — rather
// than by adding methods here. Go interfaces have no default implementations,
// so a new method would break every external implementer at compile time with
// no deprecation window. Implementers can therefore rely on the method set
// below staying fixed.
type SessionManager interface {
	// AppendMessage adds a message to the current branch and returns its entry ID.
	// The entry ID is used for tree navigation and must be unique within the session.
	//
	// During generation, AppendMessage is called incrementally after each
	// completed agent step rather than in a batch at the end of the turn.
	// For tool-calling steps, the assistant message (containing tool_use parts)
	// and the tool-role message (containing tool_result parts) are appended
	// together as a pair. This ensures the session never contains an orphaned
	// tool call without its result, which would break subsequent LLM requests.
	//
	// The pairing guarantee is per-call, not atomic. Kit calls AppendMessage
	// once per message, so an implementation that writes to durable storage
	// produces one write per message. A process that dies between the two
	// writes of a tool-calling step leaves an orphaned tool call in storage.
	// Implement [StepAppender] to receive a whole step in one call and write
	// it atomically.
	AppendMessage(msg LLMMessage) (entryID string, err error)

	// GetMessages returns all messages on the current branch (from root to leaf),
	// including any compaction summaries at the appropriate positions.
	GetMessages() []LLMMessage

	// BuildContext returns the message history to send to the LLM, applying
	// compaction rules and branch summaries as needed.
	// Returns: messages, currentProvider, currentModelID
	BuildContext() (messages []LLMMessage, provider string, modelID string)

	// Branch moves the leaf pointer to the given entry ID, creating a branch point.
	// Subsequent AppendMessage calls extend from this new position.
	// entryID can be empty to reset to root (new conversation branch).
	Branch(entryID string) error

	// GetCurrentBranch returns the path from root to current leaf as entry metadata.
	// Used for UI display and navigation.
	GetCurrentBranch() []BranchEntry

	// GetChildren returns direct child entry IDs for a given parent entry.
	// Used to display branch points in the conversation tree.
	GetChildren(parentID string) []string

	// GetEntry returns a specific entry by ID, or nil if not found.
	GetEntry(entryID string) *BranchEntry

	// GetSessionID returns the unique session identifier (UUID).
	GetSessionID() string

	// GetSessionName returns the user-defined display name, or empty.
	GetSessionName() string

	// SetSessionName sets a display name for the session.
	SetSessionName(name string) error

	// GetCreatedAt returns when the session was created.
	GetCreatedAt() time.Time

	// IsPersisted returns true if this session writes to durable storage.
	IsPersisted() bool

	// AppendCompaction adds a compaction entry that summarizes older messages.
	// firstKeptEntryID is the ID of the first message to preserve in context.
	// readFiles and modifiedFiles track file changes for the compaction summary.
	AppendCompaction(summary string, firstKeptEntryID string,
		tokensBefore, tokensAfter int, messagesRemoved int, readFiles, modifiedFiles []string) (string, error)

	// GetLastCompaction returns the most recent compaction entry on the current
	// branch, or nil if none exists.
	GetLastCompaction() *CompactionEntry

	// AppendExtensionData stores custom extension data in the session tree.
	// Extensions use this to persist state across restarts.
	AppendExtensionData(extType, data string) (string, error)

	// GetExtensionData returns all extension data entries of the given type
	// on the current branch. If extType is empty, returns all extension data.
	GetExtensionData(extType string) []ExtensionDataEntry

	// AppendModelChange records a provider/model switch in the session.
	AppendModelChange(provider, modelID string) (string, error)

	// GetContextEntryIDs returns the entry IDs corresponding to the messages
	// returned by BuildContext, in the same order. Used by compaction to
	// determine which entries to summarize.
	GetContextEntryIDs() []string

	// AppendBranchSummary collapses the range from fromID to the current leaf
	// on the active branch into a single summary entry and returns the new
	// entry ID. It backs [Kit.CollapseBranch]. Managers that do not track
	// branch summaries should return [ErrBranchSummaryNotSupported].
	AppendBranchSummary(fromID, summary string) (entryID string, err error)

	// Close releases resources (database connections, file handles, etc.).
	Close() error
}

// BranchEntry represents a single node in the conversation tree.
// This is a SDK-friendly struct (not the internal entry types).
type BranchEntry struct {
	ID        string
	ParentID  string
	Type      EntryType // "message", "branch_summary", "model_change", "compaction", "extension_data"
	Role      string    // for messages: "user", "assistant", "system", "tool"
	Content   string    // text content or summary
	Model     string    // model used (for messages and model_change)
	Provider  string    // provider used
	Timestamp time.Time
	Children  []string // child entry IDs (for tree display)

	// RawParts contains the full typed content parts for structured access.
	// Only populated for message entries.
	RawParts []ContentPart
}

// EntryType identifies the kind of entry in the session tree.
type EntryType string

const (
	EntryTypeMessage       EntryType = "message"
	EntryTypeBranchSummary EntryType = "branch_summary"
	EntryTypeModelChange   EntryType = "model_change"
	EntryTypeCompaction    EntryType = "compaction"
	EntryTypeExtensionData EntryType = "extension_data"
)

// CompactionEntry represents a context compaction/summarization event.
type CompactionEntry struct {
	ID               string
	Summary          string
	FirstKeptEntryID string
	TokensBefore     int
	TokensAfter      int
	MessagesRemoved  int
	ReadFiles        []string
	ModifiedFiles    []string
	Timestamp        time.Time
}

// ExtensionDataEntry represents custom extension data stored in the session.
type ExtensionDataEntry struct {
	ID        string
	ExtType   string
	Data      string
	Timestamp time.Time
}

// StepAppender is an optional interface a [SessionManager] may implement to
// receive the messages of one agent step in a single call.
//
// Kit type-asserts for it at every site that persists more than one message.
// When a session manager implements it, Kit calls AppendStep instead of
// looping over [SessionManager.AppendMessage]; otherwise Kit falls back to the
// loop, so implementing it is entirely optional and adding it to an existing
// implementation is not a breaking change.
//
// # Why this exists
//
// A tool-calling step produces two messages: an assistant message carrying the
// tool_use parts, and a tool-role message carrying the matching tool_result
// parts. Kit's per-message persistence writes them separately, so a session
// manager backed by durable storage performs two independent writes. A process
// that dies between them leaves storage holding an assistant message whose
// tool call has no result — a conversation that LLM providers reject, which
// makes the session unresumable.
//
// AppendStep hands the whole step over at once so that an implementation can
// commit it as a single unit: one fsync, one database transaction, one object
// write. Implementations that persist durably should do exactly that.
//
// # Contract
//
// AppendStep returns one entry ID per input message, in the same order. A
// partial write must not be reported as success: return an error and leave
// storage unchanged, or commit every message. Implementations must be safe for
// concurrent use, like the rest of [SessionManager].
type StepAppender interface {
	AppendStep(msgs []LLMMessage) (entryIDs []string, err error)
}

// appendMessages persists a group of messages that belong together, using
// [StepAppender] when the session manager provides it and falling back to
// per-message appends when it does not.
//
// Errors are deliberately ignored, matching the historical behaviour of the
// call sites this replaces: a persistence failure must not abort a turn that
// has already produced model output.
func appendMessages(sm SessionManager, msgs []LLMMessage) {
	if sm == nil || len(msgs) == 0 {
		return
	}
	if sa, ok := sm.(StepAppender); ok {
		_, _ = sa.AppendStep(msgs)
		return
	}
	for _, msg := range msgs {
		_, _ = sm.AppendMessage(msg)
	}
}
