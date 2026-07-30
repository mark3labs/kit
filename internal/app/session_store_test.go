package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/session"
)

// newPersistedApp creates an App backed by a session that is written to disk
// under a sandboxed HOME, and returns the app plus its tree manager.
func newPersistedApp(t *testing.T) (*App, *session.TreeManager) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	tm, err := session.CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	t.Cleanup(func() { _ = tm.Close() })
	return New(Options{TreeSession: tm}, nil), tm
}

// --------------------------------------------------------------------------
// Listing
// --------------------------------------------------------------------------

func TestListSessionsEmptyCwd(t *testing.T) {
	a := New(Options{}, nil)

	got, err := a.ListSessions("")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if got != nil {
		t.Errorf("ListSessions(\"\") = %v, want nil", got)
	}
}

func TestListSessionsProjectsSummary(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()

	tm, err := session.CreateTreeSession(cwd)
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	appendUserMessage(t, tm, "first question")
	if _, err := tm.AppendSessionInfo("named session"); err != nil {
		t.Fatalf("AppendSessionInfo: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	a := New(Options{}, nil)
	got, err := a.ListSessions(cwd)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d summaries, want 1", len(got))
	}

	s := got[0]
	if s.Path != tm.GetFilePath() {
		t.Errorf("Path = %q, want %q", s.Path, tm.GetFilePath())
	}
	if s.ID != tm.GetHeader().ID {
		t.Errorf("ID = %q, want %q", s.ID, tm.GetHeader().ID)
	}
	if s.Name != "named session" {
		t.Errorf("Name = %q, want %q", s.Name, "named session")
	}
	if s.Cwd != cwd {
		t.Errorf("Cwd = %q, want %q", s.Cwd, cwd)
	}
	if s.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", s.MessageCount)
	}
	if s.FirstMessage != "first question" {
		t.Errorf("FirstMessage = %q, want %q", s.FirstMessage, "first question")
	}
	if s.Created.IsZero() {
		t.Error("Created is zero, want the session's creation time")
	}
	if s.Modified.IsZero() {
		t.Error("Modified is zero, want the last activity time")
	}
}

func TestListAllSessionsSpansWorkingDirectories(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	for _, cwd := range []string{t.TempDir(), t.TempDir()} {
		tm, err := session.CreateTreeSession(cwd)
		if err != nil {
			t.Fatalf("CreateTreeSession: %v", err)
		}
		appendUserMessage(t, tm, "hello from "+cwd)
		if err := tm.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}

	a := New(Options{}, nil)
	got, err := a.ListAllSessions()
	if err != nil {
		t.Fatalf("ListAllSessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d summaries, want 2 (one per working directory)", len(got))
	}
}

func TestListSessionsNoSessionsYet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := New(Options{}, nil)

	got, err := a.ListSessions(t.TempDir())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d summaries, want 0", len(got))
	}
}

// --------------------------------------------------------------------------
// Deleting
// --------------------------------------------------------------------------

func TestDeleteSession(t *testing.T) {
	a, tm := newPersistedApp(t)
	path := tm.GetFilePath()

	if err := a.DeleteSession(path); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("session file still present after delete (stat err = %v)", err)
	}
}

func TestDeleteSessionRequiresPath(t *testing.T) {
	a := New(Options{}, nil)

	if err := a.DeleteSession(""); err == nil {
		t.Error("DeleteSession(\"\") = nil, want an error")
	}
}

