package daemon

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/clipboard"
	"github.com/mark3labs/kit/internal/ui/termgfx"
)

// envValueIn reads a key out of an environment slice.
func envValueIn(env []string, key string) (string, bool) {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v, true
		}
	}
	return "", false
}

func TestSessionSpecRoundTrip(t *testing.T) {
	want := SessionSpec{
		Cwd:  "/home/user/project",
		Args: []string{"-m", "anthropic/claude-sonnet-4-5", "--resume"},
		Env:  map[string]string{"PATH": "/usr/bin", "ANTHROPIC_API_KEY": "sk-test"},
	}

	payload, err := EncodeSessionSpec(want)
	if err != nil {
		t.Fatalf("EncodeSessionSpec: %v", err)
	}
	got, err := DecodeSessionSpec(payload)
	if err != nil {
		t.Fatalf("DecodeSessionSpec: %v", err)
	}
	if got.Cwd != want.Cwd || !slices.Equal(got.Args, want.Args) {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if got.Env["ANTHROPIC_API_KEY"] != "sk-test" {
		t.Fatalf("env did not survive the round trip: %+v", got.Env)
	}
}

// TestForwardableEnvCarriesCredentialsNotTheWholeEnvironment pins the
// allowlist. A session runs in a process the user cannot see, for as long
// as the daemon lives, so the environment it is given is chosen rather
// than copied.
func TestForwardableEnvCarriesCredentialsNotTheWholeEnvironment(t *testing.T) {
	env := forwardableEnv([]string{
		"PATH=/usr/bin",
		"ANTHROPIC_API_KEY=sk-test",
		"OPENAI_BASE_URL=http://localhost:1234",
		"LC_ALL=en_GB.UTF-8",
		"KIT_SOMETHING=on",
		"SECRET_COMPANY_TOKEN=nope",
		"RANDOM_DESKTOP_THING=nope",
		"EMPTY=",
	})

	for _, key := range []string{"PATH", "ANTHROPIC_API_KEY", "OPENAI_BASE_URL", "LC_ALL", "KIT_SOMETHING"} {
		if _, ok := env[key]; !ok {
			t.Errorf("%s was dropped: a session needs it to behave like a shell-launched kit", key)
		}
	}
	for _, key := range []string{"SECRET_COMPANY_TOKEN", "RANDOM_DESKTOP_THING", "EMPTY"} {
		if _, ok := env[key]; ok {
			t.Errorf("%s was forwarded: the allowlist must not carry the whole environment", key)
		}
	}
}

// TestForwardableEnvNeverCarriesDaemonOwnedKeys is the security half of
// the allowlist. KIT_ is forwarded by prefix, and the daemon's own
// per-session variables all start with it — including the ownership
// marker the crash sweep trusts to decide which processes it may kill.
func TestForwardableEnvNeverCarriesDaemonOwnedKeys(t *testing.T) {
	reserved := []string{
		RemoteSessionEnv,
		RemoteBackgroundEnv,
		sessionOwnerEnv,
		sessionCwdEnv,
		clipboard.RemoteClipboardEnv,
		termgfx.RemoteMultiplexerEnv,
		"TERM",
		"COLORTERM",
		"XDG_RUNTIME_DIR",
		"XDG_CACHE_HOME",
		"TMUX",
	}

	var environ []string
	for _, key := range reserved {
		environ = append(environ, key+"=hijacked")
	}
	env := forwardableEnv(environ)

	for _, key := range reserved {
		if _, ok := env[key]; ok {
			t.Errorf("%s was forwarded: it belongs to the daemon, not to the client", key)
		}
	}
}

// TestSpecBaseIgnoresReservedKeysOnTheWayIn is the same rule enforced on
// arrival. The client filters, but the client is the untrusted side: a
// spec is accepted from anything that can open the local socket.
func TestSpecBaseIgnoresReservedKeysOnTheWayIn(t *testing.T) {
	daemonEnv := []string{"PATH=/usr/bin", sessionOwnerEnv + "=/real/daemon/home"}
	spec := &SessionSpec{Env: map[string]string{
		sessionOwnerEnv: "/attacker/home",
		"PATH":          "/opt/bin",
	}}

	base := specBase(daemonEnv, spec)

	if got, _ := envValueIn(base, sessionOwnerEnv); got != "/real/daemon/home" {
		t.Fatalf("%s = %q, want the daemon's own value kept", sessionOwnerEnv, got)
	}
	if got, _ := envValueIn(base, "PATH"); got != "/opt/bin" {
		t.Fatalf("PATH = %q, want the spec's value: a session must find the user's programs", got)
	}
}

func TestSpecBaseDoesNotDuplicateKeys(t *testing.T) {
	base := specBase([]string{"PATH=/usr/bin", "HOME=/root"},
		&SessionSpec{Env: map[string]string{"PATH": "/opt/bin"}})

	seen := map[string]int{}
	for _, kv := range base {
		if k, _, ok := strings.Cut(kv, "="); ok {
			seen[k]++
		}
	}
	if seen["PATH"] != 1 {
		t.Fatalf("PATH appears %d times: a duplicated key leaves the value to the reader", seen["PATH"])
	}
}

// TestSpecCommandAlwaysCarriesTheSweepMarker is what keeps a crashed
// daemon's sessions collectable. A child running the user's own arguments
// looks like any other kit process from the outside; this flag is the only
// thing that tells them apart.
func TestSpecCommandAlwaysCarriesTheSweepMarker(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]*SessionSpec{
		"no spec":             nil,
		"spec with args":      {Cwd: dir, Args: []string{"-m", "openai/gpt-5"}},
		"spec with no args":   {Cwd: dir},
		"spec with a bad cwd": {Cwd: "/nope/gone"},
	}
	for name, spec := range cases {
		_, args := specCommand(spec)
		if len(args) == 0 || args[0] != SessionFlag {
			t.Errorf("%s: args = %v, want %s first", name, args, SessionFlag)
		}
		if !isSessionChildCmdline("kit " + strings.Join(args, " ")) {
			t.Errorf("%s: the sweep would not recognise %v as a session child", name, args)
		}
	}
}

