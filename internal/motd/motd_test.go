package motd

import (
	"slices"
	"testing"
	"time"
)

// TestAtIsDeterministic verifies that every instant within one calendar
// day yields the same, known message.
func TestAtIsDeterministic(t *testing.T) {
	morning := time.Date(2026, 10, 7, 8, 0, 0, 0, time.UTC)
	evening := time.Date(2026, 10, 7, 23, 0, 0, 0, time.UTC)

	first, second := At(morning), At(evening)
	if first != second {
		t.Errorf("At() changed within one day: %q vs %q", first, second)
	}
	if !slices.Contains(messages, first) {
		t.Errorf("At() = %q, want one of the known messages", first)
	}
}

// TestAtCyclesThroughAllMessages verifies that consecutive days walk
// through every message exactly once.
func TestAtCyclesThroughAllMessages(t *testing.T) {
	// Consecutive days map to consecutive indices, so len(messages)
	// consecutive days must produce every message exactly once.
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	seen := make([]string, 0, len(messages))
	for range len(messages) {
		seen = append(seen, At(start))
		start = start.AddDate(0, 0, 1)
	}

	got, want := slices.Clone(seen), slices.Clone(messages)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("cycle over %d days = %v, want all messages", len(messages), seen)
	}
}

// TestAtChangesAcrossNewYear verifies that the sequence still advances
// one step per day across the year boundary.
func TestAtChangesAcrossNewYear(t *testing.T) {
	newYearsEve := time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC)
	newYearsDay := newYearsEve.AddDate(0, 0, 1)

	if At(newYearsEve) == At(newYearsDay) {
		t.Errorf("At() repeated %q across New Year; sequence should advance", At(newYearsEve))
	}
}

// TestTodayReturnsAKnownMessage verifies that Today answers with one of
// the known messages for the live clock.
func TestTodayReturnsAKnownMessage(t *testing.T) {
	if got := Today(); !slices.Contains(messages, got) {
		t.Errorf("Today() = %q, want one of the known messages", got)
	}
}
