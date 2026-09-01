package cmd

import (
	"context"
	"testing"
	"time"

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
