package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// timeoutAfter bounds the wait for an async shortcut handler, which the
// dispatcher runs in a goroutine.
func timeoutAfter() <-chan time.Time {
	return time.After(2 * time.Second)
}

// TestShortcutMatchesKeystrokeForm is the regression test for bindings that
// could never fire. The terminal reports a shifted letter by its text ("A"),
// so a shortcut registered as "shift+a" was previously unreachable.
func TestShortcutMatchesKeystrokeForm(t *testing.T) {
	shortcuts := map[string]int{"shift+a": 1}
	msg := tea.KeyPressMsg{Code: 'a', Text: "A", Mod: tea.ModShift}

	// Guard the premise: String() really does yield the text form.
	if msg.String() != "A" {
		t.Fatalf("premise changed: String() = %q, want %q", msg.String(), "A")
	}

	if _, ok := lookupShortcut(shortcuts, msg); !ok {
		t.Error(`binding "shift+a" did not match a Shift+A press`)
	}
}

func TestShortcutMatchesTextForm(t *testing.T) {
	shortcuts := map[string]int{"A": 1}
	msg := tea.KeyPressMsg{Code: 'a', Text: "A", Mod: tea.ModShift}
	if _, ok := lookupShortcut(shortcuts, msg); !ok {
		t.Error(`binding "A" did not match a Shift+A press`)
	}
}

// The text form is tried first, so a binding on the literal character wins
// over the modifier spelling of the same press.
func TestShortcutPrefersTextFormOnCollision(t *testing.T) {
	shortcuts := map[string]int{"A": 1, "shift+a": 2}
	msg := tea.KeyPressMsg{Code: 'a', Text: "A", Mod: tea.ModShift}
	got, ok := lookupShortcut(shortcuts, msg)
	if !ok {
		t.Fatal("no match")
	}
	if got != 1 {
		t.Errorf("matched %d, want the text-form binding (1)", got)
	}
}

func TestShortcutMatchesPlainAndModifiedKeys(t *testing.T) {
	tests := []struct {
		binding string
		msg     tea.KeyPressMsg
	}{
		{"ctrl+g", tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}},
		{"alt+p", tea.KeyPressMsg{Code: 'p', Mod: tea.ModAlt}},
		{"f1", tea.KeyPressMsg{Code: tea.KeyF1}},
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}},
		{"space", tea.KeyPressMsg{Code: ' ', Text: " "}},
		{"?", tea.KeyPressMsg{Code: '/', Text: "?", Mod: tea.ModShift}},
		{"shift+/", tea.KeyPressMsg{Code: '/', Text: "?", Mod: tea.ModShift}},
		{"ctrl+shift+s", tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl | tea.ModShift}},
	}
	for _, tt := range tests {
		t.Run(tt.binding, func(t *testing.T) {
			if _, ok := lookupShortcut(map[string]int{tt.binding: 1}, tt.msg); !ok {
				t.Errorf("binding %q did not match", tt.binding)
			}
		})
	}
}

func TestShortcutNoMatchIsReported(t *testing.T) {
	shortcuts := map[string]int{"ctrl+p": 1}
	if _, ok := lookupShortcut(shortcuts, tea.KeyPressMsg{Code: 'q', Text: "q"}); ok {
		t.Error("unrelated key matched")
	}
}

// TestLeaderChordBeatsExtensionShortcut is the regression test for the latch
// bug: an extension shortcut bound to a chord suffix used to steal the key and
// return early, leaving leaderKeyActive armed forever so every subsequent key
// was swallowed by a chord that never completed.
func TestLeaderChordBeatsExtensionShortcut(t *testing.T) {
	m, _, _ := newTestAppModel(nil)
	fired := make(chan struct{}, 1)
	m.getGlobalShortcuts = func() map[string]func() {
		return map[string]func(){"s": func() { fired <- struct{}{} }}
	}

	m.leaderKeyActive = true
	m.update(tea.KeyPressMsg{Code: 's', Text: "s"})

	if m.leaderKeyActive {
		t.Error("leaderKeyActive still armed: the chord key was stolen by the shortcut")
	}
	select {
	case <-fired:
		t.Error("extension shortcut fired while a leader chord was armed")
	default:
	}
}

// TestDispatcherMatchesKeystrokeForm is the end-to-end counterpart: it drives
// a real key press through update() to prove the dispatcher itself, not just
// the lookup helper, honours the modifier spelling.
func TestDispatcherMatchesKeystrokeForm(t *testing.T) {
	m, _, _ := newTestAppModel(nil)
	fired := make(chan struct{}, 1)
	m.getGlobalShortcuts = func() map[string]func() {
		return map[string]func(){"shift+a": func() { fired <- struct{}{} }}
	}

	m.update(tea.KeyPressMsg{Code: 'a', Text: "A", Mod: tea.ModShift})

	select {
	case <-fired:
	case <-timeoutAfter():
		t.Error(`dispatcher did not fire the "shift+a" binding on a Shift+A press`)
	}
}

// With no chord armed the same shortcut must still fire normally.
func TestExtensionShortcutFiresWithoutLeader(t *testing.T) {
	m, _, _ := newTestAppModel(nil)
	fired := make(chan struct{}, 1)
	m.getGlobalShortcuts = func() map[string]func() {
		return map[string]func(){"ctrl+p": func() { fired <- struct{}{} }}
	}

	m.update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})

	select {
	case <-fired:
	case <-timeoutAfter():
		t.Error("extension shortcut did not fire")
	}
}
