package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/app"
)

// TestPromptFitsTerminal verifies that the layout is measured again when a
// prompt opens and when it closes. The prompt replaces the input area and is
// taller than it. If the transcript keeps the height that it had for the
// input area, the view is taller than the terminal. Then the terminal does
// not show the options of the prompt and the status bar.
func TestPromptFitsTerminal(t *testing.T) {
	newModel := func(t *testing.T) *AppModel {
		t.Helper()
		m, _, _ := newTestAppModel(&stubAppController{})
		m.input = NewInputComponent(80, nil)
		m = sendMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})
		checkViewHeight(t, m, "before the prompt")
		return m
	}

	t.Run("select prompt of an extension", func(t *testing.T) {
		m := newModel(t)
		ch := make(chan app.PromptResponse, 1)
		m = sendMsg(m, app.PromptRequestEvent{
			PromptType: "select",
			Message:    "Choose a target",
			Options:    []string{"local", "staging", "production"},
			ResponseCh: ch,
		})
		if view := checkViewHeight(t, m, "with the prompt"); !strings.Contains(view, "production") {
			t.Fatalf("the view does not show the last option\n%s", view)
		}

		m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
		select {
		case resp := <-ch:
			if !resp.Cancelled {
				t.Fatalf("Esc gave %+v, want a cancelled prompt", resp)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no response after Esc")
		}
		checkViewHeight(t, m, "after the prompt")
	})

	t.Run("password prompt", func(t *testing.T) {
		m := newModel(t)
		ch := make(chan app.PasswordPromptResponse, 1)
		m = sendMsg(m, app.PasswordPromptEvent{Prompt: "Password for sudo", ResponseCh: ch})
		if view := checkViewHeight(t, m, "with the prompt"); !strings.Contains(view, "Password for sudo") {
			t.Fatalf("the view does not show the prompt\n%s", view)
		}

		m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
		select {
		case resp := <-ch:
			if !resp.Cancelled {
				t.Fatalf("Esc gave %+v, want a cancelled prompt", resp)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no response after Esc")
		}
		checkViewHeight(t, m, "after the prompt")
	})
}

// checkViewHeight renders a frame and stops the test unless the frame has
// the height of the terminal and ends with the status bar. The status bar
// shows the model name. It returns the frame without ANSI codes.
func checkViewHeight(t *testing.T, m *AppModel, when string) string {
	t.Helper()
	view := m.View().Content
	if got := lipgloss.Height(view); got != m.height {
		t.Fatalf("view height %s = %d, want %d\n%s", when, got, m.height, stripAnsi(view))
	}
	plain := stripAnsi(view)
	if last := plain[strings.LastIndex(plain, "\n")+1:]; !strings.Contains(last, m.modelName) {
		t.Fatalf("last line %s = %q, want the status bar with %q\n%s", when, last, m.modelName, plain)
	}
	return plain
}
