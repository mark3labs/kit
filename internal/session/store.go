package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// SessionInfo contains metadata about a discovered session, used for listing
// and session picker display.
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

	// ParentSessionID is the UUID of the parent session (for subagent sessions).
	ParentSessionID string

	// SubagentTask is the original task prompt (for subagent sessions).
	SubagentTask string

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

	dirs, err := os.ReadDir(sessionsRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	// Gather every file first and extract them in one worker pool. A pool
	// per directory left most workers idle: typical users have hundreds of
	// directories holding a handful of sessions each.
	var paths []string
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		dirPaths, err := sessionPathsInDir(filepath.Join(sessionsRoot, dir.Name()))
		if err != nil {
			continue // skip unreadable directories
		}
		paths = append(paths, dirPaths...)
	}

	return sessionInfos(paths), nil
}

// listSessionsInDir reads all .jsonl files in a directory and extracts session info.
// Empty sessions (no messages) are automatically cleaned up and not returned.
func listSessionsInDir(dir string) ([]SessionInfo, error) {
	paths, err := sessionPathsInDir(dir)
	if err != nil {
		return nil, err
	}
	return sessionInfos(paths), nil
}

// sessionPathsInDir returns the .jsonl session files in dir, or nil if dir
// does not exist.
func sessionPathsInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	return paths, nil
}

