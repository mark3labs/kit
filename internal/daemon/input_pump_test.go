package daemon

import (
	"bytes"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

func TestInputPumpFeedEvent(t *testing.T) {
	for _, claimed := range []bool{true, false} {
		name := "unclaimed"
		if claimed {
			name = "claimed"
		}
		t.Run(name, func(t *testing.T) {
			var outcome attachOutcome
			p := &inputPump{setOutcome: func(out attachOutcome) { outcome = out }}
			prefix := []byte("\x1b[93;5u")
			release := []byte("\x1b[93;5:3u")
			if !p.feedEvent(keyEvent{Leader: true, Kind: leaderPrimary, Data: prefix}) {
				t.Fatal("leader stopped pump")
			}
			if !p.feedEvent(keyEvent{LeaderRelease: true, Data: release}) {
				t.Fatal("leader release stopped pump")
			}
			wantPrefix := append(append([]byte(nil), prefix...), release...)
			if !bytes.Equal(p.leaderBuf, wantPrefix) || p.leaderOf != leaderPrimary || len(p.pendingData) != 0 {
				t.Fatalf("leader state: buffer %q, kind %v, pending %q", p.leaderBuf, p.leaderOf, p.pendingData)
			}
			suffix := []byte("\x1b[A")
			if claimed {
				suffix = []byte{','} // Session management needs no connection write.
			}
			if got := p.feedEvent(keyEvent{Data: suffix}); got == claimed {
				t.Fatalf("continue = %t, claimed = %t", got, claimed)
			}
			if p.leaderBuf != nil || p.leaderOf != leaderNone {
				t.Fatal("suffix did not clear leader state")
			}
			if claimed {
				if len(p.pendingData) != 0 || !outcome.wantSwitch || outcome.switchTo != renameSentinel {
					t.Fatalf("claimed suffix: pending %q, outcome %+v", p.pendingData, outcome)
				}
			} else if want := append(wantPrefix, suffix...); !bytes.Equal(p.pendingData, want) {
				t.Fatalf("unclaimed bytes = %q, want %q", p.pendingData, want)
			}
		})
	}
}

func TestInputPumpFeedChunkRefreshReplyInSameRead(t *testing.T) {
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = outW
	defer func() { os.Stdout = old; _ = outW.Close(); _ = outR.Close() }()
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	if err := client.SetWriteDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	p := &inputPump{conn: newClientConn(client)}
	defer p.close()
	done := make(chan bool, 1)
	go func() {
		done <- p.feedChunk([]byte("before\x1dr\x1b]11;rgb:ffff/0000/8080\x1b\\after"))
	}()
	if err := server.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for i, wantType := range []FrameType{FrameData, FrameTerminal, FrameData} {
		frame, err := ReadFrame(server)
		if err != nil {
			t.Fatal(err)
		}
		if frame.Type != wantType {
			t.Fatalf("frame %d type = %v, want %v", i, frame.Type, wantType)
		}
		switch i {
		case 0, 2:
			want := "before"
			if i == 2 {
				want = "after"
			}
			if string(frame.Payload) != want {
				t.Fatalf("frame %d data = %q, want %q", i, frame.Payload, want)
			}
		case 1:
			info, err := DecodeTerminalInfo(frame.Payload)
			if err != nil || info.Background != "#ff0080" {
				t.Fatalf("terminal info = %+v, error %v", info, err)
			}
		}
	}
	if !<-done {
		t.Fatal("chunk stopped pump")
	}
	if p.probeC != nil || p.probe.active {
		t.Fatal("reply did not complete probe")
	}
	query := make([]byte, len("\x1b]11;?\x1b\\"))
	if err := outR.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(outR, query); err != nil || string(query) != "\x1b]11;?\x1b\\" {
		t.Fatalf("query = %q, error %v", query, err)
	}
}
