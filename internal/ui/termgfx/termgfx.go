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
//     placeholder cells draw nothing. Graphics used, placed directly at the
//     cursor instead.
//   - tmux with allow-passthrough on: answers DA1 itself while forwarding the
//     graphics query onward, so the terminal's reply arrives after the probe
//     has stopped reading and leaks into the input stream as visible garbage.
//     The query is therefore not sent under a multiplexer at all.
//
// A daemon session splits the two halves across machines: the probe runs in
// the child on the daemon host, while the terminal — and any multiplexer
// wrapped around it — is on the client. The client names its multiplexer for
// the child; see RemoteMultiplexerEnv.
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
	// decides only between the two graphics modes: placeholder cells carry the
	// image id as a 24-bit foreground colour, which a smaller palette would
	// quantise into a different id, whereas a direct placement carries the id
	// in the escape sequence and does not care.
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

// RemoteMultiplexerEnv names the multiplexer the CLIENT of a daemon session
// runs inside, as one of the Multiplexer constants, or is unset when the
// client owns a bare terminal.
//
// A daemon session's child renders on the daemon host while the terminal, and
// any multiplexer around it, lives on the client machine. TMUX and ZELLIJ
// describe a process's own pane, so they never cross the wire, and the child
// therefore cannot see what will handle the escape sequences it writes. Left
// to itself it probes a terminal that answers through tmux, believes the
// answer, and draws Kitty graphics that tmux discards — leaving the empty
// placeholder cells the image should have filled. The client names its
// multiplexer instead and the daemon plants it here, so the child reaches the
// decision the client would have reached locally.
const RemoteMultiplexerEnv = "KIT_REMOTE_MULTIPLEXER"

// The multiplexers detection knows by name. They are the values carried by
// RemoteMultiplexerEnv and returned by LocalMultiplexer.
const (
	// MultiplexerTmux is tmux, which drops graphics escapes unless
	// allow-passthrough is on and the sequence is wrapped for it.
	MultiplexerTmux = "tmux"
	// MultiplexerScreen is GNU screen, which drops them outright. tmux's
	// own default TERM also names screen, so this covers a tmux that was
	// not otherwise identified.
	MultiplexerScreen = "screen"
	// MultiplexerZellij is zellij, which forwards graphics but strips the
	// combining marks Unicode placeholders are built from.
	MultiplexerZellij = "zellij"
)

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
		"multiplexer", multiplexer(),
		"remote", os.Getenv(RemoteMultiplexerEnv) != "")
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
// Kitty graphics need protocol support and a reported cell size. The cell size
// matters because a multiplexer that merely forwards the graphics query to the
// terminal behind it will answer yes without being able to draw; see
// [Capabilities.CellWidth].
//
// The choice between the two graphics modes turns on whether placeholder cells
// survive to the terminal and whether it has truecolor to carry the image id
// in. See [Capabilities.UnicodePlaceholders] and [Capabilities.TrueColor].
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
	if !c.KittyGraphics || c.CellWidth <= 0 || c.CellHeight <= 0 {
		return ModeHalfBlock
	}
	// Truecolor gates the placeholder encoding rather than graphics as a whole.
	// Placeholder cells carry the image id as a 24-bit foreground colour, which
	// a smaller palette would quantise into a different id, leaving the
	// terminal with nothing to draw. A direct placement carries the id in the
	// escape sequence itself, so it is unaffected and remains the better
	// fallback for a graphics-capable terminal that lacks truecolor.
	if c.UnicodePlaceholders && c.TrueColor {
		return ModePlaceholder
	}
	return ModeDirect
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
		// PreviewMode treats a missing one as "not really capable". Use the
		// terminal's real geometry when it offers one so the image is resampled
		// at the right resolution, and fall back to a plausible default when it
		// does not — the override means the user is asserting support the probe
		// cannot confirm.
		//
		// Truecolor is still resolved honestly rather than assumed. The user is
		// asserting graphics support, not a colour depth, and claiming truecolor
		// that is not there would select placeholder cells whose image id gets
		// quantised into a different id. Reporting it truthfully instead falls
		// back to a direct placement, which carries the id in the escape
		// sequence and draws correctly at any colour depth.
		c := Capabilities{
			KittyGraphics:       true,
			TrueColor:           terminalTrueColor(),
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
	// visible garbage. Worse, believing that borrowed answer makes the renderer
	// emit graphics tmux then throws away. Zellij forwards both consistently
	// and answers in order.
	var query string
	if !insideTmuxLike() {
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
	c.TrueColor = terminalTrueColor()
	c.UnicodePlaceholders = unicodePlaceholdersWork()
	return c, nil
}

// terminalTrueColor reports whether the terminal accepts 24-bit colour.
func terminalTrueColor() bool {
	return colorprofile.Env(os.Environ()) >= colorprofile.TrueColor
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

// LocalMultiplexer reports the multiplexer this process runs inside, named by
// the Multiplexer constants, or "" when it talks to a terminal directly.
//
// It reads only this process's own environment, so a daemon session client
// calls it to describe itself and the child reads RemoteMultiplexerEnv
// instead; see multiplexer.
func LocalMultiplexer() string {
	if os.Getenv("ZELLIJ") != "" {
		return MultiplexerZellij
	}
	if os.Getenv("TMUX") != "" {
		return MultiplexerTmux
	}
	// TERM is the fallback, and the reason screen is named at all: tmux's
	// default TERM is screen-256color, so a tmux whose TMUX variable did not
	// reach this process still identifies itself here.
	term := os.Getenv("TERM")
	switch {
	case strings.HasPrefix(term, "tmux"):
		return MultiplexerTmux
	case strings.HasPrefix(term, "screen"):
		return MultiplexerScreen
	}
	return ""
}

// multiplexer reports the multiplexer standing between this process and the
// terminal that will draw its output.
//
// The client's answer wins when there is one: a daemon session child inherits
// the client's TERM but none of its pane variables, and the daemon's own
// environment describes a terminal that is not the one being drawn to.
func multiplexer() string {
	if m := strings.ToLower(strings.TrimSpace(os.Getenv(RemoteMultiplexerEnv))); m != "" {
		return m
	}
	return LocalMultiplexer()
}

// insideTmuxLike reports whether output passes through a multiplexer that
// discards graphics escape sequences, which is what makes the graphics query
// unsafe to ask and its answer unsafe to believe.
func insideTmuxLike() bool {
	switch multiplexer() {
	case MultiplexerTmux, MultiplexerScreen:
		return true
	}
	return false
}

// insideZellij reports whether output passes through a zellij pane.
func insideZellij() bool {
	return multiplexer() == MultiplexerZellij
}
