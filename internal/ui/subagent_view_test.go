package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/ui/commands"
	kit "github.com/mark3labs/kit/pkg/kit"
)

type inspectorTestController struct {
	stubAppController
	runs    []kit.SubagentRun
	stopped string
}

func (c *inspectorTestController) SubagentRuns() []kit.SubagentRun { return c.runs }
func (c *inspectorTestController) GetSubagentRun(id string) (kit.SubagentRun, bool) {
	for _, r := range c.runs {
		if r.ID == id {
			return r, true
		}
	}
	return kit.SubagentRun{}, false
}
func (c *inspectorTestController) KillSubagent(id string) bool { c.stopped = id; return true }

// childRunEvents is one turn of agent output: text, a shell tool with live
// output, and more text.
func childRunEvents() []app.Event {
	return []app.Event{
		app.StepStartedEvent{StepNumber: 1},
		app.StreamChunkEvent{Content: "Let me check."},
		app.ToolCallStartedEvent{ToolCallID: "t1", ToolName: "bash", ToolArgs: `{"command":"ls"}`},
		app.ToolExecutionEvent{ToolCallID: "t1", ToolName: "bash", ToolArgs: `{"command":"ls"}`, IsStarting: true},
		app.ToolOutputEvent{ToolCallID: "t1", ToolName: "bash", Chunk: "a.go\n"},
		app.ToolExecutionEvent{ToolCallID: "t1", ToolName: "bash", IsStarting: false},
		app.ToolResultEvent{ToolCallID: "t1", ToolName: "bash", ToolArgs: `{"command":"ls"}`, Result: "a.go"},
		app.StepStartedEvent{StepNumber: 2},
		app.StreamChunkEvent{Content: "Done."},
	}
}

// childModel returns a model whose controller retains the given runs, sized
// so that View works.
func childModel(t *testing.T, runs ...kit.SubagentRun) (*AppModel, *inspectorTestController) {
	t.Helper()
	c := &inspectorTestController{runs: runs}
	m, _ := newTestAppModelWithRealStream(c)
	m.parentPane = m.conversationPane
	m.input = NewInputComponent(80, nil)
	return sendMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30}), c
}

// sendRun sends live updates for a run, numbering them from seq.
func sendRun(m *AppModel, runID string, seq uint64, status string, events ...app.Event) (*AppModel, uint64) {
	if len(events) == 0 {
		seq++
		return sendMsg(m, app.SubagentRunEvent{RunID: runID, Seq: seq, Status: status}), seq
	}
	for _, e := range events {
		seq++
		m = sendMsg(m, app.SubagentRunEvent{RunID: runID, Seq: seq, Status: status, Event: e})
	}
	return m, seq
}

// paneItems describes the items of a pane as "role|raw content".
func paneItems(p *conversationPane) []string {
	var out []string
	for _, item := range p.messages {
		raw := ""
		if r, ok := item.(interface{ RawContent() string }); ok {
			raw = r.RawContent()
		}
		role := ""
		if r, ok := item.(interface{ Role() string }); ok {
			role = r.Role()
		}
		if role == "system" {
			// A receipt or notice holds its rendered text once it is drawn,
			// and a hidden pane is not drawn.
			raw = ""
		}
		out = append(out, fmt.Sprintf("%T|%s|%s", item, role, raw))
	}
	return out
}

// TestChildPaneMatchesParentTranscript is the contract of the subagent view:
// the same events make the same transcript in the parent pane and in a child
// pane, because both go through applyPaneEvent.
func TestChildPaneMatchesParentTranscript(t *testing.T) {
	parent, _ := childModel(t)
	parent = sendMsg(parent, app.SpinnerEvent{Show: true})
	for _, e := range childRunEvents() {
		parent = sendMsg(parent, e)
	}
	parent = sendMsg(parent, app.StepCompleteEvent{})

	m, _ := childModel(t, kit.SubagentRun{ID: "r", ParentSessionID: "p"})
	m, seq := sendRun(m, "r", 0, "running")
	m, seq = sendRun(m, "r", seq, "running", childRunEvents()...)
	m, _ = sendRun(m, "r", seq, "completed")

	child := m.childPanes["r"]
	if child == nil {
		t.Fatal("no child pane was made")
	}
	if m.conversationPane != m.parentPane {
		t.Fatal("a hidden run update changed the shown pane")
	}
	got, want := paneItems(child), paneItems(parent.conversationPane)
	if !slices.Equal(got, want) {
		t.Fatalf("child transcript differs from parent transcript\nchild:  %q\nparent: %q", got, want)
	}
}

