package daemon

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"

	"github.com/mark3labs/kit/internal/clipboard"
)

// Client-side session driving, independent of how the frames get to the
// daemon. RunHost feeds this a sidecar stream; RunLocal feeds it a Unix
// socket. Everything below the transport — raw mode, the input pumps, the
// chord table, clipboard interception, resize handling — is shared.

// SessionEntry describes one live session on a daemon, as reported by the
// session list. It is the daemon's own type so the picker can be supplied
// by the caller: the UI layer converts to its own view model, which keeps
// internal/daemon free of a dependency on internal/ui.
type SessionEntry struct {
	ID      uint64
	Clients int
	Started time.Time
	Cwd     string
	Name    string
	// Host is the saved host name a session belongs to. Empty for the
	// local daemon; set by the hub when several daemons are listed at once.
	Host string
}

// SessionChoice is what a picker returns.
type SessionChoice struct {
	// ID is the logical session to attach to. Zero means "start a new
	// session" unless Cancel is set.
	ID uint64
	// Host names the daemon the session belongs to, for the multi-host
	// picker. Empty means the daemon currently connected.
	Host string
	// Cancel reports that the user dismissed the picker.
	Cancel bool
}

// SessionPicker presents live sessions and returns the user's choice. It
// runs with the terminal in cooked mode and owns the screen while it runs.
//
// input is the terminal input the picker must read from. The client keeps
// a single reader on os.Stdin for its whole life, so a picker that opened
// its own would race it for keystrokes.
type SessionPicker func(entries []SessionEntry, input *os.File) (SessionChoice, error)

// AttachOptions configures a client attach loop.
type AttachOptions struct {
	// Name identifies the daemon in user-facing messages.
	Name string
	// Host is the saved host name this client is connected to, matching
	// the Host the picker reports for this daemon's own sessions. Empty
	// means the local daemon. It is how a cross-host choice is told apart
	// from one on this daemon, so it must be the picker's value, not a
	// display string.
	Host string
	// Reattach is the command that reattaches to this daemon, quoted in
	// the message printed after a detach.
	Reattach string
	// Pick chooses among live sessions. When nil, the client always
	// starts a new session and in-session switching is disabled.
	Pick SessionPicker
	// Target selects a session up front: zero consults Pick, and any
	// other value attaches directly.
	Target uint64
	// ForceNew skips the picker and starts a new session.
	ForceNew bool
	// Hub, when set, handles a cross-host switch request.
	Hub SessionPicker
	// HubEntries supplies sessions from other hosts for the hub picker.
	HubEntries func() []SessionEntry
}

// attachOutcome reports why a single attached session stopped.
type attachOutcome struct {
	// detached means the user detached; the session keeps running.
	detached bool
	// switchTo is set when the user asked for another session. Zero with
	// wantSwitch means "start a new one".
	wantSwitch bool
	switchTo   uint64
	switchHost string
	// ended means the remote session finished on its own.
	ended bool
}

// clientConn owns a frame stream to one daemon: a single reader goroutine
// demultiplexes control frames to a channel and session data to the
// terminal. It survives across session switches, so switching does not
// reconnect.
type clientConn struct {
	rw   io.ReadWriter
	sink *frameSink

	ctrlCh chan Frame

	attached atomic.Bool

	endedCh   chan struct{}
	endedOnce sync.Once
	closedCh  chan struct{}
	closeOnce sync.Once

	// curID is the logical session this connection is bound to, needed to
	// resolve the relative cycle chords.
	curID atomic.Uint64

	// stdinCh carries terminal input for the connection's whole life.
	//
	// The reader behind it must NOT be per-attach: os.Stdin.Read blocks
	// and cannot be cancelled, so a reader started for each attached
	// session would still be parked in Read after a session switch. Two
	// readers on one fd split the keystrokes between them at random, and
	// the ones delivered to a finished session's pump are simply dropped
	// — chords stop working after the first switch, more so with each
	// one. One reader per connection, shared by every session, is the
	// only arrangement that keeps input whole.
	stdinCh  chan []byte
	stdinErr chan error

	// divert, when non-nil, sends terminal input to a running picker
	// instead of the session pump.
	divertMu sync.Mutex
	divert   chan []byte
}

// readStdin starts the connection's single terminal reader. Chunks go to
// the session pump, or to the picker while one is on screen.
func (c *clientConn) readStdin() {
	go func() {
		defer close(c.stdinCh)
		buf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				if ch := c.divertTarget(); ch != nil {
					ch <- chunk
				} else {
					c.stdinCh <- chunk
				}
			}
			if err != nil {
				c.stdinErr <- err
				return
			}
		}
	}()
}

