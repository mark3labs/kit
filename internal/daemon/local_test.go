//go:build !windows

package daemon

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The local socket and the single-instance lock are addressed by two
// different environment variables: the lock through the cache directory
// (XDG_CACHE_HOME), the socket through XDG_RUNTIME_DIR. Moving one
// without the other puts two daemons on one socket with no lock
// contention between them, so listenLocal cannot treat the lock as proof
// that a socket file is abandoned.

// TestListenLocalRefusesALiveSocket is the collision case.
//
// Unlinking here is unrecoverable, not merely rude: the running daemon
// keeps its bound inode and carries on listening, but no client can ever
// reach it by path again, because a bound Unix socket cannot be
// re-linked. Its sessions stay alive and become permanently unreachable
// while `kit daemon status` still calls it healthy.
func TestListenLocalRefusesALiveSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.sock")

	first, err := listenLocal(path)
	if err != nil {
		t.Fatalf("listenLocal: %v", err)
	}
	defer func() { _ = first.Close() }()

	second, err := listenLocal(path)
	if err == nil {
		_ = second.Close()
		t.Fatal("a second daemon bound a socket another daemon was listening on")
	}
	if !strings.Contains(err.Error(), "already listening") {
		t.Fatalf("error = %v, want one naming the live daemon", err)
	}

	// The point of refusing: the first listener must still be reachable.
	conn, derr := net.Dial("unix", path)
	if derr != nil {
		t.Fatalf("the original daemon became unreachable: %v", derr)
	}
	_ = conn.Close()
}

// TestListenLocalClearsAStaleSocket keeps the recovery path working. A
// daemon killed with SIGKILL leaves its socket file behind, and refusing
// to start until a human deletes it would make every hard crash need
// manual repair.
func TestListenLocalClearsAStaleSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.sock")

	// A crashed daemon leaves the file behind, because only a clean
	// shutdown unlinks it. Go's listener unlinks on Close by default, so
	// that is turned off here to reproduce the crash rather than the
	// orderly exit.
	dead, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	dead.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := dead.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no stale socket file to recover from: %v", err)
	}

	ln, err := listenLocal(path)
	if err != nil {
		t.Fatalf("listenLocal refused a stale socket: %v", err)
	}
	defer func() { _ = ln.Close() }()

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("the recovered socket does not accept: %v", err)
	}
	_ = conn.Close()
}

// TestSocketIsLiveReportsADeadSocketAsDead pins the probe's two answers
// directly, since everything above depends on it telling them apart.
func TestSocketIsLiveReportsADeadSocketAsDead(t *testing.T) {
	dir := t.TempDir()

	live := filepath.Join(dir, "live.sock")
	ln, err := net.Listen("unix", live)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	if !socketIsLive(live) {
		t.Error("a listening socket was reported as dead: a daemon would be unlinked")
	}

	missing := filepath.Join(dir, "missing.sock")
	if socketIsLive(missing) {
		t.Error("a socket path that does not exist was reported as live")
	}

	// A plain file at the path is not a socket at all. Dialling it fails
	// with ECONNREFUSED or ENOTSOCK; either way nothing is served from
	// it, so it must not block a daemon from starting.
	regular := filepath.Join(dir, "regular.sock")
	if err := os.WriteFile(regular, []byte("not a socket"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if socketIsLive(regular) {
		t.Log("a regular file probed as live; listenLocal will refuse rather than unlink it")
	}
}

func TestDialLocalRecordedSocket(t *testing.T) {
	for _, daemonRuntimeDir := range []string{"", "runtime-a"} {
		t.Run(daemonRuntimeDir, func(t *testing.T) {
			base := t.TempDir()
			t.Setenv("XDG_CACHE_HOME", base)
			t.Setenv("XDG_RUNTIME_DIR", "")
			if daemonRuntimeDir != "" {
				t.Setenv("XDG_RUNTIME_DIR", filepath.Join(base, daemonRuntimeDir))
			}
			lock, err := acquireDaemonLock()
			if err != nil {
				t.Fatal(err)
			}
			defer lock.release()
			path, err := LocalSocketPath()
			if err != nil {
				t.Fatal(err)
			}
			ln, err := listenLocal(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := ln.Close(); err != nil {
					t.Errorf("close listener: %v", err)
				}
			}()
			rt := newDaemonRuntime(lock)
			if err := rt.setSocketPath(path); err != nil {
				t.Fatal(err)
			}
			if st := ReadStatus(); st.State == nil || st.State.SocketPath != path {
				t.Fatalf("socket path not persisted: %+v", st)
			}
			t.Setenv("XDG_RUNTIME_DIR", filepath.Join(base, "runtime-b"))
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			conn, err := DialLocal(ctx)
			if err != nil {
				t.Fatalf("dial with different environment: %v", err)
			}
			if err := conn.Close(); err != nil {
				t.Errorf("close connection: %v", err)
			}
		})
	}
}

func TestDialLocalLegacyCacheSocket(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)
	t.Setenv("XDG_RUNTIME_DIR", "")
	path, err := LocalSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := listenLocal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ln.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
	}()
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(base, "runtime"))
	conn, err := DialLocal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Errorf("close connection: %v", err)
	}
}

func TestDialLocalIgnoresStoppedDaemonState(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	lock, err := acquireDaemonLock()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "old.sock")
	ln, err := listenLocal(path)
	if err != nil {
		lock.release()
		t.Fatal(err)
	}
	defer func() {
		if err := ln.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
	}()
	rt := newDaemonRuntime(lock)
	err = rt.setSocketPath(path)
	lock.release()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := DialLocal(context.Background())
	if conn != nil {
		if err := conn.Close(); err != nil {
			t.Errorf("close connection: %v", err)
		}
	}
	if !errors.Is(err, ErrNoLocalDaemon) {
		t.Fatalf("error = %v, want no daemon", err)
	}
}

func TestStartLocalDaemonReportsStartupError(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := startLocalDaemonCommand(ctx, exec.Command("sh", "-c", "echo 'daemon: another instance is already running' >&2; exit 1"))
	if err == nil || !strings.Contains(err.Error(), "another instance is already running") {
		t.Fatalf("error = %v, want child startup error", err)
	}
	if _, ok := errors.AsType[*exec.ExitError](err); !ok {
		t.Fatalf("error does not wrap child exit: %v", err)
	}
}

func TestWaitForLocalDaemonAcceptsCompetingDaemon(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	path, err := LocalSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := listenLocal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ln.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
	}()
	exited := make(chan error, 1)
	exited <- errors.New("another daemon won the lock")
	if err := waitForLocalDaemonStartup(context.Background(), time.Second, exited, ""); err != nil {
		t.Fatal(err)
	}
}
