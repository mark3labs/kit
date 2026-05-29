package kit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// Runtime context-file management (Issue #36)
// ---------------------------------------------------------------------------
//
// Project context files (AGENTS.md and friends) are normally auto-discovered
// during Kit.New() and injected into the system prompt. SDK consumers building
// multi-tenant chatbots often need to swap context per user/session at runtime
// without restarting the agent. The methods below provide that surface.
//
// Every mutation recomposes the system prompt and applies it to the underlying
// agent so the next turn sees the updated project context.

// AddContextFile registers a project context file (e.g. an AGENTS.md
// equivalent) on this Kit instance. The file does not need to exist on
// disk — Path is treated as an opaque identifier used both for de-duplication
// and for the "Instructions from: <Path>" header injected into the system
// prompt. If a context file with the same Path is already loaded the new
// content replaces it.
//
// Returns an error when cf is nil or has an empty Path. AddContextFile is
// safe to call from any goroutine.
func (m *Kit) AddContextFile(cf *ContextFile) error {
	if cf == nil {
		return fmt.Errorf("AddContextFile: context file is nil")
	}
	if cf.Path == "" {
		return fmt.Errorf("AddContextFile: context file path is required")
	}

	// Take a defensive copy so later mutations by the caller don't race with
	// the agent reading the composed prompt.
	stored := &ContextFile{
		Path:    cf.Path,
		Content: strings.TrimSpace(cf.Content),
	}

	m.runtimeMu.Lock()
	replaced := false
	for i, existing := range m.contextFiles {
		if existing.Path == stored.Path {
			m.contextFiles[i] = stored
			replaced = true
			break
		}
	}
	if !replaced {
		m.contextFiles = append(m.contextFiles, stored)
	}
	m.runtimeMu.Unlock()

	m.applyComposedSystemPrompt()
	return nil
}

// AddContextFileContent is a convenience wrapper around [Kit.AddContextFile]
// that builds the ContextFile from a path and inline content string. Use this
// when the context originates from a database, API response, or any other
// non-filesystem source.
func (m *Kit) AddContextFileContent(path, content string) (*ContextFile, error) {
	cf := &ContextFile{Path: path, Content: content}
	if err := m.AddContextFile(cf); err != nil {
		return nil, err
	}
	return cf, nil
}

// LoadAndAddContextFile reads a file from disk and registers it as a project
// context file via [Kit.AddContextFile]. The absolute path is stored on the
// resulting ContextFile.
func (m *Kit) LoadAndAddContextFile(path string) (*ContextFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadAndAddContextFile: %w", err)
	}
	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		abs = path
	}
	cf := &ContextFile{
		Path:    abs,
		Content: strings.TrimSpace(string(data)),
	}
	if err := m.AddContextFile(cf); err != nil {
		return nil, err
	}
	return cf, nil
}

// RemoveContextFile removes the context file with the given path and
// recomposes the system prompt. Returns true when a matching file was found
// and removed, false otherwise.
func (m *Kit) RemoveContextFile(path string) bool {
	m.runtimeMu.Lock()
	found := false
	for i, cf := range m.contextFiles {
		if cf.Path == path {
			m.contextFiles = append(m.contextFiles[:i], m.contextFiles[i+1:]...)
			found = true
			break
		}
	}
	m.runtimeMu.Unlock()

	if !found {
		return false
	}
	m.applyComposedSystemPrompt()
	return true
}

// SetContextFiles replaces the active context-file set with the provided
// slice. Pass nil or an empty slice to clear all context. The system prompt
// is recomposed and applied. ContextFiles with empty Paths are rejected and
// no mutation is performed.
func (m *Kit) SetContextFiles(files []*ContextFile) error {
	// Validate first so a bad input doesn't partially mutate state.
	for i, cf := range files {
		if cf == nil {
			return fmt.Errorf("SetContextFiles: context file at index %d is nil", i)
		}
		if cf.Path == "" {
			return fmt.Errorf("SetContextFiles: context file at index %d has empty path", i)
		}
	}

	// Defensive copies so caller-side mutation cannot race with composition.
	copied := make([]*ContextFile, len(files))
	for i, cf := range files {
		copied[i] = &ContextFile{
			Path:    cf.Path,
			Content: strings.TrimSpace(cf.Content),
		}
	}

	m.runtimeMu.Lock()
	m.contextFiles = copied
	m.runtimeMu.Unlock()

	m.applyComposedSystemPrompt()
	return nil
}
