package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// fakeSessionChild starts a long-running process that looks exactly like a
// session child: the session flag in its command line and a daemon-home
// marker in its environment.
//
// Two details matter. The marker has to survive in argv[0], and on systems
// where coreutils is a multi-call binary `sleep` refuses to run under any
// other name — so the process kept alive is bash, which ignores argv[0].
// The trailing ":" stops bash exec-optimising its last command away, which
// would replace argv[0] again. A helper that exits immediately would make
// these tests pass for the wrong reason, so the caller checks it is alive.
func fakeSessionChild(t *testing.T, owner string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("bash", "-c",
		`exec -a "kit `+pickDirFlagName+`" bash -c "sleep 30; :"`)
	cmd.Env = append(os.Environ(), sessionOwnerEnv+"="+owner)
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start a helper process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	// Give the shell time to exec.
	time.Sleep(300 * time.Millisecond)
	if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
		t.Skipf("the helper process did not stay running: %v", err)
	}
	return cmd
}

// newTestTable builds a session table whose registry and temp files live
// under t.TempDir(), so tests never touch a real daemon's state.
func newTestTable(t *testing.T) *sessionTable {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	return newSessionTable(newDaemonRuntime(nil))
}

// fakeSession registers a session with no child process, which is enough
// for the binding logic.
func (t *sessionTable) fakeSession(id uint64) *remoteSession {
	s := &remoteSession{
		id:      id,
		started: time.Now(),
		clients: make(map[uint32]winSize),
	}
	t.mu.Lock()
	t.sessions[id] = s
	if id > t.nextID {
		t.nextID = id
	}
	t.mu.Unlock()
	return s
}

func TestDetachWireKeepsTheSessionRunning(t *testing.T) {
	table := newTestTable(t)
	sess := table.fakeSession(1)

	const wire uint32 = 7
	table.mu.Lock()
	table.wireMap[wire] = 1
	table.mu.Unlock()
	sess.attachClient(wire, winSize{80, 24})

	table.detachWire(wire)

	if table.sessionCount() != 1 {
		t.Fatal("detaching a client must not end the session")
	}
	if len(sess.clientIDs()) != 0 {
		t.Fatal("the client should no longer be attached")
	}
	table.mu.Lock()
	_, bound := table.wireMap[wire]
	table.mu.Unlock()
	if bound {
		t.Fatal("the wire binding should be gone")
	}
}

func TestUnbindAllKeepsLocalClients(t *testing.T) {
	table := newTestTable(t)
	table.fakeSession(1)
	table.fakeSession(2)

	sink := newFrameSink(os.NewFile(0, os.DevNull))
	remote := table.conns.addRemote(3, sink)
	local := table.conns.addLocal(sink)

	table.mu.Lock()
	table.wireMap[remote.id] = 1
	table.wireMap[local.id] = 2
	table.mu.Unlock()

	// The tunnel died: its clients are gone, but local socket clients are
	// still connected and their sessions must be untouched.
	table.unbindAll()

	if table.conns.get(remote.id) != nil {
		t.Fatal("the sidecar connection should have been dropped")
	}
	if table.conns.get(local.id) == nil {
		t.Fatal("a local client must survive a tunnel restart")
	}
	if table.sessionCount() != 2 {
		t.Fatal("sessions must survive a tunnel restart")
	}
	table.mu.Lock()
	_, stillBound := table.wireMap[local.id]
	table.mu.Unlock()
	if !stillBound {
		t.Fatal("the local client's session binding should be intact")
	}
}

func TestAttachSessionRefusesAnUnknownSession(t *testing.T) {
	table := newTestTable(t)
	var buf lockedBuffer
	sink := newFrameSink(&buf)
	conn := table.conns.addLocal(sink)

	payload := make([]byte, 8)
	payload[7] = 99 // a session that was never created
	table.attachSession(conn.id, payload)

	frame, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("no ack was sent: %v", err)
	}
	if frame.Type != FrameSessionAttachAck {
		t.Fatalf("frame type = %#x, want an attach ack", frame.Type)
	}
	if frame.Payload[8] != 0 {
		t.Fatal("attaching to a missing session must be refused, not silently accepted")
	}
}

