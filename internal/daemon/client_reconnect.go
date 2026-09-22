package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Reconnecting a client to a daemon that went away.
//
// Sessions outlive the daemon (see sessionhost.go), so a daemon that is
// stopped, restarted, upgraded or killed costs the user a CONNECTION, not
// a session. This file is what turns that fact into something the user
// experiences: the client keeps the terminal, keeps the alt screen, says
// what is happening, and reattaches to the same logical session when a
// daemon answers again.
//
// Everything here is bounded. A daemon that never comes back must leave
// the user at their shell with an honest message, not at a spinner.

const (
	// reconnectWindow is how long a client keeps trying before it gives
	// up. It has to comfortably cover the slowest thing that legitimately
	// interrupts a daemon: a package upgrade that stops the service,
	// replaces the binary and starts it again, or a `systemctl restart`
	// behind a busy machine's start-up.
	reconnectWindow = 30 * time.Second
	// reconnectFirstDelay is the pause before the first retry. A daemon
	// restart usually finishes inside it, so the common case is one short
	// pause and no visible flapping.
	reconnectFirstDelay = 250 * time.Millisecond
	// reconnectMaxDelay caps the backoff, so a long outage still retries
	// often enough to feel immediate when the daemon returns.
	reconnectMaxDelay = 2 * time.Second
)

// reconnectToDaemon redials until a daemon answers or the window closes.
//
// The status line is drawn on the alt screen the client still owns, which
// is why this cannot simply print to stderr: the user is looking at the
// screen their session was on, and a message written anywhere else would
// be invisible until the client exits.
//
// Failure to dial is not reported per attempt. During a restart EVERY
// attempt fails until the last one, so a message each time would be a
// wall of noise ending in success; only the final cause is kept, for the
// parting message.
func reconnectToDaemon(ctx context.Context, opts AttachOptions, run clientRun) (io.ReadWriter, func(), error) {
	if opts.Redial == nil {
		return nil, nil, errors.New("daemon: this client cannot reconnect")
	}
	reconnectStatus(reconnectingMessage(run))

	deadline := time.Now().Add(reconnectWindow)
	delay := reconnectFirstDelay
	var last error
	for attempt := 1; ; attempt++ {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(delay):
		}

		// Each dial gets what is left of the window rather than the whole
		// of it, so a dial that hangs cannot push the total wait past what
		// the user was promised.
		left := time.Until(deadline)
		if left <= 0 {
			if last == nil {
				last = errors.New("the daemon did not come back")
			}
			return nil, nil, last
		}
		dctx, cancel := context.WithTimeout(ctx, left)
		stream, closer, err := opts.Redial(dctx)
		cancel()
		if err == nil {
			return stream, closer, nil
		}
		last = err
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}

		delay = min(delay*2, reconnectMaxDelay)
		reconnectStatus(fmt.Sprintf("%s (attempt %d)", reconnectingMessage(run), attempt+1))
	}
}

// reconnectingMessage describes what the client is waiting for.
//
// It names the session only when that session actually survives the
// daemon. Promising a session is "still running" while it is being
// reaped would be the most misleading thing this client could say.
func reconnectingMessage(run clientRun) string {
	if run.current != 0 && run.survives() {
		return fmt.Sprintf("The daemon went away. Session %d is still running \u2014 reconnecting\u2026", run.current)
	}
	return "The daemon went away \u2014 reconnecting\u2026"
}

// reconnectParting is the message printed after a reconnect has run out
// of time, once the alt screen is gone.
//
// It says what happened to the SESSION, because that is the user's actual
// question. A session hosted separately kept running and can be picked up
// again; one the daemon hosted itself went with it, and pretending
// otherwise would send the user looking for work that no longer exists.
func reconnectParting(opts AttachOptions, run clientRun, cause error) string {
	var b strings.Builder
	b.WriteString("Lost the connection to the daemon")
	if cause != nil && !errors.Is(cause, context.Canceled) {
		fmt.Fprintf(&b, " (%v)", cause)
	}
	b.WriteString(".\n")
	switch {
	case run.current != 0 && run.survives():
		fmt.Fprintf(&b, "Session %d is still running — reattach with: %s %d",
			run.current, reattachHint(opts), run.current)
	case run.current != 0:
		// Attached to a session the daemon was hosting itself, so it died
		// with the daemon. Saying so is kinder than a reattach command
		// that will report no such session.
		fmt.Fprintf(&b, "Session %d was hosted by that daemon itself, so it stopped with it.", run.current)
	case run.daemon.Features.Has(FeatureReattach):
		fmt.Fprintf(&b, "Any sessions are still running — list them with: %s", listHint(opts))
	default:
		b.WriteString("That daemon hosted its sessions itself, so they stopped with it.")
	}
	return b.String()
}

// reattachHint is the command that brings a session back, defaulting to
// the local one when the caller supplied none.
func reattachHint(opts AttachOptions) string {
	if opts.Reattach != "" {
		return opts.Reattach
	}
	return "kit attach"
}

// listHint is the command that lists a daemon's sessions.
func listHint(opts AttachOptions) string {
	if opts.Host != "" {
		return "kit ls --host " + opts.Host
	}
	return "kit ls"
}

// reconnectStatus paints a one-line status over the alt screen while a
// reconnect is in progress.
//
// The screen is cleared first: what is on it is the last frame of a
// session that is no longer being updated, and leaving it under a status
// line would suggest the session is still live.
func reconnectStatus(msg string) {
	_, _ = fmt.Fprintf(os.Stdout, "\x1b[2J\x1b[H%s", msg)
}
