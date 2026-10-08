package daemon

import "testing"

func TestSessionManagementChordsOnlyStopPump(t *testing.T) {
	for _, tc := range []struct {
		key      byte
		sentinel uint64
	}{{',', renameSentinel}, {'k', killSentinel}} {
		control, claimed := dispatchChord(nil, AttachOptions{}, leaderPrimary, keyEvent{Data: []byte{tc.key}})
		if !claimed || !control.stop || !control.outcome.wantSwitch || control.outcome.switchTo != tc.sentinel {
			t.Fatalf("chord %c: %+v", tc.key, control)
		}
		// A nil connection proves dispatch never sends a destructive request.
		if _, claimed := dispatchChord(nil, AttachOptions{}, leaderLegacy, keyEvent{Data: []byte{tc.key}}); claimed {
			t.Fatalf("legacy leader claimed %c", tc.key)
		}
	}
}
