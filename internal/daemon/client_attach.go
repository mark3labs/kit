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

	"charm.land/lipgloss/v2"
	"github.com/creack/pty"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"

	"github.com/mark3labs/kit/internal/clipboard"
	"github.com/mark3labs/kit/internal/ui/termgfx"
)

// Client-side session driving, independent of how the frames get to the
// daemon. RunHost feeds this an iroh stream; RunLocal feeds it a Unix
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
//
// ctx cancels the picker. It is the one blocking call in an attached
// client that the user cannot always end from the keyboard, so it has to
// honour the same cancellation as the session loop around it.
type SessionPicker func(ctx context.Context, entries []SessionEntry, input *os.File) (SessionChoice, error)

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
	// The context bounds the queries it makes.
	HubEntries func(ctx context.Context) []SessionEntry
	// Spec describes how a NEW session should be started: the directory
	// the user ran the command in, the arguments they gave it, and the
	// environment it needs. Nil asks for the daemon's default, which is
	// the directory picker in the daemon user's home.
	//
	// Honoured on the local socket only. RunHost clears it: see
	// SessionSpec for why argv must not cross a network boundary.
	Spec *SessionSpec
	// Redial reopens a connection to the SAME daemon after the stream
	// drops, so a daemon that is restarted or upgraded interrupts a
	// session instead of ending it. Nil disables reconnection, and the
	// client then reports a lost connection the way it always did.
	//
	// It must dial the same daemon the client started against: a
	// reconnect reattaches by logical session id, and every daemon numbers
	// its sessions from 1, so a redial that landed somewhere else would
	// silently drop the user into a stranger's session.
	Redial Redialer
}

// Redialer opens a fresh connection to a daemon. It returns the frame
// stream and a function that releases it; the caller owns both.
type Redialer func(ctx context.Context) (io.ReadWriter, func(), error)

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
	// lost means the CONNECTION went away without the session ending: the
	// daemon was stopped, restarted or killed.
	//
	// It is deliberately not the same as ended. Sessions outlive their
	// daemon now, so reporting a lost connection as a finished session
	// would tell the user their work is gone while it is still running,
	// and would stop the client reconnecting to it.
	lost bool
}

// clientConn owns a frame stream to one daemon: a single reader goroutine
// demultiplexes control frames to a channel and session data to the
// terminal. It survives across session switches, so switching does not
// reconnect.
type clientConn struct {
	rw   io.ReadWriter
	sink *frameSink

	ctrlCh chan Frame

	// helloCh carries the daemon's hello to whoever asks for it, and
	// helloOnce makes sure a second reply (a daemon that answers twice, a
	// reconnect racing a stale frame) cannot block the read loop.
	helloCh   chan Hello
	helloOnce sync.Once

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
	// The reader behind it must NOT be per-attach: a reader started for
	// each attached session would still be parked in Read after a session
	// switch. Two readers on one fd split the keystrokes between them at
	// random, and the ones delivered to a finished session's pump are
	// simply dropped — chords stop working after the first switch, more so
	// with each one. One reader per connection, shared by every session,
	// is the only arrangement that keeps input whole.
	stdinCh  chan []byte
	stdinErr chan error

	// stdinReader is the cancellable reader behind stdinCh.
	//
	// os.Stdin.Read cannot be interrupted, and a goroutine parked in it
	// holds the file's read lock for as long as it stays parked. That lock
	// outlives this client: a connection that ends in an error hands
	// control back to a caller that may need the terminal itself — the
	// error renderer queries the terminal's background colour, and a
	// cross-host switch starts a second client with its own reader — and
	// that caller then blocks behind our reader forever. Reading through a
	// cancel reader lets stopStdin give the terminal back.
	stdinReader cancelreader.CancelReader
	// stdinStop asks the reader to stop; stdinDone reports that it has.
	// Closing a cancel reader while a read is in flight is a data race on
	// the file, so the two are separate: stopStdin cancels, waits for the
	// goroutine to leave, and only then closes.
	stdinStop     chan struct{}
	stdinDone     chan struct{}
	stdinStopOnce sync.Once
	// stdinStopped is closed once stopStdin has finished, and
	// stdinReleased records whether the terminal actually came back.
	// Concurrent callers wait on the first so none of them reports a
	// hand-off that has not happened yet.
	stdinStopped  chan struct{}
	stdinReleased atomic.Bool

	// divert, when non-nil, sends terminal input to a running picker
	// instead of the session pump.
	divertMu sync.Mutex
	divert   *pickerTTY
}

