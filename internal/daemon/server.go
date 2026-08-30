package daemon

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
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

	// If the tunnel process dies unexpectedly (crash, kill), restart it
	// with the same identity: the endpoint id is stable, so paired clients
	// find us again. Live sessions do not survive the restart.
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		tun, err := StartTunnel(ctx, TunnelOptions{
			Mode: "serve",
			Args: []string{"--timeout", "30"},
			Env:  []string{"KIT_TUNNEL_SECRET=" + secretHex},
		})
		if err != nil {
			return err
		}
		ready, err := tun.WaitStatus(ctx, "READY", 30*time.Second)
		if err != nil {
			tun.Close()
			return fmt.Errorf("daemon: tunnel failed to start: %w", err)
		}
		nodeID, _ := strings.CutPrefix(ready, "READY node_id=")
		rt.setEndpoint(nodeID)
		fmt.Printf("  Endpoint:     %s\n", shortEndpoint(nodeID))
		fmt.Println("  Waiting for paired clients. Pair a new one with: kit daemon pair")
		fmt.Println()

		err = runSessions(ctx, tun, rt)

		tun.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
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
type remoteSession struct {
	id   uint32
	cmd  *exec.Cmd
	ptmx *os.File
}

// authChallenge is an in-flight reconnect handshake awaiting the client's
// signature. Keyed by the 8-byte correlation key (first bytes of c_nonce).
type authChallenge struct {
	clientPub []byte
	cNonce    []byte
	sNonce    []byte
}

// sessionTable tracks live sessions. The tunnel's stdout frames are read by
// a single goroutine, so map access is confined to it plus teardown paths
// guarded by mu.
type sessionTable struct {
	tunnel       *Tunnel
	rt           *daemonRuntime
	mu           sync.Mutex
	sessions     map[uint32]*remoteSession
	pendingAuths map[[8]byte]authChallenge      // confined to the frame loop
	clipboards   map[uint32]*ClipboardCollector // in-flight image transfers, frame loop only
	sessionTemps map[uint32][]string            // temp files injected into each child
	writeMu      sync.Mutex                     // tunnel stdin is shared by all session pumps
}

// writeTo sends one frame to the tunnel stdin. Errors are the caller's to
// ignore or handle; a closed tunnel means everything is going away anyway.
func (t *sessionTable) writeTo(frame Frame) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return WriteFrame(t.tunnel.Stdin(), frame.Type, frame.Session, frame.Payload)
}

func runSessions(ctx context.Context, tun *Tunnel, rt *daemonRuntime) error {
	table := &sessionTable{
		tunnel:       tun,
		rt:           rt,
		sessions:     make(map[uint32]*remoteSession),
		pendingAuths: make(map[[8]byte]authChallenge),
		clipboards:   make(map[uint32]*ClipboardCollector),
		sessionTemps: make(map[uint32][]string),
	}
	defer table.teardownAll()

	// Child exits are noticed by the per-session PTY reader; when a client
	// detaches (BYE) or the tunnel drops the session (SESSION_CLOSED), the
	// reader loop tears that one session down.
	for {
		frame, err := ReadFrame(tun.Stdout())
		if err != nil {
			return nil // tunnel stream/process ended
		}
		switch frame.Type {
		case FrameAuthRequest:
			table.handleAuthRequest(frame.Payload)
		case FrameAuthPayload:
			table.handleAuthPayload(frame.Payload)
		case FrameClipboard:
			table.handleClipboardChunk(ctx, frame.Session, frame.Payload)
		case FrameSessionOpen:
			table.openSession(frame.Session)
		case FrameSessionClosed, FrameBye:
			table.closeSession(frame.Session)
		case FrameData:
			if s := table.get(frame.Session); s != nil {
				if _, err := s.ptmx.Write(frame.Payload); err != nil {
					table.closeSession(frame.Session)
				}
			}
		case FrameResize:
			if s := table.get(frame.Session); s != nil {
				if cols, rows, derr := DecodeResize(frame.Payload); derr == nil {
					_ = pty.Setsize(s.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
				}
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
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
func (t *sessionTable) handleClipboardChunk(ctx context.Context, session uint32, payload []byte) {
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

// openSession spawns a fresh `kit --pick-dir` child for a newly verified
// client. A failure to spawn is reported to that client as a BYE; the
// daemon and other sessions continue.
func (table *sessionTable) openSession(id uint32) {
	s := &remoteSession{id: id}
	child, ptmx, err := spawnPickDir(id)
	if err != nil {
		log.Error("daemon: session spawn failed", "session_id", id, "error", err)
		_ = table.writeTo(Frame{Type: FrameBye, Session: id})
		return
	}
	s.cmd, s.ptmx = child, ptmx

	table.mu.Lock()
	table.sessions[id] = s
	active := len(table.sessions)
	table.mu.Unlock()
	table.rt.setSessions(active)
	log.Info("session started", "session_id", id)

	// PTY -> remote: raw child output as DATA frames tagged with the id.
	go func(s *remoteSession) {
		buf := make([]byte, chunkSize)
		for {
			n, err := s.ptmx.Read(buf)
			if n > 0 {
				// Write errors mean the tunnel is gone; the restart loop
				// takes over from here.
				_ = table.writeTo(Frame{Type: FrameData, Session: s.id, Payload: buf[:n]})
			}
			if err != nil {
				return // EIO when the child exits, or PTY closed
			}
		}
	}(s)

	// Child lifecycle: when the child quits (user exited the remote TUI),
	// end exactly this session — other clients are unaffected.
	go func(s *remoteSession) {
		_, _ = s.cmd.Process.Wait()
		table.closeSession(s.id)
	}(s)
}

// applyResize was removed: the child exists before the session is
// registered and stdout frames are processed in order, so a RESIZE always
// finds a live PTY. Resizes that arrive during the handshake are buffered
// by the pipe and applied right after registration.

// closeSession tears down one session: tell the client we are done, stop
// the child, free the table slot, and remove any clipboard tempfiles that
// were injected but never consumed. Idempotent.
func (t *sessionTable) closeSession(id uint32) {
	t.mu.Lock()
	s, ok := t.sessions[id]
	delete(t.sessions, id)
	temps := t.sessionTemps[id]
	delete(t.sessionTemps, id)
	delete(t.clipboards, id)
	active := len(t.sessions)
	t.mu.Unlock()
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

	_ = t.writeTo(Frame{Type: FrameBye, Session: id})

	if s.ptmx != nil {
		_ = s.ptmx.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	log.Info("session ended", "session_id", id)
}

func (t *sessionTable) teardownAll() {
	t.mu.Lock()
	ids := make([]uint32, 0, len(t.sessions))
	for id := range t.sessions {
		ids = append(ids, id)
	}
	t.mu.Unlock()
	for _, id := range ids {
		t.closeSession(id)
	}
}

func (t *sessionTable) get(id uint32) *remoteSession {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sessions[id]
}

// remoteClipboardPath is the stable per-session file the daemon streams
// client clipboard images into. The child reads it on every Ctrl-V (see
// internal/clipboard.RemoteClipboardEnv), so a paste is a file rewrite
// followed by a synthetic 0x16 keystroke — the child's own clipboard
// pipeline then renders the preview exactly like a local paste.
func remoteClipboardPath(session uint32) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("kit-remote-clip-%d", session))
}

// spawnPickDir starts a kit child with the hidden --pick-dir flag in the
// daemon user's home directory, so the remote peer picks the session's
// working directory from the modal rendered inside the PTY.
func spawnPickDir(session uint32) (*exec.Cmd, *os.File, error) {
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
