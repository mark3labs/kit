package app

import (
	"sync"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// MessageStore is a thread-safe in-memory store for the conversation history.
// On-disk persistence is handled by the TreeManager at the app/SDK layer.
type MessageStore struct {
	mu       sync.RWMutex
	messages []kit.LLMMessage
}

// NewMessageStore creates an empty MessageStore.
func NewMessageStore() *MessageStore {
	return &MessageStore{}
}

// NewMessageStoreWithMessages creates a MessageStore pre-populated with the
// given messages. This is used when loading an existing session at startup.
func NewMessageStoreWithMessages(msgs []kit.LLMMessage) *MessageStore {
	cp := make([]kit.LLMMessage, len(msgs))
	copy(cp, msgs)
	return &MessageStore{messages: cp}
}

// Add appends a single message to the store.
func (s *MessageStore) Add(msg kit.LLMMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

// Replace replaces the entire message history with the given slice. This is
// used after an agent step returns the full updated conversation (including
// tool calls and results).
func (s *MessageStore) Replace(msgs []kit.LLMMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := make([]kit.LLMMessage, len(msgs))
	copy(cp, msgs)
	s.messages = cp
}

// GetAll returns a snapshot copy of the current message slice.
// The returned slice is safe to modify without affecting the store.
func (s *MessageStore) GetAll() []kit.LLMMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := make([]kit.LLMMessage, len(s.messages))
	copy(cp, s.messages)
	return cp
}

// Clear removes all messages from the store.
func (s *MessageStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = s.messages[:0]
}

// Len returns the number of messages currently in the store.
func (s *MessageStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.messages)
}
