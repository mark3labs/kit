package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestTerminalCapabilitiesAtomicSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "terminal.json")
	profile := int(colorprofile.TrueColor)
	first := TerminalInfo{Term: "xterm-kitty", ColorTerm: "truecolor", Background: "#AABBCC", ColorProfile: &profile}
	if err := writeTerminalCapabilities(path, first); err != nil {
		t.Fatal(err)
	}
	first.Background = "#aabbcc"
	got, err := ReadTerminalCapabilities(path)
	if err != nil || !reflect.DeepEqual(got, first) {
		t.Fatalf("read = %+v, %v; want %+v", got, err, first)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %o", stat.Mode().Perm())
	}
	second := TerminalInfo{Term: "dumb", Background: BackgroundUnknown}
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 100 {
			info, err := ReadTerminalCapabilities(path)
			if err != nil || (!reflect.DeepEqual(info, first) && !reflect.DeepEqual(info, second)) {
				t.Errorf("partial snapshot: %+v, %v", info, err)
				return
			}
		}
	})
	for range 50 {
		for _, info := range []TerminalInfo{second, first} {
			if err := writeTerminalCapabilities(path, info); err != nil {
				t.Fatal(err)
			}
		}
	}
	wg.Wait()
}

func TestTerminalCapabilitiesReadErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "terminal.json")
	if _, err := ReadTerminalCapabilities(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error = %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadTerminalCapabilities(path); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}

func TestTerminalCapabilitiesProfileValues(t *testing.T) {
	for n, profile := range []colorprofile.Profile{colorprofile.NoTTY, colorprofile.ASCII, colorprofile.ANSI, colorprofile.ANSI256, colorprofile.TrueColor} {
		if int(profile) != n+1 {
			t.Fatalf("documented color profile %d = %d", n, profile)
		}
	}
}

func TestChildEnvDropsInheritedCapabilityFile(t *testing.T) {
	env := childEnv([]string{RemoteTerminalFileEnv + "=/stale"}, TerminalInfo{}, nil)
	if hasEnv(env, RemoteTerminalFileEnv) {
		t.Fatal("inherited capability file leaked into child")
	}
}

type recordingTerminalIO struct {
	noopSessionIO
	updates []TerminalInfo
}

func (r *recordingTerminalIO) UpdateTerminal(info TerminalInfo) error {
	r.updates = append(r.updates, info)
	return nil
}

func TestReattachUpdatesTerminalAndLegacyKeepsSnapshot(t *testing.T) {
	table := newTestTable(t)
	sess := table.fakeSession(1)
	recorder := &recordingTerminalIO{}
	sess.io = recorder
	var buf lockedBuffer
	conn := table.conns.addLocal(newFrameSink(&buf))
	info := TerminalInfo{Term: "xterm-256color", Background: "#ffffff"}
	table.conns.setTerminal(conn.id, info)
	table.attachSession(t.Context(), conn.id, encodeSessionID(1))
	if len(recorder.updates) != 1 || !reflect.DeepEqual(recorder.updates[0], info) {
		t.Fatalf("updates = %+v", recorder.updates)
	}
	table.conns.setTerminal(conn.id, TerminalInfo{})
	table.attachSession(t.Context(), conn.id, encodeSessionID(1))
	if len(recorder.updates) != 1 {
		t.Fatal("legacy client cleared capabilities")
	}
}

func TestTerminalCapabilitiesSurviveLiveSessionSweep(t *testing.T) {
	table := newTestTable(t)
	path := table.sessionEnv(7, "")[RemoteTerminalFileEnv]
	info := TerminalInfo{Background: "#ffffff"}
	if err := writeTerminalCapabilities(path, info); err != nil {
		t.Fatal(err)
	}
	sweepStaleTempFiles([]uint64{7})
	if got, err := ReadTerminalCapabilities(path); err != nil || !reflect.DeepEqual(got, info) {
		t.Fatalf("live snapshot after sweep = %+v, %v", got, err)
	}
	sweepStaleTempFiles(nil)
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ended session file was not swept: %v", err)
	}
}
