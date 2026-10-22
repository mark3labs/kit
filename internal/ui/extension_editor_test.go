package ui

import (
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/app"
)

type editorObserver struct {
	model   *AppModel
	applied chan string
}

func (m editorObserver) Init() tea.Cmd  { return nil }
func (m editorObserver) View() tea.View { return tea.NewView("") }
func (m editorObserver) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := m.model.Update(msg)
	if _, ok := msg.(app.EditorTextSetEvent); ok {
		m.applied <- m.model.input.(*InputComponent).textarea.Value()
	}
	return m, cmd
}

func TestExtensionSetEditorTextFromHandleKey(t *testing.T) {
	a := app.New(app.Options{}, nil)
	returned := make(chan string, 2)
	applied := make(chan string, 1)
	model := NewAppModel(a, AppModelOptions{GetEditorInterceptor: func() *EditorInterceptor {
		return &EditorInterceptor{HandleKey: func(key, currentText string) EditorKeyAction {
			if key == "a" {
				a.SetEditorTextFromExtension("replacement")
			}
			returned <- key + ":" + currentText
			return EditorKeyAction{Type: EditorKeyConsumed}
		}}
	}})
	p := tea.NewProgram(editorObserver{model: model, applied: applied},
		tea.WithContext(a.Context()), tea.WithInput(nil), tea.WithOutput(io.Discard),
		tea.WithoutRenderer(), tea.WithoutSignalHandler())
	a.SetProgram(p)
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() { defer close(finished); _, err := p.Run(); done <- err }()
	t.Cleanup(func() {
		p.Kill()
		a.Close()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("program cleanup timed out")
		}
	})
	await := func(ch <-chan string) string {
		t.Helper()
		select {
		case value := <-ch:
			return value
		case <-time.After(5 * time.Second):
			t.Fatal("editor event loop stalled")
			return ""
		}
	}
	go p.Send(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if got := await(returned); got != "a:" {
		t.Fatalf("callback snapshot = %q", got)
	}
	if got := await(applied); got != "replacement" {
		t.Fatalf("applied editor = %q", got)
	}
	go p.Send(tea.KeyPressMsg{Code: 'b', Text: "b"})
	if got := await(returned); got != "b:replacement" {
		t.Fatalf("subsequent callback = %q", got)
	}
	a.QuitFromExtension()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("program did not quit")
	}
}
