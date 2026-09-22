//go:build windows

package daemon

import (
	"context"
	"errors"
)

// Session supervisors need a local socket, and the daemon has none on
// Windows yet (see local_windows.go): a named pipe plus the matching
// peer-identity check, not a Unix socket. Until that exists, a Windows
// daemon hosts its sessions itself and they end with it, which is what
// every daemon did before supervisors were introduced.
//
// Nothing above this file cares. spawnSession asks whether hosting is
// supported and falls back on its own, so the only visible difference is
// the FeatureReattach bit, which a Windows daemon does not advertise and
// a client therefore does not promise the user.

// hostedSessionsSupported reports that this platform cannot put sessions
// in supervisor processes.
func hostedSessionsSupported() bool { return false }

// errNoSessionHost reports that a supervisor could not be started, so the
// caller falls back to hosting the session in the daemon itself.
var errNoSessionHost = errors.New("daemon: session hosts are not supported on Windows yet")

// RunSessionHost is unsupported on Windows.
func RunSessionHost(context.Context, string) error { return errNoSessionHost }

// spawnSessionHost always fails on Windows; spawnSession falls back to a
// session hosted in the daemon.
func (t *sessionTable) spawnSessionHost(context.Context, uint64, TerminalInfo, *SessionSpec) (sessionIO, error) {
	return nil, errNoSessionHost
}

// adoptHostedSessions finds nothing to adopt on Windows.
func (t *sessionTable) adoptHostedSessions(context.Context) []uint64 { return nil }
