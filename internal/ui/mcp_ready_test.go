package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/app"
)

// An extension that prints its own summary of the MCP servers hides the
// message for each server that loaded. A failed load is still reported.
func TestHideMCPStartupMessagesKeepsFailures(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)

	m = sendMsg(m, app.MCPServerLoadedEvent{ServerName: "systemd", ToolCount: 2})
	if len(m.messages) != 1 {
		t.Fatalf("got %d messages without the flag; want 1", len(m.messages))
	}

	m.getUIVisibility = func() *UIVisibility {
		return &UIVisibility{HideMCPStartupMessages: true}
	}
	m = sendMsg(m, app.MCPServerLoadedEvent{ServerName: "zypper", ToolCount: 3})
	if len(m.messages) != 1 {
		t.Fatalf("got %d messages after a hidden load; want 1", len(m.messages))
	}

	m = sendMsg(m, app.MCPServerLoadedEvent{ServerName: "snapper", Error: errors.New("not found")})
	if len(m.messages) != 2 {
		t.Fatalf("got %d messages after a failed load; want 2", len(m.messages))
	}
	if got := xansi.Strip(m.messages[1].Render(m.width)); !strings.Contains(got, "snapper") {
		t.Errorf("failure message %q does not name the server", got)
	}
}

// MCPToolsReadyEvent reaches the extension callback, off the Update goroutine.
func TestMCPToolsReadyEmitsExtensionEvent(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.extEvents = newExtEventDispatcher()
	called := make(chan struct{})
	m.emitMCPToolsReady = func() { close(called) }

	sendMsg(m, app.MCPToolsReadyEvent{})

	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("OnMCPToolsReady was not emitted")
	}
}

// The end of MCP loading arrives only once, so an overlay that is open at
// that moment must not swallow it.
func TestMCPToolsReadySurvivesModalState(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.extEvents = newExtEventDispatcher()
	called := make(chan struct{})
	m.emitMCPToolsReady = func() { close(called) }
	m.overlay = newOverlayDialog("T", "b", false, "", "", 0, 0, "center", nil, 80, 24)
	m.state = stateOverlay

	updated, _ := m.Update(app.MCPToolsReadyEvent{})
	m = updated.(*AppModel)

	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("OnMCPToolsReady was swallowed by the open overlay")
	}
	if m.state != stateOverlay {
		t.Errorf("state = %v; want the overlay to stay open", m.state)
	}
}

// PrintBlock with Markdown renders the text as markdown. Without the flag
// the markdown syntax stays as it is.
func TestExtensionBlockMarkdown(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)

	m = sendMsg(m, app.ExtensionPrintEvent{Level: "block", Text: "**loaded**", Markdown: true})
	m = sendMsg(m, app.ExtensionPrintEvent{Level: "block", Text: "**loaded**"})
	if len(m.messages) != 2 {
		t.Fatalf("got %d messages; want 2", len(m.messages))
	}

	md := xansi.Strip(m.messages[0].Render(m.width))
	if strings.Contains(md, "**") || !strings.Contains(md, "loaded") {
		t.Errorf("markdown block = %q; want the text without the markers", md)
	}
	plain := xansi.Strip(m.messages[1].Render(m.width))
	if !strings.Contains(plain, "**loaded**") {
		t.Errorf("plain block = %q; want the text as it is", plain)
	}
}
