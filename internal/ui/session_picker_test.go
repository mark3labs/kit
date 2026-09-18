package ui

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/creack/pty"
)

// TestSessionPickerLeavesTheAltScreen pins the exit contract of the picker.
//
// Bubble Tea restores the screen state it entered when the program shuts
// down: a final view that still claims the alternate screen makes the
// runtime emit the exit sequence AFTER the last frame, which drops the alt
// screen of whatever started the picker. The attach client used to lose
// its screen that way, and every following frame of the session was drawn
// into the shell's scrollback instead.
//
// The picker therefore always leaves the alternate screen itself, and the
// attach client re-enters it (see daemon.runPicker).
func TestSessionPickerLeavesTheAltScreen(t *testing.T) {
	m := newSessionPickerModel([]SessionEntry{{ID: 1, Started: time.Now()}}, "Live sessions")
	m.width, m.height = 80, 24

	if v := m.View(); !v.AltScreen {
		t.Fatal("a running picker must own the alternate screen")
	}

	m.quitting = true
	if v := m.View(); v.AltScreen {
		t.Fatal("the final frame must leave the alternate screen")
	}

	m.quitting, m.cancelled = false, true
	if v := m.View(); v.AltScreen {
		t.Fatal("a cancelled picker must leave the alternate screen")
	}
}

// TestSessionPickerGroupsByHost checks that sessions from several daemons
// are labelled, and that the local daemon (empty host) is named rather
// than shown under a blank heading.
func TestSessionPickerGroupsByHost(t *testing.T) {
	rows := buildRows([]SessionEntry{
		{ID: 1, Host: ""},
		{ID: 1, Host: "mev"},
	})

	var headers []string
	selectable := 0
	for _, r := range rows {
		if r.selectable {
			selectable++
			continue
		}
		headers = append(headers, r.header)
	}
	if selectable != 3 { // two sessions plus "start a new session"
		t.Fatalf("selectable rows = %d, want 3", selectable)
	}
	if len(headers) != 2 || headers[0] != "this machine" || headers[1] != "mev" {
		t.Fatalf("headers = %v, want [this machine mev]", headers)
	}
}

// TestSessionPickerHonoursCancellation checks that a cancelled context ends
// the picker.
//
// The picker blocks in the Bubble Tea run loop reading a terminal. Inside
// an attached client that terminal is in raw mode, so the caller cannot
// interrupt it with a signal, and without a context the picker would hold
// the client open for as long as the user left it on screen. The session
// loop around it already honours cancellation, so the picker must too.
func TestSessionPickerHonoursCancellation(t *testing.T) {
	// A pty gives the picker a real terminal to open without a keystroke
	// ever arriving, which is the state cancellation has to break out of.
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Skipf("no pty available: %v", err)
	}
	// Only the master is closed here: Bubble Tea takes ownership of the
	// input file and closes it during shutdown, so closing it from this
	// goroutine too would race that shutdown. The real caller closes its
	// pty after the picker has fully returned, which is ordered.
	defer func() { _ = ptmx.Close() }()

	ctx, cancel := context.WithCancel(t.Context())
	entries := []SessionEntry{{ID: 1, Started: time.Now()}}

	done := make(chan error, 1)
	go func() {
		_, rerr := RunSessionPicker(ctx, entries, tty, "Live sessions")
		done <- rerr
	}()

	// Let the program reach its run loop before pulling the context.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case rerr := <-done:
		if !errors.Is(rerr, context.Canceled) {
			t.Fatalf("picker error = %v, want context.Canceled", rerr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled context did not end the picker")
	}
}

// TestSessionPickerNamesASoleRemoteHost covers the plain `kit attach`
// picker on a machine with nothing running locally: every row is on a
// paired host, and without a header the list is indistinguishable from a
// list of local sessions.
func TestSessionPickerNamesASoleRemoteHost(t *testing.T) {
	rows := buildRows([]SessionEntry{{ID: 1, Host: "homelab"}})

	var headers []string
	for _, r := range rows {
		if !r.selectable {
			headers = append(headers, r.header)
		}
	}
	if len(headers) != 1 || headers[0] != "homelab" {
		t.Fatalf("headers = %v, want the sole remote host named", headers)
	}
}

// TestSessionPickerLeavesLocalOnlyListsUngrouped keeps the common case
// quiet: sessions on this machine need no header, because there is nowhere
// else they could be.
func TestSessionPickerLeavesLocalOnlyListsUngrouped(t *testing.T) {
	rows := buildRows([]SessionEntry{{ID: 1}, {ID: 2}})

	for _, r := range rows {
		if !r.selectable {
			t.Fatalf("a local-only list grew a %q header", r.header)
		}
	}
}

// pressKey drives the picker the way Bubble Tea would.
func pressKey(t *testing.T, m *sessionPickerModel, key string) {
	t.Helper()
	m.handleKey(pickerKey(key))
}

// pickerKey builds the key message for a key name the picker understands.
func pickerKey(name string) tea.KeyPressMsg {
	switch name {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		r := []rune(name)[0]
		return tea.KeyPressMsg{Code: r, Text: string(r)}
	}
}

