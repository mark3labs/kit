package kit

// This file exposes a handful of internal accessors to the external kit_test
// package. Because it ends in _test.go it is only compiled during testing and
// is therefore not part of the public SDK surface.

// ConfigValueIsSetForTest reports whether key is explicitly set in this Kit's
// isolated configuration store. Used by tests to assert the tri-state
// precedence contract per-instance.
func (m *Kit) ConfigValueIsSetForTest(key string) bool { return m.v.IsSet(key) }

// ConfigStringForTest returns the string value of key from this Kit's isolated
// configuration store.
func (m *Kit) ConfigStringForTest(key string) string { return m.v.GetString(key) }

// ConfigFloatForTest returns the float64 value of key from this Kit's isolated
// configuration store.
func (m *Kit) ConfigFloatForTest(key string) float64 { return m.v.GetFloat64(key) }

// ConfigBoolForTest returns the bool value of key from this Kit's isolated
// configuration store.
func (m *Kit) ConfigBoolForTest(key string) bool { return m.v.GetBool(key) }

// ConfigStringSliceForTest returns the string slice value of key from this
// Kit's isolated configuration store.
func (m *Kit) ConfigStringSliceForTest(key string) []string {
	return m.v.GetStringSlice(key)
}

// AdjustPostCompactionTokensForTest exposes adjustPostCompactionTokens to the
// external kit_test package.
func AdjustPostCompactionTokensForTest(lastInputTokens, originalTokens, compactedTokens int) int {
	return adjustPostCompactionTokens(lastInputTokens, originalTokens, compactedTokens)
}

// SetLastInputTokensForTest sets the API-reported token baseline, simulating
// a completed API turn.
func (m *Kit) SetLastInputTokensForTest(n int) {
	m.lastInputTokensMu.Lock()
	m.lastInputTokens = n
	m.lastInputTokensMu.Unlock()
}

// LastInputTokensForTest returns the current API-reported token baseline.
func (m *Kit) LastInputTokensForTest() int {
	m.lastInputTokensMu.RLock()
	defer m.lastInputTokensMu.RUnlock()
	return m.lastInputTokens
}

// PersistAndEmitCompactionForTest exposes persistAndEmitCompaction so tests
// can exercise the post-compaction token adjustment without an LLM call.
func (m *Kit) PersistAndEmitCompactionForTest(summary, firstKeptEntryID string, originalTokens, compactedTokens, messagesRemoved int) error {
	return m.persistAndEmitCompaction(summary, firstKeptEntryID, originalTokens, compactedTokens, messagesRemoved, nil, nil)
}