// TestChildViewShowsLiveActivity: a shown child pane gets the activity row
// and the live shell output, as the parent does, and the composer is
// replaced by the read-only bar.
func TestChildViewShowsLiveActivity(t *testing.T) {
	m, _ := childModel(t, kit.SubagentRun{ID: "r", ParentSessionID: "p", Agent: "explore", Model: "prov/model-x"})
	m, seq := sendRun(m, "r", 0, "running")
	m, _ = sendRun(m, "r", seq, "running",
		app.ToolCallStartedEvent{ToolCallID: "t1", ToolName: "bash", ToolArgs: `{"command":"sleep 10"}`},
		app.ToolExecutionEvent{ToolCallID: "t1", ToolName: "bash", ToolArgs: `{"command":"sleep 10"}`, IsStarting: true},
		app.ToolOutputEvent{ToolCallID: "t1", ToolName: "bash", Chunk: "tick 1\n"},
	)
	m.paneRequest = &paneRequest{runID: "r", parentSessionID: "p"}
	m.applyPaneRequest()
	view := stripAnsi(m.View().Content)
	for _, want := range []string{"Running sleep 10", "tick 1", "Subagent · explore", "prov/model-x", "esc parent", "ctrl+k stop"} {
		if !strings.Contains(view, want) {
			t.Errorf("child view does not contain %q:\n%s", want, view)
		}
	}
	if m.state != stateInput {
		t.Fatalf("parent state changed to %v", m.state)
	}
}

// TestParentEventsWhileChildShown: the parent keeps its own transcript while
// the user views a child, and the child is not changed by parent events.
func TestParentEventsWhileChildShown(t *testing.T) {
	m, _ := childModel(t, kit.SubagentRun{ID: "r", ParentSessionID: "p"})
	m, _ = sendRun(m, "r", 0, "running")
	m.paneRequest = &paneRequest{runID: "r", parentSessionID: "p"}
	m.applyPaneRequest()
	child := m.conversationPane
	before := len(child.messages)

	m = sendMsg(m, app.SpinnerEvent{Show: true})
	m = sendMsg(m, app.StreamChunkEvent{Content: "parent text"})
	m = sendMsg(m, streamAppendFlushMsg{})
	if m.conversationPane != child {
		t.Fatal("a parent event changed the shown pane")
	}
	if len(child.messages) != before {
		t.Fatalf("parent event changed the child pane: %q", paneItems(child))
	}
	if items := paneItems(m.parentPane); len(items) == 0 || !strings.Contains(items[len(items)-1], "parent text") {
		t.Fatalf("parent pane missed its event: %q", items)
	}
	if m.state != stateWorking {
		t.Fatalf("parent state = %v, want working", m.state)
	}

	m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.childView != nil || m.conversationPane != m.parentPane {
		t.Fatal("esc did not show the parent pane")
	}
}

// TestChildStreamFlushGoesToChildPane: a child's flush tick names its pane
// and applies the buffered chunks there, not in the shown pane.
func TestChildStreamFlushGoesToChildPane(t *testing.T) {
	m, _ := childModel(t, kit.SubagentRun{ID: "r", ParentSessionID: "p"})
	m, seq := sendRun(m, "r", 0, "running")
	updated, cmd := m.Update(app.SubagentRunEvent{RunID: "r", Seq: seq + 1, Status: "running", Event: app.StreamChunkEvent{Content: "child text"}})
	m = updated.(*AppModel)
	child := m.childPanes["r"]
	if len(child.pendingStreamChunks) != 1 {
		t.Fatalf("chunk was not buffered: %d", len(child.pendingStreamChunks))
	}
	var flush tea.Msg
	var collect func(tea.Cmd)
	collect = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		switch msg := cmd().(type) {
		case tea.BatchMsg:
			for _, c := range msg {
				collect(c)
			}
		case streamAppendFlushMsg:
			flush = msg
		}
	}
	collect(cmd)
	if flush == nil || flush.(streamAppendFlushMsg).paneID != "r" {
		t.Fatalf("flush tick = %#v, want one for pane r", flush)
	}
	m = sendMsg(m, flush)
	if items := paneItems(child); !strings.Contains(items[len(items)-1], "child text") {
		t.Fatalf("child pane = %q", items)
	}
	if len(m.parentPane.messages) != 0 {
		t.Fatalf("parent pane got child text: %q", paneItems(m.parentPane))
	}
}

// TestChildPaneReplaysSnapshotAndSkipsOldUpdates: a pane made from a
// snapshot shows the retained events and ignores the live updates that the
// snapshot already contains.
func TestChildPaneReplaysSnapshotAndSkipsOldUpdates(t *testing.T) {
	run := kit.SubagentRun{
		ID: "r", ParentSessionID: "p", Prompt: "Find the bug", Status: "completed", Seq: 5,
		Events: []kit.Event{
			kit.MessageUpdateEvent{Chunk: "Looking."},
			kit.ToolResultEvent{ToolCallID: "t1", ToolName: "bash", ToolArgs: `{"command":"ls"}`, Result: "a.go"},
			kit.MessageUpdateEvent{Chunk: "Found it."},
		},
	}
	m, _ := childModel(t, run)
	m = sendMsg(m, tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl | tea.ModAlt})
	m = sendMsg(m, SubagentRunSelectedMsg{Index: 0})
	if m.childView == nil || m.paneID != "r" {
		t.Fatal("the picked run is not shown")
	}
	want := paneItems(m.conversationPane)
	if len(want) < 4 || !strings.Contains(want[0], "Find the bug") || !strings.Contains(want[len(want)-2], "Found it.") {
		t.Fatalf("replayed pane = %q", want)
	}
	if m.child.working {
		t.Fatal("a completed run shows as working")
	}
	// A late update that the snapshot already contains changes nothing.
	m = sendMsg(m, app.SubagentRunEvent{RunID: "r", Seq: 4, Status: "running", Event: app.StreamChunkEvent{Content: "again"}})
	if got := paneItems(m.conversationPane); !slices.Equal(got, want) || m.child.working {
		t.Fatalf("old update was applied: %q", got)
	}
}

