package motd

import (
	"reflect"
	"testing"
	"time"
)

func TestAtIsDeterministic(t *testing.T) {
	morning := time.Date(2026, 10, 7, 8, 0, 0, 0, time.UTC)
	evening := time.Date(2026, 10, 7, 23, 0, 0, 0, time.UTC)

	if got := At(morning); got != At(evening) {
		t.Errorf("At() changed within one day: %q vs %q", got, At(evening))
	}
	if got := At(morning); !contains(messages, got) {
		t.Errorf("At() = %q, want one of the known messages", got)
	}
}

func TestAtCyclesThroughAllMessages(t *testing.T) {
	// Consecutive days map to consecutive indices, so len(messages)
	// consecutive days must produce every message exactly once.
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	seen := make([]string, 0, len(messages))
	for i := 0; i < len(messages); i++ {
		seen = append(seen, At(start.AddDate(0, 0, i)))
	}

	if !sameMembers(seen, messages) {
		t.Errorf("cycle over %d days = %v, want all messages", len(messages), seen)
	}
}

func TestTodayReturnsAKnownMessage(t *testing.T) {
	if got := Today(); !contains(messages, got) {
		t.Errorf("Today() = %q, want one of the known messages", got)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// sameMembers reports whether the two lists hold the same strings,
// ignoring order and repeats.
func sameMembers(a, b []string) bool {
	count := func(list []string) map[string]int {
		out := make(map[string]int, len(list))
		for _, s := range list {
			out[s]++
		}
		return out
	}
	return reflect.DeepEqual(count(a), count(b))
}
