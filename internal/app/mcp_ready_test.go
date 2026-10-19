package app

import (
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// mcpReadyModel is a minimal program model that quits once it receives
// MCPToolsReadyEvent.
type mcpReadyModel struct {
	seen chan struct{}
}

func (m mcpReadyModel) Init() tea.Cmd { return nil }

func (m mcpReadyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(MCPToolsReadyEvent); ok {
		close(m.seen)
		return m, tea.Quit
	}
	return m, nil
}

func (m mcpReadyModel) View() tea.View { return tea.NewView("") }

// TestNotifyMCPToolsReadyBeforeSetProgram covers MCP servers that finish
// loading before the TUI program is registered. The event used to be dropped
// in that case, which left the tool list stale and kept OnMCPToolsReady from
// firing.
func TestNotifyMCPToolsReadyBeforeSetProgram(t *testing.T) {
	a := New(Options{}, nil)
	defer a.Close()

	a.NotifyMCPToolsReady() // no program registered yet

	model := mcpReadyModel{seen: make(chan struct{})}
	p := tea.NewProgram(model,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithoutRenderer(),
		tea.WithoutSignalHandler(),
	)
	a.SetProgram(p)

	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()

	select {
	case <-model.seen:
	case <-time.After(5 * time.Second):
		p.Kill()
		<-done
		t.Fatal("MCPToolsReadyEvent was not delivered after SetProgram")
	}
	if err := <-done; err != nil {
		t.Fatalf("program: %v", err)
	}
}
