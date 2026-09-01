package daemon

import (
	"context"
	"io"
	"net"
	"os"
	"strings"
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
	defer func() { _ = serverConn.Close() }()
	defer func() { _ = clientConn.Close() }()

	table := newSessionTable(newDaemonRuntime(nil))
	sink := newFrameSink(serverConn)
	wire := table.conns.addLocal(sink)

	ctx := t.Context()
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

// TestHostSwitchRefusesAForeignHostSession is the regression test for a
// cross-host choice being attached to the wrong daemon.
//
// Every daemon numbers its sessions from 1, so sending a remote session's
// id over this connection would silently bind whichever local session
// happens to share that number. RunClient must hand the choice back to
// the caller instead of attaching it here.
func TestHostSwitchRefusesAForeignHostSession(t *testing.T) {
	cases := []struct {
		name      string
		connected string // the host this client is attached to
		choice    SessionChoice
		wantHost  string // "" means no switch expected
	}{
		{
			name:      "remote session chosen from the local daemon",
			connected: "",
			choice:    SessionChoice{ID: 2, Host: "violet"},
			wantHost:  "violet",
		},
		{
			name:      "local session chosen from a remote daemon",
			connected: "violet",
			choice:    SessionChoice{ID: 2, Host: ""},
			wantHost:  "",
		},
		{
			name:      "another host chosen from a remote daemon",
			connected: "violet",
			choice:    SessionChoice{ID: 1, Host: "homelab"},
			wantHost:  "homelab",
		},
		{
			name:      "session on the daemon we are already talking to",
			connected: "violet",
			choice:    SessionChoice{ID: 3, Host: "violet"},
		},
		{
			name:      "local session on the local daemon",
			connected: "",
			choice:    SessionChoice{ID: 3, Host: ""},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sw := hostSwitch(AttachOptions{Host: tc.connected}, tc.choice)
			if tc.wantHost == "" && tc.choice.Host == tc.connected {
				if sw != nil {
					t.Fatalf("a same-host choice must attach here, got a switch to %q", sw.Host)
				}
				return
			}
			if sw == nil {
				t.Fatal("a cross-host choice was attached to the current daemon by bare id")
			}
			if sw.Host != tc.wantHost || sw.Session != tc.choice.ID {
				t.Fatalf("switch = %+v, want host %q session %d", sw, tc.wantHost, tc.choice.ID)
			}
		})
	}
}

// TestChooseSessionTagsChoicesWithTheCurrentHost guards the inverse of
// TestHostSwitchRefusesAForeignHostSession: a choice made on the daemon we
// are already connected to must NOT be treated as a cross-host switch.
//
// chooseSession returns bare ids for --new and for a direct target. If
// those carry no host, hostSwitch reads them as a switch to the local
// daemon and `kit attach --host NAME` silently bounces home.
func TestChooseSessionTagsChoicesWithTheCurrentHost(t *testing.T) {
	for _, host := range []string{"", "violet"} {
		t.Run("host="+host, func(t *testing.T) {
			forceNew := AttachOptions{Host: host, ForceNew: true, Pick: nil}
			choice, err := chooseSession(t.Context(), nil, forceNew)
			if err != nil {
				t.Fatalf("chooseSession: %v", err)
			}
			if sw := hostSwitch(forceNew, choice); sw != nil {
				t.Fatalf("--new on host %q was treated as a switch to %q", host, sw.Host)
			}

			direct := AttachOptions{Host: host, Target: 7, Pick: localPickerStub}
			choice, err = chooseSession(t.Context(), nil, direct)
			if err != nil {
				t.Fatalf("chooseSession: %v", err)
			}
			if choice.ID != 7 {
				t.Fatalf("target id = %d, want 7", choice.ID)
			}
			if sw := hostSwitch(direct, choice); sw != nil {
				t.Fatalf("a direct id on host %q was treated as a switch to %q", host, sw.Host)
			}
		})
	}
}

// localPickerStub stands in for a picker that is never called.
func localPickerStub(_ context.Context, entries []SessionEntry, _ *os.File) (SessionChoice, error) {
	return SessionChoice{}, nil
}

