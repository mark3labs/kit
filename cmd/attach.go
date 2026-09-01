package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mark3labs/kit/internal/daemon"
	"github.com/mark3labs/kit/internal/ui"
)

// Flags for `kit attach`.
var (
	attachNew  bool
	attachHost string
	attachAll  bool
)

// sessionPickerFor bridges the daemon's session model to the TUI picker.
//
// internal/daemon must not import internal/ui: the daemon runs headless as
// a service, and the client is driven from here. This converter is the
// same pattern cmd/root.go uses for the extension widget providers.
func sessionPickerFor(entries []daemon.SessionEntry, input *os.File, title string) (daemon.SessionChoice, error) {
	view := make([]ui.SessionEntry, len(entries))
	for i, e := range entries {
		view[i] = ui.SessionEntry{
			ID:      e.ID,
			Clients: e.Clients,
			Started: e.Started,
			Cwd:     e.Cwd,
			Name:    e.Name,
			Host:    e.Host,
		}
	}
	pick, err := ui.RunSessionPicker(view, input, title)
	if err != nil {
		return daemon.SessionChoice{Cancel: true}, err
	}
	if pick.Cancelled {
		return daemon.SessionChoice{Cancel: true}, nil
	}
	if pick.Index < 0 {
		return daemon.SessionChoice{ID: 0}, nil // start a new session
	}
	chosen := entries[pick.Index]
	return daemon.SessionChoice{ID: chosen.ID, Host: chosen.Host}, nil
}

// localPicker is the single-daemon picker.
func localPicker(entries []daemon.SessionEntry, input *os.File) (daemon.SessionChoice, error) {
	return sessionPickerFor(entries, input, "Live sessions")
}

// hubPicker is the cross-host picker behind Ctrl-] w.
func hubPicker(entries []daemon.SessionEntry, input *os.File) (daemon.SessionChoice, error) {
	return sessionPickerFor(entries, input, "Sessions across all paired hosts")
}

var attachCmd = &cobra.Command{
	Use:   "attach [session-id]",
	Short: "Attach to a kit session on this machine or a paired host",
	Long: `Attach to a kit session.

With no arguments, lists the live sessions on the local daemon and lets
you pick one, or start a new one. A local daemon is started automatically
if none is running.

  kit attach                 # pick a session (or start one)
  kit attach 3               # attach straight to session 3
  kit attach --new           # skip the picker, start a new session
  kit attach --host homelab  # attach on a paired remote host
  kit attach --all           # list sessions across every paired host

Sessions keep running when you detach, so you can leave one working and
come back to it later — from this terminal or another machine.

Inside a session, Ctrl-] is the multiplexer prefix:

  Ctrl-] d    detach, leaving the session running
  Ctrl-] s    switch session
  Ctrl-] c    start a new session
  Ctrl-] n/p  next / previous session
  Ctrl-] w    switch across hosts
  Ctrl-] Ctrl-]  send a literal Ctrl-]`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		var target uint64
		if len(args) == 1 {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("session id must be a number: %q", args[0])
			}
			target = id
		}

		opts := daemon.AttachOptions{
			Pick:     localPicker,
			Target:   target,
			ForceNew: attachNew,
		}

		// The hub is only wired when there is more than one daemon to
		// choose between; otherwise Ctrl-] w has nothing to offer.
		if hosts, err := daemon.ListHosts(); err == nil && len(hosts) > 0 {
			opts.Hub = hubPicker
			opts.HubEntries = func() []daemon.SessionEntry {
				return remoteSessionEntries(hosts, attachHost)
			}
		}

		if attachAll {
			return runHubAttach(cmd, opts)
		}
		return runFollowingHostSwitches(ctx, attachHost, opts)
	},
}

// maxHostSwitches bounds one invocation's cross-host hops, so a session
// list that keeps pointing elsewhere cannot spin forever.
const maxHostSwitches = 16

