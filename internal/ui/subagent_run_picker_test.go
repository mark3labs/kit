package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	kit "github.com/mark3labs/kit/pkg/kit"
)

func TestSubagentRunPickerSelectionUsesSelectedRunParent(t *testing.T) {
	ctrl := &inspectorTestController{runs: []kit.SubagentRun{
		{ID: "other", ParentSessionID: "other-parent", Agent: "builder", Status: "completed"},
		{ID: "picked", ParentSessionID: "parent", Agent: "explore", Status: "running"},
	}}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking
	m.openSubagentRunPicker()
	if !m.subagentRunPickerOpen() || !strings.Contains(stripAnsi(m.subagentRunPicker.RenderOverlay()), "Subagent Runs") {
		t.Fatal("picker did not open")
	}
	_, cmd := m.subagentRunPicker.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Fatal("down returned a command")
	}
	_, cmd = m.subagentRunPicker.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter returned no selection")
	}
	msg := cmd()
	// Registry order can change while the picker is open. Selection uses the
	// run ID captured by the picker, not the current registry index.
	ctrl.runs = []kit.SubagentRun{ctrl.runs[1], ctrl.runs[0]}
	m = sendMsg(m, msg)
	if m.subagentView == nil || m.subagentView.parentSessionID != "parent" || m.subagentView.viewedRunID != "picked" {
		t.Fatalf("inspector = %#v, want selected run under parent", m.subagentView)
	}
	if m.state != stateWorking {
		t.Fatal("picker changed parent state")
	}
}

func TestSubagentRunPickerEscape(t *testing.T) {
	m, _, _ := newTestAppModel(&inspectorTestController{runs: []kit.SubagentRun{{ID: "r", ParentSessionID: "p"}}})
	m.openSubagentRunPicker()
	_, cmd := m.subagentRunPicker.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = sendMsg(m, cmd())
	if m.subagentRunPicker != nil || m.subagentView != nil {
		t.Fatal("escape did not cancel picker")
	}
}

func TestSubagentRunPickerShortcut(t *testing.T) {
	m, _, _ := newTestAppModel(&inspectorTestController{runs: []kit.SubagentRun{{ID: "r", ParentSessionID: "p"}}})
	m.state = stateWorking
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl | tea.ModAlt})
	if !updated.(*AppModel).subagentRunPickerOpen() {
		t.Fatal("ctrl+alt+a did not open picker")
	}
}

func TestSubagentStatusHintFitsOnlyWhenSpaceAllows(t *testing.T) {
	m, _, _ := newTestAppModel(&inspectorTestController{runs: []kit.SubagentRun{{Agent: "explore", Status: "running"}}})
	m.subagentStatusCheckedAt = time.Time{}
	m.width = 100
	if got := m.renderSubagentStatusBar(); !strings.Contains(got, "ctrl+alt+a runs") {
		t.Fatalf("wide status row lacks hint: %q", got)
	}
	m.width = 15
	got := m.renderSubagentStatusBar()
	if strings.Contains(got, "ctrl+alt+a") || lipgloss.Width(got) > m.width {
		t.Fatalf("narrow row = %q", got)
	}
}
