package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/kit/internal/message"
)

// EntryType identifies the kind of entry stored in a JSONL session file.
// Following pi's design, sessions are append-only JSONL files where each line
// is a typed entry linked by id/parent_id to form a tree structure.
type EntryType string

const (
	EntryTypeSession       EntryType = "session"
	EntryTypeMessage       EntryType = "message"
	EntryTypeModelChange   EntryType = "model_change"
	EntryTypeBranchSummary EntryType = "branch_summary"
	EntryTypeLabel         EntryType = "label"
	EntryTypeSessionInfo   EntryType = "session_info"
)

// CurrentVersion is the session format version for JSONL tree sessions.
const CurrentVersion = 3

// SessionHeader is the first line in a JSONL session file. It carries
// metadata about the session and does NOT participate in the tree structure
// (it has no ID or ParentID).
type SessionHeader struct {
	Type          EntryType `json:"type"`                     // always "session"
	Version       int       `json:"version"`                  // format version (3)
	ID            string    `json:"id"`                       // session UUID
	Timestamp     time.Time `json:"timestamp"`                // creation time
	Cwd           string    `json:"cwd"`                      // working directory
	ParentSession string    `json:"parent_session,omitempty"` // path to parent if forked
}

// Entry is the common structure shared by all tree entries (everything except
// the session header). Every entry has an ID, an optional ParentID (empty for
// root entries), a type tag, and a timestamp.
type Entry struct {
	Type      EntryType `json:"type"`
	ID        string    `json:"id"`
	ParentID  string    `json:"parent_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// MessageEntry stores a conversation message as a tree entry. The message
// content uses the same type-tagged parts format as the existing session
// persistence layer, enabling reuse of MarshalParts/UnmarshalParts.
type MessageEntry struct {
	Entry
	Role     string          `json:"role"`
	Parts    json.RawMessage `json:"parts"` // type-tagged parts array
	Model    string          `json:"model,omitempty"`
	Provider string          `json:"provider,omitempty"`
}

// ModelChangeEntry records a provider/model switch in the session tree.
type ModelChangeEntry struct {
	Entry
	Provider string `json:"provider"`
	ModelID  string `json:"model_id"`
}

// BranchSummaryEntry provides LLM-generated context from an abandoned branch.
// When the user navigates away from a branch, a summary of that branch's
// conversation is stored so the LLM retains context about what was explored.
type BranchSummaryEntry struct {
	Entry
	FromID  string `json:"from_id"` // leaf of the summarized branch
	Summary string `json:"summary"`
}

// LabelEntry bookmarks a specific entry with a user-defined label.
type LabelEntry struct {
	Entry
	TargetID string `json:"target_id"`
	Label    string `json:"label"`
}

// SessionInfoEntry stores a user-defined display name for the session.
type SessionInfoEntry struct {
	Entry
	Name string `json:"name"`
}

// GenerateEntryID creates a unique entry identifier (16 hex chars).
func GenerateEntryID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GenerateSessionID creates a unique session identifier (32 hex chars).
func GenerateSessionID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// NewEntry creates a base Entry with a generated ID and current timestamp.
func NewEntry(entryType EntryType, parentID string) Entry {
	return Entry{
		Type:      entryType,
		ID:        GenerateEntryID(),
		ParentID:  parentID,
		Timestamp: time.Now(),
	}
}

// --- Entry constructors ---

// NewMessageEntry creates a MessageEntry from a message.Message, linking it
// to the given parent entry in the tree.
func NewMessageEntry(parentID string, msg message.Message) (*MessageEntry, error) {
	parts, err := message.MarshalParts(msg.Parts)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message parts: %w", err)
	}
	return &MessageEntry{
		Entry:    NewEntry(EntryTypeMessage, parentID),
		Role:     string(msg.Role),
		Parts:    parts,
		Model:    msg.Model,
		Provider: msg.Provider,
	}, nil
}

// NewMessageEntryFromRaw creates a MessageEntry with pre-marshaled parts.
func NewMessageEntryFromRaw(parentID, role string, parts json.RawMessage, model, provider string) *MessageEntry {
	return &MessageEntry{
		Entry:    NewEntry(EntryTypeMessage, parentID),
		Role:     role,
		Parts:    parts,
		Model:    model,
		Provider: provider,
	}
}

// NewModelChangeEntry creates a ModelChangeEntry.
func NewModelChangeEntry(parentID, provider, modelID string) *ModelChangeEntry {
	return &ModelChangeEntry{
		Entry:    NewEntry(EntryTypeModelChange, parentID),
		Provider: provider,
		ModelID:  modelID,
	}
}

// NewBranchSummaryEntry creates a BranchSummaryEntry.
func NewBranchSummaryEntry(parentID, fromID, summary string) *BranchSummaryEntry {
	return &BranchSummaryEntry{
		Entry:   NewEntry(EntryTypeBranchSummary, parentID),
		FromID:  fromID,
		Summary: summary,
	}
}

// NewLabelEntry creates a LabelEntry.
func NewLabelEntry(parentID, targetID, label string) *LabelEntry {
	return &LabelEntry{
		Entry:    NewEntry(EntryTypeLabel, parentID),
		TargetID: targetID,
		Label:    label,
	}
}

// NewSessionInfoEntry creates a SessionInfoEntry.
func NewSessionInfoEntry(parentID, name string) *SessionInfoEntry {
	return &SessionInfoEntry{
		Entry: NewEntry(EntryTypeSessionInfo, parentID),
		Name:  name,
	}
}

// --- JSONL marshaling helpers ---

// MarshalEntry serializes any entry to a JSON line (no trailing newline).
func MarshalEntry(entry any) ([]byte, error) {
	return json.Marshal(entry)
}

// entryEnvelope is used for initial unmarshaling to determine the entry type.
type entryEnvelope struct {
	Type EntryType `json:"type"`
}

// UnmarshalEntry deserializes a JSON line into the appropriate entry type.
// Returns one of: *SessionHeader, *MessageEntry, *ModelChangeEntry,
// *BranchSummaryEntry, *LabelEntry, *SessionInfoEntry.
func UnmarshalEntry(data []byte) (any, error) {
	var env entryEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("failed to detect entry type: %w", err)
	}

	switch env.Type {
	case EntryTypeSession:
		var h SessionHeader
		if err := json.Unmarshal(data, &h); err != nil {
			return nil, fmt.Errorf("failed to unmarshal session header: %w", err)
		}
		return &h, nil

	case EntryTypeMessage:
		var e MessageEntry
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message entry: %w", err)
		}
		return &e, nil

	case EntryTypeModelChange:
		var e ModelChangeEntry
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal model_change entry: %w", err)
		}
		return &e, nil

	case EntryTypeBranchSummary:
		var e BranchSummaryEntry
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal branch_summary entry: %w", err)
		}
		return &e, nil

	case EntryTypeLabel:
		var e LabelEntry
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal label entry: %w", err)
		}
		return &e, nil

	case EntryTypeSessionInfo:
		var e SessionInfoEntry
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("failed to unmarshal session_info entry: %w", err)
		}
		return &e, nil

	default:
		return nil, fmt.Errorf("unknown entry type: %q", env.Type)
	}
}

// ToMessage converts a MessageEntry back to a message.Message by
// unmarshaling the type-tagged parts.
func (e *MessageEntry) ToMessage() (message.Message, error) {
	parts, err := message.UnmarshalParts(e.Parts)
	if err != nil {
		return message.Message{}, fmt.Errorf("failed to unmarshal parts: %w", err)
	}
	return message.Message{
		ID:        e.ID,
		Role:      message.MessageRole(e.Role),
		Parts:     parts,
		Model:     e.Model,
		Provider:  e.Provider,
		CreatedAt: e.Timestamp,
		UpdatedAt: e.Timestamp,
	}, nil
}