// pickerTTY diverts terminal input to a picker for as long as it is open.
//
// The picker is a Bubble Tea program and needs a REAL terminal on its
// input: with a plain pipe it takes the "input is not a tty" path, where
// newline mapping is disabled (bubbletea tea.go: mapNl is hardcoded
// false). Our terminal is in raw mode, so ONLCR is off too and every
// rendered line would step one column further right — the frame arrives
// as a staircase.
//
// Allocating a pty pair solves it honestly: the picker reads the slave,
// which is a genuine terminal, while the connection's single reader keeps
// ownership of the real stdin and forwards keystrokes into the master.
type pickerTTY struct {
	conn   *clientConn
	master *os.File
	slave  *os.File
	ch     chan []byte
	done   chan struct{}
}

// pickerInput opens a pty-backed input channel for a picker.
func (c *clientConn) pickerInput() (*pickerTTY, error) {
	master, slave, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("daemon: picker tty: %w", err)
	}
	p := &pickerTTY{
		conn:   c,
		master: master,
		slave:  slave,
		ch:     make(chan []byte, 8),
		done:   make(chan struct{}),
	}
	// Forward diverted keystrokes into the pty until the picker closes.
	go func() {
		for {
			select {
			case chunk, ok := <-p.ch:
				if !ok {
					return
				}
				if _, werr := master.Write(chunk); werr != nil {
					return
				}
			case <-p.done:
				return
			}
		}
	}()
	c.divertMu.Lock()
	c.divert = p.ch
	c.divertMu.Unlock()
	return p, nil
}

// File is the terminal handed to the picker.
func (p *pickerTTY) File() *os.File { return p.slave }

// Close restores input to the session pump and releases the pty.
func (p *pickerTTY) Close() {
	p.conn.divertMu.Lock()
	if p.conn.divert == p.ch {
		p.conn.divert = nil
	}
	p.conn.divertMu.Unlock()
	close(p.done)
	_ = p.slave.Close()
	_ = p.master.Close()
}

// divertTarget returns the active picker's input channel, or nil.
func (c *clientConn) divertTarget() chan []byte {
	c.divertMu.Lock()
	defer c.divertMu.Unlock()
	return c.divert
}

func newClientConn(rw io.ReadWriter) *clientConn {
	return &clientConn{
		rw:       rw,
		sink:     newFrameSink(rw),
		ctrlCh:   make(chan Frame, 16),
		endedCh:  make(chan struct{}),
		closedCh: make(chan struct{}),
		stdinCh:  make(chan []byte, 8),
		stdinErr: make(chan error, 1),
	}
}

func (c *clientConn) sessionEnded() { c.endedOnce.Do(func() { close(c.endedCh) }) }
func (c *clientConn) streamClosed() { c.closeOnce.Do(func() { close(c.closedCh) }) }

func (c *clientConn) setCurrent(id uint64) { c.curID.Store(id) }
func (c *clientConn) current() uint64      { return c.curID.Load() }

// write sends one frame. Clients have no wire-id allocator: the session
// field is always zero and the daemon stamps the real id.
func (c *clientConn) write(t FrameType, payload []byte) error {
	return c.sink.write(Frame{Type: t, Session: 0, Payload: payload})
}

// readLoop demultiplexes the daemon's frames until the stream ends.
func (c *clientConn) readLoop() {
	defer c.streamClosed()
	for {
		frame, err := ReadFrame(c.rw)
		if err != nil {
			return
		}
		switch frame.Type {
		case FrameData:
			// Data is dropped unless a session is attached; otherwise it
			// would scribble over a picker that owns the screen.
			if c.attached.Load() {
				if _, werr := os.Stdout.Write(frame.Payload); werr != nil {
					return
				}
			}
		case FrameSessionListReply, FrameSessionAttachAck:
			select {
			case c.ctrlCh <- frame:
			default: // a stale reply nobody is waiting for
			}
		case FrameBye, FrameSessionClosed:
			c.sessionEnded()
			return
		}
	}
}

var (
	errSessionEnded = errors.New("remote session ended")
	errStreamClosed = errors.New("daemon: connection closed")
)

