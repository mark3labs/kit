package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/message"
)

// newTestMessage builds a minimal user message for session tests.
func newTestMessage(text string) message.Message {
	return message.Message{
		Role: message.RoleUser,
		Parts: []message.ContentPart{
			message.TextContent{Text: text},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestSetParentLink_PersistsAcrossReopen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()

	tm, err := CreateTreeSession(cwd)
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}

	// Stamp the parent link on a fresh session (the Kit.Subagent flow),
	// then append messages afterwards.
	if err := tm.SetParentLink("/parent/session.jsonl", "parent-uuid", "research the auth flow"); err != nil {
		t.Fatalf("SetParentLink: %v", err)
	}
	if _, err := tm.AppendMessage(newTestMessage("hello")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	path := tm.GetFilePath()
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	header := reopened.GetHeader()
	if header.ParentSession != "/parent/session.jsonl" {
		t.Errorf("ParentSession = %q, want %q", header.ParentSession, "/parent/session.jsonl")
	}
	if header.ParentSessionID != "parent-uuid" {
		t.Errorf("ParentSessionID = %q, want %q", header.ParentSessionID, "parent-uuid")
	}
	if header.SubagentTask != "research the auth flow" {
		t.Errorf("SubagentTask = %q, want %q", header.SubagentTask, "research the auth flow")
	}
	if got := reopened.MessageCount(); got != 1 {
		t.Errorf("MessageCount = %d, want 1 (entries must survive header rewrite)", got)
	}
}

func TestSetParentLink_RewritesExistingEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()

	tm, err := CreateTreeSession(cwd)
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}

	// Append entries BEFORE stamping — the rewrite must preserve them.
	if _, err := tm.AppendMessage(newTestMessage("first")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if _, err := tm.AppendMessage(newTestMessage("second")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	leafBefore := tm.GetLeafID()

	if err := tm.SetParentLink("", "parent-uuid", ""); err != nil {
		t.Fatalf("SetParentLink: %v", err)
	}

	// Appends after the rewrite must still work.
	if _, err := tm.AppendMessage(newTestMessage("third")); err != nil {
		t.Fatalf("AppendMessage after SetParentLink: %v", err)
	}

	path := tm.GetFilePath()
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	if got := reopened.MessageCount(); got != 3 {
		t.Errorf("MessageCount = %d, want 3", got)
	}
	if reopened.GetHeader().ParentSessionID != "parent-uuid" {
		t.Errorf("ParentSessionID = %q, want %q", reopened.GetHeader().ParentSessionID, "parent-uuid")
	}
	if reopened.GetHeader().SubagentTask != "" {
		t.Errorf("SubagentTask = %q, want empty (not provided)", reopened.GetHeader().SubagentTask)
	}
	if entry := reopened.GetEntry(leafBefore); entry == nil {
		t.Error("pre-rewrite entry ID no longer resolvable after reopen")
	}
}

func TestSetParentLink_InMemorySession(t *testing.T) {
	tm := InMemoryTreeSession(t.TempDir())

	if err := tm.SetParentLink("", "parent-uuid", "task"); err != nil {
		t.Fatalf("SetParentLink on in-memory session: %v", err)
	}
	if got := tm.GetHeader().ParentSessionID; got != "parent-uuid" {
		t.Errorf("ParentSessionID = %q, want %q", got, "parent-uuid")
	}
}

func TestFindSessionPathByID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()

	first, err := CreateTreeSession(cwd)
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	defer func() { _ = first.Close() }()
	second, err := CreateTreeSession(cwd)
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	defer func() { _ = second.Close() }()

	path, err := FindSessionPathByID(cwd, second.GetSessionID())
	if err != nil {
		t.Fatalf("FindSessionPathByID: %v", err)
	}
	if path != second.GetFilePath() {
		t.Errorf("path = %q, want %q", path, second.GetFilePath())
	}

	// Lookup from a different cwd must fall back to the global scan.
	path, err = FindSessionPathByID(t.TempDir(), first.GetSessionID())
	if err != nil {
		t.Fatalf("FindSessionPathByID (fallback scan): %v", err)
	}
	if path != first.GetFilePath() {
		t.Errorf("fallback path = %q, want %q", path, first.GetFilePath())
	}

	if _, err := FindSessionPathByID(cwd, "no-such-session"); err == nil {
		t.Error("expected error for unknown session ID")
	}
	if _, err := FindSessionPathByID(cwd, ""); err == nil {
		t.Error("expected error for empty session ID")
	}
}

func TestFindSessionPathByID_SkipsMalformedFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()

	tm, err := CreateTreeSession(cwd)
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	defer func() { _ = tm.Close() }()

	// Drop a malformed .jsonl next to the real session.
	dir := filepath.Dir(tm.GetFilePath())
	if err := os.WriteFile(filepath.Join(dir, "broken.jsonl"), []byte("not json\n"), 0644); err != nil {
		t.Fatalf("write malformed file: %v", err)
	}

	path, err := FindSessionPathByID(cwd, tm.GetSessionID())
	if err != nil {
		t.Fatalf("FindSessionPathByID: %v", err)
	}
	if path != tm.GetFilePath() {
		t.Errorf("path = %q, want %q", path, tm.GetFilePath())
	}
}
