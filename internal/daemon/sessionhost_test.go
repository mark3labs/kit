//go:build !windows

package daemon

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Tests for the machinery that lets a session outlive its daemon.
//
// The end-to-end behaviour is exercised with a REAL supervisor process
// (TestSessionHostSurvivesItsDaemon): a supervisor that merely looks
// right in a unit test but dies with its parent would pass every mock and
// still lose the user's work, which is the exact failure this change
// exists to remove.

// fakeSessionHost is a supervisor stand-in: it speaks the host protocol
// over a real Unix socket without starting a PTY or a child. It is used
// where the test is about the DAEMON's half of the link.
type fakeSessionHost struct {
	t    *testing.T
	ln   net.Listener
	path string
	info SessionHostInfo

	// received collects the frames the daemon sent us.
	received chan Frame
	// conns hands each accepted connection to the test.
	conns chan net.Conn
}

func newFakeSessionHost(t *testing.T, id uint64) *fakeSessionHost {
	return newFakeSessionHostWith(t, id, nil)
}

// newFakeSessionHostWith builds a stand-in whose hello has been adjusted.
//
// The adjustment happens BEFORE the accept loop starts. Mutating the
// hello afterwards would race the goroutine that serialises it, which is
// a race in the test rather than in the code under test — and a flaky
// test here would be one nobody trusts about the thing that matters most.
func newFakeSessionHostWith(t *testing.T, id uint64, adjust func(*SessionHostInfo)) *fakeSessionHost {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "host.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Skipf("no unix sockets available: %v", err)
	}
	h := &fakeSessionHost{
		t:    t,
		ln:   ln,
		path: path,
		info: SessionHostInfo{
			Protocol: ProtocolName,
			Version:  ProtocolVersion,
			Features: ProtocolFeatures,
			ID:       id,
			PID:      os.Getpid(),
			ChildPID: os.Getpid(),
			Started:  time.Now().Add(-time.Hour),
			Name:     "a-named-session",
			Cwd:      "/somewhere",
		},
		received: make(chan Frame, 32),
		conns:    make(chan net.Conn, 4),
	}
	if adjust != nil {
		adjust(&h.info)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go h.accept()
	return h
}

func (h *fakeSessionHost) accept() {
	for {
		conn, err := h.ln.Accept()
		if err != nil {
			return
		}
		payload, err := EncodeSessionHostInfo(h.info)
		if err != nil {
			return
		}
		_ = WriteFrame(conn, FrameHello, 0, payload)
		h.conns <- conn
		go func(c net.Conn) {
			for {
				f, rerr := ReadFrame(c)
				if rerr != nil {
					return
				}
				select {
				case h.received <- f:
				default:
				}
			}
		}(conn)
	}
}

// send pushes child output towards the daemon.
func (h *fakeSessionHost) send(conn net.Conn, t FrameType, payload []byte) {
	if err := WriteFrame(conn, t, 0, payload); err != nil {
		h.t.Fatalf("session host write: %v", err)
	}
}

// awaitFrame waits for a frame of a given type from the daemon.
func (h *fakeSessionHost) awaitFrame(want FrameType) Frame {
	h.t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case f := <-h.received:
			if f.Type == want {
				return f
			}
		case <-deadline:
			h.t.Fatalf("no %#x frame from the daemon", byte(want))
		}
	}
}

