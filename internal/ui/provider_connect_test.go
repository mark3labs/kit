package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// pressKeys sends each rune as a printable key press to the component.
func pressKeys(t *testing.T, c *ProviderConnectComponent, text string) {
	t.Helper()
	for _, r := range text {
		updated, _ := c.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		c = updated.(*ProviderConnectComponent)
	}
}

func TestListConnectableProvidersOrderAndShape(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	entries := ListConnectableProviders()
	if len(entries) < 10 {
		t.Fatalf("expected many providers, got %d", len(entries))
	}
	if entries[0].ID != "anthropic" || entries[1].ID != "openai" {
		t.Errorf("priority order broken: %s, %s", entries[0].ID, entries[1].ID)
	}
	seenCopilot := false
	for _, e := range entries {
		if e.ID == "github-copilot" {
			seenCopilot = true
			if !e.OAuthOnly {
				t.Error("github-copilot must be OAuthOnly")
			}
		}
		if e.ID == "ollama" || e.ID == "custom" {
			t.Errorf("%s has no API key and must be skipped", e.ID)
		}
	}
	if !seenCopilot {
		t.Error("github-copilot missing from list")
	}
}

func TestProviderConnectFlow(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c := NewProviderConnect("", 100, 40)

	if c.stage != connectStageList {
		t.Fatalf("initial stage = %v, want list", c.stage)
	}
	if !strings.Contains(c.RenderOverlay(), "Connect a provider") {
		t.Error("list overlay lacks title")
	}

	// Filter to Groq and select it.
	pressKeys(t, c, "groq")
	items := c.popup.Items()
	if len(items) == 0 || items[0].Meta.(ProviderConnectEntry).ID != "groq" {
		t.Fatalf("filter 'groq' did not rank groq first: %+v", items)
	}
	updated, cmd := c.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	c = updated.(*ProviderConnectComponent)
	_ = runCmd(cmd)
	if c.stage != connectStageKey {
		t.Fatalf("stage after select = %v, want key", c.stage)
	}
	if !strings.Contains(c.RenderOverlay(), "Groq API key") {
		t.Error("key overlay lacks provider title")
	}

	// Empty submit is rejected inline.
	updated, cmd = c.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	c = updated.(*ProviderConnectComponent)
	if msg := runCmd(cmd); msg != nil {
		t.Errorf("empty key produced message %T", msg)
	}
	if c.errMsg == "" {
		t.Error("expected inline error for empty key")
	}

	// Typed key is masked in the overlay.
	pressKeys(t, c, "gsk_secret")
	view := c.RenderOverlay()
	if strings.Contains(view, "gsk_secret") {
		t.Error("key is visible in the overlay")
	}
	if !strings.Contains(view, "••••") {
		t.Error("key is not masked with bullets")
	}

	// Enter submits the key with the provider.
	updated, cmd = c.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	c = updated.(*ProviderConnectComponent)
	msg, ok := runCmd(cmd).(ProviderKeySubmittedMsg)
	if !ok {
		t.Fatalf("expected ProviderKeySubmittedMsg, got %T", msg)
	}
	if msg.ProviderID != "groq" || msg.Key != "gsk_secret" || msg.ProviderName != "Groq" {
		t.Errorf("submitted = %+v", msg)
	}
	if c.IsActive() {
		t.Error("component still active after submit")
	}

	// SetError reopens the key step.
	c.SetError("boom")
	if !c.IsActive() || c.stage != connectStageKey || !strings.Contains(c.RenderOverlay(), "boom") {
		t.Error("SetError did not reopen the key input with the message")
	}
}

func TestProviderConnectEscAndOAuth(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Preselect opens the key input directly; Esc goes back to the list;
	// Esc again cancels.
	c := NewProviderConnect("groq", 100, 40)
	if c.stage != connectStageKey || c.entry.ID != "groq" {
		t.Fatalf("preselect: stage=%v entry=%s", c.stage, c.entry.ID)
	}
	updated, _ := c.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	c = updated.(*ProviderConnectComponent)
	if c.stage != connectStageList {
		t.Fatalf("Esc from key input: stage=%v, want list", c.stage)
	}
	_, cmd := c.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if _, ok := runCmd(cmd).(ProviderConnectCancelledMsg); !ok {
		t.Error("Esc from list did not cancel")
	}

	// Preselecting an OAuth-only provider stays on the list.
	c = NewProviderConnect("copilot", 100, 40)
	if c.stage != connectStageList {
		t.Fatal("copilot preselect must not open a key input")
	}
	// Selecting it emits the OAuth hint message.
	items := c.popup.Items()
	idx := -1
	for i, it := range items {
		if it.Meta.(ProviderConnectEntry).ID == "github-copilot" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("copilot not in filtered list")
	}
	c.popup.SetCursor(idx)
	_, cmd = c.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m, ok := runCmd(cmd).(ProviderConnectOAuthMsg); !ok || m.ProviderID != "github-copilot" {
		t.Errorf("expected ProviderConnectOAuthMsg for copilot, got %T", m)
	}
}

func TestProviderConnectPasteGoesToKeyInput(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c := NewProviderConnect("groq", 100, 40)
	updated, _ := c.Update(tea.PasteMsg{Content: "pasted-key"})
	c = updated.(*ProviderConnectComponent)
	if got := c.input.Value(); got != "pasted-key" {
		t.Errorf("paste not applied: %q", got)
	}
}
