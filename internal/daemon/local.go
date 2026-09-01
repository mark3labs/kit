//go:build !windows

package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
)

// Local transport: a Unix domain socket that lets clients on the same
// machine drive the daemon without the sidecar, the network, or pairing.
//
// Authorization is the peer's uid. A remote client proves itself with an
// ed25519 signature checked against the pairing allowlist, but a local
// client is already running as the user whose sessions it wants to reach,
// so demanding a second credential would protect nothing. Two independent
// mechanisms enforce that:
//
//   - the socket lives in a 0700 directory and is itself 0600, and
//   - every accepted connection's peer uid is compared against our own.
//
// The first is what actually stops other users (Linux enforces permissions
// on connect); the second is defence in depth for platforms with weaker
// socket permission semantics, and it also catches a socket accidentally
// created somewhere world-writable.

const socketFileName = "daemon.sock"

// ErrNoLocalDaemon is returned when nothing is listening on the local
// socket. Callers use it to decide whether to auto-start a daemon.
var ErrNoLocalDaemon = errors.New("daemon: no local daemon is running")

// LocalSocketPath returns the path of the local control socket.
//
// XDG_RUNTIME_DIR is preferred: it is user-private (0700), on tmpfs, and
// cleared on logout, which is exactly the lifetime a socket wants. The
// cache dir is the fallback, alongside daemon.lock and daemon.json, so all
// daemon state stays in one place on systems without a runtime dir.
func LocalSocketPath() (string, error) {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		dir := filepath.Join(runtimeDir, "kit")
		if err := os.MkdirAll(dir, 0o700); err == nil {
			return filepath.Join(dir, socketFileName), nil
		}
		// Fall through to the cache dir when the runtime dir is unusable.
	}
	dir, err := daemonRuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, socketFileName), nil
}

// listenLocal binds the local control socket.
//
// A leftover socket file from a crashed daemon is removed first. This is
// safe because Serve already holds the single-instance flock by the time
// it calls us: no other daemon can be listening, so any file still there
// is stale by definition.
func listenLocal(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("daemon: socket dir: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("daemon: remove stale socket: %w", err)
		}
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("daemon: listen on %s: %w", path, err)
	}
	// Narrow the socket before announcing it. Between Listen and Chmod the
	// socket carries the process umask, which is why the parent directory
	// is 0700 as well.
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("daemon: secure socket: %w", err)
	}
	return ln, nil
}

// serveLocal accepts local clients until the listener closes. Each
// connection gets its own wire id and frame sink, and is served by its own
// goroutine, so one local client cannot stall another or the sidecar.
func serveLocal(ctx context.Context, ln net.Listener, table *sessionTable) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed, or the daemon is shutting down
		}
		if err := checkPeer(conn); err != nil {
			log.Warn("daemon: rejected local client", "error", err)
			_ = conn.Close()
			continue
		}
		go serveLocalConn(ctx, conn, table)
	}
}

// serveLocalConn drives one local client connection for its lifetime.
func serveLocalConn(ctx context.Context, conn net.Conn, table *sessionTable) {
	defer func() { _ = conn.Close() }()

	sink := newFrameSink(conn)
	wire := table.conns.addLocal(sink)
	defer func() {
		sink.close()
		table.conns.remove(wire.id)
		// Dropping the connection detaches its sessions; they keep
		// running, exactly as when a remote client disconnects.
		table.detachWire(wire.id)
	}()

	log.Debug("local client connected", "wire", wire.id)
	// The client has no wire-id allocator of its own, so it stamps every
	// frame with session 0 and we substitute the id assigned above.
	_ = table.runFrameSource(ctx, conn, sink, wire.id)
	log.Debug("local client disconnected", "wire", wire.id)
}

// DialLocal connects to the local daemon's control socket.
//
// The context bounds the dial: a socket that exists but is not being
// accepted on would otherwise block past the caller's deadline.
func DialLocal(ctx context.Context) (net.Conn, error) {
	path, err := LocalSocketPath()
	if err != nil {
		return nil, err
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		// A missing socket and a socket nobody is listening on are the
		// same situation to a caller: no daemon to talk to.
		// Match by value: net.Dial wraps ECONNREFUSED in *net.OpError, so
		// errors.Is identifies it exactly. A substring match on the
		// message breaks if the wrapping text changes, and RunLocal would
		// then fail instead of starting a daemon.
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			return nil, ErrNoLocalDaemon
		}
		return nil, fmt.Errorf("daemon: connect to local daemon: %w", err)
	}
	return conn, nil
}

// sessionCwdEnv names the file a session child writes its chosen working
// directory into. The daemon sets it when spawning the child; the child
// writes the directory once the picker resolves, and the daemon reads it
// back for the session list.
const sessionCwdEnv = "KIT_SESSION_CWD_FILE"

// ReportSessionCwd records the working directory a session settled on, so
// the daemon can show it in the session list. It is a no-op outside a
// daemon-hosted session, and a best-effort write: a session list without a
// directory is a cosmetic loss, never a reason to fail starting up.
func ReportSessionCwd(dir string) {
	path := os.Getenv(sessionCwdEnv)
	if path == "" || dir == "" {
		return
	}
	_ = os.WriteFile(path, []byte(dir), 0o600)
}

// StartLocalDaemon launches a background daemon and waits for its socket.
//
// This is the tmux behaviour: `kit attach` on a machine with no daemon
// starts one rather than telling the user to. The child is fully detached
// (its own process group, no inherited terminal) so it outlives the
// terminal that spawned it — a daemon killed by the hangup of the shell
// that started it would defeat the point of detachable sessions.
//
// Two clients racing to start a daemon is safe: the loser fails on the
// single-instance flock and both then connect to the winner's socket.
func StartLocalDaemon(ctx context.Context) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve kit binary: %w", err)
	}
	cmd := exec.Command(exe, "daemon")
	cmd.Dir = homeDir()
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = detachedProcAttr()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	// Do not wait for the child: it is meant to outlive us. Reaping is
	// left to init once this process exits.
	go func() { _ = cmd.Wait() }()

	return waitForLocalDaemon(ctx, 5*time.Second)
}

// waitForLocalDaemon polls the socket until a daemon answers.
func waitForLocalDaemon(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := DialLocal(ctx)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("daemon did not come up within %s: %w", timeout, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// RunLocal attaches the terminal to the local daemon, starting one when
// none is running.
func RunLocal(ctx context.Context, opts AttachOptions) error {
	conn, err := DialLocal(ctx)
	if errors.Is(err, ErrNoLocalDaemon) {
		fmt.Fprintln(os.Stderr, "Starting the kit daemon…")
		if serr := StartLocalDaemon(ctx); serr != nil {
			return serr
		}
		conn, err = DialLocal(ctx)
	}
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if opts.Name == "" {
		opts.Name = "local"
	}
	// The picker reports local sessions with an empty host, so this
	// client's identity is the empty string, not the display name.
	opts.Host = ""
	if opts.Reattach == "" {
		opts.Reattach = "kit attach"
	}
	return RunClient(ctx, conn, opts)
}

// ListLocalSessions reports the local daemon's live sessions without
// attaching. Returns ErrNoLocalDaemon when nothing is running, so a caller
// can distinguish "no daemon" from "a daemon with no sessions".
func ListLocalSessions(ctx context.Context) ([]SessionEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := DialLocal(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := newClientConn(conn)
	go client.readLoop()
	return client.listSessions()
}
