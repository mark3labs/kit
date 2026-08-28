package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/mark3labs/kit/internal/daemon"
)

var daemonCode string

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run Kit as a remote daemon, waiting for a pairing connection",
	Long: `Run Kit as a remote daemon.

Generates a short-lived pairing code and waits for a remote peer to
connect with "kit --remote CODE". On a verified connection the remote
peer picks a working directory (starting in this user's home directory)
and the session runs entirely on this machine, rendered inside the peer's
terminal.

After each session the pairing code rotates; press Ctrl+C to stop.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return daemon.Serve(ctx, daemon.ServeOptions{Code: daemonCode})
	},
}

func init() {
	daemonCmd.Flags().StringVar(&daemonCode, "code", "", "use a fixed pairing code instead of a random one (testing)")
	_ = daemonCmd.Flags().MarkHidden("code")
	rootCmd.AddCommand(daemonCmd)
}
