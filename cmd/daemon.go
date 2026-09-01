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
clients. Local clients connect over a Unix socket with 'kit attach';
remote clients pair once and then connect over an end-to-end encrypted
iroh connection. Each client picks a working directory and gets its own
session: the session runs entirely on this machine, rendered inside the
peer's terminal. Multiple clients can hold sessions at the same time,
and exiting a session only disconnects that client.

Sessions survive a client disconnect, but NOT a restart of this daemon:
a session's terminal is owned by this process, so stopping the daemon
stops its sessions. They are shut down cleanly on SIGINT/SIGTERM, and
any that survive a hard crash are cleaned up on the next start.

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
		if s.Endpoint != "" {
			fmt.Printf("  Endpoint:         %s\n", s.Endpoint)
		}
		clients, _ := daemon.ListAuthorized()
		fmt.Printf("  Paired clients:   %d\n", len(clients))
		fmt.Printf("  Active sessions:  %d\n", s.SessionsActive)
		return nil
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
	daemonServiceCmd.AddCommand(daemonServiceInstallCmd)
	daemonServiceCmd.AddCommand(daemonServiceRemoveCmd)
	daemonCmd.AddCommand(daemonServiceCmd)
	rootCmd.AddCommand(daemonCmd)
}