func TestDeleteSessionMissingFile(t *testing.T) {
	a := New(Options{}, nil)

	err := a.DeleteSession(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err == nil {
		t.Fatal("DeleteSession on a missing file = nil, want an error")
	}
	if !strings.Contains(err.Error(), "absent.jsonl") {
		t.Errorf("error %q does not name the offending path", err)
	}
}

// --------------------------------------------------------------------------
// Export
// --------------------------------------------------------------------------

func TestExportSessionNoSession(t *testing.T) {
	a := New(Options{}, nil)

	if _, _, err := a.ExportSession(""); !errors.Is(err, ErrNoSession) {
		t.Errorf("ExportSession err = %v, want ErrNoSession", err)
	}
}

func TestExportSessionInMemory(t *testing.T) {
	a, _ := newSessionApp(t)

	if _, _, err := a.ExportSession(""); !errors.Is(err, ErrSessionNotPersisted) {
		t.Errorf("ExportSession err = %v, want ErrSessionNotPersisted", err)
	}
}

func TestExportSessionToExplicitPath(t *testing.T) {
	a, tm := newPersistedApp(t)
	appendUserMessage(t, tm, "hello")

	dst := filepath.Join(t.TempDir(), "out.jsonl")
	gotPath, written, err := a.ExportSession(dst)
	if err != nil {
		t.Fatalf("ExportSession: %v", err)
	}
	if gotPath != dst {
		t.Errorf("path = %q, want %q", gotPath, dst)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if written != len(data) {
		t.Errorf("reported %d bytes, file holds %d", written, len(data))
	}

	src, err := os.ReadFile(tm.GetFilePath())
	if err != nil {
		t.Fatalf("ReadFile source: %v", err)
	}
	if string(data) != string(src) {
		t.Error("exported file is not a byte-for-byte copy of the session")
	}
}

func TestExportSessionDerivesNameFromSessionName(t *testing.T) {
	a, tm := newPersistedApp(t)
	appendUserMessage(t, tm, "hello")
	if _, err := tm.AppendSessionInfo("my great/session"); err != nil {
		t.Fatalf("AppendSessionInfo: %v", err)
	}

	t.Chdir(t.TempDir())
	gotPath, _, err := a.ExportSession("")
	if err != nil {
		t.Fatalf("ExportSession: %v", err)
	}
	// Separators and spaces are replaced so the name is usable as a file name.
	if gotPath != "session_my_great_session.jsonl" {
		t.Errorf("path = %q, want the sanitised session name", gotPath)
	}
	if _, err := os.Stat(gotPath); err != nil {
		t.Errorf("expected the export at %q: %v", gotPath, err)
	}
}

func TestExportSessionDerivesNameFromIDWhenUnnamed(t *testing.T) {
	a, tm := newPersistedApp(t)
	appendUserMessage(t, tm, "hello")

	t.Chdir(t.TempDir())
	gotPath, _, err := a.ExportSession("")
	if err != nil {
		t.Fatalf("ExportSession: %v", err)
	}
	want := "session_" + shortSessionID(tm.GetHeader().ID) + ".jsonl"
	if gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestExportSessionUnwritableDestination(t *testing.T) {
	a, tm := newPersistedApp(t)
	appendUserMessage(t, tm, "hello")

	// A path whose parent directory does not exist.
	dst := filepath.Join(t.TempDir(), "missing", "out.jsonl")
	if _, _, err := a.ExportSession(dst); err == nil {
		t.Fatal("ExportSession to an unwritable path = nil, want an error")
	}
}

// --------------------------------------------------------------------------
// Share
// --------------------------------------------------------------------------

func TestWriteShareableSessionNoSession(t *testing.T) {
	a := New(Options{}, nil)

	if _, err := a.WriteShareableSession("prompt", "model"); !errors.Is(err, ErrNoSession) {
		t.Errorf("WriteShareableSession err = %v, want ErrNoSession", err)
	}
}

func TestWriteShareableSessionInMemory(t *testing.T) {
	a, _ := newSessionApp(t)

	if _, err := a.WriteShareableSession("prompt", "model"); !errors.Is(err, ErrSessionNotPersisted) {
		t.Errorf("WriteShareableSession err = %v, want ErrSessionNotPersisted", err)
	}
}

func TestWriteShareableSessionSplicesSystemPrompt(t *testing.T) {
	a, tm := newPersistedApp(t)
	appendUserMessage(t, tm, "hello")
	if _, err := tm.AppendModelChange("anthropic", "claude-sonnet-4-5"); err != nil {
		t.Fatalf("AppendModelChange: %v", err)
	}

	tmpPath, err := a.WriteShareableSession("be helpful", "fallback-model")
	if err != nil {
		t.Fatalf("WriteShareableSession: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpPath) })

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("got %d lines, want at least header + system prompt + message", len(lines))
	}

	// Line 0 must remain the session header.
	header, err := session.UnmarshalEntry([]byte(lines[0]))
	if err != nil {
		t.Fatalf("UnmarshalEntry(line 0): %v", err)
	}
	h, ok := header.(*session.SessionHeader)
	if !ok {
		t.Fatalf("line 0 is %T, want *session.SessionHeader", header)
	}
	if h.ID != tm.GetHeader().ID {
		t.Errorf("header ID = %q, want %q", h.ID, tm.GetHeader().ID)
	}

	// Line 1 must be the spliced-in system prompt.
	entry, err := session.UnmarshalEntry([]byte(lines[1]))
	if err != nil {
		t.Fatalf("UnmarshalEntry(line 1): %v", err)
	}
	sp, ok := entry.(*session.SystemPromptEntry)
	if !ok {
		t.Fatalf("line 1 is %T, want *session.SystemPromptEntry", entry)
	}
	if sp.Content != "be helpful" {
		t.Errorf("Content = %q, want %q", sp.Content, "be helpful")
	}
	if sp.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q, want the session's model", sp.Model)
	}
	if sp.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", sp.Provider, "anthropic")
	}
}

func TestWriteShareableSessionFallsBackToGivenModel(t *testing.T) {
	a, tm := newPersistedApp(t)
	appendUserMessage(t, tm, "hello")

	tmpPath, err := a.WriteShareableSession("be helpful", "fallback-model")
	if err != nil {
		t.Fatalf("WriteShareableSession: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpPath) })

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	entry, err := session.UnmarshalEntry([]byte(lines[1]))
	if err != nil {
		t.Fatalf("UnmarshalEntry: %v", err)
	}
	sp := entry.(*session.SystemPromptEntry)
	if sp.Model != "fallback-model" {
		t.Errorf("Model = %q, want the fallback when the session records none", sp.Model)
	}
}

func TestWriteShareableSessionPreservesAllEntries(t *testing.T) {
	a, tm := newPersistedApp(t)
	appendUserMessage(t, tm, "one")
	appendUserMessage(t, tm, "two")

	src, err := os.ReadFile(tm.GetFilePath())
	if err != nil {
		t.Fatalf("ReadFile source: %v", err)
	}
	srcLines := strings.Split(strings.TrimRight(string(src), "\n"), "\n")

	tmpPath, err := a.WriteShareableSession("be helpful", "model")
	if err != nil {
		t.Fatalf("WriteShareableSession: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpPath) })

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	shared := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	// The shared file is the original plus exactly one system-prompt line.
	if len(shared) != len(srcLines)+1 {
		t.Fatalf("shared file has %d lines, want %d (original + system prompt)", len(shared), len(srcLines)+1)
	}
	for i, line := range srcLines[1:] {
		if shared[i+2] != line {
			t.Errorf("entry %d changed:\n got %q\nwant %q", i, shared[i+2], line)
		}
	}
}

// --------------------------------------------------------------------------
// File name helpers
// --------------------------------------------------------------------------

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "session", "session"},
		{"spaces", "my session", "my_session"},
		{"unix separator", "a/b", "a_b"},
		{"windows separator", `a\b`, "a_b"},
		{"colon", "a:b", "a_b"},
		{"mixed", `my project:/a b`, "my_project__a_b"},
		{"empty", "", ""},
		{"unicode kept", "días", "días"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeFileName(tt.in); got != tt.want {
				t.Errorf("sanitizeFileName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestShortSessionID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"long is truncated", "0123456789abcdef", "0123456789ab"},
		{"exact length", "0123456789ab", "0123456789ab"},
		// A short ID must be returned as-is rather than panicking on a slice
		// past the end.
		{"short is kept", "abc", "abc"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortSessionID(tt.in); got != tt.want {
				t.Errorf("shortSessionID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