// TestStopStdinReleasesTheTerminal covers the hand-back that makes an
// attach client survivable.
//
// A goroutine parked in os.Stdin.Read holds the file's read lock, and that
// lock outlives the client: whoever runs next — the error renderer, which
// queries the terminal, or the second client started by a cross-host
// switch — blocks behind it, or has its keystrokes stolen by it. The
// symptoms were a hang with a blank screen after any failed attach, and a
// completely deaf terminal after Ctrl-] w.
func TestStopStdinReleasesTheTerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() { _ = w.Close() }()

	realStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = realStdin; _ = r.Close() }()

	conn := newClientConn(struct {
		io.Reader
		io.Writer
	}{Reader: strings.NewReader(""), Writer: io.Discard})
	conn.readStdin()

	if _, err := w.Write([]byte("a")); err != nil {
		t.Fatalf("write: %v", err)
	}
	select {
	case chunk := <-conn.stdinCh:
		if string(chunk) != "a" {
			t.Fatalf("read %q, want %q", chunk, "a")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the terminal reader delivered nothing")
	}

	conn.stopStdin()

	// The reader is gone, so a later reader on the same terminal gets the
	// next keystroke — and, crucially, gets it at all.
	if _, err := w.Write([]byte("b")); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := make(chan string, 1)
	go func() {
		buf := make([]byte, 1)
		if n, rerr := os.Stdin.Read(buf); rerr == nil && n == 1 {
			got <- string(buf[:n])
		}
	}()
	select {
	case s := <-got:
		if s != "b" {
			t.Fatalf("second reader saw %q, want %q", s, "b")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stdin was never handed back: the cancelled reader still owns it")
	}
}

// TestStopStdinWithNobodyReading covers the shutdown path taken after the
// session pump has already gone.
//
// The reader forwards keystrokes into a buffered channel. Once the pump
// stops, nothing drains it, so a reader that cannot abandon a send parks
// there for good — and stopStdin, which waits for the reader to leave
// before closing it, would wait forever. Anything typed between the last
// frame and the client's exit lands in exactly that window.
func TestStopStdinWithNobodyReading(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() { _ = w.Close() }()

	realStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = realStdin; _ = r.Close() }()

	conn := newClientConn(struct {
		io.Reader
		io.Writer
	}{Reader: strings.NewReader(""), Writer: io.Discard})
	conn.readStdin()

	// Overrun the channel with nobody receiving, so the reader is parked
	// on a send when the client tears down.
	for range cap(conn.stdinCh) * 3 {
		if _, werr := w.Write([]byte("x")); werr != nil {
			t.Fatalf("write: %v", werr)
		}
	}
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() { conn.stopStdin(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stopStdin deadlocked on a reader parked mid-send")
	}
}

// TestStayOnCurrentNeverSpawnsASession covers the choice made when a user
// dismisses a session picker.
//
// The switch loop reuses the choice that opened the picker, and for a
// client started with --new that choice is 0 — which the daemon reads as
// "spawn a session". Pressing Ctrl-] s and then Esc therefore answered a
// dismissed picker with a brand new session, abandoning the one the user
// was working in and leaving an empty session behind on the daemon.
func TestStayOnCurrentNeverSpawnsASession(t *testing.T) {
	for _, host := range []string{"", "violet"} {
		t.Run("host="+host, func(t *testing.T) {
			conn := newClientConn(struct {
				io.Reader
				io.Writer
			}{Reader: strings.NewReader(""), Writer: io.Discard})
			opts := AttachOptions{Host: host}

			// The client began with --new, so the choice that opened the
			// picker is the "spawn one" sentinel.
			opened := SessionChoice{ID: 0, Host: host}
			conn.setCurrent(4) // ...and the daemon assigned session 4

			stay := stayOnCurrent(conn, opts)

			if stay.ID == opened.ID {
				t.Fatal("a dismissed picker reattached the --new sentinel: this spawns a second session")
			}
			if stay.ID != 4 {
				t.Fatalf("stay.ID = %d, want the current session 4", stay.ID)
			}
			// The choice must stay on this daemon, or the switch loop
			// reads it as a cross-host hop.
			if sw := hostSwitch(opts, stay); sw != nil {
				t.Fatalf("staying put on host %q was read as a switch to %q", host, sw.Host)
			}
		})
	}
}
