package daemon

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	"github.com/creack/pty"

	"github.com/mark3labs/kit/internal/clipboard"
)

// Serve runs the daemon until ctx is cancelled: bind the stable endpoint
// derived from the daemon identity, then host remote sessions for paired
// clients. Each client authenticates by signing the handshake with its
// pairing key; the signature is checked against the allowlist written by
// `kit daemon pair`. First-time clients pair through `kit daemon pair`,
// which runs its own short-lived bootstrap endpoint.
func Serve(ctx context.Context) error {
	// Single instance per user: the lock is held for the daemon's lifetime
	// and released automatically on crash, so there is no stale-lock state.
	lock, err := acquireDaemonLock()
	if err != nil {
		return err
	}
	defer lock.release()
	defer clearState()
	rt := newDaemonRuntime(lock)

	fmt.Println()
	fmt.Println("  kit daemon")
	fmt.Println()

	// The session table outlives tunnel restarts AND this daemon: logical
	// sessions keep running (detached) while clients come and go, and
	// hosted ones keep running while DAEMONS come and go.
	table := newSessionTable(rt)

	// Sessions left by a previous daemon come first. The hosted ones are
	// adopted — dialled, put back in the table with their ids, names and
	// ages intact, and made available to the clients still waiting on
	// them. Only then is the rest swept: a session this daemon could not
	// adopt because its PTY master died with the daemon that held it is
	// unreachable by construction, and is ended rather than left running
	// where nothing can ever reach it.
	adopted := table.adoptHostedSessions(ctx)
	if len(adopted) > 0 {
		fmt.Printf("  Adopted %s from a previous daemon.\n", countSessions(len(adopted)))
	}
	table.seedSessionIDs(adopted)
	sweepOrphanSessions(table.run, adopted)
	sweepStaleTempFiles(table.sessionIDs())

	// The local socket is bound first and closed only on shutdown. The
	// lock above guarantees no other daemon owns this socket.
	sockPath, err := LocalSocketPath()
	if err != nil {
		return err
	}
	ln, lnErr := listenLocal(sockPath)
	if lnErr != nil {
		log.Warn("daemon: local sessions are disabled", "error", lnErr)
	} else {
		defer func() { _ = ln.Close() }()
		defer func() { _ = os.Remove(sockPath) }()
		go serveLocal(ctx, ln, table)
		fmt.Printf("  Local socket: %s\n", sockPath)
	}

	// Remote sessions run on an in-process iroh endpoint. A failure to
	// bind it is not fatal: local sessions still work, exactly as they did
	// when the retired transport sidecar was missing.
	listener, remoteErr := bindRemoteListener(ctx, table)
	if remoteErr != nil {
		if lnErr != nil {
			// No socket of any kind, so this daemon cannot serve the
			// sessions it adopted a moment ago. Let GO of them rather
			// than ending them: they were running before this process
			// started and a daemon that failed to bind is no reason to
			// destroy them. killAll here would call Terminate on every
			// adopted session and take the user's work with it.
			table.releaseSessions()
			return fmt.Errorf("%w (and remote sessions failed too: %v)", lnErr, remoteErr)
		}
		log.Warn("daemon: remote sessions are disabled", "error", remoteErr)
		fmt.Println("  Remote sessions: unavailable")
		fmt.Println("  Attach locally with: kit attach")
		fmt.Println()
	} else {
		defer listener.close()
		nodeID := listener.endpointID()
		rt.setEndpoint(nodeID)
		fmt.Printf("  Endpoint:     %s\n", shortEndpoint(nodeID))
		fmt.Println("  Waiting for paired clients. Pair a new one with: kit daemon pair")
		fmt.Println()
		go listener.run(ctx)
		go func() {
			// Reaching the home relay is what makes the endpoint
			// discoverable; report how that went without blocking startup.
			if err := listener.h.waitOnline(ctx, 0); err != nil {
				if ctx.Err() == nil {
					log.Warn("daemon: relay connection failed — remote clients cannot discover this endpoint", "error", err)
				}
				return
			}
			log.Debug("daemon: relay connected")
		}()
	}

	<-ctx.Done()
	return shutdown(table)
}

// bindRemoteListener loads the daemon identity and binds the stable
// endpoint on it.
func bindRemoteListener(ctx context.Context, table *sessionTable) (*remoteListener, error) {
	seed, err := LoadDaemonIdentity()
	if err != nil {
		return nil, err
	}
	return listenRemote(ctx, seed, table)
}

// shutdown lets go of every session and reports a clean exit.
//
// "Lets go of", not "ends": a hosted session is detached and keeps
// running, so stopping or restarting the daemon costs the user nothing.
// Only sessions this daemon hosts itself are ended, because those cannot
// be reached by any future daemon. See releaseSessions.
//
// A cancelled context is how a SIGINT/SIGTERM stop arrives, which is
// success, not failure: returning ctx.Err() here would exit non-zero and
// have systemd log "status=1/FAILURE" for an ordinary `systemctl stop`.
func shutdown(table *sessionTable) error {
	released, ended := table.releaseSessions()
	if released > 0 {
		log.Info("daemon: leaving sessions running for the next daemon", "count", released)
		fmt.Printf("\n  %s left running. Reattach with: kit attach\n",
			capitalise(countSessions(released)))
	}
	if ended > 0 {
		log.Info("daemon: stopped sessions this daemon hosted itself", "count", ended)
	}
	// The registry is left in place when sessions were released: it is how
	// a later sweep tells an adopted session from an orphan, and removing
	// it would make the next daemon's records start empty.
	if released == 0 {
		removeSessionRegistry()
	}
	return nil
}

