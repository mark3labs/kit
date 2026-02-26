package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/message"
)

// Session represents a complete conversation session with metadata.
// It stores all messages exchanged during a conversation along with
// contextual information about the session such as the provider, model,
// and timestamps. Sessions can be saved to and loaded from JSON files
// for persistence across program runs.
type Session struct {
	// Version indicates the session format version for compatibility
	Version string `json:"version"`
	// CreatedAt is the timestamp when the session was first created
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the session was last modified
	UpdatedAt time.Time `json:"updated_at"`
	// Metadata contains contextual information about the session
	Metadata Metadata `json:"metadata"`
	// Messages is the ordered list of all messages in this session, stored
	// as custom content blocks (crush-style). Each message contains a
	// heterogeneous Parts slice serialized as type-tagged JSON.
	Messages []message.Message `json:"messages"`
}

// Metadata contains session metadata that provides context about the
// environment and configuration used during the conversation.
type Metadata struct {
	// KitVersion is the version of KIT used for this session
	KitVersion string `json:"kit_version"`
	// Provider is the LLM provider used (e.g., "anthropic", "openai", "gemini")
	Provider string `json:"provider"`
	// Model is the specific model identifier used for the conversation
	Model string `json:"model"`
}

// NewSession creates a new session with default values.
func NewSession() *Session {
	return &Session{
		Version:   "2.0",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []message.Message{},
		Metadata:  Metadata{},
	}
}

// AddMessage adds a message to the session.
func (s *Session) AddMessage(msg message.Message) {
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if msg.UpdatedAt.IsZero() {
		msg.UpdatedAt = time.Now()
	}

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// SetMetadata sets the session metadata.
func (s *Session) SetMetadata(metadata Metadata) {
	s.Metadata = metadata
	s.UpdatedAt = time.Now()
}

// sessionJSON is the on-disk format with parts serialized as JSON strings.
type sessionJSON struct {
	Version   string        `json:"version"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Metadata  Metadata      `json:"metadata"`
	Messages  []messageJSON `json:"messages"`
}

type messageJSON struct {
	ID        string          `json:"id"`
	Role      string          `json:"role"`
	Parts     json.RawMessage `json:"parts"` // type-tagged JSON array
	Model     string          `json:"model,omitempty"`
	Provider  string          `json:"provider,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// SaveToFile saves the session to a JSON file.
func (s *Session) SaveToFile(filePath string) error {
	s.UpdatedAt = time.Now()

	sj := sessionJSON{
		Version:   s.Version,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		Metadata:  s.Metadata,
		Messages:  make([]messageJSON, len(s.Messages)),
	}

	for i, msg := range s.Messages {
		parts, err := message.MarshalParts(msg.Parts)
		if err != nil {
			return fmt.Errorf("failed to marshal parts for message %s: %w", msg.ID, err)
		}
		sj.Messages[i] = messageJSON{
			ID:        msg.ID,
			Role:      string(msg.Role),
			Parts:     parts,
			Model:     msg.Model,
			Provider:  msg.Provider,
			CreatedAt: msg.CreatedAt,
			UpdatedAt: msg.UpdatedAt,
		}
	}

	data, err := json.MarshalIndent(sj, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %v", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// LoadFromFile loads a session from a JSON file.
func LoadFromFile(filePath string) (*Session, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %v", err)
	}

	var sj sessionJSON
	if err := json.Unmarshal(data, &sj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %v", err)
	}

	session := &Session{
		Version:   sj.Version,
		CreatedAt: sj.CreatedAt,
		UpdatedAt: sj.UpdatedAt,
		Metadata:  sj.Metadata,
		Messages:  make([]message.Message, len(sj.Messages)),
	}

	for i, mj := range sj.Messages {
		parts, err := message.UnmarshalParts(mj.Parts)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal parts for message %s: %w", mj.ID, err)
		}
		session.Messages[i] = message.Message{
			ID:        mj.ID,
			Role:      message.MessageRole(mj.Role),
			Parts:     parts,
			Model:     mj.Model,
			Provider:  mj.Provider,
			CreatedAt: mj.CreatedAt,
			UpdatedAt: mj.UpdatedAt,
		}
	}

	return session, nil
}

// ConvertFromFantasyMessage converts a fantasy.Message to a message.Message.
// This function bridges between the fantasy message format and the
// session's internal message format for persistence.
func ConvertFromFantasyMessage(msg fantasy.Message) message.Message {
	return message.FromFantasyMessage(msg)
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return "msg_" + hex.EncodeToString(bytes)
}