// TestSessionRegistryRoundTrip covers the crash-recovery record: what the
// daemon writes must be readable by the next daemon run.
func TestSessionRegistryRoundTrip(t *testing.T) {
	table := newTestTable(t)

	if got := readSessionRecords(); len(got) != 0 {
		t.Fatalf("a fresh runtime dir should hold no records, got %d", len(got))
	}

	// A session with no started process is not registered: there is no pid
	// to recover.
	table.fakeSession(1)
	table.syncSessionRegistry()
	if got := readSessionRecords(); len(got) != 0 {
		t.Fatalf("a session with no process should not be recorded, got %d", len(got))
	}

	// A record from another run is what a sweep looks for.
	want := []sessionRecord{{ID: 4, PID: 1234, Cwd: "/tmp", Started: time.Now(), Run: "old-run"}}
	if err := writeSessionRecords(want); err != nil {
		t.Fatalf("write records: %v", err)
	}
	got := readSessionRecords()
	if len(got) != 1 || got[0].ID != 4 || got[0].PID != 1234 || got[0].Run != "old-run" {
		t.Fatalf("record did not survive the round trip: %+v", got)
	}

	removeSessionRegistry()
	if got := readSessionRecords(); len(got) != 0 {
		t.Fatal("the registry should be gone after removal")
	}
}

func TestReadSessionRecordsToleratesCorruption(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	path, err := sessionsFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A corrupt registry must never stop a daemon starting; it is a
	// recovery hint, not a source of truth.
	if got := readSessionRecords(); got != nil {
		t.Fatalf("corrupt registry should yield no records, got %+v", got)
	}
}

// TestIsSessionChildRejectsForeignProcesses guards the sweep's targeting.
// Every case here is a process a sweep must NOT kill.
func TestIsSessionChildRejectsForeignProcesses(t *testing.T) {
	const owner = "/some/daemon/home"

	if isSessionChild(os.Getpid(), owner) {
		t.Fatal("the sweep must never target the daemon itself")
	}
	if isSessionChild(0, owner) || isSessionChild(-1, owner) {
		t.Fatal("invalid pids must be rejected")
	}

	// A live process that is not a session child at all.
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start a helper process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	if isSessionChild(cmd.Process.Pid, owner) {
		t.Fatal("a foreign process was identified as a session child; a sweep would kill it")
	}
}

// TestIsSessionChildRejectsAnotherDaemonsSession is the regression test for
// a sweep killing a live session belonging to a DIFFERENT daemon.
//
// Two daemons run at once whenever their runtime directories differ (a
// test daemon under its own XDG_CACHE_HOME beside a packaged one), so the
// single-instance lock does not make pids unambiguous. Their session
// children are indistinguishable by command line, which is why ownership
// is proved from the environment instead.
func TestIsSessionChildRejectsAnotherDaemonsSession(t *testing.T) {
	// A process that looks exactly like a session child — right flag,
	// alive, signalable — but was spawned by a daemon with a different
	// runtime directory.
	cmd := fakeSessionChild(t, "/other/daemon/home")

	if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
		t.Fatalf("the helper process is not running, so this proves nothing: %v", err)
	}
	if isSessionChild(cmd.Process.Pid, "/our/daemon/home") {
		t.Fatal("a sweep would kill a live session belonging to another daemon")
	}
	// The same process IS claimed by the daemon that owns it.
	if !isSessionChild(cmd.Process.Pid, "/other/daemon/home") {
		t.Skip("environment inspection is unavailable on this platform")
	}
}

// TestSweepOrphanSessionsSkipsTheCurrentRun makes sure a sweep never
// touches the sessions of the daemon doing the sweeping.
func TestSweepOrphanSessionsSkipsTheCurrentRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start a helper process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	const run = "this-run"
	if err := writeSessionRecords([]sessionRecord{
		{ID: 1, PID: cmd.Process.Pid, Started: time.Now(), Run: run},
	}); err != nil {
		t.Fatal(err)
	}

	sweepOrphanSessions(run)

	if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
		t.Fatal("the sweep killed a process belonging to the current run")
	}
}

// TestSweepSparesAnUnprovenProcess covers the case that actually bit: a
// registry naming a pid the daemon cannot prove it owns. The sweep must
// leave it alone rather than signal it.
func TestSweepSparesAnUnprovenProcess(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	cmd := fakeSessionChild(t, "/some/other/home")

	if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
		t.Fatalf("the helper process is not running, so this proves nothing: %v", err)
	}
	if err := writeSessionRecords([]sessionRecord{
		{ID: 1, PID: cmd.Process.Pid, Started: time.Now(), Run: "a-dead-run"},
	}); err != nil {
		t.Fatal(err)
	}

	sweepOrphanSessions("this-run")

	if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
		t.Fatal("the sweep killed a process it could not prove it owned")
	}
}

func TestTerminateProcessEndsAChild(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start a helper process: %v", err)
	}
	pid := cmd.Process.Pid
	done := make(chan struct{})
	go func() { _, _ = cmd.Process.Wait(); close(done) }()

	terminateProcess(pid)

	select {
	case <-done:
	case <-time.After(childGrace + 2*time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("terminateProcess did not end the child")
	}
}
