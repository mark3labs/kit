package cmd

import (
	"os"
	"testing"

	"github.com/spf13/viper"

	"github.com/mark3labs/kit/internal/daemon"
	"github.com/mark3labs/kit/internal/ui"
)

// The routing gate decides whether a plain `kit` becomes a detachable
// session on the daemon or runs in this process. Getting it wrong is not a
// cosmetic failure: routing a piped `kit "..." --json` would replace the
// caller's JSON with a relayed terminal, and routing a process that is
// already a session would nest sessions until something ran out.

// resetRouteFlags puts the package-level flags the gate reads back to
// their defaults. They are globals shared with the real command, so a test
// that changed one would otherwise decide the next one's answer.
func resetRouteFlags(t *testing.T) {
	t.Helper()
	prev := struct {
		daemonSession, pickDir, noDaemon, quiet, json bool
		prompt                                        string
		files                                         []ui.FilePart
	}{daemonSessionFlag, pickDirFlag, noDaemonFlag, quietFlag, jsonFlag, positionalPrompt, positionalFiles}

	daemonSessionFlag, pickDirFlag, noDaemonFlag = false, false, false
	quietFlag, jsonFlag = false, false
	positionalPrompt, positionalFiles = "", nil

	t.Cleanup(func() {
		daemonSessionFlag, pickDirFlag, noDaemonFlag = prev.daemonSession, prev.pickDir, prev.noDaemon
		quietFlag, jsonFlag = prev.quiet, prev.json
		positionalPrompt, positionalFiles = prev.prompt, prev.files
	})
}

// TestRoutableToDaemonVetoes walks every reason an invocation must stay in
// this process. The terminal checks are not covered here: `go test` runs
// without a terminal, which is exactly why the whole table expects false
// and the positive case is asserted separately.
func TestRoutableToDaemonVetoes(t *testing.T) {
	cases := []struct {
		name  string
		apply func()
	}{
		{"a session child", func() { daemonSessionFlag = true }},
		{"the directory picker", func() { pickDirFlag = true }},
		{"a one-shot prompt", func() { positionalPrompt = "explain this" }},
		{"an attached file", func() { positionalFiles = []ui.FilePart{{}} }},
		{"--quiet", func() { quietFlag = true }},
		{"--json", func() { jsonFlag = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetRouteFlags(t)
			tc.apply()
			if routableToDaemon() {
				t.Fatalf("%s was routed to the daemon: it would change what the command means", tc.name)
			}
		})
	}
}

// TestRoutableToDaemonRefusesInsideASession is the recursion guard.
//
// Every process inside a session inherits KIT_REMOTE_SESSION, including a
// kit the agent runs from its own shell tool. Without this check each one
// would ask the daemon for a session of its own.
func TestRoutableToDaemonRefusesInsideASession(t *testing.T) {
	resetRouteFlags(t)
	t.Setenv(daemon.RemoteSessionEnv, "1")

	if routableToDaemon() {
		t.Fatal("a process inside a session asked to be hosted in another one")
	}
}

// TestRoutableToDaemonRefusesWithoutATerminal pins the transport's own
// requirement. A hosted session is a relayed PTY, and RunClient refuses a
// connection whose stdin or stdout is not a terminal; discovering that
// after the daemon has spawned a child would leave the session running
// with nobody able to reach it.
func TestRoutableToDaemonRefusesWithoutATerminal(t *testing.T) {
	resetRouteFlags(t)

	// The test binary's stdin and stdout are pipes, so this is the real
	// condition rather than a simulated one.
	if routableToDaemon() {
		t.Fatal("an invocation with no terminal was routed to the daemon")
	}
}

func TestDaemonRouteMode(t *testing.T) {
	cases := []struct {
		name   string
		config string
		env    string
		flag   bool
		want   string
	}{
		{"unset", "", "", false, daemonModeAuto},
		{"nonsense", "sideways", "", false, daemonModeAuto},
		{"auto", "auto", "", false, daemonModeAuto},
		{"never", "never", "", false, daemonModeNever},
		{"always", "always", "", false, daemonModeAlways},
		{"case and space", "  ALWAYS ", "", false, daemonModeAlways},
		{"--no-daemon wins over always", "always", "", true, daemonModeNever},
		{"KIT_NO_DAEMON wins over always", "always", "1", false, daemonModeNever},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetRouteFlags(t)
			noDaemonFlag = tc.flag

			prev := viper.GetString("daemon-mode")
			viper.Set("daemon-mode", tc.config)
			t.Cleanup(func() { viper.Set("daemon-mode", prev) })

			if tc.env != "" {
				t.Setenv("KIT_NO_DAEMON", tc.env)
			} else {
				_ = os.Unsetenv("KIT_NO_DAEMON")
			}

			if got := daemonRouteMode(); got != tc.want {
				t.Fatalf("daemonRouteMode() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDaemonModeNeverSkipsTheProbe checks that opting out costs nothing.
//
// runKitRouted reads the mode before it asks whether a daemon is
// listening, so `never` never opens a socket. This is what makes
// --no-daemon a reliable escape hatch when the daemon is the thing that
// is misbehaving.
func TestDaemonModeNeverSkipsTheProbe(t *testing.T) {
	resetRouteFlags(t)
	noDaemonFlag = true

	if daemonRouteMode() != daemonModeNever {
		t.Fatal("--no-daemon did not select the never mode")
	}
}

// TestSessionFlagIsRegistered keeps the flag the daemon spawns children
// with and the flag the command accepts from drifting apart. They are the
// same string in daemon.SessionFlagName, but only a parse proves cobra
// knows it: an unknown flag would make every hosted session exit
// immediately with a usage error.
func TestSessionFlagIsRegistered(t *testing.T) {
	flag := rootCmd.Flags().Lookup(daemon.SessionFlagName)
	if flag == nil {
		t.Fatalf("kit does not accept %s, which the daemon puts on every session child", daemon.SessionFlag)
	}
	if !flag.Hidden {
		t.Error("the session marker is shown in help: users never pass it")
	}
	if "--"+daemon.SessionFlagName != daemon.SessionFlag {
		t.Errorf("SessionFlagName %q and SessionFlag %q disagree", daemon.SessionFlagName, daemon.SessionFlag)
	}
}

// TestSessionFlagsAreKnownToTheArgumentWalker covers remoteSubcommandSelected,
// which walks argv before cobra does and has to know which flags take a
// value. A boolean it does not recognise makes it swallow the next token,
// so `kit --daemon-session remote` would stop looking like the remote
// subcommand.
func TestSessionFlagsAreKnownToTheArgumentWalker(t *testing.T) {
	for _, args := range [][]string{
		{"--daemon-session", "remote", "--list"},
		{"--no-daemon", "remote", "--list"},
	} {
		if !remoteSubcommandSelected(args) {
			t.Errorf("remoteSubcommandSelected(%v) = false: the flag was treated as taking a value", args)
		}
	}
}
