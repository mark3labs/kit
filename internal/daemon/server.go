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

	// The session table outlives tunnel restarts: logical sessions keep
	// running (detached) while clients come and go.
	table := newSessionTable(rt)

	// Anything still running from a previous daemon run is unreachable:
	// its PTY master died with the process that owned it. Kill it now,
	// while the single-instance lock guarantees no live daemon owns those
	// pids, and clear the scratch files it left behind.
	sweepOrphanSessions(table.run)
	sweepStaleTempFiles(table.run)
	defer removeSessionRegistry()

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
			table.killAll()
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

// shutdown ends every session and reports a clean exit.
//
// A cancelled context is how a SIGINT/SIGTERM stop arrives, which is
// success, not failure: returning ctx.Err() here would exit non-zero and
// have systemd log "status=1/FAILURE" for an ordinary `systemctl stop`.
func shutdown(table *sessionTable) error {
	n := table.sessionCount()
	if n > 0 {
		log.Info("daemon: stopping sessions", "count", n)
	}
	table.killAll()
	removeSessionRegistry()
	return nil
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
	id      uint64 // logical id, daemon-assigned monotonic
	cmd     *exec.Cmd
	ptmx    *os.File
	started time.Time

	mu      sync.Mutex
	name    string             // user-set display name, empty until renamed
	clients map[uint32]winSize // attached wire ids -> last known size (0,0 = unknown)
}

// setName records a user-supplied display name.
func (s *remoteSession) setName(name string) {
	s.mu.Lock()
	s.name = name
	s.mu.Unlock()
}

// displayName returns the session's name, or "" when it has none.
func (s *remoteSession) displayName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