// TestSpecCommandWithNoArgumentsStillSkipsThePicker is the regression this
// whole design exists for.
//
// A bare `kit` in a project directory sends a spec with a directory and an
// EMPTY argument list. Reading that as "no spec" put the user in front of
// a directory picker rooted in their home directory, having just told kit
// exactly where they were.
func TestSpecCommandWithNoArgumentsStillSkipsThePicker(t *testing.T) {
	project := t.TempDir()

	dir, args := specCommand(&SessionSpec{Cwd: project})

	if dir != project {
		t.Fatalf("dir = %q, want the directory the user ran kit in (%q)", dir, project)
	}
	if slices.Contains(args, pickDirFlagName) {
		t.Fatalf("args = %v, want no picker: the directory is already known", args)
	}
}

// TestSpecCommandFallsBackToThePicker covers a client run in a directory
// this daemon cannot see. The session still starts; it starts somewhere
// that exists, and offers the picker rather than pretending.
func TestSpecCommandFallsBackToThePicker(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "deleted-since")
	for name, spec := range map[string]*SessionSpec{
		"no spec":       nil,
		"empty cwd":     {},
		"missing cwd":   {Cwd: missing},
		"cwd is a file": {Cwd: writeTempFile(t)},
	} {
		dir, args := specCommand(spec)
		if dir != homeDir() {
			t.Errorf("%s: dir = %q, want the home directory", name, dir)
		}
		if !slices.Contains(args, pickDirFlagName) {
			t.Errorf("%s: args = %v, want the directory picker", name, args)
		}
	}
}

// TestSpecCommandRootsThePickerWhereTheClientIs covers `kit attach`: the
// user chose the machine, not the directory, so the picker stays — but it
// opens where they are standing rather than in their home directory.
func TestSpecCommandRootsThePickerWhereTheClientIs(t *testing.T) {
	project := t.TempDir()

	dir, args := specCommand(&SessionSpec{Cwd: project, Pick: true})

	if dir != project {
		t.Fatalf("dir = %q, want the picker rooted at the client's directory (%q)", dir, project)
	}
	if !slices.Contains(args, pickDirFlagName) {
		t.Fatalf("args = %v, want the picker: the client did not name a directory", args)
	}
}