// sessionInfos extracts session info from every path, newest first.
// Unreadable or malformed files are skipped. Empty sessions (no messages)
// are deleted and not returned.
//
// Extraction is parallelized across a worker pool because each file needs
// a full JSONL scan to compute MessageCount; for users with many sessions
// this is the dominant cost of opening the session picker.
func sessionInfos(paths []string) []SessionInfo {
	results := make([]*SessionInfo, len(paths))

	// Worker pool sized to GOMAXPROCS, capped to avoid thrashing for tiny lists.
	workers := max(min(runtime.GOMAXPROCS(0), len(paths)), 1)

	var wg sync.WaitGroup
	jobs := make(chan int, len(paths))
	for range workers {
		wg.Go(func() {
			for i := range jobs {
				info, err := extractSessionInfo(paths[i])
				if err != nil {
					continue // skip malformed session files
				}
				results[i] = info
			}
		})
	}
	for i := range paths {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	sessions := make([]SessionInfo, 0, len(results))
	for i, info := range results {
		if info == nil {
			continue
		}
		// Clean up and skip empty sessions (no messages).
		if info.MessageCount == 0 {
			_ = os.Remove(paths[i])
			continue
		}
		sessions = append(sessions, *info)
	}

	// Sort by modification time, newest first.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Modified.After(sessions[j].Modified)
	})
	return sessions
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

	// A bufio.Reader, not a bufio.Scanner: a Scanner fails the whole file
	// with "token too long" on any line past its buffer cap, and a message
	// carrying an inline image easily runs to several megabytes.
	//
	// Lines are read with ReadSlice so that a long line is never collected
	// unless it has to be: everything this function needs from an entry
	// sits in its first few fields, ahead of the (potentially huge) parts
	// array. See scanEntryHead.
	reader := bufio.NewReaderSize(f, sessionScanBufSize)
	lineNum := 0
	var lastTimestamp time.Time

	for done := false; !done; {
		line, err := reader.ReadSlice('\n')
		long := false
		switch {
		case err == bufio.ErrBufferFull:
			// Only a prefix of the line is in hand. Copy it: the next
			// ReadSlice overwrites the reader's buffer.
			long = true
			line = bytes.Clone(line)
		case err == io.EOF:
			done = true
		case err != nil:
			return nil, fmt.Errorf("failed to read session file: %w", err)
		}

		// finish consumes the rest of a long line, which must happen
		// before the next line is read. With keep it returns the complete
		// line; otherwise the rest is discarded as it streams past. last
		// is the line's final non-whitespace byte.
		finish := func(keep bool) (full []byte, last byte, err error) {
			full, last = line, lastNonSpace(line, 0)
			for long {
				chunk, rerr := reader.ReadSlice('\n')
				if keep {
					full = append(full, chunk...)
				}
				last = lastNonSpace(chunk, last)
				switch {
				case rerr == bufio.ErrBufferFull:
					continue
				case rerr == io.EOF:
					done = true
				case rerr != nil:
					return nil, 0, fmt.Errorf("failed to read session file: %w", rerr)
				}
				long = false
			}
			return full, last, nil
		}

		if !long && len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		lineNum++

		if lineNum == 1 {
			full, _, err := finish(true)
			if err != nil {
				return nil, err
			}
			// Parse header.
			var h SessionHeader
			if err := json.Unmarshal(full, &h); err != nil {
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
			info.ParentSessionID = h.ParentSessionID
			info.SubagentTask = h.SubagentTask
			continue
		}

		head, ok := scanEntryHead(line)
		wantPreview := info.FirstMessage == ""
		needFull := !ok || (wantPreview && head.Type == EntryTypeMessage && head.Role == "user")
		full, last, err := finish(needFull)
		if err != nil {
			return nil, err
		}
		if !ok {
			// The fast path could not read the head (unusual field order,
			// or a head longer than the buffer): parse the whole line.
			if err := json.Unmarshal(full, &head); err != nil {
				continue
			}
		} else if last != '}' {
			// A torn line (e.g. a crash mid-write) is not a whole JSON
			// object; a full parse would reject it, so skip it too.
			continue
		}

		if !head.Timestamp.IsZero() && head.Timestamp.After(lastTimestamp) {
			lastTimestamp = head.Timestamp
		}

		switch head.Type {
		case EntryTypeMessage:
			info.MessageCount++
			// Capture first user message as preview.
			if head.Role == "user" && wantPreview {
				var msgEntry struct {
					Parts json.RawMessage `json:"parts"`
				}
				if err := json.Unmarshal(full, &msgEntry); err == nil {
					info.FirstMessage = extractTextPreview(msgEntry.Parts)
				}
			}
		case EntryTypeSessionInfo:
			if head.Name != "" {
				info.Name = head.Name
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

// sessionScanBufSize is the read buffer for session scans. It bounds the
// prefix of a long line that scanEntryHead sees, and comfortably holds the
// head fields of any entry Kit writes.
const sessionScanBufSize = 64 * 1024

// entryHead holds the fields extractSessionInfo needs from a non-header
// session entry.
type entryHead struct {
	Type      EntryType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Role      string    `json:"role,omitempty"`
	Name      string    `json:"name,omitempty"`
}

// complete reports whether every field relevant to h.Type has been read.
func (h *entryHead) complete(haveType, haveTS, haveRole, haveName bool) bool {
	if !haveType || !haveTS {
		return false
	}
	switch h.Type {
	case EntryTypeMessage:
		return haveRole
	case EntryTypeSessionInfo:
		return haveName
	default:
		return true
	}
}

// scanEntryHead reads the head fields of a JSONL entry, stopping as soon as
// it has all of them.
//
// Session entries are written with type, id, timestamp and role ahead of
// the parts array, which can hold megabytes of base64 image data. Decoding
// the whole line to get four small fields was most of the cost of listing
// sessions. line may be only a prefix of the entry.
//
// ok is false if the head could not be read from line (a prefix cut short,
// fields in an unexpected order past the prefix, or malformed JSON); the
// caller then falls back to decoding the complete line. Each value is
// decoded with json.Unmarshal, so the results match a full decode.
func scanEntryHead(line []byte) (h entryHead, ok bool) {
	dec := jsontext.NewDecoder(bytes.NewReader(line))
	if tok, err := dec.ReadToken(); err != nil || tok.Kind() != '{' {
		return entryHead{}, false
	}
	var haveType, haveTS, haveRole, haveName bool
	for !h.complete(haveType, haveTS, haveRole, haveName) {
		if dec.PeekKind() == '}' {
			// End of the object: every field present has been read.
			return h, true
		}
		name, err := dec.ReadToken()
		if err != nil {
			return entryHead{}, false
		}
		var target any
		switch name.String() {
		case "type":
			target, haveType = &h.Type, true
		case "timestamp":
			target, haveTS = &h.Timestamp, true
		case "role":
			target, haveRole = &h.Role, true
		case "name":
			target, haveName = &h.Name, true
		default:
			if err := dec.SkipValue(); err != nil {
				return entryHead{}, false
			}
			continue
		}
		val, err := dec.ReadValue()
		if err != nil {
			return entryHead{}, false
		}
		if err := json.Unmarshal(val, target); err != nil {
			return entryHead{}, false
		}
	}
	return h, true
}

// lastNonSpace returns the last byte of b that is not JSON whitespace, or
// prev if b is all whitespace.
func lastNonSpace(b []byte, prev byte) byte {
	for _, c := range slices.Backward(b) {
		switch c {
		case ' ', '\t', '\r', '\n':
			continue
		}
		return c
	}
	return prev
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

// FindSessionPathByID locates the JSONL session file whose header ID matches
// the given session UUID. The session directory for cwd is searched first
// (the common case for subagent sessions, which live alongside the parent's
// sessions), then all session directories under ~/.kit/sessions, and finally
// the bare bucket. Only file headers (first line) are read, so the scan is
// cheap even with many sessions. Returns an error when no session with that
// ID exists.
func FindSessionPathByID(cwd, id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("session ID is required")
	}

	// Fast path: the session directory for the working directory.
	if path, ok := findSessionInDir(DefaultSessionDir(cwd), id); ok {
		return path, nil
	}

	// Fall back to scanning all session directories.
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("session %q not found", id)
	}
	sessionsRoot := filepath.Join(home, ".kit", "sessions")
	if dirs, err := os.ReadDir(sessionsRoot); err == nil {
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			if path, ok := findSessionInDir(filepath.Join(sessionsRoot, dir.Name()), id); ok {
				return path, nil
			}
		}
	}

	// Bare sessions deliberately live outside the cwd-keyed sessions/ subtree
	// (see DefaultSessionDir), so neither the fast path nor the scan above can
	// reach them. Without this a subagent session started in bare mode could
	// not be resumed by ID. Checked last so ordinary lookups are unaffected.
	if path, ok := findSessionInDir(DefaultSessionDir(BareSessionKey), id); ok {
		return path, nil
	}

	return "", fmt.Errorf("session %q not found", id)
}

// findSessionInDir scans a single session directory for a session file whose
// header ID matches id. Malformed files are skipped.
func findSessionInDir(dir, id string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		header, err := readSessionHeader(path)
		if err != nil {
			continue
		}
		if header.ID == id {
			return path, true
		}
	}
	return "", false
}

// readSessionHeader reads and parses only the first line (the session header)
// of a JSONL session file.
func readSessionHeader(path string) (*SessionHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var h SessionHeader
		if err := json.Unmarshal([]byte(line), &h); err != nil {
			return nil, fmt.Errorf("failed to parse header: %w", err)
		}
		if h.Type != EntryTypeSession {
			return nil, fmt.Errorf("first line is not a session header")
		}
		return &h, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan session header: %w", err)
	}
	return nil, fmt.Errorf("empty session file")
}
