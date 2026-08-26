package termgfx

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// fakeTerminal replays a canned reply stream, so the probe's read loop can be
// exercised without a real tty.
type fakeTerminal struct {
	reply  *bytes.Reader
	out    bytes.Buffer
	chunks int // when >0, return at most this many bytes per Read
}

func (f *fakeTerminal) Read(p []byte) (int, error) {
	if f.chunks > 0 && len(p) > f.chunks {
		p = p[:f.chunks]
	}
	return f.reply.Read(p)
}

func (f *fakeTerminal) Write(p []byte) (int, error) { return f.out.Write(p) }

const (
	kittyOK  = "\x1b_Gi=31;OK\x1b\\"
	da1Plain = "\x1b[?62;22c"
	da1Sixel = "\x1b[?62;4;22c"
)

func TestQueryClassifiesReplies(t *testing.T) {
	tests := []struct {
		name      string
		reply     string
		wantKitty bool
		wantSixel bool
	}{
		{"kitty then da1", kittyOK + da1Plain, true, false},
		{"da1 only", da1Plain, false, false},
		{"sixel only", da1Sixel, false, true},
		{"kitty and sixel", kittyOK + da1Sixel, true, true},
		{"kitty error reply", "\x1b_Gi=31;ENOTSUPPORTED\x1b\\" + da1Plain, false, false},
		{"foreign apc ignored", "\x1b_Xhello\x1b\\" + da1Plain, false, false},
		{"wrong image id ignored", "\x1b_Gi=99;OK\x1b\\" + da1Plain, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := &fakeTerminal{reply: bytes.NewReader([]byte(tt.reply))}
			got, err := query_(ft, ft, time.Second, "query")
			if err != nil {
				t.Fatalf("query_: %v", err)
			}
			if got.KittyGraphics != tt.wantKitty {
				t.Errorf("KittyGraphics = %v, want %v", got.KittyGraphics, tt.wantKitty)
			}
			if got.Sixel != tt.wantSixel {
				t.Errorf("Sixel = %v, want %v", got.Sixel, tt.wantSixel)
			}
			if ft.out.String() != "query" {
				t.Errorf("wrote %q, want %q", ft.out.String(), "query")
			}
		})
	}
}

// A reply split across reads must still be decoded as one sequence.
func TestQueryReassemblesSplitSequences(t *testing.T) {
	ft := &fakeTerminal{
		reply:  bytes.NewReader([]byte(kittyOK + da1Sixel)),
		chunks: 3,
	}
	got, err := query_(ft, ft, time.Second, "q")
	if err != nil {
		t.Fatalf("query_: %v", err)
	}
	if !got.KittyGraphics || !got.Sixel {
		t.Errorf("got %+v, want both capabilities", got)
	}
}

// A terminal that answers nothing must not hang the caller: the timeout has to
// cancel a read that would otherwise block forever.
func TestQueryTimesOutWithoutReply(t *testing.T) {
	pr, pw := io.Pipe() // never written to, so every Read blocks
	t.Cleanup(func() { _ = pw.Close() })

	start := time.Now()
	got, err := query_(pr, io.Discard, 50*time.Millisecond, "q")
	if err != nil {
		t.Fatalf("query_: %v", err)
	}
	if got != (Capabilities{}) {
		t.Errorf("got %+v, want zero capabilities", got)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("probe took %v, want it bounded by the timeout", elapsed)
	}
}

// A stream that ends mid-probe must report what was classified so far rather
// than an error.
func TestQueryHandlesEarlyEOF(t *testing.T) {
	ft := &fakeTerminal{reply: bytes.NewReader(nil)}
	got, err := query_(ft, ft, time.Second, "q")
	if err != nil {
		t.Fatalf("query_: %v", err)
	}
	if got != (Capabilities{}) {
		t.Errorf("got %+v, want zero capabilities", got)
	}
}

// Zellij needs a direct placement: it forwards the graphics protocol but drops
// the combining marks that Unicode placeholders depend on.
func TestPreviewModeChoosesDirectInZellij(t *testing.T) {
	t.Cleanup(func() {
		capsMu.Lock()
		caps = nil
		capsMu.Unlock()
	})
	t.Setenv("COLORTERM", "truecolor")
	capable := Capabilities{KittyGraphics: true, CellWidth: 10, CellHeight: 20}

	t.Setenv("ZELLIJ", "0")
	if got := previewMode(capable); got != ModeDirect {
		t.Errorf("previewMode() in zellij = %v, want %v", got, ModeDirect)
	}

	t.Setenv("ZELLIJ", "")
	if got := previewMode(capable); got != ModePlaceholder {
		t.Errorf("previewMode() in a bare terminal = %v, want %v", got, ModePlaceholder)
	}
}

// Graphics stay off inside a multiplexer even when the probe reports support,
// because the reported support is borrowed from the terminal behind it.
func TestUseKittyGraphicsRequiresCellSize(t *testing.T) {
	t.Cleanup(func() {
		capsMu.Lock()
		caps = nil
		capsMu.Unlock()
	})
	t.Setenv("COLORTERM", "truecolor")

	Set(Capabilities{KittyGraphics: true, CellWidth: 0, CellHeight: 0})
	if UseKittyGraphics() {
		t.Error("UseKittyGraphics() = true without a reported cell size")
	}
	Set(Capabilities{KittyGraphics: true, CellWidth: 10, CellHeight: 20})
	if !UseKittyGraphics() {
		t.Error("UseKittyGraphics() = false with support and a cell size")
	}
	Set(Capabilities{KittyGraphics: false, CellWidth: 10, CellHeight: 20})
	if UseKittyGraphics() {
		t.Error("UseKittyGraphics() = true without protocol support")
	}
}

