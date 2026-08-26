// Package termgfx detects which inline-graphics protocols the attached
// terminal will actually honour.
//
// Detection is empirical, not a lookup table: the probe writes a Kitty
// graphics query and a primary device attributes (DA1) request, then reads the
// replies. A terminal that understands the Kitty graphics protocol answers the
// query; every terminal answers DA1, so the DA1 reply is the guaranteed
// terminator that keeps the probe from waiting for a response that will never
// arrive.
//
// Empirical detection matters because a terminal's name does not predict what
// it can do. It matters most under a multiplexer, where the answer can be
// borrowed: both tmux and zellij forward the graphics query to the terminal
// behind them and let that terminal answer, which makes the multiplexer look
// capable when it cannot place an image. The probe therefore also requires the
// pty to report pixel geometry, which is what a forwarding multiplexer lacks.
//
// Measured on 2026-08-26 with kitty 0.48.2:
//
//   - kitty: answers the query and reports a cell size. Graphics used.
//   - zellij 0.45.0: answers (via kitty) and reports no pixel geometry. It
//     also strips the combining marks that Unicode placeholders depend on, so
//     nothing would draw. Half blocks used.
//   - tmux with allow-passthrough on: answers DA1 itself while forwarding the
//     graphics query onward, so the terminal's reply arrives after the probe
//     has stopped reading and leaks into the input stream as visible garbage.
//     The query is therefore not sent under a multiplexer at all.
//
// See internal/ui/imagepreview for the two renderers this chooses between.
package termgfx

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/log"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

// Capabilities records the graphics protocols the terminal accepted during the
// probe. A zero value means "no inline graphics", which is the safe default:
// callers fall back to half-block rendering.
type Capabilities struct {
	// KittyGraphics reports that the terminal answered the Kitty graphics
	// query, and therefore that graphics escape sequences reach it intact.
	KittyGraphics bool

	// Sixel reports that DA1 advertised attribute 4 (sixel graphics). It is a
	// free by-product of the probe's terminator and is not used for
	// thumbnails today.
	Sixel bool

	// CellWidth and CellHeight are the pixel dimensions of one terminal cell,
	// or zero when the terminal reports no pixel geometry.
	//
	// They are part of the graphics decision, not just a detail for scaling. A
	// multiplexer can forward the graphics query to the terminal behind it and
	// let that terminal answer, which makes the multiplexer look capable when
	// it cannot actually place an image. Pixel geometry is what such a
	// multiplexer does not report, so requiring it turns a borrowed answer into
	// a truthful one. See ptyPixelSize.
	CellWidth  int
	CellHeight int

	// TrueColor reports that the terminal accepts 24-bit colour.
	//
	// It is captured when capabilities are resolved rather than read at draw
	// time, so that the preview mode is a pure function of this struct. It
	// matters for graphics because placeholder cells carry the image id in
	// their foreground colour: a 256-colour terminal would quantise that colour
	// and corrupt the id.
	TrueColor bool

	// UnicodePlaceholders reports that placeholder cells reach the terminal
	// intact, and therefore that an image can be drawn as view text.
	//
	// This is a separate capability from the protocol itself. Zellij 0.45
	// forwards the graphics protocol and draws real images, but discards the
	// combining marks that tell each placeholder cell which part of the image
	// it holds, so placeholders render nothing there while direct placement
	// works. Naming the capability rather than the terminal keeps the decision
	// in one place: a future release that fixes the marks changes detection,
	// not the renderer.
	UnicodePlaceholders bool
}

// kittyProbeID is the image id used by the query. Kitty echoes the id back in
// its reply, which lets the probe tell its own response apart from any
// unrelated APC traffic. The value is arbitrary but must be non-zero.
const kittyProbeID = 31

// kittyQuery asks the terminal to validate a one-pixel RGB image without
// displaying it: a=q (query action), s=1,v=1 (1x1), f=24 (24-bit RGB),
// t=d (direct payload). "AAAA" is base64 for three zero bytes, i.e. one black
// pixel. A terminal that implements the protocol replies with
// "\x1b_Gi=31;OK\x1b\\"; one that does not replies with nothing.
const kittyQuery = "\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\"

// requestCellSize asks the terminal for the pixel size of one cell. The reply
// is CSI 6 ; height ; width t.
const requestCellSize = "\x1b[16t"

// defaultTimeout bounds the probe. Terminals answer in single-digit
// milliseconds; the budget exists only so a terminal that answers neither the
// Kitty query nor DA1 cannot stall startup.
const defaultTimeout = 500 * time.Millisecond

// EnvOverride is the environment variable that bypasses detection.
//
//	KIT_IMAGE_PROTOCOL=kitty      force the Kitty graphics protocol
//	KIT_IMAGE_PROTOCOL=halfblock  force half-block thumbnails
//	KIT_IMAGE_PROTOCOL=auto       probe the terminal (default)
const EnvOverride = "KIT_IMAGE_PROTOCOL"

