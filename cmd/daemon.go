package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/mark3labs/kit/internal/daemon"
)

var daemonCode string

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run Kit as a remote daemon, waiting for a pairing connection",
	Long: `Run Kit as a remote daemon.

Generates a pairing code and waits for remote peers to connect with
"kit --remote CODE". Each verified peer picks a working directory
(starting in this user's home directory) and gets its own session:
the session runs entirely on this machine, rendered inside the peer's
terminal. Multiple clients can hold sessions at the same time, and
exiting a session only disconnects that client.

The pairing code stays valid while the daemon runs; press Ctrl+C to
stop. Only one daemon may run per user; use "kit daemon status" to
inspect a running instance and "kit daemon service install" to manage
it via systemd (user service).`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return daemon.Serve(ctx, daemon.ServeOptions{Code: daemonCode})
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the pairing code and state of a running daemon",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		st := daemon.ReadStatus()
		if !st.Running {
			fmt.Println("kit daemon is not running.")
			fmt.Println("Start one with: kit daemon  (or: kit daemon service install)")
			if st.State != nil {
				fmt.Printf("(stale state on disk from pid %d, started %s, code %s)\n",
					st.State.PID, st.State.StartedAt.Format("2006-01-02 15:04"), st.State.Code)
			}
			return nil
		}
		s := st.State
		if s == nil {
			fmt.Printf("kit daemon is running (pid unknown — state file not written yet)\n")
			return nil
		}
		uptime := time.Since(s.StartedAt).Round(time.Second)
		fmt.Printf("kit daemon is running (pid %d, up %s)\n", s.PID, uptime)
		fmt.Printf("  Pairing code:     %s\n", s.Code)
		fmt.Printf("  Connect with:     kit --remote %s\n", normalizeForHint(s.Code))
		if s.Endpoint != "" {
			fmt.Printf("  Endpoint:         %s\n", s.Endpoint)
		}
		fmt.Printf("  Active sessions:  %d\n", s.SessionsActive)
		return nil
	},
}

// normalizeForHint strips the display dash so the connect hint is
// copy-pasteable.
func normalizeForHint(displayCode string) string {
	out := make([]byte, 0, len(displayCode))
	for i := 0; i < len(displayCode); i++ {
		if displayCode[i] != '-' {
			out = append(out, displayCode[i])
		}
	}
	return string(out)
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
	RunE: func(_ *cobra.Command, _ []string) error {
		return daemon.InstallSystemService()
	},
}

var daemonServiceRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Stop and uninstall the systemd user service",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return daemon.RemoveSystemService()
	},
}

func init() {
	daemonCmd.Flags().StringVar(&daemonCode, "code", "", "use a fixed pairing code instead of a random one (testing)")
	_ = daemonCmd.Flags().MarkHidden("code")
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonServiceCmd.AddCommand(daemonServiceInstallCmd)
	daemonServiceCmd.AddCommand(daemonServiceRemoveCmd)
	daemonCmd.AddCommand(daemonServiceCmd)
	rootCmd.AddCommand(daemonCmd)
}
