//go:build !windows

package daemon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestHostTerminalUpdateTransport(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	transport := newHostIO(1, client, SessionHostInfo{})
	info := TerminalInfo{Term: "xterm-kitty", Background: "#ffffff", Multiplexer: "tmux"}
	result := make(chan error, 1)
	go func() { result <- transport.UpdateTerminal(info) }()
	frame, err := ReadFrame(server)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if frame.Type != FrameTerminal {
		t.Fatalf("update sent terminal input or wrong frame: %v", frame.Type)
	}
	path := filepath.Join(t.TempDir(), "terminal.json")
	host := &sessionHost{cfg: SessionHostConfig{Env: map[string]string{RemoteTerminalFileEnv: path}}}
	var wire bytes.Buffer
	if err := WriteFrame(&wire, frame.Type, frame.Session, frame.Payload); err != nil {
		t.Fatal(err)
	}
	// No PTY is needed: capability frames must never write terminal input.
	host.readDaemon(t.Context(), &wire)
	got, err := ReadTerminalCapabilities(path)
	if err != nil || !reflect.DeepEqual(got, info) {
		t.Fatalf("host snapshot = %+v, %v; want %+v", got, err, info)
	}
}

// This helper is a real child: it reads the file only after SIGWINCH.
func TestTerminalCapabilityChild(t *testing.T) {
	if os.Getenv("KIT_TEST_TERMINAL_CHILD") != "1" {
		return
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	defer signal.Stop(ch)
	fmt.Println("ready")
	<-ch
	info, err := ReadTerminalCapabilities(os.Getenv(RemoteTerminalFileEnv))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(info); err != nil {
		t.Fatal(err)
	}
}

func TestFrameTerminalWhileAttachedUpdatesChild(t *testing.T) {
	for _, hosted := range []bool{false, true} {
		t.Run(fmt.Sprintf("hosted_%t", hosted), func(t *testing.T) {
			table := newTestTable(t)
			path := table.sessionEnv(1, "")[RemoteTerminalFileEnv]
			if err := writeTerminalCapabilities(path, TerminalInfo{Background: "#000000"}); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestTerminalCapabilityChild$")
			cmd.Env = append(os.Environ(), "KIT_TEST_TERMINAL_CHILD=1", RemoteTerminalFileEnv+"="+path)
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
			lines := make(chan string, 4)
			go func() {
				defer close(lines)
				scanner := bufio.NewScanner(out)
				for scanner.Scan() {
					lines <- scanner.Text()
				}
			}()
			nextLine := func() string {
				t.Helper()
				select {
				case line, ok := <-lines:
					if !ok {
						t.Fatal("child exited without a snapshot")
					}
					return line
				case <-time.After(5 * time.Second):
					t.Fatal("child did not receive the terminal update")
					return ""
				}
			}
			if got := nextLine(); got != "ready" {
				t.Fatalf("child startup = %q", got)
			}
			sess := table.fakeSession(1)
			if hosted {
				daemonSide, hostSide := net.Pipe()
				defer func() { _ = daemonSide.Close() }()
				defer func() { _ = hostSide.Close() }()
				sess.io = newHostIO(1, daemonSide, SessionHostInfo{})
				host := &sessionHost{cmd: cmd, cfg: SessionHostConfig{Env: map[string]string{RemoteTerminalFileEnv: path}}}
				done := make(chan struct{})
				go func() { defer close(done); host.readDaemon(t.Context(), hostSide) }()
				defer func() { _ = hostSide.Close(); <-done }()
			} else {
				sess.io = &ptyIO{cmd: cmd, terminalPath: path}
			}
			var replies lockedBuffer
			conn := table.conns.addLocal(newFrameSink(&replies))
			// Attach first, with no terminal description, then send a live
			// update through the actual server frame path.
			table.attachSession(t.Context(), conn.id, encodeSessionID(1))
			profile := 3
			want := TerminalInfo{Term: "xterm", ColorProfile: &profile, Background: "#ffffff"}
			payload, err := EncodeTerminalInfo(want)
			if err != nil {
				t.Fatal(err)
			}
			var wire bytes.Buffer
			if err := WriteFrame(&wire, FrameTerminal, 0, payload); err != nil {
				t.Fatal(err)
			}
			if err := table.runFrameSource(t.Context(), &wire, conn.id); err != nil {
				t.Fatal(err)
			}
			got, err := DecodeTerminalInfo([]byte(nextLine()))
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("child snapshot = %+v, %v; want %+v", got, err, want)
			}
			if got := table.conns.terminalFor(conn.id); !reflect.DeepEqual(got, want) {
				t.Fatalf("connection snapshot = %+v", got)
			}
		})
	}
}