// TestHostIOLooksLikeAPTY is the property the whole refactor rests on:
// the session table above sessionIO cannot tell the two apart.
func TestHostIOLooksLikeAPTY(t *testing.T) {
	host := newFakeSessionHost(t, 4)
	io, info, err := dialSessionHost(t.Context(), host.path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = io.Close() }()

	if info.ID != 4 || info.Name != "a-named-session" {
		t.Fatalf("hello did not survive the dial: %+v", info)
	}
	if !io.Hosted() {
		t.Fatal("a hosted session must report itself as hosted")
	}

	conn := <-host.conns

	// Daemon -> session: terminal input arrives as DATA.
	if _, err := io.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if got := host.awaitFrame(FrameData); string(got.Payload) != "hello" {
		t.Fatalf("input arrived as %q", got.Payload)
	}

	// Session -> daemon: child output is returned by Read, exactly as a
	// PTY master would.
	host.send(conn, FrameData, []byte("world"))
	buf := make([]byte, 64)
	n, err := io.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "world" {
		t.Fatalf("output read back as %q", buf[:n])
	}

	// Resize and rename travel to where the session lives.
	if err := io.Resize(winSize{100, 40}); err != nil {
		t.Fatal(err)
	}
	if got := host.awaitFrame(FrameResize); len(got.Payload) != 4 {
		t.Fatalf("resize payload = %v", got.Payload)
	}
	io.Rename("renamed")
	if got := host.awaitFrame(FrameSessionRename); string(got.Payload) != "renamed" {
		t.Fatalf("rename arrived as %q", got.Payload)
	}
}

// TestHostIOReadDoesNotDropAPartialFrame covers the bug a naive
// implementation has: a DATA frame larger than the caller's buffer must
// be returned across several Reads, never truncated. The lost bytes would
// be the child's output, and the corruption would be invisible.
func TestHostIOReadDoesNotDropAPartialFrame(t *testing.T) {
	host := newFakeSessionHost(t, 1)
	io, _, err := dialSessionHost(t.Context(), host.path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = io.Close() }()
	conn := <-host.conns

	want := bytes.Repeat([]byte("abcdefgh"), 64) // 512 bytes
	host.send(conn, FrameData, want)

	var got []byte
	small := make([]byte, 7) // deliberately not a divisor of the payload
	for len(got) < len(want) {
		n, rerr := io.Read(small)
		if rerr != nil {
			t.Fatalf("read failed after %d of %d bytes: %v", len(got), len(want), rerr)
		}
		got = append(got, small[:n]...)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("output was corrupted by being read in small pieces")
	}
}

// TestHostIOCloseIsNotTerminate is the distinction that makes a daemon
// restart safe. Close means "this daemon is letting go"; Terminate means
// "this session is over". Confusing them would end every session on
// shutdown, which is the behaviour being removed.
func TestHostIOCloseIsNotTerminate(t *testing.T) {
	host := newFakeSessionHost(t, 1)
	io, _, err := dialSessionHost(t.Context(), host.path)
	if err != nil {
		t.Fatal(err)
	}
	// Not read directly: fakeSessionHost.accept already owns a reader on
	// this connection, and a second one here would race it — the accept
	// reader could consume the very frame this test exists to catch, and
	// the test would pass by accident.
	<-host.conns

	_ = io.Close()

	// The supervisor sees the connection drop and NOTHING else: no BYE,
	// which is the frame that would stop it.
	select {
	case f := <-host.received:
		t.Fatalf("Close sent frame %#x; a daemon shutting down must send nothing, "+
			"and BYE in particular would kill every session", byte(f.Type))
	case <-time.After(time.Second):
	}
}

// TestHostIOTerminateSaysGoodbye is the other half: ending a session for
// real must reach the supervisor, or its child would be left running with
// nothing driving it.
func TestHostIOTerminateSaysGoodbye(t *testing.T) {
	host := newFakeSessionHost(t, 1)
	io, _, err := dialSessionHost(t.Context(), host.path)
	if err != nil {
		t.Fatal(err)
	}
	<-host.conns

	io.Terminate()
	host.awaitFrame(FrameBye)
}

// TestHostIOReadEndsOnBye checks that a finished session surfaces to the
// session table the same way a closed PTY does.
func TestHostIOReadEndsOnBye(t *testing.T) {
	host := newFakeSessionHost(t, 1)
	io, _, err := dialSessionHost(t.Context(), host.path)
	if err != nil {
		t.Fatal(err)
	}
	conn := <-host.conns

	host.send(conn, FrameBye, nil)

	if _, rerr := io.Read(make([]byte, 16)); rerr == nil {
		t.Fatal("Read must report the end of a session")
	}
	// Wait must be released too, or the session would never be retired.
	done := make(chan struct{})
	go func() { io.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Wait did not return after the session ended")
	}
}

