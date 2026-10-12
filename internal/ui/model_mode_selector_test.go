package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestModelModeSelectorDefault(t *testing.T) {
	s := NewModelModeSelector(nil, "", 100, 30)
	_, cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter must select Default")
	}
	msg, ok := cmd().(ModelModeSelectedMsg)
	if !ok || msg.Mode != "" {
		t.Fatalf("selection = %#v", msg)
	}
}

func TestModelModeSelectorShowsUnavailableReasonAndPricing(t *testing.T) {
	s := NewModelModeSelector([]ModelModeOption{{Name: "fast", Supported: false, Reason: "unsupported tier"}}, "", 100, 30)
	rendered := s.RenderOverlay()
	if !strings.Contains(rendered, "fast") {
		t.Fatal("missing catalog mode")
	}
}

func TestModelModeCommandRunsSetterWithoutMutatingUI(t *testing.T) {
	called := false
	m := &AppModel{modelMode: "previous", setModelMode: func(mode string) error {
		called = true
		if mode != "fast" {
			t.Fatalf("mode = %q", mode)
		}
		return errors.New("unsupported")
	}}
	cmd := m.applyModelMode("fast")
	if called {
		t.Fatal("setter must run in a command, not Update")
	}
	msg, ok := cmd().(modelModeUpdateMsg)
	if !ok || msg.Err == nil {
		t.Fatalf("result = %#v", msg)
	}
	if m.modelMode != "previous" {
		t.Fatal("command mutated UI state")
	}
}
