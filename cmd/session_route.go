package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/mark3labs/kit/internal/daemon"
)

// Deciding whether a plain `kit` runs in this process or as a detachable
// session hosted by the local daemon.
//
// A hosted session survives the terminal it was started from: close the
// window, lose the ssh connection, or press Ctrl-] d, and the agent keeps
// working. Reattach with `kit attach`. Nothing about the session behaves
// differently — it runs the same arguments in the same directory with the
// same environment (see daemon.SessionSpec) — so hosting is the better
// default whenever a daemon is there to do it.
//
// Whenever it is NOT, kit runs exactly as it always has. That fallback is
// the point: a daemon is an optimisation, never a requirement, and no
// invocation may fail because one is missing or unwell.

// Daemon modes, settable as `daemon-mode` in the config file.
const (
	// daemonModeAuto hosts a session when a daemon is already running and
	// runs in-process when none is. The default.
	daemonModeAuto = "auto"
	// daemonModeNever always runs in-process.
	daemonModeNever = "never"
	// daemonModeAlways starts a daemon on demand to host the session.
	daemonModeAlways = "always"
)

// daemonProbeTimeout bounds the "is a daemon there?" dial.
//
// The answer is on a Unix socket on this machine, so a healthy daemon
// answers in well under a millisecond. The timeout exists for the
// unhealthy one: a daemon that has stopped accepting must cost a startup
// hiccup, not a hang, because the fallback — running in this process — is
// a perfectly good outcome.
const daemonProbeTimeout = 300 * time.Millisecond

// daemonRouteMode reports the configured daemon mode, defaulting to auto.
//
// KIT_NO_DAEMON is honoured as a synonym for "never" so a session, a
// script, or a test harness can turn hosting off for everything it
// spawns without editing a config file.
func daemonRouteMode() string {
	if os.Getenv("KIT_NO_DAEMON") != "" {
		return daemonModeNever
	}
	if noDaemonFlag {
		return daemonModeNever
	}
	switch strings.ToLower(strings.TrimSpace(viper.GetString("daemon-mode"))) {
	case daemonModeNever:
		return daemonModeNever
	case daemonModeAlways:
		return daemonModeAlways
	default:
		return daemonModeAuto
	}
}

// routableToDaemon reports whether this invocation is the kind that COULD
// be hosted, before asking whether a daemon is there to host it.
//
// Everything here is a veto, and each one marks a case where a hosted
// session would change the command's meaning rather than just where it
// runs:
//
//   - We are already the session. --daemon-session and --pick-dir are
//     what the daemon spawns a child with, and KIT_REMOTE_SESSION marks
//     every process inside one — including a kit the agent itself runs
//     from the shell tool. Hosting any of them would nest sessions until
//     something ran out.
//   - There is no terminal to hand over. A hosted session is a relayed
//     PTY; `kit "..." | jq` and `echo x | kit` have nothing to relay, and
//     RunClient refuses them outright.
//   - The command is not interactive. A one-shot prompt, --quiet and
//     --json all produce output for a caller that is waiting for it, and
//     a session the user is meant to detach from is the wrong shape for
//     that entirely.
func routableToDaemon() bool {
	switch {
	case daemonSessionFlag || pickDirFlag:
		return false
	case os.Getenv(daemon.RemoteSessionEnv) != "":
		return false
	case positionalPrompt != "" || len(positionalFiles) > 0:
		return false
	case suppressChrome():
		return false
	case !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())):
		return false
	}
	return true
}

// localDaemonAnswers reports whether a daemon is listening right now.
//
// The dial is the whole test: ReadStatus only proves that something holds
// the lock, which a daemon stuck before it binds its socket also does, and
// routing to it would hang the command instead of starting a session.
func localDaemonAnswers(ctx context.Context) bool {
	probe, cancel := context.WithTimeout(ctx, daemonProbeTimeout)
	defer cancel()
	conn, err := daemon.DialLocal(probe)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// runHostedSession hands this invocation to the local daemon and attaches
// to it, returning when the user detaches or the session ends.
//
// ForceNew, not the picker: `cd ~/project && kit` asks for a session here,
// not for a list of the sessions running elsewhere. The picker stays on
// `kit attach`, which is the command that exists to ask for one — but it
// is still supplied, because ForceNew only decides the FIRST session and
// Ctrl-] s must keep working once the session is up.
//
// The attach runs through runFollowingHostSwitches for the same reason:
// everything a session started with `kit attach` can do from the keyboard,
// including switching to a paired host with Ctrl-] w, a session started
// this way can do too.
func runHostedSession(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve the working directory: %w", err)
	}
	spec := daemon.SessionSpecForCommand(cwd, os.Args[1:])

	return runFollowingHostSwitches(ctx, "", daemon.AttachOptions{
		Pick:     localPicker,
		ForceNew: true,
		Spec:     &spec,
	})
}

// runKitRouted is the root command's body: a detachable session when the
// daemon can host one, and the in-process agent when it cannot.
func runKitRouted(ctx context.Context) error {
	mode := daemonRouteMode()
	if mode == daemonModeNever || !routableToDaemon() {
		return runKit(ctx)
	}

	switch mode {
	case daemonModeAlways:
		// RunLocal starts a daemon when none is listening, which is what
		// this mode asks for. A failure to start one is still not a
		// reason to fail the command: the user wanted kit, and kit runs
		// here perfectly well.
		if err := runHostedSession(ctx); err != nil {
			if !isDaemonUnavailable(err) {
				return err
			}
			fmt.Fprintf(os.Stderr, "Could not host this session on a daemon (%v); running in this terminal.\n", err)
			return runKit(ctx)
		}
		return nil

	default: // auto
		if !localDaemonAnswers(ctx) {
			return runKit(ctx)
		}
		// The daemon answered a moment ago; if it has gone since, fall
		// back rather than reporting a failure the user did not ask for.
		if err := runHostedSession(ctx); err != nil {
			if isDaemonUnavailable(err) {
				return runKit(ctx)
			}
			return err
		}
		return nil
	}
}

// isDaemonUnavailable reports an error that means "no daemon", as opposed
// to one from a session that started and then went wrong. Only the former
// may be answered by quietly running in this process.
func isDaemonUnavailable(err error) bool {
	return errors.Is(err, daemon.ErrNoLocalDaemon)
}
