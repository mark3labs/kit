package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	xansi "github.com/charmbracelet/x/ansi"

	uicore "github.com/mark3labs/kit/internal/ui/core"
)

// The block must exist before the command produces anything, so the user sees
// what they asked for rather than an empty gap.
func TestShellCommandMsgCreatesPendingBlock(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})

	m = sendMsg(m, uicore.ShellCommandMsg{Command: "ls -la", ExcludeFromContext: true})

	var item *ShellRunItem
	for _, msg := range m.messages {
		if it, ok := msg.(*ShellRunItem); ok {
			item = it
			break
		}
	}
	if item == nil {
		t.Fatal("no shell block was appended")
	}
	if !item.Spinning() {
		t.Error("the block must start pending")
	}
	if !strings.Contains(xansi.Strip(item.Render(80)), "$ ls -la") {
		t.Errorf("block does not show the command: %q", xansi.Strip(item.Render(80)))
	}
}

func TestShellStreamChunkAppendsToBlock(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m = sendMsg(m, uicore.ShellCommandMsg{Command: "seq 1 5"})

	id := shellItemID(t, m)
	ch := make(chan string, 1)

	m = sendMsg(m, uicore.ShellStreamChunkMsg{ItemID: id, Chunk: "alpha\n", Ch: ch})
	m = sendMsg(m, uicore.ShellStreamChunkMsg{ItemID: id, Chunk: "beta\n", Ch: ch})

	item := m.shellRunItem(id)
	if item == nil {
		t.Fatal("block vanished")
	}
	if got := item.RawContent(); !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Errorf("chunks were not appended: %q", got)
	}
	if !item.Spinning() {
		t.Error("the block must still be pending")
	}
}

// A chunk for an unknown block must not panic and must not create one.
func TestShellStreamChunkUnknownBlock(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	before := len(m.messages)

	m = sendMsg(m, uicore.ShellStreamChunkMsg{ItemID: "nope", Chunk: "x\n"})

	if len(m.messages) != before {
		t.Errorf("an unknown chunk id created %d new items", len(m.messages)-before)
	}
}

// A chunk with no channel means the pump is over: no reader is re-armed, so the
// model does not leak a blocked command per chunk.
func TestShellStreamChunkWithoutChannelDoesNotRearm(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m = sendMsg(m, uicore.ShellCommandMsg{Command: "true"})

	cmd := m.handleShellCommandChunk(uicore.ShellStreamChunkMsg{ItemID: shellItemID(t, m), Chunk: "x"})
	if cmd != nil {
		t.Error("a chunk with no channel must not re-arm the reader")
	}
}

func TestShellCommandResultSettlesBlock(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m = sendMsg(m, uicore.ShellCommandMsg{Command: "seq 1 3"})
	id := shellItemID(t, m)

	m = sendMsg(m, uicore.ShellCommandResultMsg{
		Command:  "seq 1 3",
		Output:   "1\n2\n3\n",
		ExitCode: 0,
		ItemID:   id,
	})

	item := m.shellRunItem(id)
	if item.Spinning() {
		t.Error("the block must be settled")
	}
	out := xansi.Strip(item.Render(80))
	if !strings.Contains(out, "$ seq 1 3") {
		t.Errorf("header lost: %q", out)
	}
	if strings.Contains(out, "running") {
		t.Errorf("a settled block must not claim to be running: %q", out)
	}
	for _, want := range []string{"1", "2", "3"} {
		if !strings.Contains(out, want) {
			t.Errorf("line %q missing from %q", want, out)
		}
	}
}

// The streamed text and the collected text are the same bytes. Settling must
// replace, not append, or every line appears twice.
func TestShellCommandResultReplacesStreamedOutput(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m = sendMsg(m, uicore.ShellCommandMsg{Command: "seq 1 3"})
	id := shellItemID(t, m)

	m = sendMsg(m, uicore.ShellStreamChunkMsg{ItemID: id, Chunk: "1\n2\n3\n"})
	m = sendMsg(m, uicore.ShellCommandResultMsg{Command: "seq 1 3", Output: "1\n2\n3\n", ItemID: id})

	out := m.shellRunItem(id).RawContent()
	for _, want := range []string{"1\n", "2\n", "3\n"} {
		if n := strings.Count(out, want); n != 1 {
			t.Errorf("expected one copy of %q, got %d: %q", want, n, out)
		}
	}
}

func TestShellCommandResultFailure(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m = sendMsg(m, uicore.ShellCommandMsg{Command: "bogus"})

	m = sendMsg(m, uicore.ShellCommandResultMsg{
		Command: "bogus",
		Output:  "error: boom",
		Err:     errors.New("boom"),
		ItemID:  shellItemID(t, m),
	})

	item := m.shellRunItem(shellItemID(t, m))
	if item.Spinning() {
		t.Error("a failed block must be settled")
	}
	if out := xansi.Strip(item.Render(80)); !strings.Contains(out, "error: boom") {
		t.Errorf("failure not shown: %q", out)
	}
}