// caps holds the resolved capabilities. It is nil until Resolve or Set runs,
// so the blocking, fd-touching probe is never an import-time side effect. This
// mirrors the lazy-capability pattern in internal/ui/style.
var (
	capsMu sync.RWMutex
	caps   *Capabilities
)

// Resolve probes the process terminal once and caches the result. Later calls
// are no-ops.
//
// Call this during startup, before the TUI takes over stdin: the probe reads
// raw bytes from stdin, so it must not run while the event loop owns the same
// fd. Headless frontends should call Set instead, so no fd that does not
// describe their client is ever probed.
func Resolve() {
	capsMu.Lock()
	if caps != nil {
		capsMu.Unlock()
		return
	}
	resolved := detect(os.Stdin, os.Stdout, defaultTimeout)
	caps = &resolved
	capsMu.Unlock()

	// Log after releasing the lock, and derive the decision from the resolved
	// value rather than re-reading the cache. sync.RWMutex is not reentrant, so
	// anything here that called Current would deadlock the whole program before
	// the TUI starts.
	log.Debug("terminal graphics probe",
		"kitty", resolved.KittyGraphics,
		"sixel", resolved.Sixel,
		"cell", fmt.Sprintf("%dx%d", resolved.CellWidth, resolved.CellHeight),
		"use_kitty", previewMode(resolved).String(),
		"tmux", insideTmux(),
		"zellij", insideZellij())
}

// Set overrides the detected capabilities. It exists for headless frontends,
// which know their client's capabilities out of band, and for tests.
func Set(c Capabilities) {
	capsMu.Lock()
	caps = &c
	capsMu.Unlock()
}

// Current returns the resolved capabilities, or the zero value when nothing
// has been resolved yet. It never probes: an unresolved value degrades to
// half-block rendering rather than reading stdin at render time.
func Current() Capabilities {
	capsMu.RLock()
	defer capsMu.RUnlock()
	if caps == nil {
		return Capabilities{}
	}
	return *caps
}

// SupportsKittyGraphics reports whether inline Kitty graphics reach the
// terminal intact.
func SupportsKittyGraphics() bool {
	return Current().KittyGraphics
}

// Mode is how image previews should be drawn.
type Mode int

const (
	// ModeHalfBlock draws previews as coloured half-block text. It works in
	// any terminal with 256 colours and survives multiplexers untouched.
	ModeHalfBlock Mode = iota

	// ModePlaceholder draws previews with the Kitty graphics protocol using
	// Unicode placeholder cells, so the image travels with the view text.
	ModePlaceholder

	// ModeDirect draws previews with the Kitty graphics protocol using a
	// direct placement at the cursor. It is for terminals that carry the
	// protocol but drop the combining marks placeholders depend on.
	ModeDirect
)

// String implements fmt.Stringer.
func (m Mode) String() string {
	switch m {
	case ModePlaceholder:
		return "placeholder"
	case ModeDirect:
		return "direct"
	default:
		return "halfblock"
	}
}

// PreviewMode reports how image previews should be drawn in this terminal.
//
// Kitty graphics need protocol support, a reported cell size, and truecolor:
//
//   - Cell size, because a multiplexer that merely forwards the graphics query
//     to the terminal behind it will answer yes without being able to draw.
//     See [Capabilities.CellWidth].
//   - Truecolor, because placeholder cells carry the image id in their
//     foreground colour. See [Capabilities.TrueColor].
//
// The choice between the two graphics modes is about the placeholder encoding
// rather than the protocol. See [Capabilities.UnicodePlaceholders].
func PreviewMode() Mode {
	return previewMode(Current())
}

// previewMode is the pure core of PreviewMode. It decides only from the
// resolved capabilities, and reads neither the environment nor the cache.
//
// Keeping it pure matters twice over. It is called for every thumbnail render,
// so it must not re-sniff the environment each time; and reading the cache
// here would make it unsafe to call while holding capsMu, which is how a
// startup deadlock was introduced once already.
func previewMode(c Capabilities) Mode {
	if !c.KittyGraphics || c.CellWidth <= 0 || c.CellHeight <= 0 || !c.TrueColor {
		return ModeHalfBlock
	}
	if !c.UnicodePlaceholders {
		return ModeDirect
	}
	return ModePlaceholder
}

// UseKittyGraphics reports whether image previews use the Kitty graphics
// protocol rather than half-block text.
func UseKittyGraphics() bool {
	return PreviewMode() != ModeHalfBlock
}