// readStdin starts the connection's single terminal reader. Chunks go to
// the session pump, or to the picker while one is on screen.
//
// The reader must be cancellable, so a terminal that will not take a
// cancel reader is a setup failure rather than something to work around.
// Falling back to reading os.Stdin directly would start a goroutine that
// nothing can stop, and stopStdin would return having released nothing —
// the captured terminal this whole path exists to prevent, reintroduced
// silently on the one path nobody exercises. RunClient has already
// established that stdin is a terminal by the time we are called, so this
// error means the terminal is genuinely unusable to us.
//
// ctx releases the terminal as soon as it is cancelled. The client also
// stops the reader when it unwinds, but that can trail the cancellation by
// as long as an in-flight daemon request takes to time out, and a caller
// that cancelled is usually a caller that wants stdin back now.
func (c *clientConn) readStdin(ctx context.Context) error {
	src, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		return fmt.Errorf("daemon: this terminal cannot be read cancellably: %w", err)
	}
	c.stdinReader = src
	go func() {
		select {
		case <-ctx.Done():
			c.stopStdin()
		case <-c.stdinStop: // stopped by the client instead
		}
	}()
	go func() {
		// stdinDone is registered first so it closes last: a waiter woken
		// by it must find the goroutine completely finished with the
		// reader, not merely past the channel close.
		defer close(c.stdinDone)
		defer close(c.stdinCh)
		buf := make([]byte, 256)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				// The picker can close between divertTarget and the
				// send, and its channel is buffered, so a blind send can
				// park here forever with no receiver — which would kill
				// keyboard input for the rest of the process. Abandon the
				// keystroke if the picker is gone, or if we are stopping:
				// a send that cannot be abandoned would outlive the
				// client and deadlock stopStdin.
				if pt := c.divertPicker(); pt != nil {
					select {
					case pt.ch <- chunk:
					case <-pt.done:
					case <-c.stdinStop:
						return
					}
				} else {
					select {
					case c.stdinCh <- chunk:
					case <-c.stdinStop:
						return
					}
				}
			}
			if err != nil {
				select {
				case c.stdinErr <- err:
				default: // nobody left to tell
				}
				return
			}
			select {
			case <-c.stdinStop:
				return
			default:
			}
		}
	}()
	return nil
}

// stopStdin ends the terminal reader and releases stdin.
//
// Safe to call more than once and from any goroutine; every caller gets
// the same answer. The reader is cancelled first and closed only once the
// goroutine has left it: closing a cancel reader under an in-flight read
// is a data race on the file.
//
// Reports whether the terminal was actually released. Cancel is not
// guaranteed to work — the fallback reader always refuses, and the
// Windows one gives up if a read is wedged — and a caller that hands the
// terminal to another client on a false promise gets the stolen-keystroke
// bug this whole path exists to prevent. Callers that pass stdin on must
// check it; callers that are exiting the process need not.
func (c *clientConn) stopStdin() bool {
	c.stdinStopOnce.Do(func() {
		close(c.stdinStop)
		if c.stdinReader == nil {
			c.stdinReleased.Store(true) // readStdin never ran: nothing holds stdin
			close(c.stdinStopped)
			return
		}
		c.stdinReader.Cancel()
		select {
		case <-c.stdinDone:
			_ = c.stdinReader.Close()
			c.stdinReleased.Store(true)
		case <-time.After(2 * time.Second):
			// The reader did not acknowledge the cancel. Leaving its fd
			// open costs one descriptor for the rest of the process;
			// closing it under a live read would corrupt an unrelated
			// file the descriptor is later reused for.
		}
		close(c.stdinStopped)
	})
	<-c.stdinStopped // a concurrent caller must not answer before the result is known
	return c.stdinReleased.Load()
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
	c.divert = p
	c.divertMu.Unlock()
	return p, nil
}

// File is the terminal handed to the picker.
func (p *pickerTTY) File() *os.File { return p.slave }