// awaitCtrl waits for a control frame of the given type.
func (c *clientConn) awaitCtrl(want FrameType, timeout time.Duration) (Frame, error) {
	deadline := time.After(timeout)
	for {
		select {
		case f := <-c.ctrlCh:
			if f.Type == want {
				return f, nil
			}
		case <-deadline:
			return Frame{}, fmt.Errorf("daemon: no answer to request %#x", want)
		case <-c.endedCh:
			return Frame{}, errSessionEnded
		case <-c.closedCh:
			return Frame{}, errStreamClosed
		}
	}
}

// listSessions asks the daemon for its live sessions.
func (c *clientConn) listSessions() ([]SessionEntry, error) {
	return c.listSessionsWithin(10 * time.Second)
}

// listSessionsWithin is listSessions bounded by an explicit timeout.
func (c *clientConn) listSessionsWithin(timeout time.Duration) ([]SessionEntry, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("daemon: no time left to list sessions")
	}
	if err := c.write(FrameSessionList, nil); err != nil {
		return nil, err
	}
	reply, err := c.awaitCtrl(FrameSessionListReply, timeout)
	if err != nil {
		return nil, err
	}
	var raw []sessionInfo
	if err := json.Unmarshal(reply.Payload, &raw); err != nil {
		return nil, fmt.Errorf("daemon: bad session list: %w", err)
	}
	entries := make([]SessionEntry, len(raw))
	for i, si := range raw {
		started, _ := time.Parse(time.RFC3339, si.Started)
		entries[i] = SessionEntry{
			ID:      si.ID,
			Clients: si.Clients,
			Started: started,
			Cwd:     si.Cwd,
			Name:    si.Name,
		}
	}
	return entries, nil
}

// attach binds this connection to a session. Logical id 0 spawns a new one.
// Returns the id the daemon assigned.
func (c *clientConn) attach(id uint64) (uint64, error) {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, id)
	if err := c.write(FrameSessionAttach, payload); err != nil {
		return 0, err
	}
	ack, err := c.awaitCtrl(FrameSessionAttachAck, 10*time.Second)
	if err != nil {
		return 0, err
	}
	if len(ack.Payload) < 9 || ack.Payload[8] != 1 {
		return 0, fmt.Errorf("the daemon refused the attach")
	}
	return binary.BigEndian.Uint64(ack.Payload[:8]), nil
}

// chooseSession runs the picker when sessions exist, and short-circuits to
// a new session when none do.
func chooseSession(conn *clientConn, opts AttachOptions) (SessionChoice, error) {
	// A choice that did not come from a picker is on this daemon by
	// definition, so it carries this client's host. Leaving it empty would
	// make hostSwitch read every --host attach as a switch to the local
	// daemon.
	here := func(id uint64) SessionChoice {
		return SessionChoice{ID: id, Host: opts.Host}
	}
	if opts.ForceNew || opts.Pick == nil {
		return here(0), nil
	}
	if opts.Target != 0 {
		return here(opts.Target), nil
	}
	entries, err := conn.listSessions()
	if err != nil {
		return SessionChoice{}, err
	}
	if len(entries) == 0 {
		return here(0), nil // nothing live: straight to a new session
	}
	// This daemon's own sessions are reported with an empty host; tag them
	// so they are distinguishable from another daemon's rows.
	for i := range entries {
		entries[i].Host = opts.Host
	}
	if opts.HubEntries != nil {
		entries = append(entries, opts.HubEntries()...)
	}
	return runPicker(conn, opts.Pick, entries)
}

// runPicker runs one picker with input diverted from the session pump.
//
// The terminal is put in raw mode here rather than left to the picker: the
// picker reads a pty slave, so its own termios setup would apply to that
// pty instead of the user's terminal.
func runPicker(conn *clientConn, pick SessionPicker, entries []SessionEntry) (SessionChoice, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return SessionChoice{Cancel: true}, fmt.Errorf("daemon: raw mode: %w", err)
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	tty, err := conn.pickerInput()
	if err != nil {
		return SessionChoice{Cancel: true}, err
	}
	defer tty.Close()
	return pick(entries, tty.File())
}

