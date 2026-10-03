package session

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/message"
)

// newAssistantToolCallMsg builds an assistant message carrying one tool call.
func newAssistantToolCallMsg(id, name string) message.Message {
	return message.Message{
		Role: message.RoleAssistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "running " + name},
			message.ToolCall{ID: id, Name: name, Input: "{}", Finished: true},
		},
	}
}

// newAssistantMultiCallMsg builds an assistant message with two tool calls.
func newAssistantMultiCallMsg(ids ...string) message.Message {
	parts := []message.ContentPart{message.TextContent{Text: "two calls"}}
	for _, id := range ids {
		parts = append(parts, message.ToolCall{ID: id, Name: "tool_" + id, Input: "{}", Finished: true})
	}
	return message.Message{
		Role:  message.RoleAssistant,
		Parts: parts,
	}
}

// newToolResultMsg builds a tool-role message with one result.
func newToolResultMsg(id, name, content string) message.Message {
	return message.Message{
		Role: message.RoleTool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: id, Name: name, Content: content},
		},
	}
}

// fantasyStepMessages mirrors the step kit hands to AppendStep: an assistant
// message with a tool call followed by the tool message with its result.
func fantasyStepMessages(callID string) []fantasy.Message {
	return []fantasy.Message{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: "running"},
				fantasy.ToolCallPart{ToolCallID: callID, ToolName: "bash", Input: "{}"},
			},
		},
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: callID,
					Output:     fantasy.ToolResultOutputContentText{Text: "ok"},
				},
			},
		},
	}
}

// countLines counts non-blank lines in a file.
func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	n := 0
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			n++
		}
	}
	return n
}

// --- AppendStep ---

func TestAppendStepPersistsWholeStep(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()

	ids, err := tm.AppendStep(context.Background(), fantasyStepMessages("call-1"))
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("len(ids) = %d, want 2", len(ids))
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Header plus exactly the two step lines: both lines of the step reached
	// the disk together.
	if got := countLines(t, path); got != 3 {
		t.Fatalf("lines = %d, want 3 (header + 2 step messages)", got)
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
	if msgs[0].Role != fantasy.MessageRoleAssistant || msgs[1].Role != fantasy.MessageRoleTool {
		t.Fatalf("roles = %v, %v; want assistant, tool", msgs[0].Role, msgs[1].Role)
	}
	// The result must answer the call, or the transcript is still broken.
	var result fantasy.ToolResultPart
	for _, p := range msgs[1].Content {
		if rp, ok := p.(fantasy.ToolResultPart); ok {
			result = rp
		}
	}
	if result.ToolCallID == "" {
		t.Fatalf("tool message carries no result")
	}
	if result.ToolCallID != "call-1" {
		t.Errorf("result ToolCallID = %q, want call-1", result.ToolCallID)
	}
}

func TestAppendStepChainsParents(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	defer func() { _ = tm.Close() }()

	userID, err := tm.AppendMessage(newTestMessage("hello"))
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	stepIDs, err := tm.AppendStep(context.Background(), fantasyStepMessages("call-2"))
	if err != nil {
		t.Fatalf("AppendStep: %v", err)
	}

	// Every step entry must chain from the previous position.
	if prev := tm.GetLeafID(); prev != stepIDs[1] {
		t.Fatalf("leaf = %q, want last step id %q", prev, stepIDs[1])
	}
	wantParents := []string{userID, stepIDs[0]}
	for i, id := range stepIDs {
		e := tm.GetEntry(id)
		if e == nil {
			t.Fatalf("step entry %d missing", i)
		}
		if me, ok := e.(*MessageEntry); ok {
			if me.ParentID != wantParents[i] {
				t.Errorf("entry %d parent = %q, want %q", i, me.ParentID, wantParents[i])
			}
		}
	}
}

func TestAppendStepFailureReportsError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	defer func() { _ = tm.Close() }()

	// Kill the underlying handle so the buffered writes fail. The step must
	// come back as an error, not as a silently dropped half-step.
	if err := tm.file.Close(); err != nil {
		t.Fatalf("closing file handle: %v", err)
	}
	_, err = tm.AppendStep(context.Background(), fantasyStepMessages("call-3"))
	if err == nil {
		t.Fatalf("AppendStep succeeded on a closed file, want error")
	}
}

// --- Repair of interrupted tool calls ---

