package kit

import (
	"time"
)

// SessionManager defines the contract for conversation storage backends.
// Implementations can use files (default), databases, cloud storage, etc.
//
// Implementations must be safe for concurrent use. During generation,
// AppendMessage is called incrementally from the agent's step-completion
// callback while read methods (GetMessages, GetCurrentBranch, etc.) may be
// called concurrently from the UI or extension goroutines.
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