// runFollowingHostSwitches attaches to a daemon and follows any cross-host
// switch the user makes from inside a session.
//
// A client speaks to exactly one daemon, so Ctrl-] w cannot be served on
// the current connection: it returns ErrSwitchHost and we dial the chosen
// host here. Session ids are per-daemon, so the id only means anything
// once we are talking to the daemon that issued it.
func runFollowingHostSwitches(ctx context.Context, host string, opts daemon.AttachOptions) error {
	for range maxHostSwitches {
		var err error
		if host == "" {
			err = daemon.RunLocal(ctx, opts)
		} else {
			err = daemon.RunHost(ctx, host, opts)
		}

		var switchTo *daemon.ErrSwitchHost
		if !errors.As(err, &switchTo) {
			return err
		}
		host = switchTo.Host
		opts.Target = switchTo.Session
		opts.ForceNew = false
	}
	return fmt.Errorf("too many host switches in one session (limit %d)", maxHostSwitches)
}

// runHubAttach opens the cross-host picker before connecting anywhere, so
// the first session can be on any paired machine.
func runHubAttach(cmd *cobra.Command, opts daemon.AttachOptions) error {
	ctx := cmd.Context()
	hosts, err := daemon.ListHosts()
	if err != nil {
		return err
	}

	entries, _ := daemon.ListLocalSessions() // a missing local daemon is fine
	entries = append(entries, remoteSessionEntries(hosts, "")...)
	if len(entries) == 0 {
		fmt.Println("No live sessions anywhere. Start one with: kit attach")
		return nil
	}

	choice, err := hubPicker(entries, os.Stdin)
	if err != nil || choice.Cancel {
		return err
	}
	opts.Target = choice.ID
	return runFollowingHostSwitches(ctx, choice.Host, opts)
}

// remoteSessionEntries collects sessions from every paired host, tagged
// with the host they came from.
//
// Hosts are queried one at a time and failures are skipped silently: a
// laptop that is asleep should not stop the picker from listing the hosts
// that did answer.
func remoteSessionEntries(hosts []daemon.HostEntry, skip string) []daemon.SessionEntry {
	var all []daemon.SessionEntry
	for _, h := range hosts {
		if h.Name == skip {
			continue
		}
		entries, err := daemon.ListHostSessions(h.Name, 8*time.Second)
		if err != nil {
			continue
		}
		for i := range entries {
			entries[i].Host = h.Name
		}
		all = append(all, entries...)
	}
	return all
}

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List live kit sessions",
	Long: `List the live sessions on the local daemon.

With --all, also queries every paired host. Hosts that do not answer are
skipped.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		entries, err := daemon.ListLocalSessions()
		if err != nil && err != daemon.ErrNoLocalDaemon {
			return err
		}
		if attachAll {
			if hosts, herr := daemon.ListHosts(); herr == nil {
				entries = append(entries, remoteSessionEntries(hosts, "")...)
			}
		}
		if len(entries) == 0 {
			fmt.Println("No live sessions. Start one with: kit attach")
			return nil
		}
		printSessionTable(entries)
		return nil
	},
}

// printSessionTable renders the session list for `kit ls`.
func printSessionTable(entries []daemon.SessionEntry) {
	fmt.Printf("%-6s %-10s %-10s %-10s %s\n", "ID", "HOST", "CLIENTS", "UPTIME", "DIRECTORY")
	for _, e := range entries {
		host := e.Host
		if host == "" {
			host = "local"
		}
		state := "detached"
		if e.Clients > 0 {
			state = strconv.Itoa(e.Clients)
		}
		name := e.Cwd
		if e.Name != "" {
			name = e.Name + "  " + e.Cwd
		}
		fmt.Printf("%-6d %-10s %-10s %-10s %s\n",
			e.ID, host, state,
			time.Since(e.Started).Round(time.Second),
			strings.TrimSpace(name))
	}
}

func init() {
	attachCmd.Flags().BoolVar(&attachNew, "new", false, "start a new session without showing the picker")
	attachCmd.Flags().StringVar(&attachHost, "host", "", "attach on a paired host by saved name")
	attachCmd.Flags().BoolVar(&attachAll, "all", false, "list sessions across every paired host")
	lsCmd.Flags().BoolVar(&attachAll, "all", false, "include sessions on every paired host")
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(lsCmd)
}
