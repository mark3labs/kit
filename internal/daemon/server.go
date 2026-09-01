package daemon

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
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
	if _, err := FindTunnelBinary(); err != nil {
		return err // fail fast with a clear message instead of per attempt
	}

	seed, err := LoadDaemonIdentity()
	if err != nil {
		return err
	}
	secretHex := hex.EncodeToString(seed)

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

	// If the tunnel process dies unexpectedly (crash, kill), restart it
	// with the same identity: the endpoint id is stable, so paired clients
	// find us again. Live sessions do not survive the restart.
	for {
		if ctx.Err() != nil {
			table.killAll()
			return ctx.Err()
		}
		tun, err := StartTunnel(ctx, TunnelOptions{
			Mode: "serve",
			Args: []string{"--timeout", "30"},
			Env:  []string{"KIT_TUNNEL_SECRET=" + secretHex},
		})
		if err != nil {
			table.killAll()
			return err
		}
		ready, err := tun.WaitStatus(ctx, "READY", 30*time.Second)
		if err != nil {
			tun.Close()
			table.killAll()
			return fmt.Errorf("daemon: tunnel failed to start: %w", err)
		}
		nodeID, _ := strings.CutPrefix(ready, "READY node_id=")
		rt.setEndpoint(nodeID)
		rt.setTunnel(tun)
		fmt.Printf("  Endpoint:     %s\n", shortEndpoint(nodeID))
		fmt.Println("  Waiting for paired clients. Pair a new one with: kit daemon pair")
		fmt.Println()

		err = runSessions(ctx, tun, rt, table)

		tun.Close()
		rt.setTunnel(nil)
		// Client connections died with the tunnel; their sessions keep
		// running detached and can be reattached after the restart.
		table.unbindAll()
		if ctx.Err() != nil {
			table.killAll()
			return ctx.Err()
		}
		if err != nil {
			table.killAll()
			return fmt.Errorf("daemon: tunnel ended: %w", err)
		}
		fmt.Println("  Listener restarted — endpoint unchanged, waiting…")
	}
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
// per-connection id the sidecar assigns) attach to it; a session with zero
// attached clients is detached but keeps running until the child exits or
// the daemon shuts down.
type remoteSession struct {
	id      uint64 // logical id, daemon-assigned monotonic
	cmd     *exec.Cmd
	ptmx    *os.File
	started time.Time

	mu      sync.Mutex
	clients map[uint32]winSize // attached wire ids -> last known size (0,0 = unknown)
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

// authChallenge is an in-flight reconnect handshake awaiting the client's
// signature. Keyed by the 8-byte correlation key (first bytes of c_nonce).
type authChallenge struct {
	clientPub []byte
	cNonce    []byte
	sNonce    []byte
}

// sessionTable owns the daemon's LOGICAL sessions. The tunnel's stdout
// frames are read by a single goroutine (runSessions), so most map access
// is confined to it; teardown paths run on other goroutines and take mu.
// The table is created once per daemon process and outlives tunnel
// restarts — only wire-id bindings are per tunnel.
type sessionTable struct {
	rt    *daemonRuntime
	conns *connSet

	mu           sync.Mutex
	nextID       uint64
	sessions     map[uint64]*remoteSession      // logical id -> session
	wireMap      map[uint32]uint64              // wire id -> logical id
	pendingAuths map[[8]byte]authChallenge      // confined to the frame loop
	clipboards   map[uint64]*ClipboardCollector // in-flight image transfers, frame loop only
	sessionTemps map[uint64][]string            // per-session clipboard files
}

func newSessionTable(rt *daemonRuntime) *sessionTable {
	return &sessionTable{
		rt:           rt,
		conns:        newConnSet(),
		sessions:     make(map[uint64]*remoteSession),
		wireMap:      make(map[uint32]uint64),
		pendingAuths: make(map[[8]byte]authChallenge),
		clipboards:   make(map[uint64]*ClipboardCollector),
		sessionTemps: make(map[uint64][]string),
	}
}

// writeTo sends one frame to the connection that owns frame.Session. A
// missing connection means the client has gone (detach, network loss, or a
// dead sidecar); the frame is dropped and the caller carries on, because
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

// runSessions reads frames from the CURRENT tunnel and drives the shared
// session table. The table outlives this call: when the tunnel ends, every
// sidecar-backed connection is dropped (those clients are gone; their
// sessions stay running detached) and Serve restarts the tunnel.
func runSessions(ctx context.Context, tun *Tunnel, rt *daemonRuntime, table *sessionTable) error {
	table.rebindTunnel()
	sink := newFrameSink(tun.Stdin())
	rt.setSink(sink)
	defer sink.close()
	return table.runFrameSource(ctx, tun.Stdout(), sink, 0)
}

// runFrameSource reads frames from one transport until the stream ends.
//
// fixedWire selects the addressing mode. Zero means the peer stamps the
// wire id itself, which is what the sidecar does when it relays several
// remote clients over one stream. Nonzero means a single-client transport
// (the local Unix socket): the client has no id allocator of its own and
// sends every frame with session 0, so the daemon stamps its assigned id
// on arrival. Replies carry the id back out, and single-client peers
// ignore it.
func (t *sessionTable) runFrameSource(ctx context.Context, r io.Reader, sink *frameSink, fixedWire uint32) error {
	for {
		frame, err := ReadFrame(r)
		if err != nil {
			return nil // stream ended
		}
		if fixedWire != 0 {
			frame.Session = fixedWire
		}
		switch frame.Type {
		case FrameAuthRequest:
			t.handleAuthRequest(frame.Payload)
		case FrameAuthPayload:
			t.handleAuthPayload(frame.Payload)
		case FrameSessionOpen:
			// The sidecar announces every new client connection with
			// this frame, and guarantees it reaches us before any other
			// frame for that id. Registering the connection here is what
			// lets replies find their way back out. No session is
			// spawned: that waits for an explicit attach.
			t.conns.addRemote(frame.Session, sink)
		case FrameSessionDetach, FrameSessionClosed, FrameBye:
			t.detachWire(frame.Session)
			if fixedWire == 0 {
				t.conns.remove(frame.Session)
			}
		case FrameSessionList:
			t.sendSessionList(frame.Session)
		case FrameSessionAttach:
			t.attachSession(frame.Session, frame.Payload)
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

// rebindTunnel clears the bindings of a dead tunnel. Sidecar-backed client
// connections died with it; their logical sessions stay running detached,
// and local socket clients are untouched.
func (t *sessionTable) rebindTunnel() {
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
	if _, remaining := sess.detachClient(wire); remaining == 0 {
		log.Info("session detached", "session_id", logical)
	} else {
		log.Info("client left shared session", "session_id", logical, "remaining", remaining)
	}
}

// unbindAll drops every sidecar-backed connection (the tunnel ended).
// Logical sessions keep running detached, and local clients stay live.
func (t *sessionTable) unbindAll() {
	t.rebindTunnel()
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

// applyResize records one client's size and applies the minimum across all
// attached clients to the PTY — the shared-view equivalent of tmux picking
// the smallest window.
func (s *remoteSession) applyResize(wire uint32, cols, rows int) {
	next := s.resizeClient(wire, winSize{cols, rows})
	if next.cols == 0 || next.rows == 0 {
		return // nobody has reported a real size yet
	}
	_ = pty.Setsize(s.ptmx, &pty.Winsize{Cols: uint16(next.cols), Rows: uint16(next.rows)})
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
		path := remoteClipboardPath(session)
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
		if err := os.WriteFile(remoteClipboardPath(session), nil, 0o600); err != nil {
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

	// Atomic rewrite: a concurrent child read sees the old or the new
	// image, never a torn one.
	tmp, err := os.CreateTemp("", "kit-clip-*")
	if err != nil {
		log.Error("daemon: clipboard write failed", "session_id", session, "error", err)
		return
	}
	path := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(path)
		log.Error("daemon: clipboard write failed", "session_id", session, "error", werr, "close", cerr)
		return
	}
	if err := os.Rename(path, remoteClipboardPath(session)); err != nil {
		_ = os.Remove(path)
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

// handleAuthRequest stashes the handshake parameters so the signature can// handleAuthRequest stashes the handshake parameters so the signature can
// be verified when the client's AUTH_PAYLOAD arrives.
func (t *sessionTable) handleAuthRequest(payload []byte) {
	if len(payload) < 8 {
		log.Warn("short auth request frame", "len", len(payload))
		return // nothing to correlate a denial with; drop
	}
	if len(payload) != 32+32+32 {
		log.Warn("malformed auth request", "len", len(payload))
		t.decideAuth(payload[:8], false, "malformed auth request")
		return
	}
	corr := [8]byte(payload[0:8])
	t.pendingAuths[corr] = authChallenge{
		clientPub: payload[64:96],
		cNonce:    payload[0:32],
		sNonce:    payload[32:64],
	}
	log.Info("auth request", "fp", Fingerprint(payload[64:96]))
}

// handleAuthPayload verifies the client's signature against the allowlist
// and answers the sidecar's consultation. Payload: c_nonce(32) | sig(64);
// the correlation key is the first 8 bytes of c_nonce.
func (t *sessionTable) handleAuthPayload(payload []byte) {
	if len(payload) != 32+64 {
		log.Warn("malformed auth payload", "len", len(payload))
		return
	}
	corr := [8]byte(payload[0:8])
	sig := payload[32:]
	challenge, ok := t.pendingAuths[corr]
	if !ok {
		log.Warn("auth payload without request", "corr", hex.EncodeToString(corr[:]))
		t.decideAuth(corr[:], false, "unknown handshake")
		return
	}
	// Drop the stashed challenge on every path below: the sidecar gets an
	// answer either way, and the map cannot grow under repeated
	// request/payload floods.
	delete(t.pendingAuths, corr)
	fp := Fingerprint(challenge.clientPub)
	entry, authorized, err := LookupClient(fp)
	if err != nil {
		t.decideAuth(corr[:], false, "allowlist error")
		return
	}
	if !authorized {
		log.Warn("client not paired", "fp", fp)
		t.decideAuth(corr[:], false, "client not paired — run 'kit daemon pair' on the host")
		return
	}
	pub, err := hex.DecodeString(entry.PubKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		t.decideAuth(corr[:], false, "corrupt allowlist entry")
		return
	}
	msg := append([]byte(signContext), challenge.cNonce...)
	msg = append(msg, challenge.sNonce...)
	if !ed25519.Verify(ed25519.PublicKey(pub), msg, sig) {
		log.Warn("bad client signature", "fp", fp)
		t.decideAuth(corr[:], false, "bad signature")
		return
	}
	_ = TouchClient(fp)
	log.Info("client authorized", "fp", fp)
	t.decideAuth(corr[:], true, "")
}

// decideAuth answers the sidecar's consultation. The payload mirrors what
// the Rust side parses: correlation key (8), verdict byte, optional reason.
func (t *sessionTable) decideAuth(corr []byte, allow bool, reason string) {
	out := make([]byte, 0, 8+1+len(reason))
	out = append(out, corr...)
	if allow {
		out = append(out, 1)
	} else {
		out = append(out, 0)
	}
	out = append(out, reason...)
	_ = t.writeTo(Frame{Type: FrameAuthDecision, Session: 0, Payload: out})
}

// sessionInfo is one row of the client-facing session list.
type sessionInfo struct {
	ID      uint64 `json:"id"`
	Clients int    `json:"clients"`
	Started string `json:"started"`
	Cwd     string `json:"cwd,omitempty"`
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
		}
		if sess.cmd != nil && sess.cmd.Process != nil {
			if cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", sess.cmd.Process.Pid)); err == nil {
				info.Cwd = cwd
			}
		}
		infos = append(infos, info)
	}
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

		child, ptmx, err := spawnPickDir(logical)
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
			log.Info("session started", "session_id", logical, "wire", wire)
			t.watchSession(s)
			ok = 1
		}
	} else {
		t.mu.Lock()
		sess := t.sessions[requested]
		if sess != nil {
			// A wire id drives at most one session: drop any previous
			// binding first (that session keeps running detached).
			if prev, had := t.wireMap[wire]; had && prev != requested {
				if ps := t.sessions[prev]; ps != nil {
					ps.detachClient(wire)
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
		_ = s.cmd.Process.Kill()
	}
	log.Info("session ended", "session_id", id)
}

// remoteClipboardPath is the stable per-session file the daemon streams
// client clipboard images into. The child reads it on every Ctrl-V (see
// internal/clipboard.RemoteClipboardEnv), so a paste is a file rewrite
// followed by a synthetic 0x16 keystroke — the child's own clipboard
// pipeline then renders the preview exactly like a local paste.
func remoteClipboardPath(session uint64) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("kit-remote-clip-%d", session))
}

// spawnPickDir starts a kit child with the hidden --pick-dir flag in the
// daemon user's home directory, so the remote peer picks the session's
// working directory from the modal rendered inside the PTY.
func spawnPickDir(session uint64) (*exec.Cmd, *os.File, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve kit binary: %w", err)
	}

	cmd := exec.Command(exe, "--pick-dir")
	cmd.Dir = homeDir()
	env := append(os.Environ(), "KIT_REMOTE_SESSION=1")
	if os.Getenv("TERM") == "" {
		env = append(env, "TERM=xterm-256color")
	}
	env = append(env, clipboard.RemoteClipboardEnv+"="+remoteClipboardPath(session))
	cmd.Env = env

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