// RunClient drives a daemon connection for the whole client session: pick a
// session, attach, run, and repeat when the user switches. It returns when
// the user detaches, the session ends, or the connection drops.
//
// The caller owns rw and closes it.
func RunClient(ctx context.Context, rw io.ReadWriter, opts AttachOptions) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("attaching to a session needs an interactive terminal")
	}

	// The alt screen is entered once for the whole attachment and left on
	// the way out, so the user's shell scrollback comes back untouched.
	_, _ = os.Stdout.WriteString(altScreenEnter)
	defer func() {
		_, _ = os.Stdout.WriteString(terminalResetSeq + altScreenLeave)
	}()

	conn := newClientConn(rw)
	go conn.readLoop()
	conn.readStdin()

	choice, err := chooseSession(conn, opts)
	if err != nil {
		return err
	}
	if choice.Cancel {
		return nil
	}
	if sw := hostSwitch(opts, choice); sw != nil {
		return sw
	}

	for {
		select {
		case <-conn.endedCh:
			return errSessionEnded
		case <-conn.closedCh:
			return errStreamClosed
		default:
		}

		if sw := hostSwitch(opts, choice); sw != nil {
			return sw
		}

		if assigned, err := conn.attach(choice.ID); err != nil {
			return err
		} else {
			conn.setCurrent(assigned)
		}

		out, err := runAttached(ctx, conn, opts)
		if err != nil {
			return err
		}
		switch {
		case out.wantSwitch:
			next, cancelled, rerr := resolveSwitch(conn, opts, out)
			if rerr != nil {
				return rerr
			}
			if cancelled {
				// The picker was dismissed: stay on the session we were
				// on rather than dropping the user back to the shell.
				continue
			}
			// Release the current session before binding the next one.
			// The daemon rebinds a wire id on its own, but detaching
			// first keeps the session's client count honest for anyone
			// else listing sessions in between.
			if err := conn.write(FrameSessionDetach, nil); err != nil {
				return err
			}
			choice = next
			continue
		case out.detached:
			fmt.Fprintf(os.Stderr, "\nDetached — the session keeps running on the daemon. Reattach with: %s\n", opts.Reattach)
			return nil
		case out.ended:
			fmt.Fprintln(os.Stderr, "\nSession ended.")
			return nil
		default:
			return nil
		}
	}
}

// ErrSwitchHost reports that the user chose a session on a different
// daemon. The client speaks to exactly one daemon, so honouring the choice
// means the caller dials the new host and starts a fresh client; a
// SessionChoice.Host that this connection cannot reach must never be
// attached by ID alone, because every daemon numbers its sessions from 1
// and the ID would silently resolve to a different session here.
type ErrSwitchHost struct {
	// Host is the saved name of the daemon to connect to.
	Host string
	// Session is the logical session to attach to on that daemon.
	Session uint64
}

func (e *ErrSwitchHost) Error() string {
	return fmt.Sprintf("switch to session %d on host %q", e.Session, e.Host)
}

// hostSwitch reports a choice that belongs to another daemon.
//
// A session on another daemon cannot be attached over this connection:
// session ids are per-daemon and every daemon counts from 1, so sending
// the id here would silently bind a different session.
func hostSwitch(opts AttachOptions, choice SessionChoice) *ErrSwitchHost {
	if choice.Host == opts.Host {
		return nil
	}
	return &ErrSwitchHost{Host: choice.Host, Session: choice.ID}
}

// resolveSwitch turns a switch outcome into the next session to attach to.
// The chord handlers cannot run a picker themselves — the terminal is still
// in raw mode while they run — so they hand back a sentinel and the work
// happens here, after runAttached has restored the terminal.
func resolveSwitch(conn *clientConn, opts AttachOptions, out attachOutcome) (SessionChoice, bool, error) {
	switch out.switchTo {
	case pickSentinel:
		entries, err := conn.listSessions()
		if err != nil {
			return SessionChoice{}, false, err
		}
		for i := range entries {
			entries[i].Host = opts.Host
		}
		pick := opts.Pick
		if out.switchHost == hubMarker {
			if opts.HubEntries != nil {
				entries = append(entries, opts.HubEntries()...)
			}
			pick = opts.Hub
		}
		if pick == nil {
			return SessionChoice{}, true, nil
		}
		choice, err := runPicker(conn, pick, entries)
		if err != nil {
			return SessionChoice{}, false, err
		}
		return choice, choice.Cancel, nil

	case cycleNext, cyclePrev:
		entries, err := conn.listSessions()
		if err != nil {
			return SessionChoice{}, false, err
		}
		if len(entries) < 2 {
			return SessionChoice{}, true, nil // nothing to cycle to
		}
		next := neighbourSession(entries, conn.current(), out.switchTo == cycleNext)
		return SessionChoice{ID: next, Host: opts.Host}, false, nil

	default:
		// Ctrl-] c and a direct id are always on this daemon.
		return SessionChoice{ID: out.switchTo, Host: opts.Host}, false, nil
	}
}

