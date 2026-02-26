package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionInfo contains metadata about a discovered session, used for listing
// and session picker display. Follows pi's SessionInfo design.
type SessionInfo struct {
	// Path is the absolute path to the JSONL session file.
	Path string

	// ID is the session UUID from the header.
	ID string

	// Cwd is the working directory the session was created in.
	Cwd string

	// Name is the user-defined display name (from session_info entries).
	Name string

	// ParentSessionPath is the parent session path if this session was forked.
	ParentSessionPath string

	// Created is when the session was first created.
	Created time.Time

	// Modified is the timestamp of the last activity (latest message).
	Modified time.Time

	// MessageCount is the number of message entries in the session.
	MessageCount int

	// FirstMessage is a preview of the first user message.
	FirstMessage string
}

// ListSessions finds all sessions for a given working directory, sorted by
// modification time (newest first).
func ListSessions(cwd string) ([]SessionInfo, error) {
	sessionDir := DefaultSessionDir(cwd)
	return listSessionsInDir(sessionDir)
}

// ListAllSessions finds all sessions across all working directories, sorted
// by modification time (newest first).
func ListAllSessions() ([]SessionInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to find home directory: %w", err)
	}

	sessionsRoot := filepath.Join(home, ".kit", "sessions")
	if _, err := os.Stat(sessionsRoot); os.IsNotExist(err) {
		return nil, nil
	}

	var allSessions []SessionInfo

	dirs, err := os.ReadDir(sessionsRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		dirPath := filepath.Join(sessionsRoot, dir.Name())
		sessions, err := listSessionsInDir(dirPath)
		if err != nil {
			continue // skip unreadable directories
		}
		allSessions = append(allSessions, sessions...)
	}

	// Sort by modification time, newest first.
	sort.Slice(allSessions, func(i, j int) bool {
		return allSessions[i].Modified.After(allSessions[j].Modified)
	})

	return allSessions, nil
}

// listSessionsInDir reads all .jsonl files in a directory and extracts session info.
func listSessionsInDir(dir string) ([]SessionInfo, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		info, err := extractSessionInfo(path)
		if err != nil {
			continue // skip malformed session files
		}
		sessions = append(sessions, *info)
	}

	// Sort by modification time, newest first.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Modified.After(sessions[j].Modified)
	})

	return sessions, nil
}

// extractSessionInfo reads a JSONL session file and extracts metadata.
// It only reads enough of the file to get the header and scan for messages.
func extractSessionInfo(path string) (*SessionInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	info := &SessionInfo{
		Path: path,
	}

	scanner := bufio.NewScanner(f)
	// Increase scanner buffer for large lines.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNum := 0
	var lastTimestamp time.Time

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		lineNum++

		if lineNum == 1 {
			// Parse header.
			var h SessionHeader
			if err := json.Unmarshal([]byte(line), &h); err != nil {
				return nil, fmt.Errorf("failed to parse header: %w", err)
			}
			if h.Type != EntryTypeSession {
				return nil, fmt.Errorf("first line is not a session header")
			}
			info.ID = h.ID
			info.Cwd = h.Cwd
			info.Created = h.Timestamp
			info.Modified = h.Timestamp
			info.ParentSessionPath = h.ParentSession
			continue
		}

		// For subsequent lines, only parse enough to get type and timestamp.
		var env struct {
			Type      EntryType `json:"type"`
			Timestamp time.Time `json:"timestamp"`
			Role      string    `json:"role,omitempty"`
			Name      string    `json:"name,omitempty"`
		}
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			continue
		}

		if !env.Timestamp.IsZero() && env.Timestamp.After(lastTimestamp) {
			lastTimestamp = env.Timestamp
		}

		switch env.Type {
		case EntryTypeMessage:
			info.MessageCount++
			// Capture first user message as preview.
			if env.Role == "user" && info.FirstMessage == "" {
				var msgEntry struct {
					Parts json.RawMessage `json:"parts"`
				}
				if err := json.Unmarshal([]byte(line), &msgEntry); err == nil {
					info.FirstMessage = extractTextPreview(msgEntry.Parts)
				}
			}
		case EntryTypeSessionInfo:
			if env.Name != "" {
				info.Name = env.Name
			}
		}
	}

	if !lastTimestamp.IsZero() {
		info.Modified = lastTimestamp
	}

	// Fall back to file modification time if no timestamps found.
	if info.Modified.IsZero() {
		fi, err := os.Stat(path)
		if err == nil {
			info.Modified = fi.ModTime()
		}
	}

	return info, nil
}

// extractTextPreview extracts a short text preview from type-tagged parts JSON.
func extractTextPreview(partsJSON json.RawMessage) string {
	var parts []struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(partsJSON, &parts); err != nil {
		return ""
	}

	for _, p := range parts {
		if p.Type == "text" {
			var text struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(p.Data, &text); err == nil && text.Text != "" {
				preview := text.Text
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				return preview
			}
		}
	}
	return ""
}

// DeleteSession removes a session file from disk.
func DeleteSession(path string) error {
	return os.Remove(path)
}
