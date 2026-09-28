package daemon

import (
	"net"
	"slices"
	"strings"
	"testing"
)

// TestClientEnvCarriesTheWholeEnvironment is the regression test for
// hosted sessions that lost the user's environment. Every variable the
// user set must reach the session, not only the ones an allowlist named.
func TestClientEnvCarriesTheWholeEnvironment(t *testing.T) {
	env := clientEnv([]string{
		"PATH=/usr/bin",
		"ANTHROPIC_API_KEY=sk-test",
		"BRAVE_API_KEY=brave",
		"IN_NIX_SHELL=impure",
		"VIRTUAL_ENV=/home/user/.venv",
		"EMPTY=",
		"WITH_EQUALS=a=b",
	})

	want := map[string]string{
		"PATH":              "/usr/bin",
		"ANTHROPIC_API_KEY": "sk-test",
		"BRAVE_API_KEY":     "brave",
		"IN_NIX_SHELL":      "impure",
		"VIRTUAL_ENV":       "/home/user/.venv",
		"EMPTY":             "",
		"WITH_EQUALS":       "a=b",
	}
	for key, value := range want {
		got, ok := env[key]
		if !ok || got != value {
			t.Errorf("%s = %q (present %v), want %q", key, got, ok, value)
		}
	}
}

// TestClientEnvDropsShellLocalVariables keeps variables that describe the
// client's shell process out of a new process on a new PTY.
func TestClientEnvDropsShellLocalVariables(t *testing.T) {
	env := clientEnv([]string{"PWD=/a", "OLDPWD=/b", "SHLVL=3", "_=/usr/bin/kit", "SSH_TTY=/dev/pts/1", "KEEP=1"})
	for _, key := range shellLocalEnv {
		if _, ok := env[key]; ok {
			t.Errorf("%s was forwarded: it describes the client's shell, not the session", key)
		}
	}
	if env["KEEP"] != "1" {
		t.Error("an ordinary variable was dropped")
	}
}

// TestSpecBaseFullEnvReplacesTheDaemonEnvironment checks that a full
// environment is the session's environment: a variable only the daemon
// has (the user unset it, or never had it) does not leak in, and the
// keys the daemon owns keep the daemon's values.
func TestSpecBaseFullEnvReplacesTheDaemonEnvironment(t *testing.T) {
	daemonEnv := []string{
		"PATH=/daemon/bin",
		"STALE_TOKEN=from-install-time",
		"XDG_RUNTIME_DIR=/run/user/1000",
		sessionOwnerEnv + "=/real/daemon/home",
		"TMUX=/tmp/tmux-1000/default,1,0",
	}
	spec := &SessionSpec{FullEnv: true, Env: map[string]string{
		"PATH":            "/client/bin",
		"IN_NIX_SHELL":    "impure",
		"EMPTY":           "",
		sessionOwnerEnv:   "/attacker/home",
		"XDG_RUNTIME_DIR": "/elsewhere",
	}}

	base := specBase(daemonEnv, spec)

	check := func(key, want string, present bool) {
		t.Helper()
		got, ok := envValueIn(base, key)
		if ok != present || got != want {
			t.Errorf("%s = %q (present %v), want %q (present %v)", key, got, ok, want, present)
		}
	}
	check("PATH", "/client/bin", true)
	check("IN_NIX_SHELL", "impure", true)
	check("EMPTY", "", true)
	check("STALE_TOKEN", "", false)
	check("TMUX", "", false)
	check("XDG_RUNTIME_DIR", "/run/user/1000", true)
	check(sessionOwnerEnv, "/real/daemon/home", true)
}

