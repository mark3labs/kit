package daemon

import (
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
)

// How the daemon reaches a session's terminal.
//
// There are two arrangements, and the difference between them is the
// whole reason a session can now survive its daemon:
//
//   - ptyIO: the daemon opened the PTY itself and holds the master. The
//     session dies with the daemon, because a PTY master cannot be
//     re-opened for an existing slave — nothing can adopt that child, by
//     construction. This is the original arrangement, kept for platforms
//     with no session host and for a daemon that could not start one.
//   - hostIO: a supervisor process holds the master and the daemon holds
//     a socket to it. Stopping the daemon costs the socket and nothing
//     else; the next daemon dials the same socket and is back where the
//     last one was.
//
// Everything above this interface is identical for both, which is what
// keeps the session table, the frame loop and the client protocol free of
// the distinction.
type sessionIO interface {
	// Read returns the session child's output, exactly as a PTY master
	// would. A returned error means the session has ended.
	io.Reader
	// Write sends terminal input to the session child.
	io.Writer
	// Close releases the daemon's end WITHOUT ending the session. For a
	// hosted session this is a detach: the supervisor and its child carry
	// on. For a PTY-backed one it is fatal, because there is no second
	// holder of the master.
	io.Closer

	// Resize sets the session's terminal size.
	Resize(ws winSize) error
	// Redraw makes the session repaint into a client that has just taken
	// over the screen, and restores whatever terminal state that client
	// never saw the child set.
	Redraw(ws winSize)
	// Rename records a display name where the session's own state lives,
	// so it survives a daemon restart along with the session.
	Rename(name string)
	// Terminate ends the session for good: the child is asked to exit,
	// then made to.
	Terminate()
	// Wait blocks until the session has ended.
	Wait()
	// Hosted reports whether the session outlives this daemon.
	Hosted() bool
	// PID identifies the process to record in the session registry: the
	// supervisor for a hosted session, the child itself otherwise.
	PID() int
}

// ptyIO is a session whose PTY master this daemon holds.
type ptyIO struct {
	cmd  *exec.Cmd
	ptmx *os.File
}

func (p *ptyIO) Read(b []byte) (int, error)  { return p.ptmx.Read(b) }
func (p *ptyIO) Write(b []byte) (int, error) { return p.ptmx.Write(b) }
func (p *ptyIO) Close() error                { return p.ptmx.Close() }
func (p *ptyIO) Hosted() bool                { return false }

func (p *ptyIO) PID() int {
	if p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// Resize sets the PTY size, ignoring the zero size that means no attached
// client has reported one yet.
func (p *ptyIO) Resize(ws winSize) error {
	if ws.cols == 0 || ws.rows == 0 {
		return nil
	}
	return pty.Setsize(p.ptmx, &pty.Winsize{Cols: uint16(ws.cols), Rows: uint16(ws.rows)})
}

// Redraw makes the child repaint by changing the PTY size and putting it
// back. A full-screen TUI redraws on SIGWINCH, which is the only portable
// way to force a repaint of a child we do not emulate.
//
// Both size changes happen here rather than at the client, so the gap
// between them is a local sleep instead of two network round trips. The
// child needs to observe two distinct sizes: setting the same size twice
// is not a change and produces no repaint.
func (p *ptyIO) Redraw(ws winSize) {
	if ws.cols < 2 || ws.rows < 2 {
		return
	}
	go func() {
		_ = pty.Setsize(p.ptmx, &pty.Winsize{Cols: uint16(ws.cols), Rows: uint16(ws.rows - 1)})
		time.Sleep(redrawNudgeGap)
		_ = pty.Setsize(p.ptmx, &pty.Winsize{Cols: uint16(ws.cols), Rows: uint16(ws.rows)})
	}()
}

// Rename is a no-op for a PTY-backed session: its name lives in the
// daemon's own table, and the session cannot outlive that table anyway.
func (p *ptyIO) Rename(string) {}

// Wait blocks until the child exits.
func (p *ptyIO) Wait() {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_, _ = p.cmd.Process.Wait()
}

// Terminate closes the PTY and ends the child.
func (p *ptyIO) Terminate() {
	_ = p.ptmx.Close()
	if pid := p.PID(); pid > 0 {
		// Off the caller's goroutine: Terminate runs inline on the frame
		// loop when a write to the session fails, and waiting out the
		// grace period there would stall that client for seconds.
		go terminateProcess(pid)
	}
}

// redrawNudgeGap is how long the child gets to observe the intermediate
// size during a redraw nudge. Long enough for a SIGWINCH to be delivered
// and acted on, short enough not to be seen as a resize by the user.
const redrawNudgeGap = 40 * time.Millisecond
