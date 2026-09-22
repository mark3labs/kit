package daemon

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// Regression tests for the CodeRabbit review on PR #142.
//
// Each of these covers a finding that was a real defect, not a style
// point. They are grouped here so the link between "a reviewer spotted
// this" and "it can never come back" stays visible.

// TestLocalHelloHidesReattachWhereUnsupported covers the finding that the
// daemon advertised FeatureReattach unconditionally.
//
// The bit is a promise that sessions survive a daemon restart, and the
// client prints "Session N is still running" on the strength of it. On a
// platform with no supervisors that promise is false, and it would be
// made at the exact moment the user needs the truth.
func TestLocalHelloHidesReattachWhereUnsupported(t *testing.T) {
	h := localHello(RoleDaemon)
	if got := h.Features.Has(FeatureReattach); got != hostedSessionsSupported() {
		t.Fatalf("hello advertises reattach=%v while this platform hosts sessions=%v; "+
			"a client would promise the user work that does not survive",
			got, hostedSessionsSupported())
	}

	// Everything else is unaffected: clearing one bit must not disturb
	// the rest of the feature set.
	for _, f := range []Feature{FeatureTerminalInfo, FeatureSessionSpec, FeatureClipboard} {
		if !h.Features.Has(f) {
			t.Errorf("clearing reattach also dropped %v", f)
		}
	}
	if err := h.Compatible(); err != nil {
		t.Fatalf("a hello from this build is incompatible with itself: %v", err)
	}
}

// TestAttachReadsPerSessionDurability drives the real ack parser.
//
// The tenth byte is ADDITIVE: a nine-byte ack from an older daemon must
// read as "no answer" rather than as "this session does not survive".
// Collapsing those two would make every pre-ack daemon report its
// surviving sessions as lost — telling users their work was destroyed
// while it is still running.
func TestAttachReadsPerSessionDurability(t *testing.T) {
	cases := []struct {
		name string
		ack  []byte
		want sessionDurability
	}{
		{
			"an older daemon sends nine bytes",
			func() []byte {
				b := make([]byte, 9)
				binary.BigEndian.PutUint64(b[:8], 3)
				b[8] = 1
				return b
			}(),
			durabilityUnknown,
		},
		{
			"this daemon reports a hosted session",
			func() []byte {
				b := make([]byte, 10)
				binary.BigEndian.PutUint64(b[:8], 3)
				b[8], b[9] = 1, 1
				return b
			}(),
			durabilityHosted,
		},
		{
			"this daemon reports a session it hosts itself",
			func() []byte {
				b := make([]byte, 10)
				binary.BigEndian.PutUint64(b[:8], 3)
				b[8], b[9] = 1, 0
				return b
			}(),
			durabilityInDaemon,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := newClientConn(silentStream{})
			// attach writes the request, then waits on the control
			// channel; queueing the ack first lets it complete without a
			// daemon on the other end.
			conn.ctrlCh <- Frame{Type: FrameSessionAttachAck, Payload: tc.ack}

			assigned, durability, err := conn.attach(t.Context(), 3)
			if err != nil {
				t.Fatalf("attach failed: %v", err)
			}
			if assigned != 3 {
				t.Fatalf("assigned = %d, want 3", assigned)
			}
			if durability != tc.want {
				t.Fatalf("durability = %d, want %d", durability, tc.want)
			}
		})
	}
}

