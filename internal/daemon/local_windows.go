//go:build windows

package daemon

import (
	"context"
	"errors"
	"net"
)

// The local control socket is not implemented on Windows yet: it needs a
// named pipe plus the matching peer-identity check, not a Unix socket.
// Remote sessions over the sidecar are unaffected.

// ErrNoLocalDaemon is returned when nothing is listening on the local
// socket. Callers use it to decide whether to auto-start a daemon.
var ErrNoLocalDaemon = errors.New("daemon: local sessions are not supported on Windows yet")

// LocalSocketPath reports that no local socket exists on this platform.
func LocalSocketPath() (string, error) { return "", ErrNoLocalDaemon }

// listenLocal always fails on Windows; Serve treats this as "run remote
// sessions only" rather than as a fatal error.
func listenLocal(string) (net.Listener, error) { return nil, ErrNoLocalDaemon }

func serveLocal(context.Context, net.Listener, *sessionTable) {}

// sessionCwdEnv names the file a session child writes its chosen working
// directory into.
const sessionCwdEnv = "KIT_SESSION_CWD_FILE"

// ReportSessionCwd is a no-op on Windows, where the daemon hosts no local
// sessions.
func ReportSessionCwd(string) {}

// DialLocal always fails on Windows.
func DialLocal(context.Context) (net.Conn, error) { return nil, ErrNoLocalDaemon }

// StartLocalDaemon is unsupported on Windows.
func StartLocalDaemon(context.Context) error { return ErrNoLocalDaemon }

// RunLocal is unsupported on Windows: there is no local socket transport
// yet. Remote sessions over the sidecar are unaffected.
func RunLocal(context.Context, AttachOptions) error { return ErrNoLocalDaemon }

// ListLocalSessions is unsupported on Windows.
func ListLocalSessions() ([]SessionEntry, error) { return nil, ErrNoLocalDaemon }