// TestDialSessionHostRefusesAnotherProtocolVersion makes sure a
// supervisor we cannot drive is refused rather than half-driven.
func TestDialSessionHostRefusesAnotherProtocolVersion(t *testing.T) {
	host := newFakeSessionHostWith(t, 1, func(i *SessionHostInfo) {
		i.Version = ProtocolVersion + 1
	})

	if _, _, err := dialSessionHost(t.Context(), host.path); err == nil {
		t.Fatal("a supervisor from another protocol version was adopted")
	}
}

// TestAdoptSkipsADeadSocket checks the housekeeping half of adoption: a
// socket whose supervisor is gone is unlinked, not dialled forever.
func TestAdoptSkipsADeadSocket(t *testing.T) {
	isolateRuntimeDir(t)
	dir, err := sessionsDir()
	if err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "9.sock")
	if err := os.WriteFile(stale, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	table := newSessionTable(newDaemonRuntime(nil))
	if adopted := table.adoptHostedSessions(t.Context()); len(adopted) != 0 {
		t.Fatalf("adopted %v from a dead socket", adopted)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("a dead supervisor's socket was left behind")
	}
}

// TestSessionIDFromSocket covers the parser that decides what adoption
// even looks at.
func TestSessionIDFromSocket(t *testing.T) {
	cases := map[string]struct {
		id uint64
		ok bool
	}{
		"1.sock":       {1, true},
		"42.sock":      {42, true},
		"0.sock":       {0, false}, // 0 means "spawn a new session" on the wire
		"x.sock":       {0, false},
		"1.sock.tmp":   {0, false},
		".cfg-1-99999": {0, false},
		"1":            {0, false},
	}
	for name, want := range cases {
		id, ok := sessionIDFromSocket(name)
		if ok != want.ok || id != want.id {
			t.Errorf("sessionIDFromSocket(%q) = %d,%v want %d,%v", name, id, ok, want.id, want.ok)
		}
	}
}

// TestReleaseSessionsLeavesHostedSessionsRunning is the shutdown
// contract, stated as a test because getting it wrong is silent and
// destroys work: a `systemctl restart kit` would take every session with
// it and the daemon would report a clean exit.
func TestReleaseSessionsLeavesHostedSessionsRunning(t *testing.T) {
	isolateRuntimeDir(t)
	host := newFakeSessionHost(t, 1)
	io, _, err := dialSessionHost(t.Context(), host.path)
	if err != nil {
		t.Fatal(err)
	}
	<-host.conns

	table := newSessionTable(newDaemonRuntime(nil))
	table.mu.Lock()
	table.sessions[1] = &remoteSession{
		id: 1, io: io, started: time.Now(),
		clients: make(map[uint32]winSize), modes: newTermModes(),
	}
	table.mu.Unlock()

	released, ended := table.releaseSessions()
	if released != 1 || ended != 0 {
		t.Fatalf("releaseSessions() = %d released, %d ended; want 1, 0", released, ended)
	}

	// No BYE reached the supervisor, so its child is still running.
	select {
	case f := <-host.received:
		if f.Type == FrameBye {
			t.Fatal("shutdown sent BYE to a hosted session; the user's work would be destroyed")
		}
	case <-time.After(300 * time.Millisecond):
	}
}

// TestSeedSessionIDsNeverReusesAnID pins the guarantee that makes every
// durable reference to a session safe: its id, its socket name, its
// scratch files, and whatever the user wrote down to reattach with.
func TestSeedSessionIDsNeverReusesAnID(t *testing.T) {
	isolateRuntimeDir(t)

	// A previous daemon got as far as session 7 and left 3 running.
	if err := writeSessionRecords([]sessionRecord{
		{ID: 3, Hosted: true, Run: "old"},
		{ID: 7, Run: "old"},
	}); err != nil {
		t.Fatal(err)
	}

	table := newSessionTable(newDaemonRuntime(nil))
	table.seedSessionIDs([]uint64{3})

	table.mu.Lock()
	next := table.nextID + 1
	table.mu.Unlock()
	if next <= 7 {
		t.Fatalf("the next session would be id %d, reusing a number that has already been handed out", next)
	}
}