// Close restores input to the session pump and releases the pty.
func (p *pickerTTY) Close() {
	p.conn.divertMu.Lock()
	if p.conn.divert == p {
		p.conn.divert = nil
	}
	p.conn.divertMu.Unlock()
	close(p.done)
	_ = p.slave.Close()
	_ = p.master.Close()
}

// divertPicker returns the active picker, or nil when none is running.
// The reader needs the picker itself, not just its channel, so it can
// observe the done signal while trying to send.
func (c *clientConn) divertPicker() *pickerTTY {
	c.divertMu.Lock()
	defer c.divertMu.Unlock()
	return c.divert
}

func newClientConn(rw io.ReadWriter) *clientConn {
	return &clientConn{
		rw:           rw,
		sink:         newFrameSink(rw),
		ctrlCh:       make(chan Frame, 16),
		helloCh:      make(chan Hello, 1),
		endedCh:      make(chan struct{}),
		closedCh:     make(chan struct{}),
		stdinCh:      make(chan []byte, 8),
		stdinErr:     make(chan error, 1),
		stdinStop:    make(chan struct{}),
		stdinDone:    make(chan struct{}),
		stdinStopped: make(chan struct{}),
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
		case FrameHello:
			peer, herr := DecodeHello(frame.Payload)
			if herr != nil {
				// Unreadable, but it still proves the daemon knows the
				// frame. Report it as a legacy daemon so the waiter is
				// released instead of timing out.
				peer = legacyHello(RoleDaemon)
			}
			peer.Role = RoleDaemon
			c.helloOnce.Do(func() { c.helloCh <- peer; close(c.helloCh) })
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

// greet announces this client to the daemon and waits briefly for the
// daemon's own hello.
//
// The wait is short and its expiry is NOT an error. Every kit released
// before the hello existed answers nothing at all, and those daemons must
// keep working: silence means "legacy daemon, protocol v1, no optional
// features", which is exactly what such a daemon is. A client that
// blocked here would have traded a working connection for a guarantee it
// does not need.
//
// An incompatible daemon IS an error, and it is reported here rather than
// left to fail later as a session that half works.
func (c *clientConn) greet() (Hello, error) {
	payload, err := EncodeHello(localHello(RoleClient))
	if err == nil {
		_ = c.write(FrameHello, payload)
	}
	select {
	case peer := <-c.helloCh:
		if cerr := peer.Compatible(); cerr != nil {
			return peer, cerr
		}
		return peer, nil
	case <-c.closedCh:
		return legacyHello(RoleDaemon), errStreamClosed
	case <-time.After(helloTimeout):
		return legacyHello(RoleDaemon), nil
	}
}

// helloTimeout bounds the wait for the daemon's hello. It is generous
// relative to a local socket round trip and to an already-established
// iroh stream, and short enough that a legacy daemon is not a noticeable
// pause before the first frame is drawn.
const helloTimeout = 1500 * time.Millisecond

// attach binds this connection to a session. Logical id 0 spawns a new one.
//
// Returns the assigned logical id and what the daemon said about whether
// that session outlives it. A daemon too old to send the flag reports
// durabilityUnknown, and the caller falls back to the daemon-wide
// FeatureReattach bit.
func (c *clientConn) attach(id uint64) (assigned uint64, durability sessionDurability, err error) {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, id)
	if werr := c.write(FrameSessionAttach, payload); werr != nil {
		return 0, durabilityUnknown, werr
	}
	ack, err := c.awaitCtrl(FrameSessionAttachAck, 10*time.Second)
	if err != nil {
		return 0, durabilityUnknown, err
	}
	if len(ack.Payload) < 9 || ack.Payload[8] != 1 {
		// The daemon refuses an attach for exactly two reasons, and they
		// need different advice: a session id that is not live any more
		// (mistyped, or ended since it was listed), or a new session it
		// could not start.
		if id == 0 {
			return 0, durabilityUnknown, fmt.Errorf("the daemon could not start a session")
		}
		return 0, durabilityUnknown, fmt.Errorf("no live session %d on this daemon — list the live ones with 'kit ls'", id)
	}
	// The flags byte is optional, so its ABSENCE must not read as "this
	// session does not survive": that is a real answer, and an old daemon
	// has not given one.
	durability = durabilityUnknown
	if len(ack.Payload) >= 10 {
		if ack.Payload[9] == 1 {
			durability = durabilityHosted
		} else {
			durability = durabilityInDaemon
		}
	}
	return binary.BigEndian.Uint64(ack.Payload[:8]), durability, nil
}

// chooseSession runs the picker when sessions exist, and short-circuits to
// a new session when none do.
//
// "None" means none anywhere this invocation can reach — this daemon's own
// sessions AND the other daemons the caller offered through HubEntries.
// Testing the local list alone would hide every remote session behind the
// presence of a local one: with nothing running on this machine the client
// would go straight to a new session, and the sessions waiting on a paired
// host would never be listed, or even asked for.
func chooseSession(ctx context.Context, conn *clientConn, opts AttachOptions) (SessionChoice, error) {
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
	entries = candidateSessions(ctx, entries, opts)
	if len(entries) == 0 {
		return here(0), nil // nothing live anywhere: straight to a new session
	}
	return runPicker(ctx, conn, opts.Pick, entries, opts.Host)
}

// candidateSessions assembles everything one invocation can attach to:
// this daemon's own sessions followed by the other daemons' sessions the
// caller supplied.
//
// The daemon reports its own sessions with an empty host, which is also
// what a local-daemon client reports for itself, so they are tagged here
// to stay distinguishable from another daemon's rows once the two lists
// are merged. hostSwitch reads that tag to decide whether a choice can be
// served on this connection at all.
func candidateSessions(ctx context.Context, local []SessionEntry, opts AttachOptions) []SessionEntry {
	for i := range local {
		local[i].Host = opts.Host
	}
	if opts.HubEntries == nil {
		return local
	}
	return append(local, opts.HubEntries(ctx)...)
}

// runPicker runs one picker with input diverted from the session pump.
//
// The terminal is put in raw mode here rather than left to the picker: the
// picker reads a pty slave, so its own termios setup would apply to that
// pty instead of the user's terminal.
//
// host is the daemon this client is connected to. A picker reports "start
// a new session" as id 0 with no host, because the entry belongs to no
// listed session; that new session is always started on this daemon, so
// the choice is tagged with host here. Left untagged, hostSwitch would
// read it as a switch to the host named "" — the local daemon — which
// 'kit remote --host' cannot serve at all, and 'kit attach --host' would
// serve by silently starting the session on the wrong machine.
func runPicker(ctx context.Context, conn *clientConn, pick SessionPicker, entries []SessionEntry, host string) (SessionChoice, error) {
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
	// The picker owns the alternate screen while it runs and leaves it on
	// exit, which would drop the client's own alt screen with it: Bubble
	// Tea always restores the screen state it entered, so a picker that
	// stayed in the alt screen for its last frame still emits the exit
	// sequence when the program shuts down. Re-enter it here instead of
	// asking the picker not to leave. The session repaints right after
	// (runAttached sends a redraw), so an empty alt screen is never seen.
	defer func() { _, _ = os.Stdout.WriteString(altScreenEnter) }()
	choice, err := pick(ctx, entries, tty.File())
	if err == nil && !choice.Cancel && choice.ID == 0 && choice.Host == "" {
		choice.Host = host
	}
	return choice, err
}

// RunClient drives a daemon connection for the whole client session: pick a
// session, attach, run, and repeat when the user switches. It returns when
// the user detaches or the session ends.
//
// It deliberately OUTLIVES the connection. A daemon that is stopped,
// restarted, upgraded or killed takes its socket with it — but not its
// sessions, which run in supervisor processes of their own and are
// adopted by the next daemon (see FeatureReattach and sessionhost.go). A
// dropped stream is therefore an interruption, not an ending: the client
// redials, reattaches to the same logical session and repaints. Without
// this the sessions would survive a daemon restart and the user would
// still lose every screen they had open, which is most of the point.
//
// The caller owns rw and closes it. Connections opened by a reconnect are
// owned and closed here.
func RunClient(ctx context.Context, rw io.ReadWriter, opts AttachOptions) (err error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("attaching to a session needs an interactive terminal")
	}

	// The alt screen is entered once for the WHOLE client — reconnects
	// included — and left on the way out, so the user's shell scrollback
	// comes back untouched and a redial does not flash the terminal. The
	// parting message is deferred with it: printed inside the alt screen it
	// would be drawn over the session's last frame and then scrubbed away
	// with it.
	var parting string
	_, _ = os.Stdout.WriteString(altScreenEnter)
	defer func() {
		_, _ = os.Stdout.WriteString(terminalResetSeq + altScreenLeave)
		if parting != "" {
			fmt.Fprintln(os.Stderr, parting)
		}
	}()

	// Describe this terminal before anything else takes stdin: the probe
	// is a synchronous OSC query, so it has to finish before a connection's
	// reader owns the fd and before a picker draws. The daemon has no other
	// way to learn any of it — the PTY it owns reports no colour depth and
	// answers no background query. Probed ONCE for the whole client: the
	// terminal does not change under a reconnect, and asking again would
	// mean querying a terminal whose input another reader may still hold.
	localTerm := detectTerminalInfo()

	stream := rw
	// closeStream is set only for connections this function opened. The
	// caller's own rw is never closed here.
	var closeStream func()
	defer func() {
		if closeStream != nil {
			closeStream()
		}
	}()

	for {
		run, rerr := runClientSession(ctx, stream, opts, localTerm)
		parting = run.parting

		// Anything that is not a lost connection is the client's real
		// outcome: a detach, a finished session, a host switch, a failure
		// worth reporting. Reconnecting past any of those would ignore
		// what the user asked for.
		if !errors.Is(rerr, errStreamClosed) || opts.Redial == nil || ctx.Err() != nil {
			return rerr
		}
		// A terminal reader that would not let go makes a second client on
		// this terminal unsafe: two readers on one fd split the user's
		// keystrokes between them at random. Stop rather than reconnect
		// into a session that would drop half of what is typed at it.
		if !run.stdinReleased {
			return fmt.Errorf("%w (this terminal did not release its input; start a new one to continue)", rerr)
		}
		if closeStream != nil {
			closeStream()
			closeStream = nil
		}

		next, closer, cerr := reconnectToDaemon(ctx, opts, run)
		if cerr != nil {
			parting = reconnectParting(opts, run, cerr)
			return nil
		}
		stream, closeStream = next, closer

		// Go straight back to the session we were on. A client that had
		// not attached to anything yet has nothing to resume and falls
		// back into the ordinary choose-a-session path.
		opts.Target, opts.ForceNew = run.current, false
	}
}