// TestChildEnvFromFullSpecIsTheClientEnvironment runs the whole spawn
// path: a variable set only in the client reaches the child, a variable
// only the daemon has does not, and the daemon's per-session variables
// still win.
func TestChildEnvFromFullSpecIsTheClientEnvironment(t *testing.T) {
	t.Setenv("KIT_TEST_ONLY_IN_CLIENT", "yes")
	spec := SessionSpecForCommand("/tmp", nil)

	env := childEnv(specBase([]string{"DAEMON_ONLY=1"}, &spec),
		TerminalInfo{Term: "xterm-kitty"},
		map[string]string{sessionOwnerEnv: "/owner"})

	if v, _ := envValueIn(env, "KIT_TEST_ONLY_IN_CLIENT"); v != "yes" {
		t.Error("a client variable did not reach the session child")
	}
	if _, ok := envValueIn(env, "DAEMON_ONLY"); ok {
		t.Error("a variable only the daemon has leaked into the session child")
	}
	if v, _ := envValueIn(env, sessionOwnerEnv); v != "/owner" {
		t.Errorf("%s = %q, want the daemon's value", sessionOwnerEnv, v)
	}
	if v, _ := envValueIn(env, "TERM"); v != "xterm-kitty" {
		t.Errorf("TERM = %q, want the client's terminal", v)
	}
}

// TestInheritedSpecKeepsFullEnv makes a second session (Ctrl-] c) get the
// same complete environment as the first.
func TestInheritedSpecKeepsFullEnv(t *testing.T) {
	next := inheritedSpec(&SessionSpec{Cwd: "/p", FullEnv: true, Env: map[string]string{"A": "1"}})
	if next == nil || !next.FullEnv || next.Env["A"] != "1" {
		t.Fatalf("inheritedSpec = %+v, want the full environment kept", next)
	}
}

// hugeEnv builds an environment far larger than one frame.
func hugeEnv() map[string]string {
	huge := map[string]string{"PATH": "/usr/bin"}
	for i := range 40 {
		huge[string(rune('A'+i))+"_HUGE_VAR"] = strings.Repeat("x", 4096)
	}
	return huge
}

// TestSpecFramesSendsALargeSpecInParts is the fix for a large environment
// (a nix dev shell) that was dropped in full: a daemon that joins parts
// gets all of it.
func TestSpecFramesSendsALargeSpecInParts(t *testing.T) {
	spec := SessionSpec{Cwd: "/home/user/project", Args: []string{"-c"}, Env: hugeEnv(), FullEnv: true}

	frames, dropped, ok := specFrames(spec, true)
	if !ok || len(dropped) != 0 {
		t.Fatalf("specFrames ok=%v dropped=%v, want everything sent", ok, dropped)
	}
	if len(frames) < 2 {
		t.Fatalf("got %d frames, want the spec split in parts", len(frames))
	}

	var j specJoiner
	var joined []byte
	for i, f := range frames {
		if f.Type != FrameSessionSpecPart {
			t.Fatalf("frame %d type %#x, want FrameSessionSpecPart", i, f.Type)
		}
		if len(f.Payload) > maxPayload {
			t.Fatalf("frame %d is %d bytes, over the frame limit", i, len(f.Payload))
		}
		payload, done, err := j.add(f.Payload)
		if err != nil {
			t.Fatalf("part %d: %v", i, err)
		}
		if done != (i == len(frames)-1) {
			t.Fatalf("part %d: done = %v", i, done)
		}
		joined = payload
	}
	got, err := DecodeSessionSpec(joined)
	if err != nil {
		t.Fatal(err)
	}
	if !got.FullEnv || len(got.Env) != len(spec.Env) || got.Env["A_HUGE_VAR"] != spec.Env["A_HUGE_VAR"] {
		t.Fatalf("the joined spec lost its environment: %d of %d vars", len(got.Env), len(spec.Env))
	}
}

