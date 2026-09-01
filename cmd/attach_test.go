package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/mark3labs/kit/internal/daemon"
)

// TestRemoteSessionEntriesHonoursCancellation checks that a cancelled
// context stops the fan-out of host queries.
//
// Every paired host is queried at once and each query starts a sidecar
// process, so a caller that gives up — a picker torn down, the client
// shutting down — must be able to end them. Without a context the
// goroutines ran to their eight second timeout regardless, holding a
// sidecar each for the whole of it.
//
// The hosts here are not paired, so each query fails on the host lookup
// before any process starts; what is under test is that the call returns
// on a dead context rather than the transport behaviour.
func TestRemoteSessionEntriesHonoursCancellation(t *testing.T) {
	hosts := []daemon.HostEntry{
		{Name: "unreachable-a"},
		{Name: "unreachable-b"},
		{Name: "unreachable-c"},
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // already dead when the queries start

	done := make(chan struct{})
	go func() {
		defer close(done)
		entries, skipped := remoteSessionEntries(ctx, hosts, "")
		if len(entries) != 0 {
			t.Errorf("entries = %d, want none from unreachable hosts", len(entries))
		}
		if len(skipped) != len(hosts) {
			t.Errorf("skipped = %v, want all %d hosts reported", skipped, len(hosts))
		}
	}()

	select {
	case <-done:
	case <-time.After(hostQueryTimeout):
		t.Fatal("a cancelled context did not stop the host queries")
	}
}

// TestRemoteSessionEntriesSkipsTheCurrentHost pins the hub's contract: the
// daemon the client is already talking to is left out, because the client
// lists that one itself and a duplicate row would attach by an id that
// means something different on each daemon.
func TestRemoteSessionEntriesSkipsTheCurrentHost(t *testing.T) {
	hosts := []daemon.HostEntry{{Name: "keep-me"}, {Name: "skip-me"}}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, skipped := remoteSessionEntries(ctx, hosts, "skip-me")

	if len(skipped) != 1 || skipped[0] != "keep-me" {
		t.Fatalf("skipped = %v, want only [keep-me]: the current host must not be queried", skipped)
	}
}

// TestHubAttachReportsCancellation checks that a cancelled discovery is
// reported as cancellation rather than as an empty world.
//
// Cancellation empties both listings: the local query fails and every host
// query is recorded as skipped. Without a check that is indistinguishable
// from "nothing is running anywhere", so the command printed exactly that
// and exited zero — telling the user something untrue about their machines
// while also marking every host unreachable.
func TestHubAttachReportsCancellation(t *testing.T) {
	cmd := &cobra.Command{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cmd.SetContext(ctx)

	err := runHubAttach(cmd, daemon.AttachOptions{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runHubAttach error = %v, want context.Canceled", err)
	}
}

// TestAppendRemoteSessionsReportsCancellationBeforeNamingHosts pins the
// ordering of the cancellation check in the `--all` listing.
//
// This is the mid-flight case: the local sessions were listed fine and the
// user gave up during the host fan-out. Cancellation fails every host
// query at once, so all of them end up on the skipped list — and naming
// them tells the user their machines are unreachable, which is a false
// report about their infrastructure, printed on the way to returning the
// cancellation anyway.
func TestAppendRemoteSessionsReportsCancellationBeforeNamingHosts(t *testing.T) {
	if hosts, err := daemon.ListHosts(); err != nil || len(hosts) == 0 {
		t.Skip("no paired hosts on this machine: nothing could be named unreachable")
	}

	ctx, cancel := context.WithCancel(t.Context())
	local := []daemon.SessionEntry{{ID: 1}} // already listed before the cancel
	cancel()

	var entries []daemon.SessionEntry
	var err error
	stderr := captureStderr(t, func() {
		entries, err = appendRemoteSessions(ctx, local)
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if strings.Contains(stderr, "Skipped unreachable host") {
		t.Fatalf("a cancelled listing named hosts as unreachable: %q", stderr)
	}
	if len(entries) != len(local) {
		t.Fatalf("entries = %d, want the %d already listed locally", len(entries), len(local))
	}
}

// captureStderr runs fn with os.Stderr redirected and returns what it
// wrote. Reads happen on a goroutine so a write larger than the pipe
// buffer cannot deadlock fn.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	real := os.Stderr
	os.Stderr = w

	out := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		out <- b.String()
	}()

	fn()

	os.Stderr = real
	_ = w.Close()
	got := <-out
	_ = r.Close()
	return got
}