// clientRun reports how one connection's worth of client work ended.
// RunClient reads it to decide between reporting and reconnecting.
type clientRun struct {
	// parting is the message to print once the alt screen is gone.
	parting string
	// current is the logical session the client was attached to, and the
	// one a reconnect reattaches to. Zero means it never got that far.
	current uint64
	// stdinReleased records whether the terminal reader gave the terminal
	// back. A reconnect needs it, because it starts a reader of its own.
	stdinReleased bool
	// daemon is what the daemon said about itself, so a lost connection
	// can be described honestly: a daemon with FeatureReattach left the
	// session running, and one without it did not.
	daemon Hello
	// sessionDurability is what the daemon said about whether THIS
	// session outlives it, from the attach ack. A daemon too old to answer
	// leaves it unknown, and the daemon-wide feature bit decides instead.
	sessionDurability sessionDurability
}

// sessionDurability answers whether one session outlives its daemon.
//
// A plain bool cannot express this: "the daemon said no" and "the daemon
// is too old to say" need different answers, and conflating them would
// have every pre-ack daemon report its surviving sessions as lost.
type sessionDurability uint8

const (
	// durabilityUnknown: the daemon sent no flag. It predates the ack
	// byte, so the platform-wide FeatureReattach bit is the best answer
	// available.
	durabilityUnknown sessionDurability = iota
	// durabilityHosted: the session runs in a supervisor process and
	// survives its daemon.
	durabilityHosted
	// durabilityInDaemon: the daemon hosts this session itself, so the
	// session ends when the daemon does — whatever the platform supports
	// in general.
	durabilityInDaemon
)