// TestSweepSparesAdoptedAndHostedSessions is the regression test for the
// most destructive mistake available here: the start-up sweep killing the
// sessions the daemon has just adopted.
func TestSweepSparesAdoptedAndHostedSessions(t *testing.T) {
	isolateRuntimeDir(t)

	adoptedChild := fakeSessionChild(t, mustRuntimeDir(t))
	hostedChild := fakeSessionChild(t, mustRuntimeDir(t))

	if err := writeSessionRecords([]sessionRecord{
		{ID: 1, PID: adoptedChild.Process.Pid, Run: "a-dead-run", Hosted: true},
		{ID: 2, PID: hostedChild.Process.Pid, Run: "a-dead-run", Hosted: true},
	}); err != nil {
		t.Fatal(err)
	}

	// Session 1 was adopted; session 2 was not, but is marked hosted and
	// so must still be left alone — a supervisor that did not answer is
	// far more likely to be gone already than to be ours to kill.
	sweepOrphanSessions("this-run", []uint64{1})

	for _, c := range []struct {
		name string
		pid  int
	}{{"an adopted session", adoptedChild.Process.Pid}, {"an unadopted hosted session", hostedChild.Process.Pid}} {
		if !processExists(c.pid) {
			t.Errorf("the start-up sweep killed %s", c.name)
		}
	}
}

func mustRuntimeDir(t *testing.T) string {
	t.Helper()
	dir, err := daemonRuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestScrollbackRingBuffer covers the buffer that puts a screen back
// after a daemon restart.
func TestScrollbackRingBuffer(t *testing.T) {
	s := newScrollback(10)
	if got := s.Bytes(); got != nil {
		t.Fatalf("a fresh scrollback returned %q", got)
	}

	s.Write([]byte("abcde"))
	if got := string(s.Bytes()); got != "abcde" {
		t.Fatalf("Bytes() = %q", got)
	}

	// Past the limit, the OLDEST output is what goes: the newest bytes
	// are the ones still on the user's screen.
	s.Write([]byte("fghijkl"))
	if got := string(s.Bytes()); got != "cdefghijkl" {
		t.Fatalf("after overflow Bytes() = %q, want the last 10 bytes", got)
	}

	// A single write larger than the buffer replaces it entirely.
	s.Write([]byte("0123456789ABCDEF"))
	if got := string(s.Bytes()); got != "6789ABCDEF" {
		t.Fatalf("after a huge write Bytes() = %q", got)
	}

	// The returned slice is a copy: a caller holding it while the session
	// keeps writing must not see it change under them.
	held := s.Bytes()
	s.Write([]byte("zzzzz"))
	if string(held) != "6789ABCDEF" {
		t.Fatal("Bytes() handed out a view of the live buffer")
	}

	s.Reset()
	if got := s.Bytes(); got != nil {
		t.Fatalf("Reset left %q behind", got)
	}
}

// TestSessionHostConfigIsRemovedAfterReading checks the hand-off file
// does not outlive its purpose: it carries the caller's whole
// environment, API keys included.
func TestSessionHostConfigIsRemovedAfterReading(t *testing.T) {
	isolateRuntimeDir(t)
	cfg := SessionHostConfig{
		ID:     5,
		Socket: filepath.Join(t.TempDir(), "5.sock"),
		Spec:   &SessionSpec{Env: map[string]string{"ANTHROPIC_API_KEY": "sk-secret"}},
	}
	path, err := writeSessionHostConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config holding an API key is mode %o, want 600", info.Mode().Perm())
	}

	got, err := readSessionHostConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 5 {
		t.Fatalf("config did not round trip: %+v", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the config file, which holds the caller's environment, was left on disk")
	}
}

// TestReadSessionHostConfigRejectsNonsense makes sure a supervisor
// refuses to start rather than binding a socket for a session it cannot
// identify.
func TestReadSessionHostConfigRejectsNonsense(t *testing.T) {
	if _, err := readSessionHostConfig(""); err == nil {
		t.Error("an empty config path must be refused")
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSessionHostConfig(bad); err == nil {
		t.Error("a config naming no session must be refused")
	}
}

var _ = context.Background
