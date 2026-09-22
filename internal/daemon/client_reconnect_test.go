package daemon

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// Tests for the client's half of surviving a daemon restart.
//
// The session outliving the daemon is only half the feature. If the
// client still exits the moment the socket drops, the user loses their
// screen on every `systemctl restart kit` and the surviving session is
// something they have to go and find. These tests pin the decisions that
// make the difference invisible.

// TestLostConnectionIsNotAFinishedSession is the regression test for the
// bug this change had to fix in the client: runAttached used to report a
// closed stream as `ended`, so a killed daemon told the user "Session
// ended." while their session was still running in its supervisor.
//
// The two outcomes must stay distinct, because they mean opposite things
// to everything downstream: `ended` stops the client, `lost` reconnects.
func TestLostConnectionIsNotAFinishedSession(t *testing.T) {
	ended := attachOutcome{ended: true}
	lost := attachOutcome{lost: true}

	if ended.lost {
		t.Error("a finished session must not be reported as a lost connection")
	}
	if lost.ended {
		t.Error("a lost connection must not be reported as a finished session: " +
			"the client would stop instead of reconnecting, and would tell the user " +
			"their work was gone while it was still running")
	}
}

// TestReconnectPartingDescribesTheSession checks the message printed when
// a reconnect runs out of time. The user's question is always "what
// happened to my work", and the honest answer depends on what the daemon
// said it could do.
func TestReconnectPartingDescribesTheSession(t *testing.T) {
	opts := AttachOptions{Reattach: "kit attach"}

	t.Run("a daemon that keeps its sessions", func(t *testing.T) {
		run := clientRun{
			current: 7,
			daemon:  Hello{Version: ProtocolVersion, Features: FeatureReattach},
		}
		got := reconnectParting(opts, run, errors.New("connection refused"))
		for _, want := range []string{"7", "still running", "kit attach 7"} {
			if !strings.Contains(got, want) {
				t.Errorf("parting message does not mention %q:\n%s", want, got)
			}
		}
	})

	t.Run("a daemon whose sessions died with it", func(t *testing.T) {
		run := clientRun{
			current: 7,
			daemon:  Hello{Version: ProtocolVersion}, // no FeatureReattach
		}
		got := reconnectParting(opts, run, errors.New("connection refused"))
		if strings.Contains(got, "still running") {
			t.Errorf("a daemon without FeatureReattach took its sessions with it; "+
				"telling the user otherwise sends them looking for work that is gone:\n%s", got)
		}
		if !strings.Contains(got, "stopped with it") {
			t.Errorf("parting message does not say the sessions are gone:\n%s", got)
		}
	})

	t.Run("a client that never attached", func(t *testing.T) {
		run := clientRun{daemon: Hello{Version: ProtocolVersion, Features: FeatureReattach}}
		got := reconnectParting(opts, run, nil)
		if strings.Contains(got, "session 0") {
			t.Errorf("a client with no session must not name session 0:\n%s", got)
		}
		if !strings.Contains(got, "kit ls") {
			t.Errorf("parting message should point at the session list:\n%s", got)
		}
	})

	t.Run("a remote host", func(t *testing.T) {
		remote := AttachOptions{Host: "homelab", Reattach: "kit attach --host homelab"}
		run := clientRun{daemon: Hello{Version: ProtocolVersion, Features: FeatureReattach}}
		got := reconnectParting(remote, run, nil)
		if !strings.Contains(got, "kit ls --host homelab") {
			t.Errorf("a remote client must be told how to list THAT host's sessions:\n%s", got)
		}
	})
}

// TestReconnectRetriesUntilTheDaemonAnswers covers the common case: the
// daemon is down for a moment while it restarts, and the client waits it
// out rather than giving up on the first refused connection.
func TestReconnectRetriesUntilTheDaemonAnswers(t *testing.T) {
	attempts := 0
	opts := AttachOptions{
		Redial: func(context.Context) (io.ReadWriter, func(), error) {
			attempts++
			if attempts < 3 {
				return nil, nil, errors.New("connection refused")
			}
			return silentStream{}, func() {}, nil
		},
	}

	stream, closer, err := reconnectToDaemon(t.Context(), opts, clientRun{current: 1})
	if err != nil {
		t.Fatalf("reconnect gave up while the daemon was restarting: %v", err)
	}
	if stream == nil || closer == nil {
		t.Fatal("a successful reconnect must return a usable stream and its closer")
	}
	if attempts != 3 {
		t.Fatalf("redialled %d times, want 3", attempts)
	}
}

// TestReconnectGivesUp checks the other end of it: a daemon that never
// comes back must leave the user at their shell with a message, not at a
// spinner forever.
func TestReconnectGivesUp(t *testing.T) {
	// A context deadline stands in for the reconnect window, so the test
	// does not have to wait out the real one.
	ctx, cancel := context.WithTimeout(t.Context(), 600*time.Millisecond)
	defer cancel()

	opts := AttachOptions{
		Redial: func(context.Context) (io.ReadWriter, func(), error) {
			return nil, nil, errors.New("connection refused")
		},
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := reconnectToDaemon(ctx, opts, clientRun{current: 1})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("reconnect reported success against a daemon that never answered")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("reconnect never gave up; the user would be stuck")
	}
}

// TestReconnectWithoutARedialerFails makes sure a client that cannot
// reconnect says so instead of pretending.
func TestReconnectWithoutARedialerFails(t *testing.T) {
	if _, _, err := reconnectToDaemon(t.Context(), AttachOptions{}, clientRun{}); err == nil {
		t.Fatal("a client with no Redial must not report a successful reconnect")
	}
}

// TestGreetToleratesALegacyDaemon is the compatibility guarantee, from
// the client's side: a daemon too old to know the hello frame answers
// nothing, and the client must carry on rather than hang or fail.
func TestGreetToleratesALegacyDaemon(t *testing.T) {
	// A connection that accepts writes and never answers, which is
	// exactly what a pre-hello daemon looks like.
	conn := newClientConn(silentStream{})

	start := time.Now()
	peer, err := conn.greet()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("a daemon that sent no hello was treated as an error: %v", err)
	}
	if peer.Version != ProtocolVersion {
		t.Errorf("a silent daemon should be read as protocol v%d, got v%d",
			ProtocolVersion, peer.Version)
	}
	if peer.Features != 0 {
		t.Errorf("a silent daemon must not be credited with features: %v", peer.Features)
	}
	if elapsed > 5*time.Second {
		t.Errorf("greeting a legacy daemon took %s; it must not stall the first frame", elapsed)
	}
}

// silentStream accepts every write and never produces a byte, blocking on
// Read the way an idle connection does.
type silentStream struct{}

func (silentStream) Write(p []byte) (int, error) { return len(p), nil }
func (silentStream) Read([]byte) (int, error) {
	select {} // an idle daemon: nothing to read, and no EOF either
}
