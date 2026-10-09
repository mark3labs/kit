package ui

import (
	"strings"
	"testing"

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

func TestSubagentsCommandOpensNativeInspector(t *testing.T) {
	c := &inspectorTestController{runs: []kit.SubagentRun{{ID: "child", ParentSessionID: "parent"}}}
	m := &AppModel{
		appCtrl: c, state: stateWorking, width: 80, height: 24,
		extensionCommands: []commands.ExtensionCommand{{
			Name: "subagents",
			Execute: func(string) (string, error) {
				panic("built-in command must not call the extension")
			},
		}},
	}
	command := commands.GetCommandByName("/subagents")
	if command == nil {
		t.Fatal("missing /subagents command")
	}
	if m.handleSlashCommand(command, "") == nil || m.subagentView == nil {
		t.Fatal("/subagents did not open the native inspector")
	}
	for _, name := range []string{"/subagent-sessions", "/agents"} {
		if commands.GetCommandByName(name) != nil {
			t.Fatalf("removed command %q is still registered", name)
		}
	}
}

func TestSubagentInspectorNavigation(t *testing.T) {
	c := &inspectorTestController{runs: []kit.SubagentRun{{ID: "a", ParentSessionID: "parent"}, {ID: "b", ParentSessionID: "parent"}}}
	m := &AppModel{appCtrl: c, state: stateWorking, width: 80, height: 24}
	if m.openSubagentInspector() == nil {
		t.Fatal("missing refresh command")
	}
	m.handleSubagentInspectorKey("left")
	m.handleSubagentInspectorKey("ctrl+k")
	if c.stopped != "a" {
		t.Fatalf("stopped %q", c.stopped)
	}
	c.runs = []kit.SubagentRun{{ID: "b", ParentSessionID: "parent"}}
	m.refreshSubagentView()
	if m.subagentView.index != 0 {
		t.Fatal("invalid index after eviction")
	}
	m.handleSubagentInspectorKey("esc")
	if m.subagentView != nil || m.state != stateWorking {
		t.Fatal("parent state changed")
	}
}

func TestSubagentInspectorStaleTickDoesNotRestartAfterReopen(t *testing.T) {
	c := &inspectorTestController{runs: []kit.SubagentRun{{ID: "child", ParentSessionID: "parent"}}}
	m := &AppModel{appCtrl: c, state: stateWorking, width: 80, height: 24}
	m.openSubagentInspector()
	stale := subagentInspectorTick{generation: m.subagentInspectorGeneration}
	m.handleSubagentInspectorKey("esc")
	m.openSubagentInspector()
	generation := m.subagentInspectorGeneration
	_, cmd := m.Update(stale)
	if cmd != nil || m.subagentInspectorGeneration != generation {
		t.Fatal("stale tick restarted the inspector refresh chain")
	}
}

func TestSubagentInspectorKeepsParentFilterAfterEviction(t *testing.T) {
	c := &inspectorTestController{runs: []kit.SubagentRun{{ID: "other", ParentSessionID: "other"}, {ID: "a", ParentSessionID: "parent"}}}
	m := &AppModel{appCtrl: c, state: stateWorking, width: 80, height: 24}
	m.openSubagentInspector()
	c.runs = []kit.SubagentRun{{ID: "other", ParentSessionID: "other"}}
	m.refreshSubagentView()
	if m.subagentView.childList != nil || m.subagentView.viewedRunID != "" || len(m.subagentView.runs) != 0 {
		t.Fatal("evicted inspector retained stale child content")
	}
	if got := m.renderSubagentInspector(); !strings.Contains(got, "No retained subagent runs") {
		t.Fatalf("empty state not shown: %q", got)
	}
}

func TestSubagentInspectorUpScrollsAndDoesNotClose(t *testing.T) {
	c := &inspectorTestController{runs: []kit.SubagentRun{{ID: "child", ParentSessionID: "parent", Prompt: strings.Repeat("line\\n", 30)}}}
	m := &AppModel{appCtrl: c, state: stateWorking, width: 80, height: 8}
	m.openSubagentInspector()
	m.subagentView.childList.GotoBottom()
	before := m.subagentView.childList.offsetLine
	m.handleSubagentInspectorKey("up")
	if m.subagentView == nil {
		t.Fatal("up closed inspector")
	}
	if m.subagentView.childList.offsetLine >= before {
		t.Fatal("up did not scroll up")
	}
}

func TestSubagentInspectorShowsActiveToolAndUsesOwnedRenderer(t *testing.T) {
	c := &inspectorTestController{runs: []kit.SubagentRun{{ID: "live", ParentSessionID: "parent", Status: "running", Events: []kit.Event{kit.ToolCallEvent{ToolCallID: "tool-1", ToolName: "bash", ToolArgs: `{"command":"sleep 10"}`}, kit.MessageUpdateEvent{Chunk: "in progress"}}}}}
	parentRenderer := newMessageRenderer(80, false)
	m := &AppModel{appCtrl: c, state: stateWorking, width: 80, height: 24, renderer: parentRenderer}
	m.openSubagentInspector()
	items := m.subagentView.childList
	if items.Len() != 3 {
		t.Fatalf("expected header, live message, active tool; got %d", items.Len())
	}
	if _, ok := items.ItemAt(1).(*StreamingMessageItem); !ok {
		t.Fatalf("live child message has type %T", items.ItemAt(1))
	}
	tool, ok := items.ItemAt(2).(ToolInspectable)
	if !ok {
		t.Fatal("active tool does not use normal tool item")
	}
	if info, _ := tool.ToolCall(); !strings.Contains(info.Args, "sleep 10") {
		t.Fatalf("active args missing: %q", info.Args)
	}
	if m.subagentView.renderer == parentRenderer {
		t.Fatal("child shares parent renderer")
	}
	if m.renderer != parentRenderer {
		t.Fatal("parent renderer changed")
	}
}

func TestSubagentInspectorUsesNormalItemsAndLeavesParentUntouched(t *testing.T) {
	c := &inspectorTestController{runs: []kit.SubagentRun{{ID: "child", ParentSessionID: "parent", Agent: "worker", DroppedEvents: 2, Events: []kit.Event{kit.MessageUpdateEvent{Chunk: "Hello "}, kit.MessageUpdateEvent{Chunk: "world"}, kit.ToolResultEvent{ToolName: "bash", ToolArgs: `{"command":"echo"}`, Result: "done"}}}}}
	m := &AppModel{appCtrl: c, state: stateWorking, width: 80, height: 24, renderer: newMessageRenderer(80, false)}
	parentState := m.state
	m.openSubagentInspector()
	items := m.subagentView.childList
	if items.Len() != 3 {
		t.Fatalf("expected header, assistant, tool items, got %d", items.Len())
	}
	if got := items.ItemAt(1).(*TextMessageItem); got.Role() != "assistant" || got.RawContent() != "Hello world" {
		t.Fatalf("assistant item: %#v", got)
	}
	if _, ok := items.ItemAt(2).(ToolInspectable); !ok {
		t.Fatal("tool result lacks normal tool metadata")
	}
	if !strings.Contains(items.ItemAt(0).(*TextMessageItem).RawContent(), "2 dropped") {
		t.Fatal("missing truncation warning")
	}
	m.handleSubagentInspectorKey("esc")
	if m.state != parentState || m.messages != nil {
		t.Fatal("child view changed parent conversation")
	}
}
