package ui

import (
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/ui/core"
)

func TestKillCommandRequiresConfirmationAndCallsCallback(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	calls := 0
	m.killSession = func() error { calls++; return nil }
	m = sendMsg(m, core.SubmitMsg{Text: "/kill"})
	if calls != 0 || !m.killConfirm || !strings.Contains(lastKillMessage(m), "Enter /kill again") {
		t.Fatal("first /kill did not request confirmation")
	}
	updated, cmd := m.Update(core.SubmitMsg{Text: "/kill"})
	m = updated.(*AppModel)
	if calls != 1 || !m.quitting || cmd == nil {
		t.Fatalf("confirmed /kill calls=%d quitting=%v cmd nil=%v", calls, m.quitting, cmd == nil)
	}
}

func TestKillCommandWithoutDaemonCallbackQuits(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m = sendMsg(m, core.SubmitMsg{Text: "/kill"})
	if m.quitting {
		t.Fatal("must confirm before quitting")
	}
	updated, cmd := m.Update(core.SubmitMsg{Text: "/kill"})
	m = updated.(*AppModel)
	if !m.quitting || cmd == nil {
		t.Fatal("non-daemon /kill must quit Kit")
	}
}