func TestRepairAddsInterruptedResults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()

	if _, err := tm.AppendMessage(newTestMessage("list files")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if _, err := tm.AppendMessage(newAssistantToolCallMsg("call-1", "bash")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	// Simulate the crash: no result is appended. Close without Sync? Close
	// flushes, which is what a dying process manages for a completed line.
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	msgs, _, _ := reopened.BuildContext()
	if len(msgs) != 3 {
		t.Fatalf("len(messages) = %d, want 3 (user, assistant, repaired tool)", len(msgs))
	}
	last := msgs[len(msgs)-1]
	if last.Role != fantasy.MessageRoleTool {
		t.Fatalf("last role = %q, want tool", last.Role)
	}
	part, ok := last.Content[0].(fantasy.ToolResultPart)
	if !ok {
		t.Fatalf("last message part is %T, want ToolResultPart", last.Content[0])
	}
	if part.ToolCallID != "call-1" {
		t.Errorf("repaired result ToolCallID = %q, want call-1", part.ToolCallID)
	}
	if _, isErr := part.Output.(fantasy.ToolResultOutputContentError); !isErr {
		t.Errorf("repaired result output is %T, want error content", part.Output)
	}

	// The repair must survive a reopen: the synthetic result is a normal
	// entry on disk.
	if err := reopened.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	again, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("reopen after repair: %v", err)
	}
	defer func() { _ = again.Close() }()
	if got := countLines(t, path); got != 4 {
		t.Errorf("lines after reopen = %d, want 4 (no second repair)", got)
	}

	// The session must be extendable: the repaired tail is a normal leaf.
	if _, err := again.AppendMessage(newTestMessage("continue")); err != nil {
		t.Fatalf("AppendMessage after repair: %v", err)
	}
}

func TestRepairFillsOnlyMissingResults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()

	if _, err := tm.AppendMessage(newAssistantMultiCallMsg("call-a", "call-b")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	// A partial step: one result made it to disk before the process died.
	if _, err := tm.AppendMessage(newToolResultMsg("call-a", "tool_a", "done")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	msgs, _, _ := reopened.BuildContext()
	if len(msgs) != 3 {
		t.Fatalf("len(messages) = %d, want 3 (assistant, tool, repaired tool)", len(msgs))
	}
	part, ok := msgs[2].Content[0].(fantasy.ToolResultPart)
	if !ok {
		t.Fatalf("repaired message part is %T", msgs[2].Content[0])
	}
	if part.ToolCallID != "call-b" {
		t.Errorf("repaired ToolCallID = %q, want call-b (call-a already answered)", part.ToolCallID)
	}
}

func TestRepairSkipsCleanSessions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()

	if _, err := tm.AppendMessage(newTestMessage("q")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if _, err := tm.AppendMessage(newAssistantToolCallMsg("call-1", "bash")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if _, err := tm.AppendMessage(newToolResultMsg("call-1", "bash", "ok")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("clean session changed on open:\nbefore: %q\nafter:  %q", before, after)
	}
}

func TestRepairStopsAtUserBoundary(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()

	if _, err := tm.AppendMessage(newAssistantToolCallMsg("call-1", "bash")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	// The user already moved on: the turn is closed, and a result injected
	// after their next message would corrupt the alternation the repair
	// exists to protect. (Older providers would reject the orphaned call
	// either way, but this state is not what a crash produces.)
	if _, err := tm.AppendMessage(newTestMessage("never mind")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	if got := countLines(t, path); got != 3 {
		t.Errorf("lines = %d, want 3 (no repair appended)", got)
	}
}

func TestRepairStopsAtCompaction(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()

	userID, err := tm.AppendMessage(newTestMessage("q"))
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if _, err := tm.AppendMessage(newAssistantToolCallMsg("call-1", "bash")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	// Compaction resets the branch: everything before it is out of context.
	if _, err := tm.AppendCompaction("summary", userID, 100, 50, 1, nil, nil); err != nil {
		t.Fatalf("AppendCompaction: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	if got := countLines(t, path); got != 4 {
		t.Errorf("lines = %d, want 4 (no repair appended)", got)
	}
}

// --- Session file lock ---

func TestSessionLockBlocksOtherProcess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Simulate another process holding the lock on the file.
	other, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = other.Close() }()
	if err := lockFileExclusive(other); err != nil {
		t.Fatalf("lockFileExclusive: %v", err)
	}

	if _, err := OpenTreeSession(path); err == nil {
		t.Fatalf("OpenTreeSession succeeded while another process holds the lock")
	} else if !strings.Contains(err.Error(), "another process") {
		t.Errorf("error = %v, want a 'already open in another process' message", err)
	}

	// Releasing the lock lets the open through.
	unlockFile(other)
	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession after unlock: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSessionLockIsReentrantInProcess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()

	// Overlapping in-process opens are legitimate (resume opens the new
	// session before closing the old) and must not conflict.
	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("overlapping OpenTreeSession: %v", err)
	}

	if err := reopened.Close(); err != nil {
		t.Fatalf("Close reopened: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close original: %v", err)
	}

	// After both are closed the file must be free for a fresh open.
	third, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession after closes: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatalf("Close third: %v", err)
	}
}

// --- Torn final lines ---

func TestOpenToleratesTornFinalLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()
	if _, err := tm.AppendMessage(newTestMessage("complete")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// A process died mid-append: the final line is a JSON prefix with no
	// trailing newline.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	if _, err := f.WriteString(`{"type":"message","id":"tor`); err != nil {
		t.Fatalf("write torn line: %v", err)
	}
	_ = f.Close()

	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession with torn tail: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	msgs, _, _ := reopened.BuildContext()
	if len(msgs) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(msgs))
	}
	if got := textOfFantasy(t, msgs[0]); got != "complete" {
		t.Fatalf("context text = %q, want the one complete message", got)
	}

	// The fragment is trimmed, so appends do not grow it into corruption.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.HasSuffix(bytes.TrimRight(data, "\x00"), []byte("\n")) {
		t.Errorf("file does not end with a newline after trim: %q", data[len(data)-40:])
	}
	if strings.Contains(string(data), `"id":"tor`) {
		t.Errorf("torn fragment still present after trim")
	}

	if _, err := reopened.AppendMessage(newTestMessage("after")); err != nil {
		t.Fatalf("AppendMessage after trim: %v", err)
	}
}

func TestOpenRejectsCorruptMiddleLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	path := writeSessionFile(t,
		`{"type":"message","id":"m1","timestamp":"2026-01-01T00:00:00Z","role":"user","parts":[]}`,
		`{"type":"message"broken`,
		`{"type":"message","id":"m2","timestamp":"2026-01-01T00:00:01Z","role":"user","parts":[]}`,
	)
	if _, err := OpenTreeSession(path); err == nil {
		t.Fatalf("OpenTreeSession succeeded on a corrupt middle line, want error")
	}
}

func TestOpenRejectsTornHeader(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	path := filepathJoin(t, "torn-header.jsonl")
	content := `{"type":"session","id":"s`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := OpenTreeSession(path); err == nil {
		t.Fatalf("OpenTreeSession succeeded on a torn header, want error")
	} else if !strings.Contains(err.Error(), "torn") {
		t.Errorf("error = %v, want a torn-header message", err)
	}
}

func TestOpenRejectsUnknownEntryType(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// A valid JSON line with an entry type this build does not know is a
	// forward-compatibility case: it must be an error, never silently
	// dropped the way a torn write is.
	path := writeSessionFile(t,
		`{"type":"from_the_future","id":"x","timestamp":"2026-01-01T00:00:00Z"}`,
		`{"type":"message","id":"m1","timestamp":"2026-01-01T00:00:01Z","role":"user","parts":[]}`,
	)
	if _, err := OpenTreeSession(path); err == nil {
		t.Fatalf("OpenTreeSession succeeded on an unknown entry type, want error")
	} else if !strings.Contains(err.Error(), "unknown entry type") {
		t.Errorf("error = %v, want an unknown-entry-type message", err)
	}
}

// filepathJoin joins a name under a fresh temp dir (helper for tests that
// bypass the default session dir).
func filepathJoin(t *testing.T, name string) string {
	t.Helper()
	return fmt.Sprintf("%s/%s", t.TempDir(), name)
}

// textOfFantasy concatenates the text parts of a fantasy message.
func textOfFantasy(t *testing.T, msg fantasy.Message) string {
	t.Helper()
	var b strings.Builder
	for _, p := range msg.Content {
		if tp, ok := p.(fantasy.TextPart); ok {
			b.WriteString(tp.Text)
		}
	}
	return b.String()
}