// countSessions renders a session count with its noun.
func countSessions(n int) string {
	if n == 1 {
		return "1 session"
	}
	return fmt.Sprintf("%d sessions", n)
}

// capitalise upper-cases the first letter of a message that starts a line.
func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// shortEndpoint renders the first bytes of an endpoint id for display.
func shortEndpoint(id string) string {
	if len(id) > 16 {
		return id[:16] + "…"
	}
	return id
}

// remoteSession is one connected client and its kit child process.
// remoteSession is a LOGICAL session: one PTY child that can outlive its
// client connections. Clients (identified by their wire session id — the
// per-connection id the transport assigns) attach to it; a session with zero
// attached clients is detached but keeps running until the child exits or
// the daemon shuts down.
type remoteSession struct {
	id uint64 // logical id, daemon-assigned monotonic
	// io is how this daemon reaches the session's terminal: directly,
	// through a PTY master it holds, or through a socket to the supervisor
	// that holds it. See sessionio.go — the difference decides whether the
	// session survives this daemon.
	io      sessionIO
	started time.Time

	// modes remembers the terminal modes the child has set, so a client
	// that attaches after the child set them still gets them. See
	// termModes.
	//
	// A hosted session tracks them in its supervisor as well, because this
	// copy is lost whenever the daemon is. The two are not redundant: this
	// one serves a single attaching client without disturbing the others,
	// while the supervisor's covers a daemon that has just adopted a
	// session and knows nothing about it yet.
	modes *termModes

	mu      sync.Mutex
	name    string             // user-set display name, empty until renamed
	clients map[uint32]winSize // attached wire ids -> last known size (0,0 = unknown)
}

// setName records a user-supplied display name.
func (s *remoteSession) setName(name string) {
	s.mu.Lock()
	s.name = name
	s.mu.Unlock()
	// The name belongs to the SESSION, so a hosted one keeps it where the
	// session lives and it survives this daemon along with it.
	if s.io != nil {
		s.io.Rename(name)
	}
}

// displayName returns the session's name, or "" when it has none.
func (s *remoteSession) displayName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

// hosted reports whether this session outlives the daemon.
func (s *remoteSession) hosted() bool {
	return s.io != nil && s.io.Hosted()
}

// nudgeRedraw makes the session repaint into a client that has just taken
// over the screen, and restores the terminal state that client never saw
// the child set.
//
// How it is done depends on where the session lives; see sessionIO.Redraw.
func (s *remoteSession) nudgeRedraw() {
	if s.io == nil {
		return
	}
	s.io.Redraw(s.minSize())
}

// sendTermModes hands one client the terminal modes the session's child
// has set: mouse reporting, bracketed paste, focus reporting, cursor
// visibility and the keyboard protocol.
//
// Only the client that STARTED a session sees those sequences on the
// wire, because the child sends each one once and never repeats it — a
// repaint is not a mode change. Every later client (a reattach, a session
// switch, a second client sharing the view) would otherwise run the
// session in a terminal that reports no mouse at all.
//
// Sent to the asking client alone: the others are already in this state,
// and a fan-out would write over whatever they are drawing.
func (t *sessionTable) sendTermModes(s *remoteSession, wire uint32) {
	if s.modes == nil {
		return
	}
	if replay := s.modes.Replay(); len(replay) > 0 {
		_ = t.writeTo(Frame{Type: FrameData, Session: wire, Payload: replay})
	}
}

type winSize struct{ cols, rows int }

// attachClient records a client and returns the resulting minimum size
// (zeros ignored) plus whether it differs from the previous minimum.
func (s *remoteSession) attachClient(wire uint32, ws winSize) (winSize, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := minSizeLocked(s.clients)
	s.clients[wire] = ws
	next := minSizeLocked(s.clients)
	return next, next != prev
}

// detachClient removes a client and returns the new minimum size (if any
// clients remain) plus the number of remaining clients.
func (s *remoteSession) detachClient(wire uint32) (winSize, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, wire)
	return minSizeLocked(s.clients), len(s.clients)
}

// resizeClient records a client's size and returns the new minimum.
func (s *remoteSession) resizeClient(wire uint32, ws winSize) winSize {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[wire] = ws
	return minSizeLocked(s.clients)
}

// minSize reports the size the PTY is held at: the smallest attached
// client's window, or the zero size when none has reported one.
func (s *remoteSession) minSize() winSize {
	s.mu.Lock()
	defer s.mu.Unlock()
	return minSizeLocked(s.clients)
}

// clientIDs snapshots the attached wire ids.
func (s *remoteSession) clientIDs() []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]uint32, 0, len(s.clients))
	for id := range s.clients {
		ids = append(ids, id)
	}
	return ids
}