// TestChildViewKeys: ←/→ step through the siblings of the parent session,
// ctrl+k stops the shown run, and the up key scrolls without leaving.
func TestChildViewKeys(t *testing.T) {
	m, c := childModel(t,
		kit.SubagentRun{ID: "other", ParentSessionID: "q", Status: "completed"},
		kit.SubagentRun{ID: "a", ParentSessionID: "p", Status: "running", Prompt: strings.Repeat("line\n", 60)},
		kit.SubagentRun{ID: "b", ParentSessionID: "p", Status: "running"},
	)
	m = sendMsg(m, tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl | tea.ModAlt})
	m = sendMsg(m, SubagentRunSelectedMsg{Index: 2})
	if m.paneID != "b" {
		t.Fatalf("shown pane = %q, want b", m.paneID)
	}
	m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.paneID != "a" {
		t.Fatalf("right from the last sibling = %q, want a (wraps, skips other session)", m.paneID)
	}
	m = sendMsg(m, tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if c.stopped != "a" {
		t.Fatalf("ctrl+k stopped %q, want a", c.stopped)
	}
	m.scrollList.GotoBottom()
	before := m.scrollList.offsetIdx*1000 + m.scrollList.offsetLine
	m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.childView == nil {
		t.Fatal("up closed the child view")
	}
	if after := m.scrollList.offsetIdx*1000 + m.scrollList.offsetLine; after >= before {
		t.Fatal("up did not scroll")
	}
}

// TestSubagentsCommandOpensChildView: /subagents shows the latest run and
// is not dispatched to an extension command of the same name.
func TestSubagentsCommandOpensChildView(t *testing.T) {
	m, _ := childModel(t, kit.SubagentRun{ID: "old", ParentSessionID: "p"}, kit.SubagentRun{ID: "new", ParentSessionID: "p"})
	m.extensionCommands = []commands.ExtensionCommand{{
		Name:    "subagents",
		Execute: func(string) (string, error) { panic("built-in command must not call the extension") },
	}}
	command := commands.GetCommandByName("/subagents")
	if command == nil {
		t.Fatal("missing /subagents command")
	}
	m.handleSlashCommand(command, "")
	m.applyPaneRequest()
	if m.childView == nil || m.paneID != "new" {
		t.Fatalf("shown pane = %q, want the latest run", m.paneID)
	}
}

// TestHiddenPaneDefersScroll: a hidden pane does not measure items to follow
// its output; it scrolls to the bottom after it is shown.
func TestHiddenPaneDefersScroll(t *testing.T) {
	m, _ := childModel(t, kit.SubagentRun{ID: "r", ParentSessionID: "p"})
	m, seq := sendRun(m, "r", 0, "running")
	m, _ = sendRun(m, "r", seq, "running", app.ToolResultEvent{ToolName: "bash", ToolArgs: `{"command":"ls"}`, Result: strings.Repeat("x\n", 80)})
	child := m.childPanes["r"]
	if !child.scrollList.offscreen || !child.scrollList.bottomPending {
		t.Fatal("hidden pane did not defer its scroll")
	}
	m.paneRequest = &paneRequest{runID: "r", parentSessionID: "p"}
	m.applyPaneRequest()
	if !m.pendingGotoBottom || child.scrollList.offscreen {
		t.Fatal("shown pane did not request its deferred scroll")
	}
	_ = m.View()
	if !child.scrollList.AtBottom() {
		t.Fatal("shown pane is not at the bottom")
	}
	if !m.parentPane.scrollList.offscreen {
		t.Fatal("parent pane is not offscreen while the child is shown")
	}
}

// TestParentDialogShowsParentPane: a parent dialog needs the keyboard, so it
// closes the child view.
func TestParentDialogShowsParentPane(t *testing.T) {
	m, _ := childModel(t, kit.SubagentRun{ID: "r", ParentSessionID: "p"})
	m, _ = sendRun(m, "r", 0, "running")
	m.paneRequest = &paneRequest{runID: "r", parentSessionID: "p"}
	m.applyPaneRequest()
	m = sendMsg(m, app.PasswordPromptEvent{Prompt: "sudo password", ResponseCh: make(chan app.PasswordPromptResponse, 1)})
	if m.childView != nil || m.conversationPane != m.parentPane {
		t.Fatalf("child view stayed open over a parent dialog (state %v)", m.state)
	}
}