func TestEnvOverride(t *testing.T) {
	// Forcing the protocol on must produce capabilities that UseKittyGraphics
	// actually accepts. Returning support without a cell size would make the
	// override silently do nothing.
	for _, v := range []string{"kitty", "KITTY"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv(EnvOverride, v)
			t.Setenv("COLORTERM", "truecolor")
			got := detect(nil, nil, time.Second)
			if !got.KittyGraphics {
				t.Errorf("detect() = %+v, want graphics support", got)
			}
			if got.CellWidth <= 0 || got.CellHeight <= 0 {
				t.Errorf("detect() = %+v, want a usable cell size", got)
			}
			Set(got)
			t.Cleanup(func() {
				capsMu.Lock()
				caps = nil
				capsMu.Unlock()
			})
			if !UseKittyGraphics() {
				t.Error("UseKittyGraphics() = false after forcing the protocol on")
			}
		})
	}

	for _, v := range []string{"halfblock", "none", "off"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv(EnvOverride, v)
			// os.Stdin under `go test` is not a terminal, so any value that
			// does not short-circuit would fall through to a failed probe.
			if got := detect(nil, nil, time.Second); got != (Capabilities{}) {
				t.Errorf("detect() = %+v, want no capabilities", got)
			}
		})
	}
}

func TestProbeRejectsNonTerminal(t *testing.T) {
	// go test replaces stdin with /dev/null, which is not a terminal.
	if _, err := Probe(nil, nil, time.Second); err == nil {
		t.Error("Probe(nil, nil) = nil error, want an error")
	}
}

func TestSetAndCurrent(t *testing.T) {
	t.Cleanup(func() {
		capsMu.Lock()
		caps = nil
		capsMu.Unlock()
	})

	Set(Capabilities{KittyGraphics: true})
	if !SupportsKittyGraphics() {
		t.Error("SupportsKittyGraphics() = false after Set, want true")
	}

	// Resolve must not overwrite an explicitly supplied value.
	Resolve()
	if !SupportsKittyGraphics() {
		t.Error("Resolve() overwrote the value supplied by Set")
	}
}

func TestUnresolvedDefaultsToNoGraphics(t *testing.T) {
	capsMu.Lock()
	caps = nil
	capsMu.Unlock()

	if Current() != (Capabilities{}) {
		t.Error("Current() on an unresolved cache reported graphics support")
	}
	if SupportsKittyGraphics() {
		t.Error("SupportsKittyGraphics() = true before resolution, want false")
	}
}

// The query the probe sends must carry the id the classifier looks for.
func TestKittyQueryCarriesProbeID(t *testing.T) {
	if !strings.Contains(kittyQuery, "i=31") || kittyProbeID != 31 {
		t.Errorf("kittyQuery %q and kittyProbeID %d disagree", kittyQuery, kittyProbeID)
	}
	if !strings.HasPrefix(kittyQuery, "\x1b_G") || !strings.HasSuffix(kittyQuery, "\x1b\\") {
		t.Errorf("kittyQuery %q is not a well-formed APC sequence", kittyQuery)
	}
}

// TestProbeLive runs the probe against the real controlling terminal and
// reports what it found. It is skipped unless KIT_LIVE_PROBE=1.
//
// Run it inside each terminal and multiplexer you care about:
//
//	KIT_LIVE_PROBE=1 go test -v -run TestProbeLive ./internal/ui/termgfx/
func TestProbeLive(t *testing.T) {
	if os.Getenv("KIT_LIVE_PROBE") != "1" {
		t.Skip("set KIT_LIVE_PROBE=1 to probe the real terminal")
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no controlling terminal: %v", err)
	}
	defer func() { _ = tty.Close() }()

	c, err := Probe(tty, tty, 2*time.Second)
	t.Logf("TERM=%q ZELLIJ=%q TMUX=%q", os.Getenv("TERM"), os.Getenv("ZELLIJ"), os.Getenv("TMUX"))
	t.Logf("caps=%+v err=%v", c, err)
	Set(c)
	t.Logf("UseKittyGraphics=%v", UseKittyGraphics())
}

// Resolve must not deadlock. It holds capsMu while caching the result, and
// sync.RWMutex is not reentrant, so any call it makes that reads the cache
// (Current, UseKittyGraphics) hangs the process before the TUI ever starts.
// That shipped once and made kit render nothing at all.
func TestResolveDoesNotDeadlock(t *testing.T) {
	t.Cleanup(func() {
		capsMu.Lock()
		caps = nil
		capsMu.Unlock()
	})
	// Short-circuit the probe so this exercises the locking, not a terminal.
	t.Setenv(EnvOverride, "halfblock")

	capsMu.Lock()
	caps = nil
	capsMu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		Resolve()
		// Reading the cache afterwards must work too: a Resolve that returned
		// while still holding the lock would block every later reader.
		_ = Current()
		_ = UseKittyGraphics()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Resolve deadlocked")
	}
}

// The second Resolve takes the early-return path, which must also release the
// lock.
func TestResolveIsIdempotent(t *testing.T) {
	t.Cleanup(func() {
		capsMu.Lock()
		caps = nil
		capsMu.Unlock()
	})
	t.Setenv(EnvOverride, "halfblock")

	done := make(chan struct{})
	go func() {
		defer close(done)
		Resolve()
		Resolve()
		_ = Current()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("second Resolve deadlocked")
	}
}