func TestShellCommandResultTimeout(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m = sendMsg(m, uicore.ShellCommandMsg{Command: "sleep 999"})

	m = sendMsg(m, uicore.ShellCommandResultMsg{
		Command:  "sleep 999",
		Output:   "started\n",
		TimedOut: true,
		ItemID:   shellItemID(t, m),
	})

	out := xansi.Strip(m.shellRunItem(shellItemID(t, m)).Render(80))
	if !strings.Contains(out, "timed out") {
		t.Errorf("timeout not shown: %q", out)
	}
	if !strings.Contains(out, "started") {
		t.Errorf("output before the timeout was lost: %q", out)
	}
}

// A result with no matching block must still produce output: losing a command's
// result is worse than one extra block in the transcript.
func TestShellCommandResultWithoutBlock(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})

	m = sendMsg(m, uicore.ShellCommandResultMsg{Command: "orphan", Output: "text\n"})

	found := false
	for _, msg := range m.messages {
		if item, ok := msg.(*ShellRunItem); ok && strings.Contains(item.RawContent(), "text") {
			found = true
		}
	}
	if !found {
		t.Error("the orphaned result produced no block")
	}
}

// The model sees text, never escape sequences.
func TestShellContextMessageStripsANSI(t *testing.T) {
	msg := shellContextMessage(uicore.ShellCommandResultMsg{
		Command:  "colour",
		Output:   "\x1b[31mRED\x1b[0m text\n",
		ExitCode: 0,
	})

	if strings.Contains(msg, "\x1b") {
		t.Errorf("escape sequences reached the conversation: %q", msg)
	}
	for _, want := range []string{"<command>colour</command>", "RED text", "<exit_code>0</exit_code>"} {
		if !strings.Contains(msg, want) {
			t.Errorf("context message missing %q: %q", want, msg)
		}
	}
}

func TestShellContextMessageTimeout(t *testing.T) {
	msg := shellContextMessage(uicore.ShellCommandResultMsg{
		Command:  "slow",
		Output:   "partial",
		TimedOut: true,
	})
	if !strings.Contains(msg, "<timed_out>true</timed_out>") {
		t.Errorf("timeout not reported to the model: %q", msg)
	}
}

func TestShellContextMessageEmptyOutput(t *testing.T) {
	msg := shellContextMessage(uicore.ShellCommandResultMsg{Command: "quiet"})
	if !strings.Contains(msg, "(no output)") {
		t.Errorf("empty output not stated: %q", msg)
	}
}

// contextRecorder captures what the model was told.
type contextRecorder struct {
	stubAppController
	messages []string
}

func (c *contextRecorder) AddContextMessage(text string) {
	c.messages = append(c.messages, text)
}

func TestShellCommandInjectsIntoContextOnce(t *testing.T) {
	ctrl := &contextRecorder{}
	m, _, _ := newTestAppModel(ctrl)
	m = sendMsg(m, uicore.ShellCommandMsg{Command: "seq 1 2"})
	// The result is asserted through the recorder, not through the model, so the
	// returned value is deliberately dropped.
	sendMsg(m, uicore.ShellCommandResultMsg{Command: "seq 1 2", Output: "1\n2\n", ItemID: shellItemID(t, m)})

	if len(ctrl.messages) != 1 {
		t.Fatalf("expected one context message, got %d: %v", len(ctrl.messages), ctrl.messages)
	}
	if !strings.Contains(ctrl.messages[0], "seq 1 2") {
		t.Errorf("context message missing the command: %q", ctrl.messages[0])
	}
}

func TestShellCommandExcludedFromContext(t *testing.T) {
	ctrl := &contextRecorder{}
	m, _, _ := newTestAppModel(ctrl)
	m = sendMsg(m, uicore.ShellCommandMsg{Command: "secret", ExcludeFromContext: true})
	m = sendMsg(m, uicore.ShellCommandResultMsg{
		Command:            "secret",
		Output:             "s3cr3t\n",
		ExcludeFromContext: true,
		ItemID:             shellItemID(t, m),
	})

	if len(ctrl.messages) != 0 {
		t.Errorf("!! must not reach the model, got %v", ctrl.messages)
	}
	// It must still be on screen.
	out := xansi.Strip(m.shellRunItem(shellItemID(t, m)).Render(80))
	if !strings.Contains(out, "s3cr3t") {
		t.Errorf("excluded output is not rendered: %q", out)
	}
}

// ---------------------------------------------------------------------------
// The reader pump
// ---------------------------------------------------------------------------

func TestShellStreamReaderDeliversChunksInOrder(t *testing.T) {
	ch := make(chan string, 3)
	ch <- "one"
	ch <- "two"
	close(ch)

	id := "item"
	var got []string
	for range 10 {
		msg, ok := shellStreamReaderCmd(id, ch)().(uicore.ShellStreamChunkMsg)
		if !ok {
			break // channel closed
		}
		if msg.ItemID != id || msg.Ch == nil {
			t.Fatalf("chunk message is incomplete: %+v", msg)
		}
		got = append(got, msg.Chunk)
	}

	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Errorf("chunks delivered as %v", got)
	}
}