// TestSpecFramesTrimsTheLargestVariablesForAnOldDaemon keeps an old daemon
// working: the largest variables go, the rest and the directory stay, and
// the caller is told which ones went.
func TestSpecFramesTrimsTheLargestVariablesForAnOldDaemon(t *testing.T) {
	spec := SessionSpec{Cwd: "/home/user/project", Args: []string{"-c"}, Env: hugeEnv(), FullEnv: true}

	frames, dropped, ok := specFrames(spec, false)
	if !ok || len(frames) != 1 || frames[0].Type != FrameSessionSpec {
		t.Fatalf("ok=%v frames=%d, want one FrameSessionSpec", ok, len(frames))
	}
	if len(dropped) == 0 || slices.Contains(dropped, "PATH") {
		t.Fatalf("dropped = %v, want only large variables", dropped)
	}
	got, err := DecodeSessionSpec(frames[0].Payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cwd != spec.Cwd || !slices.Equal(got.Args, spec.Args) {
		t.Fatalf("trimming lost the directory or the arguments: %+v", got)
	}
	if got.Env["PATH"] != "/usr/bin" {
		t.Error("a small variable was dropped with the large ones")
	}
	if got.FullEnv {
		t.Error("a trimmed environment still claims to be complete")
	}
	if len(got.Env)+len(dropped) != len(spec.Env) {
		t.Errorf("kept %d + dropped %d != %d", len(got.Env), len(dropped), len(spec.Env))
	}
	if _, still := spec.Env[dropped[0]]; !still {
		t.Error("trimSpec changed the caller's map")
	}
}

func TestSpecFramesLeavesASmallSpecAlone(t *testing.T) {
	spec := SessionSpec{Cwd: "/tmp", Env: map[string]string{"PATH": "/usr/bin"}, FullEnv: true}
	frames, dropped, ok := specFrames(spec, true)
	if !ok || len(dropped) != 0 || len(frames) != 1 || frames[0].Type != FrameSessionSpec {
		t.Fatalf("a spec that fits was not sent as one frame: ok=%v dropped=%v frames=%d", ok, dropped, len(frames))
	}
}

// TestSpecJoinerBoundsItsBuffer keeps a client from making the daemon
// buffer without limit, and resynchronises at the last part.
func TestSpecJoinerBoundsItsBuffer(t *testing.T) {
	var j specJoiner
	chunk := append([]byte{specPartMore}, make([]byte, maxPayload-1)...)
	for range maxSpecSize/(maxPayload-1) + 2 {
		if _, _, err := j.add(chunk); err != nil {
			t.Fatalf("a part before the last one failed: %v", err)
		}
	}
	if len(j.buf) > maxSpecSize {
		t.Fatalf("joiner holds %d bytes, over the %d limit", len(j.buf), maxSpecSize)
	}
	if _, done, err := j.add([]byte{specPartLast}); err == nil || done {
		t.Fatal("an oversized spec was accepted")
	}

	payload, done, err := j.add(append([]byte{specPartLast}, `{"cwd":"/p"}`...))
	if err != nil || !done || string(payload) != `{"cwd":"/p"}` {
		t.Fatalf("the joiner did not recover after an oversized spec: %q %v %v", payload, done, err)
	}
	if _, _, err := j.add([]byte{7}); err == nil {
		t.Error("a bad marker was accepted")
	}
}

// TestFrameSourceJoinsSpecParts sends a large spec in parts through the
// daemon's frame loop and checks that the connection records all of it.
func TestFrameSourceJoinsSpecParts(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()
	defer func() { _ = clientConn.Close() }()

	table := newSessionTable(newDaemonRuntime(nil))
	wire := table.conns.addLocal(newFrameSink(serverConn))
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = table.runFrameSource(t.Context(), serverConn, wire.id)
	}()

	spec := SessionSpec{Cwd: "/p", Env: hugeEnv(), FullEnv: true}
	frames, _, ok := specFrames(spec, true)
	if !ok || len(frames) < 2 {
		t.Fatalf("want a spec in parts, got ok=%v frames=%d", ok, len(frames))
	}
	for _, f := range frames {
		if err := WriteFrame(clientConn, f.Type, 0, f.Payload); err != nil {
			t.Fatal(err)
		}
	}
	_ = clientConn.Close()
	<-done

	got := table.conns.consumeSpec(wire.id)
	if got == nil || !got.FullEnv || len(got.Env) != len(spec.Env) {
		t.Fatalf("recorded spec = %+v, want the full environment", got)
	}
}
