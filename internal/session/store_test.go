package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// writeSessionFile writes a session header plus the given raw lines.
func writeSessionFile(t *testing.T, lines ...string) string {
	t.Helper()
	header := `{"type":"session","id":"s1","timestamp":"2026-01-01T00:00:00Z","cwd":"/x"}`
	path := filepath.Join(t.TempDir(), "s.jsonl")
	content := header + "\n" + strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// TestExtractSessionInfo_HeadScanEdgeCases covers the paths around the
// early-stop head scan: fields in an unusual order, torn lines, previews
// from long lines and a long final line with no trailing newline.
func TestExtractSessionInfo_HeadScanEdgeCases(t *testing.T) {
	bigText := strings.Repeat("y", 3*sessionScanBufSize)
	userMsg := func(ts, text string) string {
		return `{"type":"message","id":"m","timestamp":"` + ts + `","role":"user","parts":[{"type":"text","data":{"text":"` + text + `"}}]}`
	}

	t.Run("parts before head fields falls back to a full parse", func(t *testing.T) {
		path := writeSessionFile(t,
			`{"parts":[{"type":"text","data":{"text":"`+bigText+`"}}],"role":"user","type":"message","id":"m","timestamp":"2026-01-02T00:00:00Z"}`)
		info, err := extractSessionInfo(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.MessageCount != 1 || !strings.HasPrefix(info.FirstMessage, "yyy") {
			t.Errorf("got count=%d preview=%.10q, want 1 and a y... preview", info.MessageCount, info.FirstMessage)
		}
		if want := "2026-01-02T00:00:00Z"; info.Modified.UTC().Format(time.RFC3339) != want {
			t.Errorf("Modified = %v, want %s", info.Modified, want)
		}
	})

	t.Run("torn lines are skipped", func(t *testing.T) {
		path := writeSessionFile(t,
			userMsg("2026-01-02T00:00:00Z", "ok"),
			`{"type":"message","id":"m2","timestamp":"2026-01-03T00:00:00Z","role":"assistant","parts":[{"ty`)
		info, err := extractSessionInfo(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.MessageCount != 1 {
			t.Errorf("MessageCount = %d, want 1 (torn line must not count)", info.MessageCount)
		}
	})

	t.Run("long torn final line is skipped", func(t *testing.T) {
		path := writeSessionFile(t,
			userMsg("2026-01-02T00:00:00Z", "ok"),
			`{"type":"message","id":"m2","timestamp":"2026-01-03T00:00:00Z","role":"assistant","parts":[{"type":"text","data":{"text":"`+bigText)
		info, err := extractSessionInfo(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.MessageCount != 1 {
			t.Errorf("MessageCount = %d, want 1", info.MessageCount)
		}
	})

	t.Run("preview from a long first user message", func(t *testing.T) {
		path := writeSessionFile(t,
			userMsg("2026-01-02T00:00:00Z", bigText),
			userMsg("2026-01-03T00:00:00Z", "second"))
		info, err := extractSessionInfo(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.MessageCount != 2 {
			t.Errorf("MessageCount = %d, want 2", info.MessageCount)
		}
		if want := strings.Repeat("y", 100) + "..."; info.FirstMessage != want {
			t.Errorf("FirstMessage = %.20q..., want 100 y's and ...", info.FirstMessage)
		}
	})

	t.Run("long lines without trailing newline, name and latest timestamp", func(t *testing.T) {
		path := writeSessionFile(t,
			userMsg("2026-01-02T00:00:00Z", "first"),
			`{"type":"session_info","id":"i","timestamp":"2026-01-04T00:00:00Z","name":"My session"}`,
			`{"type":"message","id":"m3","timestamp":"2026-01-03T00:00:00Z","role":"assistant","parts":[{"type":"text","data":{"text":"`+bigText+`"}}]}`)
		info, err := extractSessionInfo(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.MessageCount != 2 || info.FirstMessage != "first" || info.Name != "My session" {
			t.Errorf("got count=%d preview=%q name=%q", info.MessageCount, info.FirstMessage, info.Name)
		}
		if want := "2026-01-04T00:00:00Z"; info.Modified.UTC().Format(time.RFC3339) != want {
			t.Errorf("Modified = %v, want %s", info.Modified, want)
		}
	})
}
