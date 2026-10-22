package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/extensions"
)

type extensionTrigger struct{}

type extensionTestModel struct {
	action func()
	seen   chan tea.Msg
}

func (m extensionTestModel) Init() tea.Cmd  { return nil }
func (m extensionTestModel) View() tea.View { return tea.NewView("") }
func (m extensionTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case extensionTrigger:
		m.action()
	case ExtensionPrintEvent, EditorTextSetEvent, ModelChangedEvent, NewSessionRequestEvent:
		m.seen <- msg
	}
	return m, nil
}

func extensionProgram(a *App, m tea.Model) *tea.Program {
	return tea.NewProgram(m, tea.WithContext(a.Context()), tea.WithInput(nil),
		tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
}

func startExtensionProgram(t *testing.T, a *App, p *tea.Program) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() { defer close(finished); _, err := p.Run(); done <- err }()
	t.Cleanup(func() {
		p.Kill()
		a.Close()
		awaitExtension(t, finished)
	})
	return done
}

func awaitExtension[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for extension program")
		var zero T
		return zero
	}
}

func TestExtensionNotificationsFromUpdate(t *testing.T) {
	block := extensions.PrintBlockOpts{Text: "body", Subtitle: "title", BorderColor: "#123456", Markdown: true}
	tests := []struct {
		name string
		call func(*App)
		want tea.Msg
	}{
		{"plain", func(a *App) { a.PrintFromExtension("", "plain") }, ExtensionPrintEvent{Text: "plain"}},
		{"info", func(a *App) { a.PrintFromExtension("info", "info") }, ExtensionPrintEvent{Text: "info", Level: "info"}},
		{"error", func(a *App) { a.PrintFromExtension("error", "error") }, ExtensionPrintEvent{Text: "error", Level: "error"}},
		{"block", func(a *App) { a.PrintBlockFromExtension(block) }, ExtensionPrintEvent{Text: block.Text, Level: "block", Subtitle: block.Subtitle, BorderColor: block.BorderColor, Markdown: true}},
		{"editor", func(a *App) { a.SetEditorTextFromExtension("replacement") }, EditorTextSetEvent{Text: "replacement"}},
		{"model", func(a *App) { a.NotifyModelChanged("provider", "model") }, ModelChangedEvent{ProviderName: "provider", ModelName: "model"}},
		{"quit", func(a *App) { a.QuitFromExtension(); a.QuitFromExtension() }, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New(Options{}, nil)
			returned := make(chan struct{})
			seen := make(chan tea.Msg, 1)
			p := extensionProgram(a, extensionTestModel{action: func() {
				tt.call(a)
				close(returned)
			}, seen: seen})
			a.SetProgram(p)
			done := startExtensionProgram(t, a, p)
			go p.Send(extensionTrigger{})
			awaitExtension(t, returned)
			if tt.want != nil {
				if got := awaitExtension(t, seen); !reflect.DeepEqual(got, tt.want) {
					t.Fatalf("message = %#v, want %#v", got, tt.want)
				}
				a.QuitFromExtension()
			}
			if err := awaitExtension(t, done); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExtensionNotificationOrder(t *testing.T) {
	a := New(Options{}, nil)
	const count = 200
	seen := make(chan tea.Msg, count*2)
	p := extensionProgram(a, extensionTestModel{action: func() {
		for i := range count {
			a.PrintFromExtension("", fmt.Sprint(i))
			a.SetEditorTextFromExtension(fmt.Sprint(i))
		}
		a.QuitFromExtension()
	}, seen: seen})
	a.SetProgram(p)
	done := startExtensionProgram(t, a, p)
	go p.Send(extensionTrigger{})
	if err := awaitExtension(t, done); err != nil {
		t.Fatal(err)
	}
	for i := range count {
		if got := awaitExtension(t, seen); got != (ExtensionPrintEvent{Text: fmt.Sprint(i)}) {
			t.Fatalf("print %d = %#v", i, got)
		}
		if got := awaitExtension(t, seen); got != (EditorTextSetEvent{Text: fmt.Sprint(i)}) {
			t.Fatalf("editor %d = %#v", i, got)
		}
	}
}

func TestExtensionNotificationsBeforeRunAndAfterExit(t *testing.T) {
	a := New(Options{}, nil)
	seen := make(chan tea.Msg, 1)
	p := extensionProgram(a, extensionTestModel{seen: seen})
	a.SetProgram(p)
	returned := make(chan struct{})
	go func() {
		a.SetEditorTextFromExtension("before run")
		a.QuitFromExtension()
		close(returned)
	}()
	awaitExtension(t, returned)
	done := startExtensionProgram(t, a, p)
	if err := awaitExtension(t, done); err != nil {
		t.Fatal(err)
	}
	if got := awaitExtension(t, seen); got != (EditorTextSetEvent{Text: "before run"}) {
		t.Fatalf("message = %#v", got)
	}
	a.SetEditorTextFromExtension("after exit")
	a.QuitFromExtension()
	a.Close()
	a.Close()
}

func TestExtensionNewSessionShutdown(t *testing.T) {
	a := New(Options{}, nil)
	seen := make(chan tea.Msg, 1)
	p := extensionProgram(a, extensionTestModel{seen: seen})
	a.SetProgram(p)
	startExtensionProgram(t, a, p)
	done := make(chan error, 1)
	go func() { done <- a.RequestNewSessionFromExtension("") }()
	// Receive the request without replying. Cancellation must release the wait.
	awaitExtension(t, seen)
	a.Close()
	if err := awaitExtension(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown error = %v", err)
	}
}

func TestExtensionDispatcherCloseWithoutRun(t *testing.T) {
	a := New(Options{}, nil)
	p := extensionProgram(a, extensionTestModel{})
	a.SetProgram(p)
	t.Cleanup(p.Kill)
	t.Cleanup(a.Close)
	a.SetEditorTextFromExtension("unreceived")
	var producers sync.WaitGroup
	for range 8 {
		producers.Go(func() {
			for range 100 {
				a.SetEditorTextFromExtension("racing")
				a.NotifyModelChanged("p", "m")
			}
		})
	}
	done := make(chan struct{})
	go func() { a.Close(); producers.Wait(); a.Close(); close(done) }()
	awaitExtension(t, done)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.extensionMessages != nil {
		t.Fatal("closed dispatcher retains messages")
	}
}
