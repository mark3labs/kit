package daemon

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// Regression test for the burnt pairing window: the host advertises a
// window that stays open for the whole timeout, so a first attempt that
// does NOT pair (client vanished mid-handshake, rejected on the host,
// wrong code) must leave the code usable and must not leave a stale
// question blocking the window.
//
// Before the fix the sidecar served exactly one incoming connection AND
// held it for the whole decision timeout: attempt two never reached the
// host, the host never prompted again, and the client only saw
// "connect to daemon: timed out".
//
// Needs the real sidecar and network access; set KIT_TUNNEL_BIN to run it.
func TestPairWindowSurvivesRejectedAttempt(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	if os.Getenv("KIT_TUNNEL_BIN") == "" {
		t.Skip("set KIT_TUNNEL_BIN to the kit-tunnel sidecar to run the pairing end-to-end test")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	const code = "ABCD2345"
	var attempts int
	prompted := make(chan int, 8)
	hostDone := make(chan error, 1)
	go func() {
		hostDone <- RunPairWindow(ctx, PairWindowOptions{
			Code:   code,
			Window: 3 * time.Minute,
			Prompt: func(context.Context, string) bool {
				attempts++
				prompted <- attempts
				return attempts > 1 // reject the first, accept the second
			},
		})
	}()

	waitPrompt := func(want int) {
		t.Helper()
		select {
		case got := <-prompted:
			if got != want {
				t.Fatalf("prompt %d, want %d", got, want)
			}
		case <-time.After(90 * time.Second):
			t.Fatalf("the host was never prompted for attempt %d", want)
		}
	}

	// The window needs its endpoint published before a client can find it.
	time.Sleep(5 * time.Second)

	err := RunPair(ctx, PairOptions{Code: code, Name: "regress"})
	waitPrompt(1)
	if err == nil || !strings.Contains(err.Error(), "rejected on the host") {
		t.Fatalf("attempt 1: got %v, want a rejection", err)
	}

	// Same code, second attempt: the window must still serve it.
	if err := RunPair(ctx, PairOptions{Code: code, Name: "regress"}); err != nil {
		t.Fatalf("attempt 2 after a rejected attempt: %v", err)
	}
	waitPrompt(2)

	if _, err := GetHost("regress"); err != nil {
		t.Fatalf("paired host not saved: %v", err)
	}

	select {
	case err := <-hostDone:
		if err != nil {
			t.Fatalf("pair window: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the pair window did not close after a successful pairing")
	}
}

// A pairing question whose client has gone away must not block the window.
// The sidecar withdraws it with a PAIR_CANCEL frame, and a newer request
// supersedes it; either way the operator gets asked about the live client
// instead of being stuck on a dead one.
func TestAskOperatorAbandonsDeadQuestions(t *testing.T) {
	corr := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	request := func(nonceFirst byte) Frame {
		payload := make([]byte, 64)
		payload[0] = nonceFirst
		return Frame{Type: FramePairRequest, Payload: payload}
	}

	t.Run("cancel for this question withdraws it", func(t *testing.T) {
		frames := make(chan Frame, 1)
		frames <- Frame{Type: FramePairCancel, Payload: corr}
		allowed, next := PairWindowOptions{}.askOperator(
			context.Background(), "fp", corr, frames, make(chan string))
		if allowed || next != nil {
			t.Fatalf("got (%v, %v), want (false, nil)", allowed, next)
		}
	})

	t.Run("cancel for another question is ignored", func(t *testing.T) {
		frames := make(chan Frame, 2)
		frames <- Frame{Type: FramePairCancel, Payload: []byte{9, 9, 9, 9, 9, 9, 9, 9}}
		answers := make(chan string, 1)
		answers <- "y"
		allowed, next := PairWindowOptions{}.askOperator(
			context.Background(), "fp", corr, frames, answers)
		if !allowed || next != nil {
			t.Fatalf("got (%v, %v), want (true, nil): an unrelated cancel must not steal the question", allowed, next)
		}
	})

	t.Run("a newer request supersedes and is handed back", func(t *testing.T) {
		frames := make(chan Frame, 1)
		frames <- request(0x7f)
		allowed, next := PairWindowOptions{}.askOperator(
			context.Background(), "fp", corr, frames, make(chan string))
		if allowed {
			t.Fatal("a superseded question must not approve")
		}
		if next == nil || next.Type != FramePairRequest || next.Payload[0] != 0x7f {
			t.Fatalf("the newer request must be handed back for handling, got %v", next)
		}
	})

	t.Run("an expired window denies", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		allowed, next := PairWindowOptions{}.askOperator(
			ctx, "fp", corr, make(chan Frame), make(chan string))
		if allowed || next != nil {
			t.Fatalf("got (%v, %v), want (false, nil)", allowed, next)
		}
	})

	t.Run("no terminal denies without blocking", func(t *testing.T) {
		allowed, next := PairWindowOptions{}.askOperator(
			context.Background(), "fp", corr, make(chan Frame), nil)
		if allowed || next != nil {
			t.Fatalf("got (%v, %v), want (false, nil)", allowed, next)
		}
	})
}

// The sidecar reports transport failures; the client must translate them
// into something the user can act on.
func TestPairFailureAdvice(t *testing.T) {
	base := errors.New("tunnel exited")
	cases := []struct {
		name     string
		statuses string
		want     string
	}{
		{
			name:     "connect timeout points at the window",
			statuses: "STATUS ERROR msg=connect to daemon: timed out",
			want:     "could not reach the host's pairing window",
		},
		{
			name:     "unknown code",
			statuses: "STATUS ERROR msg=no daemon is live for this pairing code (wrong code, expired window, or network issue)",
			want:     "no host is listening for that pairing code",
		},
		{
			name:     "no addressing info",
			statuses: "STATUS ERROR msg=connect to daemon: No addressing information available",
			want:     "no host is listening for that pairing code",
		},
		{
			name:     "local bind failure",
			statuses: "STATUS ERROR msg=endpoint bind: permission denied",
			want:     "could not open a local network endpoint",
		},
		{
			name:     "unrecognised failure keeps the detail",
			statuses: "STATUS ERROR msg=something new",
			want:     "pairing failed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pairFailure(base, tc.statuses)
			if got == nil || !strings.Contains(got.Error(), tc.want) {
				t.Fatalf("pairFailure(%q) = %v, want it to mention %q", tc.statuses, got, tc.want)
			}
		})
	}
}
