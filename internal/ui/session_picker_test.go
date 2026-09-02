package ui

import (
	"context"
	"errors"
	"testing"
	"time"

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
	m := &sessionPickerModel{
		rows:   buildRows([]SessionEntry{{ID: 1, Started: time.Now()}}),
		title:  "Live sessions",
		width:  80,
		height: 24,
	}

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
