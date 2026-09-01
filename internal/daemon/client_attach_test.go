package daemon

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

// The chord state machine lives inside runInputPump, which needs a real
// terminal. chordTrace replicates its decision logic exactly so the chord
// table can be tested without one. It must be kept in step with the loop
// in runInputPump.
func chordTrace(t *testing.T, chunks ...[]byte) (forwarded []byte, claimed []byte) {
	t.Helper()
	sc := &keyScanner{}
	var leaderBuf []byte
	var leaderOf leaderKind
	for _, c := range chunks {
		for _, ev := range sc.Feed(c) {
			if leaderBuf != nil {
				if ev.Leader || ev.LeaderRelease {
					leaderBuf = append(leaderBuf, ev.Data...)
					continue
				}
				if isClaimed(leaderOf, ev) {
					claimed = append(claimed, ev.Data...)
					leaderBuf = nil
					leaderOf = leaderNone
					continue
				}
				forwarded = append(forwarded, leaderBuf...)
				leaderBuf = nil
				leaderOf = leaderNone
			}
			if ev.Leader {
				leaderBuf = append([]byte(nil), ev.Data...)
				leaderOf = ev.Kind
				continue
			}
			forwarded = append(forwarded, ev.Data...)
		}
	}
	return forwarded, claimed
}

// isClaimed mirrors dispatchChord's claim decision.
func isClaimed(kind leaderKind, ev keyEvent) bool {
	if len(ev.Data) != 1 || ev.Release {
		return false
	}
	if ev.Data[0] == 'd' {
		return true
	}
	return kind == leaderPrimary
}

func TestChordTable(t *testing.T) {
	ctrlBracket := []byte{leaderKey}
	ctrlX := []byte{legacyLeaderKey}
	// Kitty reports Ctrl-] as press, release, then the suffix.
	kittyBracketPress := []byte("\x1b[93;5u")
	kittyBracketRelease := []byte("\x1b[93;5:3u")
	kittyXPress := []byte("\x1b[120;5u")
	kittyXRelease := []byte("\x1b[120;5:3u")

	cases := []struct {
		name        string
		chunks      [][]byte
		wantClaimed string
		wantFwd     string
	}{
		// The client owns every suffix after its own leader.
		{"legacy detach", [][]byte{ctrlBracket, []byte("d")}, "d", ""},
		{"legacy switch", [][]byte{ctrlBracket, []byte("s")}, "s", ""},
		{"legacy cycle", [][]byte{ctrlBracket, []byte("n")}, "n", ""},
		{"kitty detach split", [][]byte{kittyBracketPress, kittyBracketRelease, []byte("d")}, "d", ""},
		{"kitty detach coalesced", [][]byte{append(append(append([]byte{}, kittyBracketPress...), kittyBracketRelease...), 'd')}, "d", ""},
		{"kitty switch", [][]byte{kittyBracketPress, kittyBracketRelease, []byte("s")}, "s", ""},

		// Ctrl-X keeps working as the deprecated detach alias.
		{"legacy ctrl-x detach", [][]byte{ctrlX, []byte("d")}, "d", ""},
		{"kitty ctrl-x detach", [][]byte{kittyXPress, kittyXRelease, []byte("d")}, "d", ""},

		// Every other Ctrl-X chord belongs to the host TUI (steer,
		// thinking, move, editor) and must reach it byte-identical.
		{"ctrl-x steer to host", [][]byte{ctrlX, []byte("s")}, "", "\x18s"},
		{"ctrl-x editor to host", [][]byte{ctrlX, []byte("e")}, "", "\x18e"},
		{"kitty ctrl-x steer to host", [][]byte{kittyXPress, kittyXRelease, []byte("s")},
			"", "\x1b[120;5u\x1b[120;5:3us"},

		// Plain typing is untouched.
		{"plain text", [][]byte{[]byte("hello")}, "", "hello"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fwd, claimed := chordTrace(t, tc.chunks...)
			if string(claimed) != tc.wantClaimed {
				t.Errorf("claimed = %q, want %q", claimed, tc.wantClaimed)
			}
			if string(fwd) != tc.wantFwd {
				t.Errorf("forwarded = %q, want %q", fwd, tc.wantFwd)
			}
		})
	}
}

func TestNeighbourSessionWraps(t *testing.T) {
	entries := []SessionEntry{{ID: 2}, {ID: 5}, {ID: 9}}
	cases := []struct {
		current uint64
		forward bool
		want    uint64
	}{
		{2, true, 5},
		{5, true, 9},
		{9, true, 2},  // wraps
		{2, false, 9}, // wraps
		{9, false, 5},
		{404, true, 2}, // a session that has gone falls back to the first
	}
	for _, c := range cases {
		if got := neighbourSession(entries, c.current, c.forward); got != c.want {
			t.Errorf("neighbour(%d, forward=%v) = %d, want %d", c.current, c.forward, got, c.want)
		}
	}
}

// TestLocalSocketRoundTrip drives a real session table over a socket pair,
// covering the paths that broke during development: the attach ack must
// reach the client, and a detach must NOT tear down the connection,
// because a switch is a detach followed by another attach.
func TestLocalSocketRoundTrip(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	table := newSessionTable(newDaemonRuntime(nil))
	sink := newFrameSink(serverConn)
	wire := table.conns.addLocal(sink)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = table.runFrameSource(ctx, serverConn, sink, wire.id) }()

	client := newClientConn(clientConn)
	go client.readLoop()

	// An empty daemon reports no sessions.
	entries, err := client.listSessions()
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no sessions on a fresh table, got %d", len(entries))
	}

	// Attaching to a session that does not exist must be refused rather
	// than hang: the client blocks on this ack.
	if _, err := client.attach(42); err == nil {
		t.Fatal("expected the attach to a missing session to be refused")
	}

	// A detach leaves the connection registered so a following attach can
	// still be answered. This is the switch path.
	if err := client.write(FrameSessionDetach, nil); err != nil {
		t.Fatalf("detach: %v", err)
	}
	// Give the daemon a moment to process the detach frame.
	time.Sleep(50 * time.Millisecond)
	if table.conns.get(wire.id) == nil {
		t.Fatal("the connection was dropped on detach; a switch could not be answered")
	}

	// The connection still works after the detach.
	if _, err := client.listSessions(); err != nil {
		t.Fatalf("list after detach: %v", err)
	}
}

// TestDecideAuthTargetsTheSidecar pins the routing of the pairing verdict.
// It is addressed to the sidecar itself on wire id 0, which no client
// connection ever owns, so routing it through writeTo would silently stop
// the daemon answering handshakes.
func TestDecideAuthTargetsTheSidecar(t *testing.T) {
	rt := newDaemonRuntime(nil)
	var buf lockedBuffer
	rt.setSink(newFrameSink(&buf))

	table := newSessionTable(rt)
	table.decideAuth(make([]byte, 8), true, "")

	frame, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("the auth decision never reached the sidecar: %v", err)
	}
	if frame.Type != FrameAuthDecision {
		t.Fatalf("frame type = %#x, want FrameAuthDecision", frame.Type)
	}
	if frame.Session != 0 {
		t.Fatalf("auth decisions must go out on wire 0, got %d", frame.Session)
	}
}

// lockedBuffer is a tiny synchronous buffer for frame round-trips.
type lockedBuffer struct {
	data []byte
	off  int
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *lockedBuffer) Read(p []byte) (int, error) {
	if b.off >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.off:])
	b.off += n
	return n, nil
}