// minSizeLocked computes the minimum nonzero size across clients. All-zero
// (no client has reported yet) yields the zero size and the caller leaves
// the PTY at its default.
func minSizeLocked(clients map[uint32]winSize) winSize {
	var out winSize
	first := true
	for _, ws := range clients {
		if ws.cols == 0 || ws.rows == 0 {
			continue
		}
		if first {
			out = ws
			first = false
			continue
		}
		if ws.cols < out.cols {
			out.cols = ws.cols
		}
		if ws.rows < out.rows {
			out.rows = ws.rows
		}
	}
	if first {
		return winSize{}
	}
	return out
}

// authorization note: remote handshakes verify signatures inline in
// iroh_serve.go; nothing auth-related flows through the frame loop.

// sessionTable owns the daemon's LOGICAL sessions. Each transport
// connection's frames are read by its own goroutine (runFrameSource), so
// map access takes mu; teardown paths run on other goroutines and take mu
// too. The table is created once per daemon process.
type sessionTable struct {
	rt    *daemonRuntime
	conns *connSet
	// run identifies this daemon run. It tags registry records so a later
	// run can tell its own state from a crashed predecessor's.
	run string
	// closing marks a daemon on its way out, so the per-session watchers
	// do not read a deliberately released session as one that ended.
	closing atomic.Bool

	mu           sync.Mutex
	nextID       uint64
	sessions     map[uint64]*remoteSession      // logical id -> session
	wireMap      map[uint32]uint64              // wire id -> logical id
	clipboards   map[uint64]*ClipboardCollector // in-flight image transfers
	sessionTemps map[uint64][]string            // per-session clipboard files
}

func newSessionTable(rt *daemonRuntime) *sessionTable {
	return &sessionTable{
		rt:           rt,
		run:          newRunNonce(),
		conns:        newConnSet(),
		sessions:     make(map[uint64]*remoteSession),
		wireMap:      make(map[uint32]uint64),
		clipboards:   make(map[uint64]*ClipboardCollector),
		sessionTemps: make(map[uint64][]string),
	}
}

// writeTo sends one frame to the connection that owns frame.Session. A
// missing connection means the client has gone (detach, network loss, or a
// dead transport); the frame is dropped and the caller carries on, because
// the session behind it keeps running regardless.
func (t *sessionTable) writeTo(frame Frame) error {
	conn := t.conns.get(frame.Session)
	if conn == nil {
		return errSinkClosed
	}
	return conn.sink.write(frame)
}

// logicalFor resolves a wire session id to its logical session.
func (t *sessionTable) logicalFor(wire uint32) *remoteSession {
	t.mu.Lock()
	defer t.mu.Unlock()
	if id, ok := t.wireMap[wire]; ok {
		return t.sessions[id]
	}
	return nil
}