// TestSessionDurabilityDistinguishesSilenceFromNo covers how the two
// signals are reconciled: the per-session answer wins when there is one,
// and the platform-wide feature bit is the fallback when there is not.
func TestSessionDurabilityDistinguishesSilenceFromNo(t *testing.T) {
	reattachable := Hello{Version: ProtocolVersion, Features: FeatureReattach}
	plain := Hello{Version: ProtocolVersion}

	cases := []struct {
		name string
		run  clientRun
		want bool
	}{
		{
			// The daemon was specific, and that beats the platform bit:
			// it can host sessions in general and still have fallen back
			// for this one.
			"daemon says this session is hosted",
			clientRun{daemon: plain, sessionDurability: durabilityHosted},
			true,
		},
		{
			"daemon says this session is not hosted",
			clientRun{daemon: reattachable, sessionDurability: durabilityInDaemon},
			false,
		},
		{
			// Silence from an older daemon: fall back to what it did say.
			"old daemon, platform supports reattach",
			clientRun{daemon: reattachable, sessionDurability: durabilityUnknown},
			true,
		},
		{
			"old daemon, no reattach support",
			clientRun{daemon: plain, sessionDurability: durabilityUnknown},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.run.survives(); got != tc.want {
				t.Fatalf("survives() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPartingMessageForASessionThatDiedWithTheDaemon checks the client
// does not hand out a reattach command for work that is gone.
func TestPartingMessageForASessionThatDiedWithTheDaemon(t *testing.T) {
	opts := AttachOptions{Reattach: "kit attach"}
	run := clientRun{
		current:           4,
		daemon:            Hello{Version: ProtocolVersion, Features: FeatureReattach},
		sessionDurability: durabilityInDaemon,
	}

	got := reconnectParting(opts, run, nil)
	if strings.Contains(got, "kit attach 4") {
		t.Errorf("offered a reattach command for a session that stopped with its daemon:\n%s", got)
	}
	if !strings.Contains(got, "stopped with it") {
		t.Errorf("parting message does not say the session is gone:\n%s", got)
	}

	// And the in-progress message must not claim it either.
	if msg := reconnectingMessage(run); strings.Contains(msg, "still running") {
		t.Errorf("reconnect status claims a dead session is still running:\n%s", msg)
	}
}

// TestReportedCwdKeepsPathWhitespace covers the finding that TrimSpace
// was corrupting directory names.
//
// A space is a legal character at either end of a path. Trimming one off
// reports a directory the session is not in, which is what `kit ls` shows
// and what a user reads to tell two sessions apart.
func TestReportedCwdKeepsPathWhitespace(t *testing.T) {
	dir := t.TempDir()

	cases := map[string]string{
		"/tmp/project ":     "/tmp/project ",     // trailing space is part of the name
		" /tmp/leading":     " /tmp/leading",     // and so is a leading one
		"/tmp/plain\n":      "/tmp/plain",        // the writer's delimiter goes
		"/tmp/crlf\r\n":     "/tmp/crlf",         // including a CRLF pair, whole
		"/tmp/inner space/": "/tmp/inner space/", // untouched in the middle
	}
	for written, want := range cases {
		path := dir + "/cwd"
		if err := writeFileString(path, written); err != nil {
			t.Fatal(err)
		}
		if got := readReportedCwd(path); got != want {
			t.Errorf("readReportedCwd(%q) = %q, want %q", written, got, want)
		}
	}

	if got := readReportedCwd(dir + "/does-not-exist"); got != "" {
		t.Errorf("a missing report reads as %q, want empty", got)
	}
	if got := readReportedCwd(""); got != "" {
		t.Errorf("an unset report path reads as %q, want empty", got)
	}
}

// TestAttachIsCancellable covers the finding that `attach` blocked for a
// full ten seconds with no way to interrupt it.
//
// The wait is made on behalf of a user who can change their mind, and a
// client that has been cancelled still holds the terminal until it
// unwinds. Ten seconds of an unresponsive terminal is the difference
// between "it is thinking" and "it is broken".
func TestAttachIsCancellable(t *testing.T) {
	conn := newClientConn(silentStream{})

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, _, err := conn.attach(ctx, 7)
		done <- err
	}()

	// Nothing will ever answer, so without cancellation this sits for the
	// full ack timeout.
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("attach returned %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("attach ignored its context and is still waiting for an ack")
	}
}

// TestListSessionsIsCancellable is the same guarantee for the other
// blocking request on this connection.
func TestListSessionsIsCancellable(t *testing.T) {
	conn := newClientConn(silentStream{})

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := conn.listSessions(ctx)
		done <- err
	}()
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("listSessions returned %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("listSessions ignored its context")
	}
}

// writeFileString is a tiny helper so the table above reads as data.
func writeFileString(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
