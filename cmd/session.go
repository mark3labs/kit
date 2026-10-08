package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mark3labs/kit/internal/daemon"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var sessionCmd = &cobra.Command{Use: "session", Short: "Manage detachable sessions"}
var sessionHost string
var sessionYes bool

func parseSessionID(s string) (uint64, error) {
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("session id must be a number: %q", s)
	}
	if id == 0 {
		return 0, fmt.Errorf("session ID must be greater than zero")
	}
	return id, nil
}

var sessionRenameCmd = &cobra.Command{
	Use: "rename <session-id> <name>", Short: "Set a session display name", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseSessionID(args[0])
		if err != nil {
			return err
		}
		name := strings.TrimSpace(args[1])
		if name == "" {
			return fmt.Errorf("session name cannot be empty")
		}
		if sessionHost != "" {
			err = daemon.RenameHostSession(cmd.Context(), sessionHost, id, name)
		} else {
			err = daemon.RenameSession(cmd.Context(), id, name)
		}
		if err != nil {
			return err
		}
		fmt.Printf("Renamed session %d to %q.\n", id, name)
		return nil
	},
}
var sessionKillCmd = &cobra.Command{
	Use: "kill <session-id>", Short: "Stop a session", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseSessionID(args[0])
		if err != nil {
			return err
		}
		if !sessionYes {
			if input, ok := cmd.InOrStdin().(*os.File); ok && !term.IsTerminal(int(input.Fd())) {
				return fmt.Errorf("confirmation needs a terminal; use --yes for scripts")
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Stop session %d", id)
			if sessionHost != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), " on %s", sessionHost)
			}
			fmt.Fprint(cmd.ErrOrStderr(), "? [y/N] ")
			answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			if err != nil {
				return fmt.Errorf("read confirmation: %w", err)
			}
			answer = strings.TrimSpace(answer)
			if answer != "y" && answer != "Y" && answer != "yes" && answer != "YES" {
				return nil
			}
		}
		if sessionHost != "" {
			err = daemon.KillHostSession(cmd.Context(), sessionHost, id)
		} else {
			err = daemon.KillSession(cmd.Context(), id)
		}
		if err != nil {
			return err
		}
		fmt.Printf("Stopped session %d.\n", id)
		return nil
	},
}

func init() {
	sessionRenameCmd.Flags().StringVar(&sessionHost, "host", "", "control a session on a paired host")
	sessionKillCmd.Flags().StringVar(&sessionHost, "host", "", "control a session on a paired host")
	sessionKillCmd.Flags().BoolVarP(&sessionYes, "yes", "y", false, "do not ask for confirmation")
	sessionCmd.AddCommand(sessionRenameCmd, sessionKillCmd)
}
