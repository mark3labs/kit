package daemon

import (
	"errors"
	"io"
	"sync"
)

// Transport abstraction for the daemon's frame plumbing.
//
// The daemon multiplexes every client on a single 32-bit wire id (see
// protocol.go). Two transports share the space:
//
//   - the in-process iroh endpoint, which assigns each remote connection
//     an id below localWireBase (iroh_serve.go), and
//   - the local Unix socket, where ids carry the top bit (localWireBase)
//     so the two allocators can never collide.
//
// A connection's frames are written through a frameSink, which serializes
// writes to one stream. Every connection owns its own sink.

// localWireBase is the first wire id handed to a local socket client. Ids
// at or above it are local by construction.
const localWireBase uint32 = 1 << 31

// errSinkClosed is returned when a frame is written to a sink whose
// underlying stream has gone away.
var errSinkClosed = errors.New("daemon: connection is closed")

// frameSink serializes frame writes to one stream. A sink is shared by
// every connection multiplexed over that stream, so the mutex covers the
// whole frame (header plus payload) and never interleaves two frames.
type frameSink struct {
	mu     sync.Mutex
	w      io.Writer
	closed bool
}

func newFrameSink(w io.Writer) *frameSink { return &frameSink{w: w} }

// write emits one frame. It is safe for concurrent use.
func (s *frameSink) write(f Frame) error {
	if s == nil {
		return errSinkClosed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.w == nil {
		return errSinkClosed
	}
	return WriteFrame(s.w, f.Type, f.Session, f.Payload)
}

// close marks the sink dead so later writes fail fast instead of erroring
// against a torn-down stream.
func (s *frameSink) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

// wireConn is one client connection: a wire id plus the sink its frames go
// out on. Sessions hold wire ids, not connections, so a connection can go
// away (detach, network loss) while its sessions keep running.
type wireConn struct {
	id    uint32
	sink  *frameSink
	local bool

	// term describes the terminal this client renders in. The daemon owns
	// a PTY, which reports none of it, so a session's child is spawned
	// against what the client reported here. Guarded by connSet.mu, so it
	// is read and written through the set rather than directly.
	term TerminalInfo

	// spec describes how a NEW session on this connection should be
	// started: working directory, arguments, environment. Nil means the
	// client asked for nothing in particular, which is the directory
	// picker in the daemon user's home. Guarded by connSet.mu.
	spec *SessionSpec
}

// connSet tracks the live client connections by wire id.
type connSet struct {
	mu        sync.Mutex
	nextLocal uint32
	conns     map[uint32]*wireConn
}

func newConnSet() *connSet {
	return &connSet{conns: make(map[uint32]*wireConn)}
}

// addRemote registers a remote (iroh) connection under the id its
// handshake assigned. Re-registering an id replaces the old entry.
func (c *connSet) addRemote(id uint32, sink *frameSink) *wireConn {
	conn := &wireConn{id: id, sink: sink}
	c.mu.Lock()
	c.conns[id] = conn
	c.mu.Unlock()
	return conn
}

// addLocal allocates a fresh local wire id and registers the connection.
func (c *connSet) addLocal(sink *frameSink) *wireConn {
	c.mu.Lock()
	c.nextLocal++
	id := localWireBase + c.nextLocal
	conn := &wireConn{id: id, sink: sink, local: true}
	c.conns[id] = conn
	c.mu.Unlock()
	return conn
}

// setTerminal records what a client said about its terminal. It arrives
// before the attach, so a new session's child can be spawned already
// describing the terminal it will be seen in. A frame for a connection
// that has already gone is dropped.
func (c *connSet) setTerminal(id uint32, info TerminalInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if conn, ok := c.conns[id]; ok {
		conn.term = info
	}
}

// terminalFor returns what a client reported about its terminal, or the
// zero value when it reported nothing (or has since disconnected).
func (c *connSet) terminalFor(id uint32) TerminalInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	if conn, ok := c.conns[id]; ok {
		return conn.term
	}
	return TerminalInfo{}
}

// setSpec records how a client wants its next new session started.
//
// Refused for a remote connection: a working directory and an argument
// list from another machine describe nothing on this one, and honouring
// argv from a paired peer would make pairing equivalent to arbitrary
// execution. Reports whether the spec was accepted, so the caller can say
// so in the log rather than dropping it silently.
func (c *connSet) setSpec(id uint32, spec SessionSpec) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	conn, ok := c.conns[id]
	if !ok || !conn.local {
		return false
	}
	stored := spec
	conn.spec = &stored
	return true
}

// consumeSpec returns the spec for a new session on this connection and
// leaves behind what the NEXT new session should inherit.
//
// The arguments describe one invocation and are spent here; the directory
// and environment describe the terminal the client is sitting in and stay
// for as long as it is connected. See inheritedSpec.
func (c *connSet) consumeSpec(id uint32) *SessionSpec {
	c.mu.Lock()
	defer c.mu.Unlock()
	conn, ok := c.conns[id]
	if !ok || conn.spec == nil {
		return nil
	}
	spec := conn.spec
	conn.spec = inheritedSpec(spec)
	return spec
}

// remove drops one connection.
func (c *connSet) remove(id uint32) {
	c.mu.Lock()
	delete(c.conns, id)
	c.mu.Unlock()
}

// get resolves a wire id to its connection, or nil when the client is gone.
func (c *connSet) get(id uint32) *wireConn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conns[id]
}

// live reports whether a client connection is still registered.
//
// Used to tell "the client is waiting for this" from "the client gave up
// while we were working on it", which decides whether a session spawned
// for a request still has anyone to belong to.
func (c *connSet) live(id uint32) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.conns[id]
	return ok
}

// removeRemotes drops every remote connection, leaving local ones alone.
func (c *connSet) removeRemotes() []uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	var dropped []uint32
	for id, conn := range c.conns {
		if !conn.local {
			dropped = append(dropped, id)
			delete(c.conns, id)
		}
	}
	return dropped
}
