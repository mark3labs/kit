package daemon

import (
	"bytes"
	"io"
	"sync"
	"testing"
)

func TestConnSetLocalIDsAreReserved(t *testing.T) {
	cs := newConnSet()
	sink := newFrameSink(io.Discard)
	local := cs.addLocal(sink)
	if local.id < localWireBase {
		t.Fatalf("local wire id %d is below the reserved base %d", local.id, localWireBase)
	}
	if !local.local {
		t.Fatal("local connection is not flagged local")
	}
	// The sidecar allocates from 1 upward; those must never reach the
	// local range or two clients would share one id.
	remote := cs.addRemote(1, sink)
	if remote.id >= localWireBase {
		t.Fatalf("remote wire id %d collided with the local range", remote.id)
	}
	if cs.get(local.id) != local || cs.get(remote.id) != remote {
		t.Fatal("connections did not resolve back to themselves")
	}
}

func TestConnSetRemoveRemotesKeepsLocals(t *testing.T) {
	cs := newConnSet()
	sink := newFrameSink(io.Discard)
	local := cs.addLocal(sink)
	cs.addRemote(1, sink)
	cs.addRemote(2, sink)

	dropped := cs.removeRemotes()
	if len(dropped) != 2 {
		t.Fatalf("expected 2 dropped sidecar connections, got %d", len(dropped))
	}
	if cs.get(1) != nil || cs.get(2) != nil {
		t.Fatal("a sidecar connection survived the tunnel teardown")
	}
	if cs.get(local.id) == nil {
		t.Fatal("the local connection was dropped with the sidecar")
	}
}

func TestFrameSinkSerializesConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	sink := newFrameSink(&buf)

	// Every frame carries a distinct payload length so an interleaved
	// write shows up as a corrupt header on read-back.
	var wg sync.WaitGroup
	for i := 1; i <= 32; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = sink.write(Frame{Type: FrameData, Session: uint32(n), Payload: bytes.Repeat([]byte{byte(n)}, n)})
		}(i)
	}
	wg.Wait()

	seen := map[uint32]int{}
	for {
		f, err := ReadFrame(&buf)
		if err != nil {
			break
		}
		seen[f.Session] = len(f.Payload)
	}
	if len(seen) != 32 {
		t.Fatalf("expected 32 intact frames, got %d", len(seen))
	}
	for session, n := range seen {
		if int(session) != n {
			t.Fatalf("frame %d has payload length %d — writes interleaved", session, n)
		}
	}
}

func TestFrameSinkWriteAfterCloseFails(t *testing.T) {
	sink := newFrameSink(&bytes.Buffer{})
	sink.close()
	if err := sink.write(Frame{Type: FramePing}); err == nil {
		t.Fatal("expected a write to a closed sink to fail")
	}
}

func TestSessionTableWriteToUnknownWireIsNotFatal(t *testing.T) {
	table := newSessionTable(newDaemonRuntime(nil))
	// A reply addressed to a client that has gone must report the closed
	// connection rather than panic: sessions outlive their clients.
	if err := table.writeTo(Frame{Type: FrameBye, Session: 999}); err == nil {
		t.Fatal("expected an error writing to an unregistered wire id")
	}
}
