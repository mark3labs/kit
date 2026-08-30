package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mark3labs/kit/internal/daemon"
)

// Remote connection flags for `kit remote`.
var (
	remotePair     string
	remoteHost     string
	remoteList     bool
	remoteForget   string
	remotePairCode string // hidden: fixed code for tests
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Connect to a paired kit daemon and run a session on it",
	Long: `Connect to a kit daemon and run a session on the remote host.

First-time use pairs this machine with the host:

  kit remote --pair A1B2C3D4     # code shown by 'kit daemon pair' on the host

After pairing, reconnect by the name you saved — no code needed:

  kit remote --host homelab

The session runs entirely on the host; this terminal just renders it.
Ctrl-] detaches; /quit ends the session.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		switch {
		case remoteList:
			hosts, err := daemon.ListHosts()
			if err != nil {
				return err
			}
			if len(hosts) == 0 {
				fmt.Println("No paired hosts. Pair one with: kit remote --pair <code>")
				return nil
			}
			fmt.Printf("%-16s %-12s %s\n", "NAME", "PAIRED", "LAST USED")
			for _, h := range hosts {
				fmt.Printf("%-16s %-12s %s\n",
					h.Name,
					h.AddedAt.Format("2006-01-02"),
					h.LastUsed.Format("2006-01-02 15:04"))
			}
			return nil
		case remoteForget != "":
			return daemon.ForgetHost(remoteForget)
		case remotePair != "":
			code := remotePair
			if remotePairCode != "" {
				code = remotePairCode // hidden testing override
			}
			return daemon.RunPair(ctx, daemon.PairOptions{Code: code, Name: remoteHost})
		case remoteHost != "":
			return daemon.RunHost(ctx, remoteHost)
		default:
			return cmd.Help()
		}
	},
}

func init() {
	remoteCmd.Flags().StringVar(&remotePair, "pair", "", "pair with a host using the code from 'kit daemon pair'")
	remoteCmd.Flags().StringVar(&remoteHost, "host", "", "connect to a paired host by saved name")
	remoteCmd.Flags().BoolVar(&remoteList, "list", false, "list paired hosts")
	remoteCmd.Flags().StringVar(&remoteForget, "forget", "", "forget a paired host by name")
	remoteCmd.Flags().StringVar(&remotePairCode, "code", "", "use a fixed pairing code (testing)")
	_ = remoteCmd.Flags().MarkHidden("code")
	rootCmd.AddCommand(remoteCmd)
}