// cursorRow returns the row the cursor sits on.
func cursorRow(t *testing.T, m *sessionPickerModel) pickerRow {
	t.Helper()
	items := m.popup.Items()
	if len(items) == 0 {
		t.Fatal("the picker has no rows")
	}
	row, ok := items[m.popup.Cursor()].Meta.(pickerRow)
	if !ok {
		t.Fatal("a picker row lost its metadata")
	}
	return row
}

// TestSessionPickerCursorSkipsHeaders keeps group headings out of the
// selection. A header names a machine; it is not something to attach to,
// so arrowing through the list must step over it in both directions.
func TestSessionPickerCursorSkipsHeaders(t *testing.T) {
	m := newSessionPickerModel([]SessionEntry{
		{ID: 1, Host: ""},
		{ID: 2, Host: "mev"},
	}, "Live sessions")

	if row := cursorRow(t, m); !row.selectable {
		t.Fatal("the picker opened on a header")
	}
	for _, key := range []string{"down", "down", "up", "up", "home", "end"} {
		pressKey(t, m, key)
		if row := cursorRow(t, m); !row.selectable {
			t.Fatalf("%q left the cursor on the %q header", key, row.header)
		}
	}
}

// TestSessionPickerFilterKeepsGrouping checks the search line: typing
// narrows the sessions, a heading left with nothing under it goes away,
// and the new-session row stays reachable however narrow the list gets.
func TestSessionPickerFilterKeepsGrouping(t *testing.T) {
	m := newSessionPickerModel([]SessionEntry{
		{ID: 1, Host: "", Cwd: "/home/ada/kit"},
		{ID: 2, Host: "mev", Name: "compiler"},
	}, "Live sessions")

	for _, ch := range "compiler" {
		pressKey(t, m, string(ch))
	}

	var headers []string
	sessions, newRows := 0, 0
	for _, item := range m.popup.Items() {
		row := item.Meta.(pickerRow)
		switch {
		case row.isNew:
			newRows++
		case !row.selectable:
			headers = append(headers, row.header)
		default:
			sessions++
		}
	}
	if sessions != 1 {
		t.Fatalf("matching sessions = %d, want 1", sessions)
	}
	if newRows != 1 {
		t.Fatalf("new-session rows = %d, want 1", newRows)
	}
	if len(headers) != 1 || headers[0] != "mev" {
		t.Fatalf("headers = %v, want only the host that still has a session", headers)
	}
	if row := cursorRow(t, m); !row.selectable {
		t.Fatal("filtering parked the cursor on a header")
	}
}

// TestSessionPickerEnterPicksTheCursorRow pins what Enter returns: the
// entry under the cursor, by its index in the caller's slice, not the row
// index of the list (which headers shift).
func TestSessionPickerEnterPicksTheCursorRow(t *testing.T) {
	m := newSessionPickerModel([]SessionEntry{
		{ID: 1, Host: "mev"},
		{ID: 2, Host: "mev"},
	}, "Live sessions")

	pressKey(t, m, "down") // second session
	pressKey(t, m, "enter")

	if !m.quitting || m.cancelled {
		t.Fatal("enter did not end the picker with a choice")
	}
	if m.choice.index != 1 {
		t.Fatalf("chosen index = %d, want 1", m.choice.index)
	}
}

// TestSessionPickerEscCancels checks the two esc steps of the search box:
// the first clears a query, the second dismisses the picker. A user who
// typed a filter should get their list back before losing the picker.
func TestSessionPickerEscCancels(t *testing.T) {
	m := newSessionPickerModel([]SessionEntry{{ID: 1}}, "Live sessions")

	pressKey(t, m, "x")
	pressKey(t, m, "esc")
	if m.cancelled {
		t.Fatal("esc on a filtered list cancelled instead of clearing the search")
	}
	if m.popup.Search() != "" {
		t.Fatalf("search = %q, want it cleared", m.popup.Search())
	}

	pressKey(t, m, "esc")
	if !m.cancelled {
		t.Fatal("esc on an empty search did not cancel the picker")
	}
}

// TestSessionPickerFilterResetsTheCursor covers the trap in a filtered
// list: the cursor used to be merely clamped, so narrowing the list from
// below the cursor parked it on the last row — "start a new session" —
// and Enter started a session instead of attaching to the match the user
// had just typed. A new query puts the cursor on the first match.
func TestSessionPickerFilterResetsTheCursor(t *testing.T) {
	m := newSessionPickerModel([]SessionEntry{
		{ID: 1, Host: "mev", Name: "alpha"},
		{ID: 2, Host: "mev", Name: "beta"},
		{ID: 3, Host: "mev", Name: "gamma"},
	}, "Live sessions")

	pressKey(t, m, "end") // on "start a new session"
	for _, ch := range "alpha" {
		pressKey(t, m, string(ch))
	}

	row := cursorRow(t, m)
	if row.isNew {
		t.Fatal("filtering left the cursor on the new-session row")
	}
	if row.entry.Name != "alpha" {
		t.Fatalf("cursor on %q, want the first match", row.entry.Name)
	}
}
