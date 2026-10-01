package ui

import (
	"errors"
	"testing"
)

// TestNewAppModelOpensModelSelectorOnProviderError checks that when the
// configured provider cannot be created at startup (for example a failed
// OAuth token refresh), the TUI opens the model selector instead of leaving
// the user stuck on a dead model.
func TestNewAppModelOpensModelSelectorOnProviderError(t *testing.T) {
	m := NewAppModel(&stubAppController{}, AppModelOptions{
		ModelName:     "claude-sonnet-4-5",
		ProviderName:  "anthropic",
		ProviderError: errors.New("failed to get valid OAuth token: refresh failed"),
		Width:         80,
		Height:        24,
	})

	if m.state != stateModelSelector {
		t.Fatalf("state = %v, want stateModelSelector", m.state)
	}
	if m.modelSelector == nil {
		t.Fatal("modelSelector = nil, want the selector to be open")
	}
}

// TestNewAppModelKeepsSessionPickerPrecedence checks that --resume still wins
// when both the session picker and the model selector would open at startup.
func TestNewAppModelKeepsSessionPickerPrecedence(t *testing.T) {
	m := NewAppModel(&stubAppController{}, AppModelOptions{
		ModelName:         "claude-sonnet-4-5",
		ProviderName:      "anthropic",
		ProviderError:     errors.New("boom"),
		ShowSessionPicker: true,
		Width:             80,
		Height:            24,
	})

	if m.state != stateSessionSelector {
		t.Fatalf("state = %v, want stateSessionSelector", m.state)
	}
	if m.modelSelector != nil {
		t.Error("modelSelector should not open when the session picker is shown")
	}

	// Once the session picker closes, the model selector takes over so the
	// user is not stranded on a dead provider.
	m = sendMsg(m, SessionSelectorCancelledMsg{})
	if m.state != stateModelSelector {
		t.Fatalf("after cancel: state = %v, want stateModelSelector", m.state)
	}
	if m.modelSelector == nil {
		t.Error("after cancel: modelSelector = nil, want it open")
	}
}

// TestNewAppModelNoSelectorWithoutProviderError guards the default path: a
// working provider must not pop the selector at startup.
func TestNewAppModelNoSelectorWithoutProviderError(t *testing.T) {
	m := NewAppModel(&stubAppController{}, AppModelOptions{
		ModelName:    "claude-sonnet-4-5",
		ProviderName: "anthropic",
		Width:        80,
		Height:       24,
	})

	if m.state != stateInput {
		t.Fatalf("state = %v, want stateInput", m.state)
	}
	if m.modelSelector != nil {
		t.Error("modelSelector = non-nil, want nil")
	}
}
