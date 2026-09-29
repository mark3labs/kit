package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/ui/core"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// pressKillKey sends a key press and then runs the command it returns (one
// level deep), as Bubble Tea would, so selection messages are delivered.
func pressKillKey(m *AppModel, key tea.KeyPressMsg) *AppModel {
	updated, cmd := m.Update(key)
	m = updated.(*AppModel)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, c := range batch {
					if c != nil {
						if inner := c(); inner != nil {
							m = sendMsg(m, inner)
						}
					}
				}
			} else {
				m = sendMsg(m, msg)
			}
		}
	}
	return m
}

func lastKillMessage(m *AppModel) string {
	if len(m.messages) == 0 {
		return ""
	}
	return stripAnsi(m.messages[len(m.messages)-1].Render(200))
}

func TestKillSubagent_NoneRunning(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m = sendMsg(m, core.SubmitMsg{Text: "/kill-subagent"})

	if m.killSubagentSelector != nil {
		t.Fatal("selector opened with no running subagents")
	}
	if got := lastKillMessage(m); !strings.Contains(got, "No subagents are running") {
		t.Fatalf("last message = %q, want the no-subagents notice", got)
	}
}

func TestKillSubagent_SelectKillsAndKeepsWorkingState(t *testing.T) {
	ctrl := &stubAppController{subagents: []kit.RunningSubagent{
		{ID: "call-1", Prompt: "find the bug", Agent: "explore", StartedAt: time.Now()},
	}}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, core.SubmitMsg{Text: "/kill-subagent"})
	if m.killSubagentSelector == nil {
		t.Fatal("selector did not open")
	}
	if m.state != stateWorking {
		t.Fatalf("state = %v, want stateWorking while the selector is open", m.state)
	}
	if !strings.Contains(stripAnsi(m.View().Content), "Kill Subagent") {
		t.Fatal("selector is not rendered")
	}

	m = pressKillKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.killSubagentSelector != nil {
		t.Fatal("selector still open after selection")
	}
	if len(ctrl.killed) != 1 || ctrl.killed[0] != "call-1" {
		t.Fatalf("killed = %v, want [call-1]", ctrl.killed)
	}
	if m.state != stateWorking {
		t.Fatalf("state = %v, want stateWorking after the kill", m.state)
	}
	if got := lastKillMessage(m); !strings.Contains(got, `Killed subagent "explore"`) {
		t.Fatalf("last message = %q, want the kill confirmation", got)
	}
}

func TestKillSubagent_EscClosesWithoutCancellingTurn(t *testing.T) {
	ctrl := &stubAppController{subagents: []kit.RunningSubagent{
		{ID: "call-1", Prompt: "task", StartedAt: time.Now()},
	}}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, core.SubmitMsg{Text: "/kill-subagent"})
	m = pressKillKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.killSubagentSelector != nil {
		t.Fatal("selector still open after esc")
	}
	if len(ctrl.killed) != 0 {
		t.Fatalf("killed = %v, want none", ctrl.killed)
	}
	if ctrl.cancelCalled != 0 || m.canceling {
		t.Fatal("esc in the selector reached the turn-cancel handler")
	}
}

func TestKillSubagent_AlreadyCompleted(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.killSubagent("gone", "subagent")
	if got := lastKillMessage(m); !strings.Contains(got, "is not running") {
		t.Fatalf("last message = %q, want the not-running notice", got)
	}
}

func TestFormatSubagentElapsed(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                             "0s",
		42 * time.Second:              "42s",
		3*time.Minute + 5*time.Second: "3m05s",
		-time.Second:                  "0s",
	} {
		if got := formatSubagentElapsed(d); got != want {
			t.Errorf("formatSubagentElapsed(%v) = %q, want %q", d, got, want)
		}
	}
}