// nudgeRedraw makes the child repaint by changing the PTY size and putting
// it back. A full-screen TUI redraws on SIGWINCH, which is the only
// portable way to force a repaint of a child we do not emulate.
//
// Both size changes happen here rather than at the client, so the gap
// between them is a local sleep instead of two network round trips. The
// child needs to observe two distinct sizes: setting the same size twice
// is not a change and produces no repaint.
func (s *remoteSession) nudgeRedraw() {
	s.mu.Lock()
	size := minSizeLocked(s.clients)
	ptmx := s.ptmx
	s.mu.Unlock()
	if ptmx == nil || size.cols < 2 || size.rows < 2 {
		return
	}
	go func() {
		_ = pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(size.cols), Rows: uint16(size.rows - 1)})
		time.Sleep(40 * time.Millisecond)
		_ = pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(size.cols), Rows: uint16(size.rows)})
	}()
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
	// run identifies this daemon run. It tags registry records and
	// per-session temp files so a later run can tell its own state from a
	// crashed predecessor's.
	run string

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
		case FrameSessionList:
			t.sendSessionList(frame.Session)
		case FrameSessionAttach:
			t.attachSession(frame.Session, frame.Payload)
		case FrameSessionRedraw:
			if sess := t.logicalFor(frame.Session); sess != nil {
				sess.nudgeRedraw()
			}
		case FrameSessionRename:
			t.renameSession(frame.Payload)
		case FrameData:
			if sess := t.logicalFor(frame.Session); sess != nil {
				if _, err := sess.ptmx.Write(frame.Payload); err != nil {
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
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
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

// killAll tears down every logical session, killing children. Used on
// daemon shutdown.
func (t *sessionTable) killAll() {
	t.mu.Lock()
	ids := make([]uint64, 0, len(t.sessions))
	for id := range t.sessions {
		ids = append(ids, id)
	}
	t.mu.Unlock()
	for _, id := range ids {
		t.retireSession(id)
	}
}

// applySize sets the session's PTY to ws, if it names a real size.
//
// The zero size means no attached client has reported one yet, and the
// PTY is left at its default rather than resized to nothing.
func (s *remoteSession) applySize(ws winSize) {
	if ws.cols == 0 || ws.rows == 0 || s.ptmx == nil {
		return
	}
	_ = pty.Setsize(s.ptmx, &pty.Winsize{Cols: uint16(ws.cols), Rows: uint16(ws.rows)})
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
	var ptmx *os.File
	if live && s != nil {
		ptmx = s.ptmx
	}
	t.mu.Unlock()
	if !live || ptmx == nil {
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
	if _, err := fmt.Fprintf(ptmx, "%c", pasteKey); err != nil {
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
func (t *sessionTable) attachSession(wire uint32, payload []byte) {
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
		}
		t.sessions[logical] = s
		t.wireMap[wire] = logical
		s.attachClient(wire, winSize{})
		t.mu.Unlock()

		child, ptmx, err := t.spawnPickDir(logical, t.conns.terminalFor(wire))
		if err != nil {
			log.Error("daemon: session spawn failed", "session_id", logical, "error", err)
			t.retireSession(logical)
			ok = 0
		} else {
			s.cmd, s.ptmx = child, ptmx
			t.mu.Lock()
			active := len(t.sessions)
			t.mu.Unlock()
			t.rt.setSessions(active)
			t.syncSessionRegistry()
			log.Info("session started", "session_id", logical, "wire", wire)
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

	ack := make([]byte, 9)
	binary.BigEndian.PutUint64(ack[:8], logical)
	ack[8] = ok
	_ = t.writeTo(Frame{Type: FrameSessionAttachAck, Session: wire, Payload: ack})
	if ok == 0 {
		log.Warn("attach failed", "wire", wire, "requested", requested)
	}
}

// watchSession starts the per-session PTY fan-out reader and the child
// lifecycle watcher.
func (t *sessionTable) watchSession(s *remoteSession) {
	// PTY -> clients: raw child output as DATA frames fanned out to every
	// attached client (shared view).
	go func(sess *remoteSession) {
		buf := make([]byte, chunkSize)
		for {
			n, err := sess.ptmx.Read(buf)
			if n > 0 {
				for _, wire := range sess.clientIDs() {
					// Write errors mean the tunnel is gone; the restart
					// loop takes over from here.
					_ = t.writeTo(Frame{Type: FrameData, Session: wire, Payload: buf[:n]})
				}
			}
			if err != nil {
				return // EIO when the child exits, or PTY closed
			}
		}
	}(s)

	// Child lifecycle: when the child quits (user exited the remote TUI),
	// retire exactly this session — other sessions are unaffected.
	go func(sess *remoteSession) {
		_, _ = sess.cmd.Process.Wait()
		t.retireSession(sess.id)
	}(s)
}

// retireSession tears down one logical session: tell every attached client
// we are done, stop the child, free the table slot, and remove its
// clipboard files. Idempotent.
func (t *sessionTable) retireSession(id uint64) {
	t.mu.Lock()
	s, ok := t.sessions[id]
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
	active := len(t.sessions)
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
	t.rt.setSessions(active)
	for _, p := range temps {
		_ = os.Remove(p)
	}

	if s.ptmx != nil {
		_ = s.ptmx.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		// SIGTERM first: the child flushes its conversation store and
		// restores the terminal. SIGKILL only if it ignores that.
		//
		// Off the frame loop: retireSession runs inline there when a PTY
		// write fails, so waiting out the grace period here would stall
		// that client's whole frame loop for seconds.
		go terminateProcess(s.cmd.Process.Pid)
	}
	t.syncSessionRegistry()
	log.Info("session ended", "session_id", id)
}

// remoteClipboardPath is the stable per-session file the daemon streams
// client clipboard images into. The child reads it on every Ctrl-V (see
// internal/clipboard.RemoteClipboardEnv), so a paste is a file rewrite
// followed by a synthetic 0x16 keystroke — the child's own clipboard
// pipeline then renders the preview exactly like a local paste.
func (t *sessionTable) remoteClipboardPath(session uint64) string {
	return filepath.Join(t.scratchDir(),
		fmt.Sprintf("%sclip-%s-%d", tempFilePrefix, t.run, session))
}

// sessionCwdPath is the stable per-session file a session's child writes
// its working directory into, once the directory picker has resolved it.
//
// Reading /proc/<pid>/cwd would be simpler but only works on Linux, and it
// reports the child's cwd rather than the directory the user picked. The
// file follows the same convention as the clipboard file above.
func (t *sessionTable) sessionCwdPath(session uint64) string {
	return filepath.Join(t.scratchDir(),
		fmt.Sprintf("%scwd-%s-%d", tempFilePrefix, t.run, session))
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
	data, err := os.ReadFile(t.sessionCwdPath(s.id))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// spawnPickDir starts a kit child with the hidden --pick-dir flag in the
// daemon user's home directory, so the peer picks the session's working
// directory from the modal rendered inside the PTY.
//
// info describes the terminal of the client that asked for the session, so
// the child renders for that terminal rather than for the daemon's own
// environment.
func (t *sessionTable) spawnPickDir(session uint64, info TerminalInfo) (*exec.Cmd, *os.File, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve kit binary: %w", err)
	}

	cmd := exec.Command(exe, "--pick-dir")
	cmd.Dir = homeDir()
	own := map[string]string{
		clipboard.RemoteClipboardEnv: t.remoteClipboardPath(session),
		sessionCwdEnv:                t.sessionCwdPath(session),
	}
	// Mark the child with this daemon's runtime directory so a later
	// sweep can prove the process is ours before signalling it.
	if home, herr := daemonRuntimeDir(); herr == nil {
		own[sessionOwnerEnv] = home
	}
	// The child renders into the CLIENT's terminal, not the daemon's; see
	// childEnv for why the PTY between them cannot answer for it.
	cmd.Env = childEnv(os.Environ(), info, own)

	// Ask the kernel to kill this child if the daemon dies, so a crash
	// cannot leave an unreachable session running (see recovery.go).
	cmd.SysProcAttr = applyChildDeathSignal(cmd.SysProcAttr)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		return nil, nil, fmt.Errorf("start child: %w", err)
	}
	return cmd, ptmx, nil
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

// sessionCount reports how many logical sessions are live.
func (t *sessionTable) sessionCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.sessions)
}
