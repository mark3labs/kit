package daemon

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Tunnel is a running kit-tunnel sidecar process. Its stdout carries the
// relayed frames; its stderr carries "STATUS ..." lifecycle lines.
type Tunnel struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	statusCh chan string
	exited   chan error

	mu       sync.Mutex
	errLines []string
	closed   bool
}

// FindTunnelBinary locates the kit-tunnel sidecar, in order:
//
//  1. KIT_TUNNEL_BIN (explicit override)
//  2. next to the kit executable (task-managed output/ layout)
//  3. a repo checkout build (dev convenience for `go run ./cmd/kit daemon`)
//  4. the embedded copy staged into the kit build by `task build`
//     (extracted to the user cache dir on first use)
//  5. PATH
func FindTunnelBinary() (string, error) {
	if p := os.Getenv("KIT_TUNNEL_BIN"); p != "" {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
		return "", fmt.Errorf("daemon: KIT_TUNNEL_BIN=%s not found", p)
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "kit-tunnel")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if wd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(wd, "contrib", "kit-tunnel", "target", "release", "kit-tunnel")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if p, err := extractEmbeddedTunnel(); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("kit-tunnel"); err == nil {
		return p, nil
	}
	return "", errors.New("daemon: kit-tunnel sidecar not found; build it with 'task tunnel' or 'cargo build --release' in contrib/kit-tunnel, and place it next to the kit binary (or set KIT_TUNNEL_BIN)")
}

// StartTunnel launches the sidecar in serve or dial mode with the given
// hex-encoded seed. Status lines are parsed from stderr; frames flow on
// stdin/stdout.
func StartTunnel(ctx context.Context, mode, seedHex string) (*Tunnel, error) {
	bin, err := FindTunnelBinary()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, mode, "--seed-hex", seedHex, "--timeout", "30")
	cmd.Stderr = nil // replaced below with our own pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("daemon: tunnel stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("daemon: tunnel stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("daemon: tunnel stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("daemon: start %s: %w", bin, err)
	}

	t := &Tunnel{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   stdout,
		statusCh: make(chan string, 32),
		exited:   make(chan error, 1),
	}

	go func() {
		t.exited <- cmd.Wait()
	}()
	go t.readStatus(stderr)

	return t, nil
}

func (t *Tunnel) readStatus(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		t.recordLine(strings.TrimSpace(sc.Text()))
	}
	// A non-EOF scan error means we lost part of the tunnel's output; keep
	// it for diagnostics instead of failing the session on it.
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		t.recordLine(fmt.Sprintf("STATUS ERROR msg=tunnel stderr: %v", err))
	}
}

// recordLine appends a line to the diagnostic buffer and forwards STATUS
// lines to the status channel.
func (t *Tunnel) recordLine(line string) {
	if line == "" {
		return
	}
	t.mu.Lock()
	t.errLines = append(t.errLines, line)
	if len(t.errLines) > 50 {
		t.errLines = t.errLines[len(t.errLines)-50:]
	}
	t.mu.Unlock()
	if rest, ok := strings.CutPrefix(line, "STATUS "); ok {
		select {
		case t.statusCh <- strings.TrimSpace(rest):
		default: // drop if unread; WaitStatus re-checks exited state
		}
	}
}

// WaitStatus blocks until the tunnel reports a status line whose first
// field is want, the process exits, or the timeout elapses. A timeout of
// zero or less means no deadline.
func (t *Tunnel) WaitStatus(ctx context.Context, want string, timeout time.Duration) (string, error) {
	var deadline <-chan time.Time
	if timeout > 0 {
		deadline = time.After(timeout)
	}
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline:
			return "", fmt.Errorf("daemon: tunnel did not report %q within %s (last: %s)",
				want, timeout, t.LastStatuses())
		case st := <-t.statusCh:
			if field := strings.Fields(st); len(field) > 0 && field[0] == want {
				return st, nil
			}
		case err := <-t.exited:
			// Re-drain statuses emitted just before exit.
			for {
				select {
				case st := <-t.statusCh:
					if field := strings.Fields(st); len(field) > 0 && field[0] == want {
						return st, nil
					}
					continue
				default:
				}
				return "", fmt.Errorf("daemon: tunnel exited before %q: %v (status: %s)", want, err, t.LastStatuses())
			}
		}
	}
}

// WaitAnyStatus blocks until the tunnel reports any of the wanted statuses
// (matched on the first field), the process exits, or ctx is done. A
// timeout of zero or less means no deadline. Returns the matched status line.
func (t *Tunnel) WaitAnyStatus(ctx context.Context, timeout time.Duration, want ...string) (string, error) {
	var deadline <-chan time.Time
	if timeout > 0 {
		deadline = time.After(timeout)
	}
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline:
			return "", fmt.Errorf("daemon: tunnel did not report %v within %s (last: %s)",
				want, timeout, t.LastStatuses())
		case st := <-t.statusCh:
			if fields := strings.Fields(st); len(fields) > 0 {
				if slices.Contains(want, fields[0]) {
					return st, nil
				}
			}
		case err := <-t.exited:
			for {
				select {
				case st := <-t.statusCh:
					if fields := strings.Fields(st); len(fields) > 0 {
						if slices.Contains(want, fields[0]) {
							return st, nil
						}
					}
					continue
				default:
				}
				return "", fmt.Errorf("daemon: tunnel exited before %v: %v (status: %s)", want, err, t.LastStatuses())
			}
		}
	}
}

// StatusCh exposes the raw status channel for event loops.
func (t *Tunnel) StatusCh() <-chan string { return t.statusCh }

// Exited returns a channel that receives the process exit error once.
func (t *Tunnel) Exited() <-chan error { return t.exited }

// Stdin is the frame destination for data flowing toward the remote peer.
func (t *Tunnel) Stdin() io.WriteCloser { return t.stdin }

// Stdout is the frame source for data flowing from the remote peer.
func (t *Tunnel) Stdout() io.ReadCloser { return t.stdout }

// LastStatuses returns the trailing stderr lines for diagnostics.
func (t *Tunnel) LastStatuses() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.Join(t.errLines, " | ")
}

// Close shuts the tunnel down in stages: closing stdin lets the process
// flush any pending BYE frame to the remote peer and exit on its own; only
// after a grace period do we kill it. An immediate kill would drop the BYE
// on the wire and leave the peer waiting on a silent connection.
func (t *Tunnel) Close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	t.mu.Unlock()

	_ = t.stdin.Close()
	select {
	case <-t.exited:
		return
	case <-time.After(2 * time.Second):
	}
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	select {
	case <-t.exited:
	case <-time.After(time.Second):
	}
}
