package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/kit/internal/session"
)

// ErrSessionNotPersisted is returned by operations that need the session's
// backing file when the active session exists only in memory.
var ErrSessionNotPersisted = errors.New("session is not persisted to disk")

// SessionSummary is an immutable value description of a session stored on
// disk. It carries what a session picker needs to identify, search, sort and
// render a session without opening the file or knowing the JSONL schema.
//
// It is deliberately narrower than the on-disk metadata: fields are added
// here as presentation layers need them, so the persistence format stays free
// to change.
type SessionSummary struct {
	// Path is the absolute path to the JSONL file backing the session.
	Path string
	// ID is the session UUID.
	ID string
	// Name is the user-defined display name, empty if unnamed.
	Name string
	// Cwd is the working directory the session was created in.
	Cwd string
	// Created is the session creation timestamp.
	Created time.Time
	// Modified is the timestamp of the most recent activity.
	Modified time.Time
	// MessageCount is the number of message entries in the session.
	MessageCount int
	// FirstMessage is a preview of the first user message, empty when the
	// session holds no user messages.
	FirstMessage string
}

// ListSessions returns summaries of the sessions recorded for cwd, newest
// first. An empty cwd yields no sessions rather than an error, so callers can
// pass through an unknown working directory unconditionally.
func (a *App) ListSessions(cwd string) ([]SessionSummary, error) {
	if cwd == "" {
		return nil, nil
	}
	infos, err := session.ListSessions(cwd)
	if err != nil {
		return nil, fmt.Errorf("list sessions for %q: %w", cwd, err)
	}
	return sessionSummaries(infos), nil
}

// ListAllSessions returns summaries of every session across all working
// directories, newest first.
func (a *App) ListAllSessions() ([]SessionSummary, error) {
	infos, err := session.ListAllSessions()
	if err != nil {
		return nil, fmt.Errorf("list all sessions: %w", err)
	}
	return sessionSummaries(infos), nil
}

// DeleteSession removes a session file from disk. Deleting the file backing
// the active session is permitted: the session keeps running in memory and
// simply stops being discoverable.
func (a *App) DeleteSession(path string) error {
	if path == "" {
		return errors.New("session path is required")
	}
	if err := session.DeleteSession(path); err != nil {
		return fmt.Errorf("delete session %q: %w", path, err)
	}
	return nil
}

// sessionSummaries projects discovered session metadata into value summaries.
func sessionSummaries(infos []session.SessionInfo) []SessionSummary {
	if len(infos) == 0 {
		return nil
	}
	out := make([]SessionSummary, 0, len(infos))
	for _, info := range infos {
		out = append(out, SessionSummary{
			Path:         info.Path,
			ID:           info.ID,
			Name:         info.Name,
			Cwd:          info.Cwd,
			Created:      info.Created,
			Modified:     info.Modified,
			MessageCount: info.MessageCount,
			FirstMessage: info.FirstMessage,
		})
	}
	return out
}

// ExportSession copies the active session's JSONL file to dstPath. When
// dstPath is empty, a name is derived from the session's display name (or a
// short form of its ID when unnamed) and written to the process's working
// directory.
//
// It returns the path written and the number of bytes copied. Returns
// ErrNoSession when no tree session is active, and ErrSessionNotPersisted
// when the session exists only in memory and so has nothing to copy.
func (a *App) ExportSession(dstPath string) (string, int, error) {
	snap, ok := a.SessionSnapshot()
	if !ok {
		return "", 0, ErrNoSession
	}
	if snap.FilePath == "" {
		return "", 0, ErrSessionNotPersisted
	}
	if dstPath == "" {
		dstPath = defaultExportName(snap)
	}

	data, err := os.ReadFile(snap.FilePath)
	if err != nil {
		return "", 0, fmt.Errorf("read session file: %w", err)
	}
	if err := os.WriteFile(dstPath, data, 0o644); err != nil {
		return "", 0, fmt.Errorf("write export file %q: %w", dstPath, err)
	}
	return dstPath, len(data), nil
}

// defaultExportName builds the fallback file name for an exported session.
func defaultExportName(snap SessionSnapshot) string {
	name := snap.Name
	if name == "" {
		name = shortSessionID(snap.ID)
	}
	return fmt.Sprintf("session_%s.jsonl", sanitizeFileName(name))
}

// WriteShareableSession writes a shareable copy of the active session to a
// temporary file and returns its path. The copy is the session's JSONL with a
// system-prompt entry inserted directly after the header, so whoever reads
// the shared file can see the system prompt and model the conversation ran
// under. fallbackModelID is used when the session records no model of its own.
//
// The caller owns the returned file and is responsible for removing it. On
// error no file is left behind. Returns ErrNoSession when no tree session is
// active and ErrSessionNotPersisted when the session is in-memory.
func (a *App) WriteShareableSession(systemPrompt, fallbackModelID string) (string, error) {
	snap, ok := a.SessionSnapshot()
	if !ok {
		return "", ErrNoSession
	}
	if snap.FilePath == "" {
		return "", ErrSessionNotPersisted
	}

	data, err := os.ReadFile(snap.FilePath)
	if err != nil {
		return "", fmt.Errorf("read session file: %w", err)
	}
	sysPromptJSON, err := a.systemPromptEntry(systemPrompt, fallbackModelID)
	if err != nil {
		return "", err
	}

	name := snap.Name
	if name == "" {
		name = "session"
	}
	return writeShareFile(sanitizeFileName(name), data, sysPromptJSON)
}

// systemPromptEntry marshals a system-prompt entry describing the given
// system prompt together with the model and provider currently in effect for
// the session. fallbackModelID is used when the session records no model
// change of its own.
func (a *App) systemPromptEntry(systemPrompt, fallbackModelID string) ([]byte, error) {
	tm := a.opts.TreeSession
	if tm == nil {
		return nil, ErrNoSession
	}
	_, provider, modelID := tm.BuildContext()
	if modelID == "" {
		modelID = fallbackModelID
	}
	data, err := session.MarshalEntry(session.NewSystemPromptEntry(systemPrompt, modelID, provider))
	if err != nil {
		return nil, fmt.Errorf("marshal system prompt entry: %w", err)
	}
	return data, nil
}

// writeShareFile writes data to a temporary JSONL file with sysPromptJSON
// spliced in after the header line, and returns the temp file's path.
func writeShareFile(name string, data, sysPromptJSON []byte) (tmpPath string, err error) {
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("kit-%s-*.jsonl", name))
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath = tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	// The header is the first line, so we write:
	// 1. First line (header) from the original data
	// 2. System prompt entry
	// 3. Remaining lines from the original data
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1] // Remove trailing empty line
	}
	if len(lines) == 0 {
		return tmpPath, nil
	}

	if _, err = tmpFile.WriteString(lines[0] + "\n"); err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if _, err = tmpFile.Write(sysPromptJSON); err != nil {
		return "", fmt.Errorf("write system prompt: %w", err)
	}
	if _, err = tmpFile.WriteString("\n"); err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "" {
			continue // Skip empty lines
		}
		if _, err = tmpFile.WriteString(lines[i] + "\n"); err != nil {
			return "", fmt.Errorf("write temp file: %w", err)
		}
	}
	return tmpPath, nil
}

// sanitizeFileName replaces path separators and other characters that are
// awkward in file names so a session name can be used to build one.
func sanitizeFileName(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == ' ' {
			return '_'
		}
		return r
	}, name)
}

// shortSessionID returns a file-name-friendly prefix of a session ID, used as
// a fallback export name for unnamed sessions.
func shortSessionID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
