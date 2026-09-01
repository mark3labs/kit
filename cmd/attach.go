package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
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
func sessionPickerFor(ctx context.Context, entries []daemon.SessionEntry, input *os.File, title string) (daemon.SessionChoice, error) {
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
	pick, err := ui.RunSessionPicker(ctx, view, input, title)
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
func localPicker(ctx context.Context, entries []daemon.SessionEntry, input *os.File) (daemon.SessionChoice, error) {
	return sessionPickerFor(ctx, entries, input, "Live sessions")
}

// hubPicker is the cross-host picker behind Ctrl-] w.
func hubPicker(ctx context.Context, entries []daemon.SessionEntry, input *os.File) (daemon.SessionChoice, error) {
	return sessionPickerFor(ctx, entries, input, "Sessions across all paired hosts")
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
	hosts, _ := daemon.ListHosts()

	for range maxHostSwitches {
		// The hub lists every daemon EXCEPT the one this client is talking
		// to, whose sessions the client already knows. That set changes on
		// every hop, so it is rebuilt here rather than once up front:
		// wired to the starting host, Ctrl-] w would list the current host
		// twice after a switch and never offer a way back to this machine.
		current := host
		if len(hosts) > 0 {
			opts.Hub = hubPicker
			opts.HubEntries = func(ctx context.Context) []daemon.SessionEntry {
				return otherDaemonSessions(ctx, hosts, current)
			}
		} else {
			opts.Hub = nil
			opts.HubEntries = nil
		}

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

// otherDaemonSessions collects the sessions the current client cannot
// reach itself: every paired host except the one it is attached to, plus
// this machine's own daemon when the client is attached elsewhere.
//
// Local sessions carry an empty host, which is exactly what the client
// reports for the local daemon, so choosing one is recognised as a switch
// back to this machine.
func otherDaemonSessions(ctx context.Context, hosts []daemon.HostEntry, current string) []daemon.SessionEntry {
	var all []daemon.SessionEntry
	if current != "" {
		if local, err := daemon.ListLocalSessions(ctx); err == nil {
			all = append(all, local...)
		}
	}
	entries, _ := remoteSessionEntries(ctx, hosts, current)
	return append(all, entries...)
}

// runHubAttach opens the cross-host picker before connecting anywhere, so
// the first session can be on any paired machine.
func runHubAttach(cmd *cobra.Command, opts daemon.AttachOptions) error {
	ctx := cmd.Context()
	hosts, err := daemon.ListHosts()
	if err != nil {
		return err
	}

	entries, _ := daemon.ListLocalSessions(ctx) // a missing local daemon is fine
	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}
	remote, skipped := remoteSessionEntries(ctx, hosts, "")
	if cerr := ctx.Err(); cerr != nil {
		// Cancellation empties both listings, and an empty listing is
		// indistinguishable from "nothing is running anywhere". Reporting
		// that would tell the user something untrue about their machines,
		// and marking every host unreachable would compound it.
		return cerr
	}
	entries = append(entries, remote...)
	reportSkippedHosts(skipped)
	if len(entries) == 0 {
		fmt.Println("No live sessions anywhere. Start one with: kit attach")
		return nil
	}

	// Runs before any client attaches, so this picker owns the screen.
	choice, err := sessionPickerFor(ctx, entries, os.Stdin, "Sessions across all paired hosts")
	if err != nil || choice.Cancel {
		return err
	}
	opts.Target = choice.ID
	return runFollowingHostSwitches(ctx, choice.Host, opts)
}

// remoteSessionEntries collects sessions from every paired host, tagged
// with the host they came from, and names the hosts that did not answer.
//
// Hosts are queried in parallel: asking one at a time made a picker wait
// out every sleeping laptop in the host book before it could draw, and the
// wait grew with each host paired. ctx cancels the queries: each one is a
// sidecar process, so a caller that gives up must not leave a fan-out of
// them running until their timeouts expire.
func remoteSessionEntries(ctx context.Context, hosts []daemon.HostEntry, skip string) (entries []daemon.SessionEntry, skipped []string) {
	type result struct {
		entries []daemon.SessionEntry
		err     error
	}

	// Each goroutine owns one slot, so the results keep the host book's
	// order however the answers interleave.
	var wg sync.WaitGroup
	results := make([]result, len(hosts))
	for i, h := range hosts {
		if h.Name == skip {
			continue
		}
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			found, err := daemon.ListHostSessions(ctx, name, hostQueryTimeout)
			for j := range found {
				found[j].Host = name
			}
			results[i] = result{entries: found, err: err}
		}(i, h.Name)
	}
	wg.Wait()

	for i, h := range hosts {
		if h.Name == skip {
			continue
		}
		if results[i].err != nil {
			skipped = append(skipped, h.Name)
			continue
		}
		entries = append(entries, results[i].entries...)
	}
	return entries, skipped
}

// hostQueryTimeout bounds one host's session listing. It covers a relay
// round trip with room to spare; a host that needs longer is asleep.
const hostQueryTimeout = 8 * time.Second

// reportSkippedHosts names the hosts that did not answer, so an incomplete
// list is never mistaken for an empty one.
func reportSkippedHosts(skipped []string) {
	if len(skipped) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "Skipped unreachable host(s): %s\n", strings.Join(skipped, ", "))
}

// appendRemoteSessions adds every paired host's sessions to entries.
//
// Cancellation is reported before the skipped hosts are named: it fails
// every host query at once, and telling the user their machines are
// unreachable when the listing was simply abandoned is a false report
// about their infrastructure. A missing host book is not an error — there
// is simply nothing to add.
func appendRemoteSessions(ctx context.Context, entries []daemon.SessionEntry) ([]daemon.SessionEntry, error) {
	hosts, err := daemon.ListHosts()
	if err != nil {
		return entries, nil
	}
	remote, skipped := remoteSessionEntries(ctx, hosts, "")
	if cerr := ctx.Err(); cerr != nil {
		return entries, cerr
	}
	reportSkippedHosts(skipped)
	return append(entries, remote...), nil
}

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List live kit sessions",
	Long: `List the live sessions on the local daemon.

With --all, also queries every paired host. Hosts that do not answer are
skipped.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		entries, err := daemon.ListLocalSessions(ctx)
		if err != nil && err != daemon.ErrNoLocalDaemon {
			return err
		}
		if attachAll {
			entries, err = appendRemoteSessions(ctx, entries)
			if err != nil {
				return err
			}
		}
		// A cancelled listing is also an empty one, and printing "no live
		// sessions" for it would be a false report.
		if cerr := ctx.Err(); cerr != nil {
			return cerr
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