// detect applies the environment override, then falls back to the live probe.
func detect(in, out *os.File, timeout time.Duration) Capabilities {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvOverride))) {
	case "kitty":
		// Forcing the protocol must also supply a cell size, because
		// UseKittyGraphics treats a missing one as "not really capable". Use
		// the terminal's real geometry when it offers one so the image is
		// resampled at the right resolution, and fall back to a plausible
		// default when it does not — the override means the user is asserting
		// support the probe cannot confirm.
		c := Capabilities{
			KittyGraphics:       true,
			TrueColor:           true,
			UnicodePlaceholders: unicodePlaceholdersWork(),
		}
		c.CellWidth, c.CellHeight = detectCellSize(out)
		if c.CellWidth <= 0 || c.CellHeight <= 0 {
			c.CellWidth, c.CellHeight = fallbackCellWidth, fallbackCellHeight
		}
		return c
	case "halfblock", "none", "off":
		return Capabilities{}
	}

	c, err := Probe(in, out, timeout)
	if err != nil {
		// A failed probe is not an error condition for the caller: it means
		// "assume no graphics" and use the half-block path.
		return Capabilities{}
	}
	return c
}

// fallbackCellWidth and fallbackCellHeight stand in for a cell size the
// terminal declines to report, and are used only when the user has forced the
// protocol on. They only affect resampling resolution: the terminal scales the
// image into the requested cell box either way.
const (
	fallbackCellWidth  = 10
	fallbackCellHeight = 20
)

// detectCellSize returns the pixel size of one cell, or zeros when the terminal
// reports no pixel geometry.
func detectCellSize(out *os.File) (width, height int) {
	if out == nil {
		return 0, 0
	}
	fd := int(out.Fd())
	pxW, pxH := ptyPixelSize(fd)
	if pxW <= 0 || pxH <= 0 {
		return 0, 0
	}
	cols, rows, err := terminalGrid(fd)
	if err != nil || cols <= 0 || rows <= 0 {
		return 0, 0
	}
	return pxW / cols, pxH / rows
}

// Probe queries the terminal directly, bypassing both the cache and the
// environment override.
//
// in must be the terminal's input and out its output; both are put in raw mode
// for the duration of the call. An error means the probe could not run (not a
// terminal, raw mode refused, write failed), which callers should read as
// "no graphics support".
func Probe(in, out *os.File, timeout time.Duration) (Capabilities, error) {
	if in == nil || out == nil {
		return Capabilities{}, fmt.Errorf("probe: nil terminal file")
	}
	inFd, outFd := int(in.Fd()), int(out.Fd())
	if !term.IsTerminal(inFd) || !term.IsTerminal(outFd) {
		return Capabilities{}, fmt.Errorf("probe: input/output is not a terminal")
	}
	// TERM=dumb terminals do not parse escape sequences, so the query would be
	// echoed as literal text over the user's screen.
	if t := os.Getenv("TERM"); t == "" || t == "dumb" {
		return Capabilities{}, fmt.Errorf("probe: terminal type %q cannot answer queries", t)
	}

	state, err := term.MakeRaw(inFd)
	if err != nil {
		return Capabilities{}, fmt.Errorf("probe: enter raw mode: %w", err)
	}
	defer func() { _ = term.Restore(inFd, state) }()

	// Ask about graphics everywhere except tmux. tmux answers DA1 itself while
	// forwarding the graphics query onward, so the terminal's reply arrives
	// after the probe has stopped reading and is printed into the TUI as
	// visible garbage. Zellij forwards both consistently and answers in order.
	var query string
	if !insideTmux() {
		query = kittyQuery
	}
	// Ask for the cell size explicitly. This is a standard sequence, not a
	// graphics one, so multiplexers either answer it themselves or forward it
	// safely. It is the only cell-size source inside zellij, which reports no
	// pty pixel geometry of its own.
	query += requestCellSize
	// DA1 goes last: every terminal answers it, so it bounds the read whether
	// or not the other queries were understood.
	query += ansi.RequestPrimaryDeviceAttributes

	c, err := query_(in, out, timeout, query)
	if err != nil {
		return c, err
	}

	// Prefer the pty's own pixel geometry, which is the terminal describing
	// itself, and fall back to whatever the cell-size query reported.
	if w, h := detectCellSize(out); w > 0 && h > 0 {
		c.CellWidth, c.CellHeight = w, h
	}

	// Capture the remaining capabilities now, so that every later decision is
	// a pure function of this struct rather than a fresh look at the
	// environment. See [Capabilities.TrueColor].
	c.TrueColor = colorprofile.Env(os.Environ()) >= colorprofile.TrueColor
	c.UnicodePlaceholders = unicodePlaceholdersWork()
	return c, nil
}

// unicodePlaceholdersWork reports whether placeholder cells survive the trip to
// the terminal.
//
// There is no query for this, so it is a known-bad list of one: zellij 0.45
// forwards the graphics protocol and draws real images, but strips the
// combining marks that placeholders are built from. Everything else is assumed
// to pass text through unchanged, which is the safe assumption — a terminal
// that mangles combining marks would garble ordinary text too.
func unicodePlaceholdersWork() bool {
	return !insideZellij()
}

