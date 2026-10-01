package codemode

import (
	"fmt"
	"maps"
	"sync"
)

// Store keeps JSON values that scripts save with store() and read with
// load(). One Store lives as long as its code mode tool, so values survive
// across script runs in the same Kit instance. Values are copied in and out
// as JSON text, so scripts never share live objects.
type Store struct {
	mu    sync.Mutex
	m     map[string]string
	bytes int
	limit int
}

// NewStore returns an empty store with the default size limit.
func NewStore() *Store {
	return &Store{m: map[string]string{}, limit: maxStoreBytes}
}

// Get returns the JSON text stored under key.
func (s *Store) Get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	return v, ok
}

// Set stores JSON text under key. It fails when the store would exceed its
// size limit.
func (s *Store) Set(key, jsonText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.m[key]
	next := s.bytes + len(jsonText)
	if exists {
		next -= len(old)
	} else {
		next += len(key)
	}
	if next > s.limit {
		return fmt.Errorf("store is full (%d MB limit)", s.limit>>20)
	}
	s.m[key] = jsonText
	s.bytes = next
	return nil
}

// Delete removes key.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.m[key]; ok {
		s.bytes -= len(v) + len(key)
		delete(s.m, key)
	}
}

// Snapshot returns a copy of all entries.
func (s *Store) Snapshot() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.m)
}

// Restore replaces all entries with m (used to reload persisted values).
// Entries beyond the size limit are dropped.
func (s *Store) Restore(m map[string]string) {
	s.mu.Lock()
	s.m = map[string]string{}
	s.bytes = 0
	s.mu.Unlock()
	for k, v := range m {
		_ = s.Set(k, v)
	}
}
