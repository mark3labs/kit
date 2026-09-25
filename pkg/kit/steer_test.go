package kit

import (
	"testing"

	"github.com/mark3labs/kit/internal/agent"
)

func steerTexts(msgs []SteerMessage) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Text
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestInjectSteer_keptWhenIdle verifies that a steer message sent while no
// generation runs (turn setup, compaction) is kept, not silently dropped.
func TestInjectSteer_keptWhenIdle(t *testing.T) {
	t.Parallel()
	m := &Kit{}

	m.InjectSteer("one")
	m.InjectSteer("two")

	if got := steerTexts(m.DrainSteer()); !equalStrings(got, []string{"one", "two"}) {
		t.Fatalf("DrainSteer() = %v, want [one two]", got)
	}
	if got := m.DrainSteer(); got != nil {
		t.Fatalf("second DrainSteer() = %v, want nil", got)
	}
}

// TestInjectSteer_channelFullKeepsOrder verifies that messages that do not
// fit in the live channel are kept and returned after the channel contents.
func TestInjectSteer_channelFullKeepsOrder(t *testing.T) {
	t.Parallel()
	m := &Kit{steerCh: make(chan agent.SteerMessage, 1)}

	m.InjectSteer("in-channel")
	m.InjectSteer("overflow")

	if got := steerTexts(m.DrainSteer()); !equalStrings(got, []string{"in-channel", "overflow"}) {
		t.Fatalf("DrainSteer() = %v, want [in-channel overflow]", got)
	}
}
