package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
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
	path := tm.GetFilePath()
	if _, err := tm.AppendMessage(newTestMessage("before")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	leafBefore := tm.GetLeafID()
	countBefore := tm.MessageCount()
	fileBefore, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Kill the underlying handle so every write fails. The step must come
	// back as an error AND leave no divergence between memory and disk: no
	// index entry, no leaf movement, no partial line in the file.
	if err := tm.file.Close(); err != nil {
		t.Fatalf("closing file handle: %v", err)
	}
	if _, err := tm.AppendStep(context.Background(), fantasyStepMessages("call-3")); err == nil {
		t.Fatalf("AppendStep succeeded on a closed file, want error")
	}
	if got := tm.MessageCount(); got != countBefore {
		t.Errorf("MessageCount = %d, want %d (indices must not advance on a failed step)", got, countBefore)
	}
	if got := tm.GetLeafID(); got != leafBefore {
		t.Errorf("leaf = %q, want %q", got, leafBefore)
	}
	fileAfter, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after failure: %v", err)
	}
	if !bytes.Equal(fileBefore, fileAfter) {
		t.Errorf("file changed on a failed step:\nbefore: %q\nafter:  %q", fileBefore, fileAfter)
	}

	// And the refusal persists: further appends on this broken handle must
	// error, not silently degrade to memory-only.
	if _, err := tm.AppendMessage(newTestMessage("nope")); err == nil {
		t.Errorf("AppendMessage on a broken file handle succeeded, want error")
	}

	// The transcript must be intact after a fresh open: only the message
	// that was persisted before the failed step is there, with no torn
	// fragment or half-step at the tail. Close reports the already-broken
	// handle, which this test closed on purpose, so the error is ignored.
	_ = tm.Close()
	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("reopen after failed step: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	msgs, _, _ := reopened.BuildContext()
	if len(msgs) != 1 || textOfFantasy(t, msgs[0]) != "before" {
		t.Fatalf("context after failed step = %v, want just the \"before\" message", msgs)
	}
	if got := countLines(t, path); got != 2 {
		t.Errorf("lines after reopen = %d, want 2 (header + one message)", got)
	}
}

// TestAppendStepRollsBackAfterWrite reaches the failure AFTER the step's
// bytes reached the file (the sync fails), which is the path rollbackStep
// exists for: the seek succeeds, the write and flush succeed, then the sync
// failure must cut the file back and leave no trace of the step.
func TestAppendStepRollsBackAfterWrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()
	if _, err := tm.AppendMessage(newTestMessage("before")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	fileBefore, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	leafBefore := tm.GetLeafID()
	countBefore := tm.MessageCount()

	tm.syncHook = func() error { return errors.New("sync failed") }
	if _, err := tm.AppendStep(context.Background(), fantasyStepMessages("call-7")); err == nil {
		t.Fatalf("AppendStep succeeded with a failing sync, want error")
	}
	tm.syncHook = nil

	// The step is gone from memory AND from the file.
	if got := tm.MessageCount(); got != countBefore {
		t.Errorf("MessageCount = %d, want %d", got, countBefore)
	}
	if got := tm.GetLeafID(); got != leafBefore {
		t.Errorf("leaf = %q, want %q", got, leafBefore)
	}
	fileAfter, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after rollback: %v", err)
	}
	if !bytes.Equal(fileBefore, fileAfter) {
		t.Errorf("file did not roll back:\nbefore: %q\nafter:  %q", fileBefore, fileAfter)
	}

	// The writer survived the rollback (reset, not dead): the next append
	// persists cleanly.
	if _, err := tm.AppendMessage(newTestMessage("after")); err != nil {
		t.Fatalf("AppendMessage after rollback: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("reopen after rollback: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	msgs, _, _ := reopened.BuildContext()
	if len(msgs) != 2 {
		t.Fatalf("len(messages) = %d, want 2 (before + after)", len(msgs))
	}
}

// TestAppendStepRollbackKeepsOtherManagersEntries pins the rollback point:
// entries another manager committed before this step started (the file end
// at seek time) must survive this step's failure.
func TestAppendStepRollbackKeepsOtherManagersEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	first, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := first.GetFilePath()
	if _, err := first.AppendMessage(newTestMessage("from first")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	second, err := OpenTreeSession(path) // reentrant in-process open
	if err != nil {
		t.Fatalf("OpenTreeSession (second manager): %v", err)
	}
	defer func() { _ = second.Close() }()
	if _, err := second.AppendMessage(newTestMessage("from second")); err != nil {
		t.Fatalf("second AppendMessage: %v", err)
	}

	first.syncHook = func() error { return errors.New("sync failed") }
	if _, err := first.AppendStep(context.Background(), fantasyStepMessages("call-8")); err == nil {
		t.Fatalf("AppendStep succeeded with a failing sync, want error")
	}
	first.syncHook = nil

	// The second manager's committed entry must still be on disk after the
	// rollback, and the file must still hold exactly the two messages.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "from second") {
		t.Errorf("rollback cut away another manager's committed entry")
	}
	if got := countLines(t, path); got != 3 {
		t.Errorf("lines = %d, want 3 (header + two messages)", got)
	}
	_ = first.Close()
}