// TestSpecCommandPassesTheUsersArguments checks the other half: a hosted
// session must be the same command the user typed, not a generic one.
func TestSpecCommandPassesTheUsersArguments(t *testing.T) {
	project := t.TempDir()
	want := []string{"-m", "openai/gpt-5", "--resume"}

	dir, args := specCommand(&SessionSpec{Cwd: project, Args: want})

	if dir != project {
		t.Fatalf("dir = %q, want %q", dir, project)
	}
	if !slices.Equal(args, append([]string{SessionFlag}, want...)) {
		t.Fatalf("args = %v, want the marker followed by the user's arguments", args)
	}
}

func writeTempFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// TestInheritedSpecKeepsTheDirectoryAndDropsTheArguments pins what a
// second session on the same connection gets.
//
// Ctrl-] c means "another session like this one", and the directory is
// the part that makes it so. The arguments are not: replaying --resume
// would answer the request with a picker, and replaying --continue would
// reopen a conversation the user had already moved on from.
func TestInheritedSpecKeepsTheDirectoryAndDropsTheArguments(t *testing.T) {
	spec := &SessionSpec{
		Cwd:  "/home/user/project",
		Args: []string{"--resume"},
		Env:  map[string]string{"PATH": "/usr/bin"},
	}

	next := inheritedSpec(spec)

	if next == nil {
		t.Fatal("inheritedSpec = nil, want the directory kept for the next session")
	}
	if next.Cwd != spec.Cwd {
		t.Errorf("Cwd = %q, want a new session in the same directory", next.Cwd)
	}
	if len(next.Args) != 0 {
		t.Errorf("Args = %v, want none: one invocation's arguments describe one session", next.Args)
	}
	if next.Env["PATH"] != "/usr/bin" {
		t.Errorf("Env was dropped: the next session needs the same credentials")
	}
	if inheritedSpec(nil) != nil || inheritedSpec(&SessionSpec{Args: []string{"-c"}}) != nil {
		t.Error("a spec with nothing worth keeping must inherit as nil")
	}
}

// TestInheritedSpecKeepsTheChoiceOfPicker keeps Ctrl-] c answering the
// same way the client's first session did.
//
// A routed `kit` named its directory, so a second session starts there
// too. A `kit attach` client did not, so a second session is still
// offered the picker — rooted where that client is, not in home.
func TestInheritedSpecKeepsTheChoiceOfPicker(t *testing.T) {
	routed := inheritedSpec(&SessionSpec{Cwd: "/p", Args: []string{"-c"}})
	if routed == nil || routed.Pick {
		t.Errorf("a routed kit inherited Pick = %+v, want a session straight in the directory", routed)
	}

	attached := inheritedSpec(&SessionSpec{Cwd: "/p", Pick: true})
	if attached == nil || !attached.Pick {
		t.Errorf("kit attach inherited Pick = %+v, want the picker kept", attached)
	}
}

// TestSpecFitsTrimsTheEnvironmentBeforeGivingUp keeps an oversized
// environment from costing the working directory. A session in the wrong
// directory is a silent error; a session without forwarded credentials
// merely falls back on the daemon's own.
func TestSpecFitsTrimsTheEnvironmentBeforeGivingUp(t *testing.T) {
	huge := map[string]string{}
	for i := range 40 {
		huge[string(rune('A'+i))+"_HUGE_VAR"] = strings.Repeat("x", 4096)
	}
	spec := SessionSpec{Cwd: "/home/user/project", Args: []string{"-c"}, Env: huge}

	trimmed, ok := specFits(spec)

	if !ok {
		t.Fatal("specFits gave up on a spec whose arguments and directory fit easily")
	}
	if trimmed.Cwd != spec.Cwd || !slices.Equal(trimmed.Args, spec.Args) {
		t.Fatalf("trimming lost the directory or the arguments: %+v", trimmed)
	}
	if trimmed.Env != nil {
		t.Fatal("the environment was kept despite overflowing the frame")
	}
	payload, err := EncodeSessionSpec(trimmed)
	if err != nil || len(payload) > maxPayload {
		t.Fatalf("trimmed spec is %d bytes (err %v), want at most %d", len(payload), err, maxPayload)
	}
}