// survives reports whether the session this client was on outlives the
// daemon that was serving it.
//
// The per-session answer wins when there is one, because a daemon that
// hosts sessions separately can still have fallen back for this one. The
// feature bit is the fallback for a daemon too old to be specific.
func (r clientRun) survives() bool {
	switch r.sessionDurability {
	case durabilityHosted:
		return true
	case durabilityInDaemon:
		return false
	default:
		return r.daemon.Features.Has(FeatureReattach)
	}
}

// runClientSession drives ONE connection: greet, describe the terminal,
// choose a session, and run it until something ends the connection or the
// user leaves.
//
// Everything that belongs to the terminal rather than to the connection —
// the alt screen, the terminal probe, the parting message — is owned by
// RunClient above, so that a reconnect replaces the connection without
// disturbing the screen.
func runClientSession(ctx context.Context, rw io.ReadWriter, opts AttachOptions, localTerm TerminalInfo) (run clientRun, err error) {
	conn := newClientConn(rw)
	go conn.readLoop()
	if serr := conn.readStdin(ctx); serr != nil {
		// No reader was ever started, so the terminal is still free.
		run.stdinReleased = true
		return run, serr
	}
	// Give the terminal back on the way out. Everything above this may
	// hand control to a caller that reads stdin itself — a cross-host
	// switch starts a second client, a reconnect starts a second reader,
	// and an error returned from here is rendered by a printer that queries
	// the terminal — and a reader still parked on stdin would swallow
	// their input, or deadlock them.
	//
	// A hand-off that cannot be made is recorded rather than hidden: the
	// next client would look alive while every keystroke went to this one.
	defer func() {
		run.stdinReleased = conn.stopStdin()
		run.current = conn.current()
		if run.stdinReleased || err == nil || errors.Is(err, errSessionEnded) ||
			errors.Is(err, errStreamClosed) {
			// Either the hand-off worked, or nothing is going to run after
			// us that needs the terminal. A lost connection is excluded
			// too: RunClient checks stdinReleased itself before it starts
			// a second reader, and says so in its own words.
			return
		}
		err = fmt.Errorf("%w (this terminal did not release its input; start a new one to continue)", err)
	}()

	// Announce ourselves and learn what this daemon can do. A daemon too
	// old to answer is not an error: see greet.
	peer, herr := conn.greet()
	run.daemon = peer
	if herr != nil {
		return run, herr
	}

	// Sent before any attach so a spawned child starts out describing this
	// terminal. A daemon too old to know the frame ignores it.
	if payload, terr := EncodeTerminalInfo(localTerm); terr == nil {
		_ = conn.write(FrameTerminal, payload)
	}
	// Likewise the spec: it has to be on record before the attach that
	// spawns the child reads it. A daemon too old to know this frame drops
	// it and starts the session in the home directory behind the picker,
	// which is what every session did before specs existed.
	if opts.Spec != nil {
		if spec, ok := specFits(*opts.Spec); !ok {
			fmt.Fprintln(os.Stderr, "This command line is too long to hand to the daemon; starting with the directory picker instead.")
		} else if payload, serr := EncodeSessionSpec(spec); serr == nil {
			_ = conn.write(FrameSessionSpec, payload)
		}
	}

	choice, err := chooseSession(ctx, conn, opts)
	if err != nil {
		return run, err
	}
	if choice.Cancel {
		return run, nil
	}
	if sw := hostSwitch(opts, choice); sw != nil {
		return run, sw
	}

	for {
		select {
		case <-conn.endedCh:
			return run, errSessionEnded
		case <-conn.closedCh:
			return run, errStreamClosed
		default:
		}

		if sw := hostSwitch(opts, choice); sw != nil {
			return run, sw
		}

		if assigned, durability, aerr := conn.attach(choice.ID); aerr != nil {
			return run, aerr
		} else {
			conn.setCurrent(assigned)
			// Recorded per attach, because it is a property of the
			// SESSION rather than of the daemon: a daemon that hosts
			// sessions separately can still have fallen back for this one.
			run.sessionDurability = durability
		}

		out, rerr := runAttached(ctx, conn, opts)
		if rerr != nil {
			return run, rerr
		}
		switch {
		case out.wantSwitch:
			next, cancelled, serr := resolveSwitch(ctx, conn, opts, out)
			if serr != nil {
				return run, serr
			}
			if cancelled {
				// The picker was dismissed: stay on the session we were
				// on rather than dropping the user back to the shell.
				//
				// choice still holds whatever got us here, and for a
				// client started with --new that is 0, meaning "spawn a
				// session". Re-attaching it would answer a dismissed
				// picker with a brand new session instead of the one the
				// user was already working in.
				choice = stayOnCurrent(conn, opts)
				continue
			}
			// Release the current session before binding the next one.
			// The daemon rebinds a wire id on its own, but detaching
			// first keeps the session's client count honest for anyone
			// else listing sessions in between.
			if werr := conn.write(FrameSessionDetach, nil); werr != nil {
				return run, werr
			}
			choice = next
			continue
		case out.detached:
			run.parting = fmt.Sprintf("Detached — the session keeps running on the daemon. Reattach with: %s %d",
				opts.Reattach, conn.current())
			return run, nil
		case out.ended:
			run.parting = "Session ended."
			return run, nil
		case out.lost:
			// The daemon went away mid-session. RunClient reconnects on
			// this error and reattaches to the same logical session.
			return run, errStreamClosed
		default:
			return run, nil
		}
	}
}

