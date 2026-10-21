package session

import (
	"bufio"
	"errors"
	"os"
	"testing"

	"github.com/mark3labs/kit/internal/message"
)

func TestTreeEntryHeaders(t *testing.T) {
	header := Entry{ID: "entry-id", ParentID: "parent-id"}
	entries := []any{
		&MessageEntry{Entry: header},
		&IncompleteOutputEntry{Entry: header},
		&ModelChangeEntry{Entry: header},
		&BranchSummaryEntry{Entry: header},
		&LabelEntry{Entry: header},
		&SessionInfoEntry{Entry: header},
		&ExtensionDataEntry{Entry: header},
		&CompactionEntry{Entry: header},
	}
	tm := InMemoryTreeSession(t.TempDir())
	for _, entry := range entries {
		if got := tm.EntryID(entry); got != header.ID {
			t.Errorf("%T ID = %q, want %q", entry, got, header.ID)
		}
		if got := tm.entryParentID(entry); got != header.ParentID {
			t.Errorf("%T ParentID = %q, want %q", entry, got, header.ParentID)
		}
	}
	for _, entry := range []any{nil, struct{}{}, &SessionHeader{ID: "session-id"}, &SystemPromptEntry{ID: "prompt-id"}} {
		if tm.EntryID(entry) != "" || tm.entryParentID(entry) != "" {
			t.Errorf("%T must not have a tree header", entry)
		}
	}
}

type appendErrorWriter struct{ err error }

func (w appendErrorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestAppendLeafFlushErrors(t *testing.T) {
	tests := []struct {
		what   string
		append func(*TreeManager) (string, error)
	}{
		{"message", func(tm *TreeManager) (string, error) {
			return tm.AppendMessage(message.Message{Role: message.RoleUser})
		}},
		{"model change", func(tm *TreeManager) (string, error) { return tm.AppendModelChange("provider", "model") }},
		{"branch summary", func(tm *TreeManager) (string, error) { return tm.AppendBranchSummary("", "summary") }},
		{"label", func(tm *TreeManager) (string, error) { return tm.AppendLabel("target", "label") }},
		{"session info", func(tm *TreeManager) (string, error) { return tm.AppendSessionInfo("name") }},
		{"extension data", func(tm *TreeManager) (string, error) { return tm.AppendExtensionData("type", "data") }},
		{"compaction", func(tm *TreeManager) (string, error) { return tm.AppendCompaction("summary", "", 0, 0, 0, nil, nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.what, func(t *testing.T) {
			tm := InMemoryTreeSession(t.TempDir())
			file, err := os.CreateTemp(t.TempDir(), "session")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = file.Close() })
			tm.file = file
			tm.writer = bufio.NewWriter(file)
			flushErr := errors.New("flush failed")
			// The durable append flushes first. Inject a buffered failure after
			// its sync to exercise the final, method-specific flush wrapper.
			tm.syncHook = func() error {
				tm.writer.Reset(appendErrorWriter{err: flushErr})
				_, err := tm.writer.WriteString("buffered")
				return err
			}
			id, err := tt.append(tm)
			if !errors.Is(err, flushErr) || err.Error() != "failed to flush "+tt.what+": flush failed" {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != "" || tm.leafID != "" {
				t.Fatalf("failed append advanced leaf: id=%q leaf=%q", id, tm.leafID)
			}
		})
	}
}