// hubMarker flags a switch request that should consult the multi-host
// picker rather than the current daemon's own.
const hubMarker = "\x00hub"

// neighbourSession returns the session before or after current in the
// daemon's ordering, wrapping at the ends. A current session that is no
// longer listed falls back to the first entry.
func neighbourSession(entries []SessionEntry, current uint64, forward bool) uint64 {
	idx := -1
	for i, e := range entries {
		if e.ID == current {
			idx = i
			break
		}
	}
	if idx < 0 {
		return entries[0].ID
	}
	if forward {
		return entries[(idx+1)%len(entries)].ID
	}
	return entries[(idx-1+len(entries))%len(entries)].ID
}

// runAttached drives one attached session until the user detaches, asks to
// switch, or the session ends. The terminal is in raw mode for the whole
// call and restored before returning, so a picker can run between sessions.
func runAttached(ctx context.Context, conn *clientConn, opts AttachOptions) (attachOutcome, error) {
	stdinFD := int(os.Stdin.Fd())
	stdoutFD := int(os.Stdout.Fd())

	oldState, err := term.MakeRaw(stdinFD)
	if err != nil {
		return attachOutcome{}, fmt.Errorf("daemon: raw mode: %w", err)
	}
	restore := func() {
		_, _ = os.Stdout.WriteString(terminalResetSeq)
		_ = term.Restore(stdinFD, oldState)
	}
	defer restore()

	conn.attached.Store(true)
	defer conn.attached.Store(false)

	done := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(done) }) }

	var out attachOutcome
	var outMu sync.Mutex
	setOutcome := func(o attachOutcome) {
		outMu.Lock()
		out = o
		outMu.Unlock()
	}

	// Window size: report the current size now and on every SIGWINCH. The
	// daemon applies the minimum across all attached clients.
	stopResize := watchResize(uintptr(stdoutFD), func(cols, rows int) {
		_ = conn.write(FrameResize, EncodeResize(cols, rows))
	})
	defer stopResize()
	if cols, rows, err := term.GetSize(stdoutFD); err == nil {
		_ = conn.write(FrameResize, EncodeResize(cols, rows))
	}
	// Ask the session to repaint: a reattached child has already drawn its
	// screen and would otherwise leave the terminal blank until the next
	// keystroke.
	_ = conn.write(FrameSessionRedraw, nil)

	go runInputPump(conn, opts, finish, setOutcome)

	select {
	case <-done:
	case <-conn.endedCh:
		setOutcome(attachOutcome{ended: true})
	case <-conn.closedCh:
		setOutcome(attachOutcome{ended: true})
	case <-ctx.Done():
	}

	outMu.Lock()
	result := out
	outMu.Unlock()

	// A switch keeps the connection; a plain exit says goodbye.
	if !result.detached && !result.wantSwitch && !result.ended {
		_ = conn.write(FrameBye, nil)
	}
	return result, nil
}

// pumpControl is what the input pump decides to do with a chord.
type pumpControl struct {
	stop    bool
	outcome attachOutcome
}