func TestAppendAfterCloseRefusedForPersistedSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// A closed session with a file path is not an in-memory session: a
	// memory-only append would vanish from the transcript the next open
	// reads. It must be refused, not silently swallowed.
	if _, err := tm.AppendMessage(newTestMessage("after close")); err == nil {
		t.Fatalf("AppendMessage after Close succeeded, want error")
	}
	if _, err := tm.AppendStep(context.Background(), fantasyStepMessages("call-9")); err == nil {
		t.Fatalf("AppendStep after Close succeeded, want error")
	}

	// In-memory sessions keep their memory-only behavior.
	mem := InMemoryTreeSession(t.TempDir())
	if _, err := mem.AppendStep(context.Background(), fantasyStepMessages("call-1")); err != nil {
		t.Fatalf("AppendStep on in-memory session: %v", err)
	}
	if mem.MessageCount() != 2 {
		t.Errorf("in-memory step MessageCount = %d, want 2", mem.MessageCount())
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

// TestOpenToleratesTornFinalLineWithNewline covers the torn final line that
// ends with a newline: a fragment an older append landed on top of, or any
// complete but corrupt final entry. The trim must cut at the line's start
// offset, not after the last newline (which would keep the bad line and
// make the session unopenable after the next append).
func TestOpenToleratesTornFinalLineWithNewline(t *testing.T) {
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

	// Corrupt final line, properly newline-terminated.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	if _, err := f.WriteString(`{"broken":}` + "\n"); err != nil {
		t.Fatalf("write bad line: %v", err)
	}
	_ = f.Close()

	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession with newline-terminated torn line: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "broken") {
		t.Errorf("corrupt line survived the trim: %q", data)
	}

	// The regression this guards against: appending after the trim, then
	// reopening, must not turn the fragment into a middle line.
	if _, err := reopened.AppendMessage(newTestMessage("after")); err != nil {
		t.Fatalf("AppendMessage after trim: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	again, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("reopen after append: %v", err)
	}
	defer func() { _ = again.Close() }()
	msgs, _, _ := again.BuildContext()
	if len(msgs) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(msgs))
	}
}

// TestSessionLockConcurrentOpensNoFalseConflict hammers the race between the
// lock table lookup and registering the entry. Conflicts are per handle, so
// two in-process openers that both miss the table must serialize on the
// table rather than fail with a false "another process" error.
func TestSessionLockConcurrentOpensNoFalseConflict(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	const openers = 8
	const rounds = 25
	var mu sync.Mutex
	var failures []error
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range openers {
		wg.Go(func() {
			<-start
			for range rounds {
				opened, err := OpenTreeSession(path)
				if err != nil {
					mu.Lock()
					failures = append(failures, err)
					mu.Unlock()
					continue
				}
				if err := opened.Close(); err != nil {
					mu.Lock()
					failures = append(failures, err)
					mu.Unlock()
				}
			}
		})
	}
	close(start)
	wg.Wait()

	if len(failures) != 0 {
		t.Fatalf("%d of %d concurrent opens failed, first: %v", len(failures), openers*rounds, failures[0])
	}

	// Every open closed again, so the file must be free for a fresh open.
	final, err := OpenTreeSession(path)
	if err != nil {
		t.Fatalf("final open: %v", err)
	}
	if err := final.Close(); err != nil {
		t.Fatalf("final close: %v", err)
	}
}

// TestSetParentLinkErrorKeepsLock checks the rewrite's error paths: the
// session must stay locked and appendable when the rewrite fails, instead of
// running on with the lock already dropped.
func TestSetParentLinkErrorKeepsLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits do not block file creation on Windows")
	}
	t.Setenv("HOME", t.TempDir())

	cwd := t.TempDir()
	tm, err := CreateTreeSession(cwd)
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()
	defer func() { _ = tm.Close() }()

	// The header rewrite creates its temp file in the session directory; a
	// read-only directory fails that create, which used to return with the
	// lock already released.
	sessionDir := filepathDir(path)
	if err := os.Chmod(sessionDir, 0o500); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer func() { _ = os.Chmod(sessionDir, 0o755) }()

	if err := tm.SetParentLink("/parent/s.jsonl", "parent-id", "task"); err == nil {
		t.Fatalf("SetParentLink succeeded on a read-only directory, want error")
	}

	// The lock must still be held: another "process" cannot take it.
	other, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = other.Close() }()
	if err := lockFileExclusive(other); err == nil {
		t.Errorf("session lock was dropped by the failed rewrite; another process could now interleave appends")
		return
	}
	unlockFile(other)

	// The session keeps working.
	if _, err := tm.AppendMessage(newTestMessage("still alive")); err != nil {
		t.Errorf("AppendMessage after failed rewrite: %v", err)
	}
}

// filepathDir is filepath.Dir without importing path/filepath for one call.
func filepathDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return "."
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