// runFrameSource reads frames from one client connection until the stream
// ends. wire is the connection's assigned id: clients have no wire-id
// allocator of their own and stamp every frame with session 0, so the
// daemon stamps the assigned id on arrival. Replies carry the id back
// out, and clients ignore it.
func (t *sessionTable) runFrameSource(ctx context.Context, r io.Reader, wire uint32) error {
	for {
		frame, err := ReadFrame(r)
		if err != nil {
			return nil // stream ended
		}
		frame.Session = wire
		switch frame.Type {
		case FrameHello:
			t.handleHello(frame.Session, frame.Payload)
		case FrameSessionDetach:
			// Detach unbinds the session but KEEPS the connection: the
			// client is still there and usually attaches to another
			// session next (a switch is detach followed by attach).
			// Dropping the connection here would leave the following
			// attach with nowhere to send its ack.
			t.detachWire(frame.Session)
		case FrameSessionClosed, FrameBye:
			// The client says it is leaving. The connection teardown that
			// follows unregisters it; here the session only detaches.
			t.detachWire(frame.Session)
		case FrameTerminal:
			// Describes the client's terminal; recorded against the
			// connection so an attach that spawns a child can hand it on.
			if info, derr := DecodeTerminalInfo(frame.Payload); derr == nil {
				t.conns.setTerminal(frame.Session, info)
			} else {
				log.Warn("bad terminal frame", "wire", frame.Session, "error", derr)
			}
		case FrameSessionSpec:
			// Describes how a new session on this connection should be
			// started; recorded against the connection like FrameTerminal,
			// and consumed by the next attach that spawns a child.
			if spec, derr := DecodeSessionSpec(frame.Payload); derr != nil {
				log.Warn("bad session spec frame", "wire", frame.Session, "error", derr)
			} else if !t.conns.setSpec(frame.Session, spec) {
				log.Warn("ignored a session spec from a remote client", "wire", frame.Session)
			}
		case FrameSessionList:
			t.sendSessionList(frame.Session)
		case FrameSessionAttach:
			t.attachSession(ctx, frame.Session, frame.Payload)
		case FrameSessionRedraw:
			if sess := t.logicalFor(frame.Session); sess != nil {
				// A client asks to repaint exactly when it has taken
				// over the screen, which is also the moment its terminal
				// needs the modes the child set before it arrived. The
				// modes go first: the repaint that follows is drawn into
				// a terminal already in the right state.
				t.sendTermModes(sess, frame.Session)
				sess.nudgeRedraw()
			}
		case FrameSessionRename:
			t.renameSession(frame.Payload)
		case FrameData:
			if sess := t.logicalFor(frame.Session); sess != nil {
				if _, err := sess.io.Write(frame.Payload); err != nil {
					t.retireSession(sess.id)
				}
			}
		case FrameResize:
			if sess := t.logicalFor(frame.Session); sess != nil {
				if cols, rows, derr := DecodeResize(frame.Payload); derr == nil {
					sess.applyResize(frame.Session, cols, rows)
				}
			}
		case FrameClipboard:
			if sess := t.logicalFor(frame.Session); sess != nil {
				t.handleClipboardChunk(ctx, sess.id, frame.Payload)
			}
		default:
			// A frame from a NEWER client than this daemon. Dropping it is
			// the contract — every frame added since v1 is additive, and a
			// client that needs one negotiated it through the feature
			// bitmap first. Logged so an unexpected one is at least
			// findable, and at debug level so a mixed-version pair does not
			// fill the journal.
			log.Debug("daemon: ignoring an unknown frame",
				"type", fmt.Sprintf("%#x", byte(frame.Type)), "wire", frame.Session)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// handleHello reports a client's announced protocol and answers with our
// own, so each end knows what the other can do.
//
// A mismatched version is reported and then SERVED anyway. The daemon is
// the long-lived side: it cannot know whether the client is older or
// newer, and refusing here would take away the one channel through which
// the client can be told what is wrong. The client makes the decision,
// because the client is the side that can print a message and exit.
//
// Nothing about the peer is kept. The daemon's behaviour does not depend
// on it: every frame added since v1 is additive, so a client simply does
// not send what it does not have, and one that sends something we lack is
// already ignored by the frame loop. Recording it would be state that
// nothing reads.
//
// Our reply goes out even to a peer whose hello was unreadable: that peer
// still needs to learn what this daemon is, and a garbled hello is far
// more likely to be a version skew than an attack (the local socket is
// already uid-restricted, and a remote peer has passed the pairing
// handshake before reaching this frame loop). A client that has gone in
// the meantime is handled by writeTo, which drops frames for a connection
// it no longer knows.
func (t *sessionTable) handleHello(wire uint32, payload []byte) {
	peer, err := DecodeHello(payload)
	switch {
	case err != nil:
		log.Warn("daemon: unreadable client hello", "wire", wire, "error", err)
	case peer.Compatible() != nil:
		log.Warn("daemon: client protocol mismatch", "wire", wire,
			"client_version", peer.Version, "daemon_version", ProtocolVersion,
			"client_build", peer.Build)
	default:
		log.Debug("daemon: client hello", "wire", wire,
			"build", peer.Build, "features", peer.Features)
	}
	if reply, merr := EncodeHello(localHello(RoleDaemon)); merr == nil {
		_ = t.writeTo(Frame{Type: FrameHello, Session: wire, Payload: reply})
	}
}

// unbindAll drops every remote connection at once. Logical sessions keep
// running detached, and local clients stay live.
func (t *sessionTable) unbindAll() {
	for _, wire := range t.conns.removeRemotes() {
		t.detachWire(wire)
	}
}

// detachWire unbinds one wire session id from its logical session. The
// logical session keeps running; with no clients left it is detached.
func (t *sessionTable) detachWire(wire uint32) {
	t.mu.Lock()
	logical, ok := t.wireMap[wire]
	if ok {
		delete(t.wireMap, wire)
	}
	t.mu.Unlock()
	if !ok {
		return
	}
	t.mu.Lock()
	sess := t.sessions[logical]
	t.mu.Unlock()
	if sess == nil {
		return
	}
	if next, remaining := sess.detachClient(wire); remaining == 0 {
		log.Info("session detached", "session_id", logical)
	} else {
		// The PTY is held at the smallest attached client's size, so a
		// client leaving can widen it. Without this the session stays
		// squeezed into a window that is no longer watching it, and the
		// clients still attached keep drawing into a corner of their
		// terminals until they happen to resize one.
		sess.applySize(next)
		log.Info("client left shared session", "session_id", logical, "remaining", remaining)
	}
}

// releaseSessions is how a daemon lets go on shutdown.
//
// A hosted session is DETACHED, not ended: its supervisor keeps the child
// running and the next daemon adopts it. This is the single most
// important line in the shutdown path — ending those sessions here would
// make every `systemctl restart kit` destroy the user's work, which is
// precisely the behaviour this design removes.
//
// A session this daemon hosts itself has nowhere to go: its PTY master
// dies with this process and no future daemon could ever reach the child,
// so it is ended cleanly instead of being left unreachable.
func (t *sessionTable) releaseSessions() (released, ended int) {
	t.closing.Store(true)
	for _, id := range t.sessionIDs() {
		t.mu.Lock()
		sess := t.sessions[id]
		t.mu.Unlock()
		if sess == nil {
			continue
		}
		if !sess.hosted() {
			t.retireSession(id)
			ended++
			continue
		}
		// Tell the clients the connection is over without telling them
		// the SESSION is over: BYE means "ended" to a client, and a client
		// that heard it would stop instead of reconnecting. Dropping the
		// stream is the honest signal, and the one RunClient retries on.
		_ = sess.io.Close()
		released++
	}
	return released, ended
}

// sessionIDs snapshots the live logical ids.
func (t *sessionTable) sessionIDs() []uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	ids := make([]uint64, 0, len(t.sessions))
	for id := range t.sessions {
		ids = append(ids, id)
	}
	return ids
}

// applySize sets the session's terminal to ws, if it names a real size.
//
// The zero size means no attached client has reported one yet, and the
// terminal is left at its default rather than resized to nothing.
func (s *remoteSession) applySize(ws winSize) {
	if s.io == nil {
		return
	}
	_ = s.io.Resize(ws)
}

// applyResize records one client's size and applies the minimum across all
// attached clients to the PTY — the shared-view equivalent of tmux picking
// the smallest window.
func (s *remoteSession) applyResize(wire uint32, cols, rows int) {
	s.applySize(s.resizeClient(wire, winSize{cols, rows}))
}

// handleClipboardChunk consumes one client clipboard frame. Chunked image
// data is reassembled and, on the final chunk, written to the session's
// stable clipboard file followed by a synthetic 0x16 into the child's PTY:
// the child's own Ctrl-V handler reads the file, fills pendingImages and
// renders the same preview a local paste gets. A clear frame (the client
// had no image) empties the file so a subsequent child Ctrl-V is a no-op.
//
// Locking: closeSession may run on any goroutine (child exit), so map
// access happens under t.mu; the collector object is only touched by the
// frame loop.
func (t *sessionTable) handleClipboardChunk(ctx context.Context, session uint64, payload []byte) {
	if ctx.Err() != nil {
		return
	}
	if len(payload) < 1 {
		return
	}
	clear := payload[0]&FrameClipboardFlagClear != 0

	// Register the stable file for teardown while the session lives.
	t.mu.Lock()
	_, live := t.sessions[session]
	if live {
		path := t.remoteClipboardPath(session)
		if !slices.Contains(t.sessionTemps[session], path) {
			t.sessionTemps[session] = append(t.sessionTemps[session], path)
		}
	}
	t.mu.Unlock()
	if !live {
		return
	}

	if clear {
		// The client found no image: empty the file so the child's next
		// Ctrl-V is a no-op. No keystroke is injected.
		if err := os.WriteFile(t.remoteClipboardPath(session), nil, 0o600); err != nil {
			log.Warn("clipboard clear failed", "session_id", session, "error", err)
		}
		t.mu.Lock()
		delete(t.clipboards, session)
		t.mu.Unlock()
		return
	}

	t.mu.Lock()
	coll, ok := t.clipboards[session]
	if !ok {
		coll = NewClipboardCollector()
		t.clipboards[session] = coll
	}
	s, live := t.sessions[session]
	var sio sessionIO
	if live && s != nil {
		sio = s.io
	}
	t.mu.Unlock()
	if !live || sio == nil {
		return
	}

	done, media, data, err := coll.Add(payload)
	if err != nil {
		t.mu.Lock()
		delete(t.clipboards, session)
		t.mu.Unlock()
		log.Warn("clipboard transfer dropped", "session_id", session, "error", err)
		return
	}
	if !done {
		return
	}
	t.mu.Lock()
	delete(t.clipboards, session)
	t.mu.Unlock()

	if err := publishClipboardImage(t.remoteClipboardPath(session), t.run, data); err != nil {
		log.Error("daemon: clipboard publish failed", "session_id", session, "error", err)
		return
	}

	// Synthetic Ctrl-V: the child reads the file via KIT_REMOTE_CLIPBOARD
	// and runs its normal pending-image preview flow.
	if _, err := fmt.Fprintf(sio, "%c", pasteKey); err != nil {
		log.Error("daemon: clipboard inject failed", "session_id", session, "error", err)
		return
	}
	log.Info("clipboard image delivered", "session_id", session, "bytes", len(data), "media_type", media)
}

// sessionInfo is one row of the client-facing session list.
type sessionInfo struct {
	ID      uint64 `json:"id"`
	Clients int    `json:"clients"`
	Started string `json:"started"`
	Cwd     string `json:"cwd,omitempty"`
	Name    string `json:"name,omitempty"`
}

// renameSession applies a client's rename request: {id u64 BE, name}.
func (t *sessionTable) renameSession(payload []byte) {
	if len(payload) < 8 {
		return
	}
	id := binary.BigEndian.Uint64(payload[:8])
	name := strings.TrimSpace(string(payload[8:]))
	// Truncate on rune boundaries: the frame documents the name as UTF-8,
	// and slicing bytes can cut a multi-byte rune in half.
	if r := []rune(name); len(r) > 64 {
		name = string(r[:64])
	}
	t.mu.Lock()
	sess := t.sessions[id]
	t.mu.Unlock()
	if sess != nil {
		sess.setName(name)
	}
}

// sendSessionList replies to a client's list request with the live
// sessions. Attach rights are pairing rights: every paired client may
// attach to any session.
func (t *sessionTable) sendSessionList(wire uint32) {
	t.mu.Lock()
	ids := make([]uint64, 0, len(t.sessions))
	for id := range t.sessions {
		ids = append(ids, id)
	}
	t.mu.Unlock()

	infos := make([]sessionInfo, 0, len(ids))
	for _, id := range ids {
		t.mu.Lock()
		sess := t.sessions[id]
		t.mu.Unlock()
		if sess == nil {
			continue
		}
		info := sessionInfo{
			ID:      id,
			Clients: len(sess.clientIDs()),
			Started: sess.started.Format(time.RFC3339),
			Name:    sess.displayName(),
			Cwd:     t.sessionCwd(sess),
		}
		infos = append(infos, info)
	}
	// A stable order keeps the picker's rows from jumping between
	// refreshes, and gives the next/previous chords a meaningful sense of
	// direction. Map iteration alone would randomise both.
	slices.SortFunc(infos, func(a, b sessionInfo) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	payload, err := json.Marshal(infos)
	if err != nil {
		return
	}
	_ = t.writeTo(Frame{Type: FrameSessionListReply, Session: wire, Payload: payload})
}

// attachSession binds a client's wire id to a logical session and answers
// with an ack the client waits for. Logical id 0 means "spawn a new
// session"; the ack carries the assigned (or attached) logical id.
func (t *sessionTable) attachSession(ctx context.Context, wire uint32, payload []byte) {
	requested := uint64(0)
	if len(payload) >= 8 {
		requested = binary.BigEndian.Uint64(payload[:8])
	}

	ok := byte(0)
	var logical uint64
	if requested == 0 {
		// New session: spawn a fresh child and bind.
		t.mu.Lock()
		t.nextID++
		logical = t.nextID
		s := &remoteSession{
			id:      logical,
			started: time.Now(),
			clients: make(map[uint32]winSize),
			modes:   newTermModes(),
		}
		t.sessions[logical] = s
		t.wireMap[wire] = logical
		s.attachClient(wire, winSize{})
		t.mu.Unlock()

		sio, err := t.spawnSession(ctx, logical, t.conns.terminalFor(wire), t.conns.consumeSpec(wire))
		if err != nil {
			log.Error("daemon: session spawn failed", "session_id", logical, "error", err)
			t.retireSession(logical)
			ok = 0
		} else {
			s.io = sio
			t.reportSessions()
			t.syncSessionRegistry()
			log.Info("session started", "session_id", logical, "wire", wire, "hosted", sio.Hosted())
			t.watchSession(s)
			ok = 1
		}
	} else {
		t.mu.Lock()
		sess := t.sessions[requested]
		if sess != nil {
			// A wire id drives at most one session: drop any previous
			// binding first (that session keeps running detached, and
			// regains the room this client was taking from it).
			if prev, had := t.wireMap[wire]; had && prev != requested {
				if ps := t.sessions[prev]; ps != nil {
					left, remaining := ps.detachClient(wire)
					if remaining > 0 {
						ps.applySize(left)
					}
				}
			}
			t.wireMap[wire] = requested
			sess.attachClient(wire, winSize{})
			ok = 1
			logical = requested
		}
		t.mu.Unlock()
		if ok == 1 {
			log.Info("client attached", "session_id", logical, "wire", wire)
		}
	}

	// The ack is 9 bytes plus a flags byte. The tenth byte is ADDITIVE: a
	// client too old to read it stops at the ninth and behaves exactly as
	// before, and a new client talking to an old daemon gets 9 bytes and
	// falls back to the daemon-wide FeatureReattach bit. That is why this
	// needs no protocol version bump.
	ack := make([]byte, 10)
	binary.BigEndian.PutUint64(ack[:8], logical)
	ack[8] = ok
	if ok == 1 {
		// Whether THIS session survives a daemon restart. The daemon-wide
		// hello cannot answer that: a daemon which supports supervisors
		// can still have fallen back to a plain PTY for one session (see
		// spawnSession), and that session dies with the daemon while its
		// neighbours do not. A client which was told otherwise would
		// promise the user work that is already gone.
		t.mu.Lock()
		if sess := t.sessions[logical]; sess != nil && sess.hosted() {
			ack[9] = 1
		}
		t.mu.Unlock()
	}
	_ = t.writeTo(Frame{Type: FrameSessionAttachAck, Session: wire, Payload: ack})
	if ok == 0 {
		log.Warn("attach failed", "wire", wire, "requested", requested)
		return
	}
	if requested == 0 && !t.conns.live(wire) {
		// We spawned this session for a client that has already gone: it
		// cancelled, or its connection dropped, between asking and being
		// told the answer. Nobody knows this session exists, so it is
		// retired rather than left as a detached session the user never
		// asked for and would find in 'kit ls' with no idea what it is.
		//
		// Narrow on purpose. Only a session created FOR THIS REQUEST is
		// eligible, and only while no other client has attached to it.
		// An established session is never touched, because outliving its
		// client is the whole point of one.
		t.retireUnclaimedSession(logical, wire)
	}
}

// retireUnclaimedSession ends a session that was just spawned for a
// client which vanished before it could be told the session existed.
//
// A session whose requester never learned its id is not a detached
// session, it is litter: nothing points at it, and the user did not ask
// for it to keep running. A session anyone else has attached to is
// somebody's work and is left alone.
func (t *sessionTable) retireUnclaimedSession(logical uint64, wire uint32) {
	t.mu.Lock()
	sess := t.sessions[logical]
	if bound, ok := t.wireMap[wire]; ok && bound == logical {
		delete(t.wireMap, wire)
	}
	t.mu.Unlock()
	if sess == nil {
		return
	}
	if _, remaining := sess.detachClient(wire); remaining > 0 {
		return // another client is watching it; it is theirs now
	}
	log.Warn("daemon: retiring a session whose client never received it",
		"session_id", logical, "wire", wire)
	t.retireSession(logical)
}

// watchSession starts the per-session PTY fan-out reader and the child
// lifecycle watcher.
func (t *sessionTable) watchSession(s *remoteSession) {
	// Session -> clients: raw child output as DATA frames fanned out to
	// every attached client (shared view).
	go func(sess *remoteSession) {
		buf := make([]byte, chunkSize)
		for {
			n, err := sess.io.Read(buf)
			if n > 0 {
				// Watch the stream for terminal modes on the way past.
				// The next client to attach is handed them; it will
				// never see the sequences themselves, because the child
				// sends each one once.
				if sess.modes != nil {
					sess.modes.Feed(buf[:n])
				}
				for _, wire := range sess.clientIDs() {
					// Write errors mean the tunnel is gone; the restart
					// loop takes over from here.
					_ = t.writeTo(Frame{Type: FrameData, Session: wire, Payload: buf[:n]})
				}
			}
			if err != nil {
				return // EIO when the child exits, or the link closed
			}
		}
	}(s)

	// Session lifecycle: when the session ends (the user exited the TUI),
	// retire exactly this one — the others are unaffected.
	go func(sess *remoteSession) {
		sess.io.Wait()
		if t.closing.Load() {
			// The daemon is shutting down and has just let go of this
			// session on purpose. Retiring it here would kill a child
			// that was deliberately left running.
			return
		}
		t.retireSession(sess.id)
	}(s)
}

// retireSession tears down one logical session: tell every attached client
// we are done, stop the child, free the table slot, and remove its
// clipboard files. Idempotent.
func (t *sessionTable) retireSession(id uint64) {
	t.mu.Lock()
	s, ok := t.sessions[id]
	// A daemon on its way out has already let go of its hosted sessions
	// deliberately (see releaseSessions). A frame that arrives in that
	// window — a client keystroke landing on a sink we have just closed —
	// fails to write and lands here, and retiring the session would end a
	// child that was left running on purpose. One late keystroke would
	// destroy the user's work on every restart.
	if ok && t.closing.Load() && s.io != nil && s.io.Hosted() {
		t.mu.Unlock()
		return
	}
	delete(t.sessions, id)
	temps := t.sessionTemps[id]
	delete(t.sessionTemps, id)
	delete(t.clipboards, id)
	wires := make([]uint32, 0)
	for wire, logical := range t.wireMap {
		if logical == id {
			wires = append(wires, wire)
		}
	}
	for _, wire := range wires {
		delete(t.wireMap, wire)
	}
	t.mu.Unlock()
	for _, wire := range wires {
		_ = t.writeTo(Frame{Type: FrameBye, Session: wire})
	}
	if !ok {
		for _, p := range temps {
			_ = os.Remove(p)
		}
		return
	}
	t.reportSessions()
	for _, p := range temps {
		_ = os.Remove(p)
	}

	if s.io != nil {
		// Terminate, not Close: this is the session ending, so the child
		// is asked to exit and then made to. SIGTERM first — the child
		// flushes its conversation store and restores the terminal — and
		// SIGKILL only if it ignores that. A hosted session's supervisor
		// does the same on its side and then exits with it.
		s.io.Terminate()
	}
	t.syncSessionRegistry()
	log.Info("session ended", "session_id", id)
}

// remoteClipboardPath is the stable per-session file the daemon streams
// client clipboard images into. The child reads it on every Ctrl-V (see
// internal/clipboard.RemoteClipboardEnv), so a paste is a file rewrite
// followed by a synthetic 0x16 keystroke — the child's own clipboard
// pipeline then renders the preview exactly like a local paste.
//
// Named by logical session id ALONE. It used to carry the daemon's run
// nonce as well, which was right when a session could not outlive its
// daemon: the id was reused by the next run and the nonce kept the two
// apart. A session now survives the daemon, and its child holds this path
// in its environment for its whole life, so a name that changed with the
// daemon would leave every adopted session pasting into a file nobody
// writes. Ids are no longer reused (see nextSessionID), which is what
// makes the nonce unnecessary rather than merely inconvenient.
func (t *sessionTable) remoteClipboardPath(session uint64) string {
	return filepath.Join(t.scratchDir(), fmt.Sprintf("%sclip-%d", tempFilePrefix, session))
}

// sessionCwdPath is the stable per-session file a session's child writes
// its working directory into, once the directory picker has resolved it.
//
// Reading /proc/<pid>/cwd would be simpler but only works on Linux, and it
// reports the child's cwd rather than the directory the user picked. The
// file follows the same convention as the clipboard file above, and for
// the same reason is named by session id alone.
func (t *sessionTable) sessionCwdPath(session uint64) string {
	return filepath.Join(t.scratchDir(), fmt.Sprintf("%scwd-%d", tempFilePrefix, session))
}

// scratchDir is where per-session files live.
//
// The daemon's own runtime directory, not the shared temp directory: two
// daemons with different runtime directories run at the same time, and a
// start-up sweep of a shared directory would delete the live clipboard and
// cwd files of the other daemon's sessions. Falling back to the temp
// directory keeps the paths working if the runtime dir is unavailable;
// the run nonce still keeps one daemon's files apart from another's.
func (t *sessionTable) scratchDir() string {
	if dir, err := daemonRuntimeDir(); err == nil {
		return dir
	}
	return os.TempDir()
}

// sessionCwd reports a session's working directory for the session list.
// It is empty until the child has chosen one, which is the honest answer
// while the directory picker is still on screen.
func (t *sessionTable) sessionCwd(s *remoteSession) string {
	return readReportedCwd(t.sessionCwdPath(s.id))
}

// spawnSession starts a session for a new logical id and returns the
// handle the daemon drives it through.
//
// A supervisor process is preferred, always. It is what makes the session
// survive this daemon being stopped, restarted or upgraded, and it costs
// one small process. Only when one cannot be started does the daemon fall
// back to holding the PTY itself — a session that works now and dies with
// the daemon is strictly better than no session at all, and the log says
// which kind the user got.
//
// Without a spec the child gets the hidden --pick-dir flag and starts in
// the daemon user's home directory, so the peer chooses the session's
// working directory from the modal rendered inside the PTY. That is the
// only thing a remote client can be offered: the daemon shares no
// directory, no argument list and no environment with it.
//
// With a spec — only ever from the local socket — the child reproduces
// the invocation the user typed: their directory, their arguments, their
// environment. This is what lets a plain `kit` in a project directory be
// hosted by the daemon without behaving like a different command.
//
// info describes the terminal of the client that asked for the session, so
// the child renders for that terminal rather than for the daemon's own
// environment.
func (t *sessionTable) spawnSession(ctx context.Context, session uint64, info TerminalInfo, spec *SessionSpec) (sessionIO, error) {
	if hostedSessionsSupported() {
		io, err := t.spawnSessionHost(ctx, session, info, spec)
		if err == nil {
			return io, nil
		}
		log.Warn("daemon: could not start a session host — this session will not survive a daemon restart",
			"session_id", session, "error", err)
	}
	return t.spawnLocalSession(session, info, spec)
}

// spawnLocalSession starts a session whose PTY master this daemon holds.
// It dies with the daemon; see spawnSession for when that is accepted.
func (t *sessionTable) spawnLocalSession(session uint64, info TerminalInfo, spec *SessionSpec) (sessionIO, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve kit binary: %w", err)
	}
	dir, specArgs := specCommand(spec)

	cmd := exec.Command(exe, specArgs...)
	cmd.Dir = dir
	owner, _ := daemonRuntimeDir()
	// The child renders into the CLIENT's terminal, not the daemon's; see
	// childEnv for why the PTY between them cannot answer for it. The
	// spec's variables go underneath, where the daemon's own per-session
	// values still override them.
	cmd.Env = childEnv(specBase(os.Environ(), spec), info, t.sessionEnv(session, owner))

	// Ask the kernel to kill this child if the daemon dies, so a crash
	// cannot leave an unreachable session running (see recovery.go).
	cmd.SysProcAttr = applyChildDeathSignal(cmd.SysProcAttr)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		return nil, fmt.Errorf("start child: %w", err)
	}
	return &ptyIO{cmd: cmd, ptmx: ptmx}, nil
}

// sessionEnv builds the per-session variables the daemon owns, whichever
// kind of session is being started.
//
// owner marks the child with the runtime directory of the daemon that
// started it, so a later sweep can prove a process is ours before
// signalling it.
func (t *sessionTable) sessionEnv(session uint64, owner string) map[string]string {
	env := map[string]string{
		clipboard.RemoteClipboardEnv: t.remoteClipboardPath(session),
		sessionCwdEnv:                t.sessionCwdPath(session),
	}
	if owner != "" {
		env[sessionOwnerEnv] = owner
	}
	return env
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}
	return "/"
}

// reportSessions publishes the session counts into the state file
// `kit daemon status` reads.
//
// Both numbers, because the difference is what a user needs: a hosted
// session comes back after a restart and a directly-hosted one does not.
func (t *sessionTable) reportSessions() {
	t.mu.Lock()
	active, hosted := len(t.sessions), 0
	for _, sess := range t.sessions {
		if sess.io != nil && sess.io.Hosted() {
			hosted++
		}
	}
	t.mu.Unlock()
	t.rt.setSessions(active, hosted)
}

// sessionCount reports how many logical sessions are live.
func (t *sessionTable) sessionCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.sessions)
}
