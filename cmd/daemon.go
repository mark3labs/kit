package cmd

import (
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mark3labs/kit/internal/daemon"
)

var pairCode string

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run Kit as a remote daemon, hosting sessions for paired clients",
	Long: `Run Kit as a remote daemon.

Hosts sessions for clients on this machine and for paired remote
clients. With a daemon running, a plain 'kit' on this machine is a
detachable session: Ctrl-] d leaves it working, 'kit attach' brings it
back, and 'kit ls' lists them. Remote clients pair once and then connect
over an end-to-end encrypted iroh connection. Each client gets its own
session: the session runs entirely on this machine, rendered inside the
peer's terminal. Multiple clients can hold sessions at the same time,
and exiting a session only disconnects that client.

Sessions survive a client disconnect AND a restart of this daemon: each
runs in a supervisor process of its own, so stopping, restarting or
upgrading the daemon costs the connection and not the work. The next
daemon adopts them again, and a client that was attached reconnects on
its own. 'kit ls' lists them; 'kit attach <id>' picks one up.

Pair a new client with 'kit daemon pair' — it shows a one-time code and
asks you to accept or reject the client on this terminal. Only one
daemon may run per user; use 'kit daemon status' to inspect a running
instance and 'kit daemon service install' to manage it via systemd.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// Stop on SIGINT/SIGTERM so the daemon tears its sessions down
		// itself. Without this, `systemctl stop` kills the process
		// outright and the children are left to systemd's cgroup kill,
		// which reaches them mid-turn with no chance to save anything.
		//
		// The notifier is installed here rather than globally so the
		// interactive TUI keeps its own Ctrl-C handling.
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return daemon.Serve(ctx)
	},
}

var daemonPairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Pair a new client: show a one-time code and confirm on this terminal",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if pairList {
			return runPairList()
		}
		if pairRevoke != "" {
			removed, err := daemon.RevokeClient(pairRevoke)
			if err != nil {
				return err
			}
			fmt.Printf("Revoked client %s (paired since %s)\n",
				removed.FP, removed.AddedAt.Format("2006-01-02"))
			return nil
		}
		return daemon.RunPairWindow(cmd.Context(), daemon.PairWindowOptions{Code: pairCode})
	},
}

var (
	pairList   bool
	pairRevoke string
)

// runPairList prints the authorized clients table.
func runPairList() error {
	clients, err := daemon.ListAuthorized()
	if err != nil {
		return err
	}
	if len(clients) == 0 {
		fmt.Println("No paired clients. Pair one with: kit daemon pair")
		return nil
	}
	fmt.Printf("%-18s %-10s %s\n", "FINGERPRINT", "PAIRED", "LAST SEEN")
	for _, c := range clients {
		fmt.Printf("%-18s %-10s %s\n",
			c.FP,
			c.AddedAt.Format("2006-01-02"),
			c.LastSeen.Format("2006-01-02 15:04"))
	}
	return nil
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the state of a running daemon",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		st := daemon.ReadStatus()
		if !st.Running {
			fmt.Println("kit daemon is not running.")
			fmt.Println("Start one with: kit daemon  (or: kit daemon service install)")
			if st.State != nil {
				fmt.Printf("(stale state on disk from pid %d, started %s)\n",
					st.State.PID, st.State.StartedAt.Format("2006-01-02 15:04"))
			}
			return nil
		}
		s := st.State
		if s == nil {
			fmt.Println("kit daemon is running (pid unknown — state file not written yet)")
			return nil
		}
		uptime := time.Since(s.StartedAt).Round(time.Second)
		fmt.Printf("kit daemon is running (pid %d, up %s)\n", s.PID, uptime)
		if s.Build != "" {
			// The RUNNING daemon's build, which is not necessarily this
			// binary's: an upgrade replaces the file on disk and leaves the
			// daemon started from the old one running. Sessions keep
			// working across that skew — compatibility is the protocol
			// version, not the release — but a user wondering why a new
			// feature is missing needs to be able to see it.
			fmt.Printf("  Build:            %s\n", s.Build)
			if s.Build != daemon.BuildVersion() {
				fmt.Printf("                    (this kit is %s — restart the daemon to match)\n",
					daemon.BuildVersion())
			}
		}
		fmt.Printf("  Protocol:         %d\n", s.Protocol)
		if s.Endpoint != "" {
			fmt.Printf("  Endpoint:         %s\n", s.Endpoint)
		}
		clients, _ := daemon.ListAuthorized()
		fmt.Printf("  Paired clients:   %d\n", len(clients))
		fmt.Printf("  Active sessions:  %d\n", s.SessionsActive)
		if s.SessionsHosted > 0 {
			fmt.Printf("    surviving a restart: %d\n", s.SessionsHosted)
		}
		return nil
	},
}

var daemonSessionHostConfig string

// daemonSessionHostCmd is the supervisor process behind one hosted
// session. It is started by the daemon and never by a user, which is why
// it is hidden: running it by hand needs a config file the daemon writes.
//
// This is the process that makes a session outlive its daemon. It holds
// the PTY master and the kit child, and offers a socket any daemon may
// dial — so stopping, restarting or upgrading the daemon costs a socket
// and nothing else. See internal/daemon/sessionhost.go.
var daemonSessionHostCmd = &cobra.Command{
	Use:    "session-host",
	Short:  "Host one daemon session (internal)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// No signal.NotifyContext here. A supervisor must not stop because
		// something signalled the terminal or the process group the daemon
		// once belonged to; it stops when its child exits, or when a
		// daemon tells it to over the socket. RunSessionHost ignores
		// SIGHUP and SIGINT for the same reason.
		return daemon.RunSessionHost(cmd.Context(), daemonSessionHostConfig)
	},
}

var daemonServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the kit daemon systemd user service",
	Args:  cobra.NoArgs,
}

var daemonServiceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and start the systemd user service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return daemon.InstallSystemService(cmd.Context())
	},
}

var daemonServiceRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Stop and uninstall the systemd user service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return daemon.RemoveSystemService(cmd.Context())
	},
}

func init() {
	daemonPairCmd.Flags().StringVar(&pairCode, "code", "", "use a fixed pairing code instead of a random one (testing)")
	_ = daemonPairCmd.Flags().MarkHidden("code")
	daemonPairCmd.Flags().BoolVar(&pairList, "list", false, "list paired clients")
	daemonPairCmd.Flags().StringVar(&pairRevoke, "revoke", "", "revoke a paired client by fingerprint (or unique prefix)")

	daemonCmd.AddCommand(daemonPairCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonSessionHostCmd.Flags().StringVar(&daemonSessionHostConfig, "config", "",
		"path to the session host configuration written by the daemon")
	daemonCmd.AddCommand(daemonSessionHostCmd)
	daemonServiceCmd.AddCommand(daemonServiceInstallCmd)
	daemonServiceCmd.AddCommand(daemonServiceRemoveCmd)
	daemonCmd.AddCommand(daemonServiceCmd)
	rootCmd.AddCommand(daemonCmd)
}