// stayOnCurrent is the choice that leaves a client where it already is,
// for a picker the user dismissed.
//
// It names the session explicitly rather than reusing the choice that
// opened the picker: that one is 0 for a client started with --new, and 0
// means "spawn a session" to the daemon.
func stayOnCurrent(conn *clientConn, opts AttachOptions) SessionChoice {
	return SessionChoice{ID: conn.current(), Host: opts.Host}
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
func resolveSwitch(ctx context.Context, conn *clientConn, opts AttachOptions, out attachOutcome) (SessionChoice, bool, error) {
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
				entries = append(entries, opts.HubEntries(ctx)...)
			}
			pick = opts.Hub
		}
		if pick == nil {
			return SessionChoice{}, true, nil
		}
		choice, err := runPicker(ctx, conn, pick, entries, opts.Host)
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
		// BYE from the daemon: the session itself is over.
		setOutcome(attachOutcome{ended: true})
	case <-conn.closedCh:
		// The stream died without a BYE, which is what a stopped,
		// restarted or killed daemon looks like. The session is very
		// probably still running in its supervisor, so this is reported as
		// a lost connection and the client reconnects.
		setOutcome(attachOutcome{lost: true})
	case <-ctx.Done():
		// Say goodbye so the session detaches cleanly, then report the
		// cancellation. Falling through would return an empty outcome,
		// which RunClient reads as an ordinary exit and reports as
		// success.
		_ = conn.write(FrameBye, nil)
		return attachOutcome{}, ctx.Err()
	}

	outMu.Lock()
	result := out
	outMu.Unlock()

	// A switch keeps the connection; a plain exit says goodbye. A lost
	// connection has nothing to say goodbye on.
	if !result.detached && !result.wantSwitch && !result.ended && !result.lost {
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

// detectTerminalInfo describes the terminal this client runs in, for the
// daemon to hand to the session's child.
//
// TERM and COLORTERM are forwarded the way ssh forwards them. The
// background colour is queried here, on the machine holding the terminal,
// rather than left to the child: the child's own query would have to reach
// this terminal through a PTY — and, for a remote session, a network round
// trip — before it could draw its first frame, and the answer decides
// whether a theme renders its light or its dark half, which is not a
// question a theme can be rendered without.
//
// The multiplexer is named for the same reason and cannot be discovered any
// other way: TMUX and ZELLIJ describe this process's own pane, so the child
// never sees them, and a child that cannot see the multiplexer draws
// graphics the multiplexer throws away.
func detectTerminalInfo() TerminalInfo {
	info := TerminalInfo{
		Term:        os.Getenv("TERM"),
		ColorTerm:   os.Getenv("COLORTERM"),
		Background:  BackgroundUnknown,
		Multiplexer: termgfx.LocalMultiplexer(),
	}
	if bg, err := lipgloss.BackgroundColor(os.Stdin, os.Stdout); err == nil {
		if hex := HexColor(bg); hex != "" {
			info.Background = hex
		}
	}
	return info
}
