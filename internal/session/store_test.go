package session

import (
	"os"
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

// TestOpenTreeSession_NoTrailingNewlineAndBlankLines checks the line
// splitter: blank lines are skipped and a last line without '\n' is still
// read (it used to need its own code path).
func TestOpenTreeSession_NoTrailingNewlineAndBlankLines(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	_, _ = tm.AppendMessage(newTestMessage("one"))
	_, _ = tm.AppendMessage(newTestMessage("two"))
	path := tm.GetFilePath()
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// Insert blank lines between entries and drop the final newline.
	mangled := strings.ReplaceAll(strings.TrimRight(string(data), "\n"), "\n", "\n\n  \r\n")
	if err := os.WriteFile(path, []byte(mangled), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	msgs, _, _ := reopened.BuildContext()
	if len(msgs) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(msgs))
	}
}