// terminalGrid returns the terminal size in cells.
func terminalGrid(fd int) (cols, rows int, err error) {
	return term.GetSize(fd)
}

// cancelGrace is how long the probe waits for the read loop to unwind after
// the timeout cancels it. A cancelled tty read returns at once; the grace
// period only covers readers whose Cancel is a no-op, so the probe returns a
// partial answer instead of blocking its caller forever.
const cancelGrace = 100 * time.Millisecond

// query_ writes the query and classifies the replies until DA1 arrives or the
// timeout cancels the read. The read loop follows the same
// cancel-reader + sequence-decoder shape lipgloss uses for its OSC background
// query, so an escape sequence split across two reads is reassembled rather
// than misparsed.
//
// The loop runs on its own goroutine so the timeout bounds this function even
// when the underlying reader cannot be cancelled. Capabilities classified
// before the cancellation are still reported.
func query_(in io.Reader, out io.Writer, timeout time.Duration, query string) (Capabilities, error) {
	rd, err := uv.NewCancelReader(in)
	if err != nil {
		return Capabilities{}, fmt.Errorf("probe: create cancel reader: %w", err)
	}
	defer func() { _ = rd.Close() }()

	if _, err := io.WriteString(out, query); err != nil {
		return Capabilities{}, fmt.Errorf("probe: write query: %w", err)
	}

	// partial is written by the read loop and read here after a timeout, so it
	// carries its own lock.
	var (
		mu      sync.Mutex
		partial Capabilities
	)
	done := make(chan Capabilities, 1)

	go func() {
		pa := ansi.GetParser()
		defer ansi.PutParser(pa)

		var c Capabilities
		var acc []byte    // one decoded sequence, reassembled across reads
		var buf [256]byte // replies are far shorter than this
		var state byte
		for {
			n, err := rd.Read(buf[:])
			if err != nil {
				// Cancelled or closed before DA1 arrived.
				done <- c
				return
			}

			p := buf[:]
			for n > 0 {
				seq, _, read, newState := ansi.DecodeSequence(p[:n], state, pa)
				acc = append(acc, seq...)

				if newState == ansi.NormalState {
					terminated := classify(acc, pa, &c)
					mu.Lock()
					partial = c
					mu.Unlock()
					if terminated {
						done <- c
						return
					}
					acc = acc[:0]
				}

				state = newState
				n -= read
				p = p[read:]
			}
		}
	}()

	select {
	case c := <-done:
		return c, nil
	case <-time.After(timeout):
	}

	rd.Cancel()
	select {
	case c := <-done:
		return c, nil
	case <-time.After(cancelGrace):
		mu.Lock()
		defer mu.Unlock()
		return partial, nil
	}
}

// classify inspects one decoded sequence and updates c. It returns true when
// the sequence is the DA1 reply, which terminates the probe.
func classify(seq []byte, pa *ansi.Parser, c *Capabilities) bool {
	switch {
	case ansi.HasApcPrefix(seq):
		// A Kitty graphics reply looks like "\x1b_Gi=31;OK\x1b\\". Requiring
		// the echoed id keeps an unrelated APC message from being mistaken
		// for support; requiring OK keeps an error reply (ENOTSUPP, EBADF)
		// from counting as one.
		data := string(pa.Data())
		if strings.HasPrefix(data, "G") &&
			strings.Contains(data, fmt.Sprintf("i=%d", kittyProbeID)) &&
			strings.Contains(data, ";OK") {
			c.KittyGraphics = true
		}
	case ansi.HasCsiPrefix(seq):
		switch pa.Command() {
		case 't': // window manipulation report
			// CSI 6 ; height ; width t answers the cell-size request.
			if kind, _ := pa.Param(0, 0); kind == 6 {
				h, _ := pa.Param(1, 0)
				w, _ := pa.Param(2, 0)
				if w > 0 && h > 0 {
					c.CellWidth, c.CellHeight = w, h
				}
			}
		case ansi.Command('?', 0, 'c'): // DA1
			// DA1 attribute 4 advertises sixel graphics.
			for i := range pa.Params() {
				if v, _ := pa.Param(i, 0); v == 4 {
					c.Sixel = true
				}
			}
			return true
		}
	}
	return false
}

// insideTmux reports whether the process is running in a tmux pane.
func insideTmux() bool {
	return os.Getenv("TMUX") != "" || strings.HasPrefix(os.Getenv("TERM"), "tmux")
}

// insideZellij reports whether the process is running in a zellij pane.
func insideZellij() bool {
	return os.Getenv("ZELLIJ") != ""
}
