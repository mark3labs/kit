package ui

import (
	"testing"
	"time"
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
