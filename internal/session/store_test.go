package session

import (
	"strings"
	"testing"
)

// TestExtractSessionInfo_LongLine guards against the session picker dropping
// sessions whose JSONL holds a line longer than a fixed scanner buffer. A
// message with an inline image easily runs to several megabytes; with the old
// bufio.Scanner (1 MiB cap) the whole file failed with "token too long" and
// the session silently disappeared from the picker.
func TestExtractSessionInfo_LongLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	if _, err := tm.AppendMessage(newTestMessage("first prompt")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	// 3 MiB of text in a single entry: well past the old 1 MiB limit.
	if _, err := tm.AppendMessage(newTestMessage(strings.Repeat("x", 3<<20))); err != nil {
		t.Fatalf("AppendMessage (large): %v", err)
	}
	if _, err := tm.AppendMessage(newTestMessage("after the big one")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	path := tm.GetFilePath()
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := extractSessionInfo(path)
	if err != nil {
		t.Fatalf("extractSessionInfo: %v", err)
	}
	if info.MessageCount != 3 {
		t.Errorf("MessageCount = %d, want 3 (entries after the long line must still be counted)", info.MessageCount)
	}
	if info.FirstMessage != "first prompt" {
		t.Errorf("FirstMessage = %q, want %q", info.FirstMessage, "first prompt")
	}
}