// runInputPump reads the terminal, intercepts the client's chords, and
// forwards everything else to the session.
func runInputPump(conn *clientConn, opts AttachOptions, finish func(), setOutcome func(attachOutcome)) {
	defer finish()

	// Input comes from the connection's shared reader; see clientConn.
	readCh := conn.stdinCh
	readErr := conn.stdinErr

	scanner := &keyScanner{}
	suppressRel := false // swallow the ctrl+v release after a successful image interception
	var leaderBuf []byte // pending chord prefix (nil = none)
	var leaderOf leaderKind
	var idleTimer *time.Timer
	var idleC <-chan time.Time

	armIdle := func() {
		if scanner.PendingEscape() {
			d := max(escIdleFlush-time.Since(scanner.escAt), 0)
			if idleTimer == nil {
				idleTimer = time.NewTimer(d)
			} else {
				idleTimer.Stop()
				idleTimer.Reset(d)
			}
			idleC = idleTimer.C
		} else if idleTimer != nil {
			idleTimer.Stop()
			idleC = nil
		}
	}
	forward := func(data []byte) bool {
		return conn.write(FrameData, data) == nil
	}
	handle := func(ev keyEvent) bool { // false = write error, give up
		if ev.Paste {
			// Image paste: read the local clipboard and stream any image
			// to the daemon. No image — forward the keystroke so the host
			// keeps its normal Ctrl-V behavior.
			img, imgErr := clipboard.ReadImage()
			if imgErr == nil && len(img.Data) > 0 {
				for _, p := range EncodeClipboardChunks(img.MediaType, img.Data) {
					if werr := conn.write(FrameClipboard, p); werr != nil {
						return false
					}
				}
				suppressRel = true // the matching release is ours
				fmt.Fprintln(os.Stderr, "Image sent from local clipboard.")
				return true // swallow the press bytes
			}
			suppressRel = false // forwarding the press; forward its release too
		}
		if ev.Release && suppressRel {
			suppressRel = false
			return true
		}
		if len(ev.Data) > 0 {
			return forward(ev.Data)
		}
		return true
	}

	armIdle()
	for {
		select {
		case chunk, ok := <-readCh:
			if !ok {
				return
			}
			for _, ev := range scanner.Feed(chunk) {
				if leaderBuf != nil {
					// The kitty keyboard protocol delivers the leader's
					// RELEASE before the chord suffix. Buffer it with the
					// prefix so the chord stays armed; a suffix that is
					// not ours is forwarded with the whole prefix run,
					// byte-identical.
					if ev.Leader || ev.LeaderRelease {
						leaderBuf = append(leaderBuf, ev.Data...)
						continue
					}
					ctrl, claimed := dispatchChord(conn, opts, leaderOf, ev)
					if claimed {
						leaderBuf = nil
						leaderOf = leaderNone
						if ctrl.stop {
							setOutcome(ctrl.outcome)
							return
						}
						continue
					}
					if !forward(leaderBuf) {
						return
					}
					leaderBuf = nil
					leaderOf = leaderNone
				}
				if ev.Leader {
					leaderBuf = append([]byte(nil), ev.Data...)
					leaderOf = ev.Kind
					continue
				}
				if !handle(ev) {
					return
				}
			}
			armIdle()
		case <-idleC:
			idleC = nil
			for _, ev := range scanner.FlushPendingEscape() {
				if !handle(ev) {
					return
				}
			}
			armIdle()
		case <-readErr:
			return
		}
	}
}

// dispatchChord interprets a chord suffix. It reports whether the client
// claimed the chord; an unclaimed chord is forwarded to the session so the
// host TUI's own bindings keep working.
//
// The legacy Ctrl-X leader claims only 'd'. Every other Ctrl-X suffix
// belongs to the host (steer, thinking, move, editor), so claiming more
// would silently break those over a remote link.
func dispatchChord(conn *clientConn, opts AttachOptions, kind leaderKind, ev keyEvent) (pumpControl, bool) {
	if len(ev.Data) != 1 || ev.Release {
		return pumpControl{}, false
	}
	switch ev.Data[0] {
	case 'd':
		if err := conn.write(FrameSessionDetach, nil); err != nil {
			return pumpControl{stop: true}, true
		}
		return pumpControl{stop: true, outcome: attachOutcome{detached: true}}, true
	}
	if kind != leaderPrimary {
		return pumpControl{}, false // the host owns every other Ctrl-X chord
	}
	switch ev.Data[0] {
	case 's':
		if opts.Pick == nil {
			return pumpControl{}, true
		}
		return pumpControl{stop: true, outcome: attachOutcome{wantSwitch: true, switchTo: pickSentinel}}, true
	case 'w':
		if opts.Hub == nil {
			return pumpControl{}, true
		}
		return pumpControl{stop: true, outcome: attachOutcome{
			wantSwitch: true, switchTo: pickSentinel, switchHost: hubMarker,
		}}, true
	case 'c':
		return pumpControl{stop: true, outcome: attachOutcome{wantSwitch: true, switchTo: 0}}, true
	case 'n':
		return pumpControl{stop: true, outcome: attachOutcome{wantSwitch: true, switchTo: cycleNext}}, true
	case 'p':
		return pumpControl{stop: true, outcome: attachOutcome{wantSwitch: true, switchTo: cyclePrev}}, true
	case leaderKey:
		// Ctrl-] Ctrl-] sends a literal Ctrl-] to the session.
		_ = conn.write(FrameData, []byte{leaderKey})
		return pumpControl{}, true
	}
	return pumpControl{}, true // unknown chord: swallowed, not forwarded
}

// pickSentinel marks "ask the picker" in an outcome's switchTo field, and
// cycleNext/cyclePrev mark a relative move. All three sit at the top of the
// uint64 range, where they cannot collide with a real logical id: the
// daemon counts those up from 1.
const (
	pickSentinel = ^uint64(0)
	cycleNext    = ^uint64(0) - 1
	cyclePrev    = ^uint64(0) - 2
)