func TestSpecFitsLeavesASmallSpecAlone(t *testing.T) {
	spec := SessionSpec{Cwd: "/tmp", Env: map[string]string{"PATH": "/usr/bin"}}
	got, ok := specFits(spec)
	if !ok || got.Env["PATH"] != "/usr/bin" {
		t.Fatalf("specFits trimmed a spec that fits: %+v (ok %v)", got, ok)
	}
}

// TestSetSpecRefusesRemoteClients is the boundary that keeps pairing from
// meaning arbitrary execution. A paired peer may ask for a session; it may
// not say what that session runs, in which directory, with what
// environment.
func TestSetSpecRefusesRemoteClients(t *testing.T) {
	conns := newConnSet()
	remote := conns.addRemote(1, nil)

	if conns.setSpec(remote.id, SessionSpec{Args: []string{"--shell", "/bin/evil"}}) {
		t.Fatal("a remote client's session spec was accepted")
	}
	if got := conns.consumeSpec(remote.id); got != nil {
		t.Fatalf("consumeSpec = %+v, want nil for a remote client", got)
	}
}

func TestSetSpecAcceptsLocalClients(t *testing.T) {
	conns := newConnSet()
	local := conns.addLocal(nil)

	if !conns.setSpec(local.id, SessionSpec{Cwd: "/home/user/project", Args: []string{"-c"}}) {
		t.Fatal("a local client's session spec was refused")
	}
	got := conns.consumeSpec(local.id)
	if got == nil || got.Cwd != "/home/user/project" || !slices.Equal(got.Args, []string{"-c"}) {
		t.Fatalf("consumeSpec = %+v, want the spec the client sent", got)
	}
}

// TestConsumeSpecSpendsTheArgumentsOnce checks the hand-over between the
// first session on a connection and the ones started from inside it.
func TestConsumeSpecSpendsTheArgumentsOnce(t *testing.T) {
	conns := newConnSet()
	local := conns.addLocal(nil)
	conns.setSpec(local.id, SessionSpec{Cwd: "/home/user/project", Args: []string{"--resume"}})

	first := conns.consumeSpec(local.id)
	if first == nil || len(first.Args) != 1 {
		t.Fatalf("first consume = %+v, want the client's arguments", first)
	}

	second := conns.consumeSpec(local.id)
	if second == nil {
		t.Fatal("second consume = nil, want the directory inherited")
	}
	if second.Cwd != "/home/user/project" {
		t.Errorf("second Cwd = %q, want the same directory", second.Cwd)
	}
	if len(second.Args) != 0 {
		t.Errorf("second Args = %v, want none: the arguments were spent on the first session", second.Args)
	}
}

// TestSpecIsForgottenWithTheConnection mirrors the terminal's lifetime:
// the spec describes the terminal a client is sitting at, so it must not
// outlive it and be handed to whoever inherits the wire id.
func TestSpecIsForgottenWithTheConnection(t *testing.T) {
	conns := newConnSet()
	local := conns.addLocal(nil)
	conns.setSpec(local.id, SessionSpec{Cwd: "/home/user/project"})

	conns.remove(local.id)

	if got := conns.consumeSpec(local.id); got != nil {
		t.Fatalf("consumeSpec after the client left = %+v, want nil", got)
	}
}

// TestSessionChildCmdlineStillMatchesTheOldMarker covers a daemon
// upgraded in place. The children of its predecessor carry only
// --pick-dir, and refusing to recognise them would leak a session per
// restart, unreachable and unkillable except by hand.
func TestSessionChildCmdlineStillMatchesTheOldMarker(t *testing.T) {
	cases := map[string]bool{
		"kit --daemon-session -m openai/gpt-5": true,
		"kit --daemon-session --pick-dir":      true,
		"kit --pick-dir":                       true,
		"kit -m openai/gpt-5":                  false,
		"vim --pick-dir-notes.txt":             true, // ownership is proved separately
		"some other program":                   false,
	}
	for cmdline, want := range cases {
		if got := isSessionChildCmdline(cmdline); got != want {
			t.Errorf("isSessionChildCmdline(%q) = %v, want %v", cmdline, got, want)
		}
	}
}