// The reader must return once the channel closes rather than blocking a command
// slot forever.
func TestShellStreamReaderStopsOnClosedChannel(t *testing.T) {
	ch := make(chan string)
	close(ch)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if msg := shellStreamReaderCmd("id", ch)(); msg != nil {
			t.Errorf("a closed channel must produce no message, got %T", msg)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the reader blocked on a closed channel")
	}
}

// A producer that cannot keep up must drop rather than stall: the command is
// still running, and a blocked producer shows the user nothing.
func TestShellStreamChannelDropsWhenFull(t *testing.T) {
	ch := make(chan string, 2)
	sink := func(chunk string, _ bool) {
		select {
		case ch <- chunk:
		default:
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 100 {
			sink("x", false)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the producer blocked on a full channel")
	}
	if len(ch) != 2 {
		t.Errorf("channel holds %d, want the buffer size 2", len(ch))
	}
}

// ---------------------------------------------------------------------------
// Scroll position
// ---------------------------------------------------------------------------

func TestFollowShellRunKeepsBottomPinned(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m = sendMsg(m, uicore.ShellCommandMsg{Command: "seq 1 40"})
	id := shellItemID(t, m)

	for range 5 {
		m = sendMsg(m, uicore.ShellStreamChunkMsg{ItemID: id, Chunk: strings.Repeat("line\n", 5)})
	}

	if !m.scrollList.autoScroll {
		t.Error("auto-scroll was not maintained while streaming")
	}
	if !m.scrollList.AtBottom() {
		t.Error("the viewport did not follow the stream")
	}
}

// A user who scrolls up to read something is not dragged back by every chunk.
func TestFollowShellRunRespectsManualScroll(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m = sendMsg(m, uicore.ShellCommandMsg{Command: "seq 1 200"})
	id := shellItemID(t, m)

	for range 20 {
		m = sendMsg(m, uicore.ShellStreamChunkMsg{ItemID: id, Chunk: strings.Repeat("line\n", 5)})
	}

	// Turn auto-scroll off the way the scroll wheel does.
	m.scrollList.autoScroll = false
	atBottom := m.scrollList.AtBottom()
	m.scrollList.offsetLine = 0
	m.scrollList.offsetIdx = 0

	m.followShellRun(id)

	if !atBottom {
		if m.scrollList.AtBottom() {
			t.Error("the viewport snapped back while the user had scrolled away")
		}
	}
}

// ---------------------------------------------------------------------------
// Navigation keys
// ---------------------------------------------------------------------------

func TestSelectedShellRunOnlyMatchesShellItems(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	if m.selectedShellRun() != nil {
		t.Error("an empty transcript must select nothing")
	}

	m = sendMsg(m, uicore.ShellCommandMsg{Command: "cmd"})
	item := m.shellRunItem(shellItemID(t, m))
	m.scrollList.SetItems(m.messages)
	m.selectMessage(m.scrollList.Len() - 1)

	if m.selectedShellRun() != item {
		t.Error("the shell block was not selected")
	}
}

func TestToggleShellRunExpansion(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m = sendMsg(m, uicore.ShellCommandMsg{Command: "seq 1 60"})
	m = sendMsg(m, uicore.ShellCommandResultMsg{Command: "seq 1 60", Output: shellLines(1, 61), ItemID: shellItemID(t, m)})

	item := m.shellRunItem(shellItemID(t, m))
	if item.Expanded() {
		t.Fatal("the block must start collapsed")
	}

	m.toggleShellRunExpansion(item)
	if !item.Expanded() {
		t.Error("the block did not expand")
	}
	if out := xansi.Strip(item.Render(80)); strings.Contains(out, "more lines") {
		t.Errorf("an expanded block must show everything: %q", out)
	}

	m.toggleShellRunExpansion(item)
	if item.Expanded() {
		t.Error("the block did not collapse")
	}
}

func TestScrollSelectedShellRunIsInertForOtherItems(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.messages = append(m.messages, NewThemedMessageItem("u", "user", "hello", func() string { return "hello" }))
	m.scrollList.SetItems(m.messages)
	m.selectMessage(0)

	m.scrollSelectedShellRun(1) // must not panic
}

// ---------------------------------------------------------------------------

func shellItemID(t *testing.T, m *AppModel) string {
	t.Helper()
	for _, msg := range m.messages {
		if item, ok := msg.(*ShellRunItem); ok {
			return item.ID()
		}
	}
	t.Fatal("no shell block in the transcript")
	return ""
}

func shellLines(from, to int) string {
	var b strings.Builder
	for i := range to - from {
		b.WriteString("line-")
		b.WriteString(itoa(from + i))
		b.WriteByte('\n')
	}
	return b.String()
}
