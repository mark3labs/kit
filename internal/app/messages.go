package app

import (
	"sync"

	"charm.land/fantasy"
	"github.com/mark3labs/mcphost/internal/session"
)

// MessageStore is a thread-safe store for the conversation history. It wraps a
// slice of fantasy.Message and optionally bridges to a session.Manager for
// on-disk persistence. Every mutation (Add, Replace, Clear) automatically
// persists to the session if one is configured.
type MessageStore struct {
	mu       sync.RWMutex
	messages []fantasy.Message
	session  *session.Manager // optional; may be nil
}

// NewMessageStore creates an empty MessageStore. If sessionManager is non-nil,
// all mutations are persisted via the manager.
func NewMessageStore(sessionManager *session.Manager) *MessageStore {
	return &MessageStore{
		session: sessionManager,
	}
}

// NewMessageStoreWithMessages creates a MessageStore pre-populated with the
// given messages. This is used when loading an existing session at startup.
// The messages are NOT written back to the session manager here — they are
// assumed to already be persisted.
func NewMessageStoreWithMessages(msgs []fantasy.Message, sessionManager *session.Manager) *MessageStore {
	cp := make([]fantasy.Message, len(msgs))
	copy(cp, msgs)
	return &MessageStore{
		messages: cp,
		session:  sessionManager,
	}
}

// Add appends a single message to the store and persists to session.
func (s *MessageStore) Add(msg fantasy.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, msg)
	s.persistLocked()
}

// Replace replaces the entire message history with the given slice and persists
// to session. This is used after an agent step returns the full updated
// conversation (including tool calls and results).
func (s *MessageStore) Replace(msgs []fantasy.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := make([]fantasy.Message, len(msgs))
	copy(cp, msgs)
	s.messages = cp
	s.persistLocked()
}

// GetAll returns a snapshot copy of the current message slice.
// The returned slice is safe to modify without affecting the store.
func (s *MessageStore) GetAll() []fantasy.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := make([]fantasy.Message, len(s.messages))
	copy(cp, s.messages)
	return cp
}

// Clear removes all messages from the store and persists the empty state to
// the session (if configured).
func (s *MessageStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = s.messages[:0]
	s.persistLocked()
}

// Len returns the number of messages currently in the store.
func (s *MessageStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.messages)
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// persistLocked writes the current message slice to the session manager.
// Must be called with s.mu held (write lock).
func (s *MessageStore) persistLocked() {
	if s.session == nil {
		return
	}
	// Errors are silently discarded — persistence is best-effort and should
	// not interrupt the user interaction.
	_ = s.session.ReplaceAllMessages(s.messages)
}
