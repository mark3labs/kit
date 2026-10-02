package style

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/parser"
)

// This file makes raw terminal output safe to draw inside the TUI.
//
// A shell command that emits colour also emits everything else: cursor moves,
// erase-in-line, save/restore cursor, carriage returns for progress bars. A
// Bubble Tea viewport has no terminal underneath it that would honour those, so
// a program that repaints a progress bar corrupts the frame instead of updating
// one line. Two passes fix this:
//
//   - StripCursorControl removes the sequences that move the cursor or erase
//     the screen, and resolves carriage returns the way a terminal would.
//   - RemapANSI16 rewrites the 16 base SGR colours to explicit 24-bit values
//     taken from the active theme, so output does not inherit whatever the
//     user's terminal happens to use for "red" on this background.

// ---------------------------------------------------------------------------
// Palette
// ---------------------------------------------------------------------------

// ANSIPalette returns the 16 standard ANSI colours for a theme: indices 0-7 are
// the normal colours and 8-15 the bright ones.
//
// The palette is derived rather than configured. Command output is content the
// theme does not own, so the mapping reuses the theme slots that already carry
// the right meaning: Error is red, Success is green, Warning is amber, Info is
// blue, and so on. A theme that wants different raw-output colours would need
// sixteen more knobs in the theme file, which is a poor trade for output most
// users never look at closely.
//
// The result must be recomputed after a theme change, because it bakes in the
// colours resolved for the active background.
func ANSIPalette(t Theme) [16]color.Color {
	return [16]color.Color{
		// 0-7: normal.
		t.VeryMuted,
		t.Error,
		t.Success,
		t.Warning,
		t.Info,
		t.Accent,
		t.Secondary,
		t.Text,

		// 8-15: bright. Each is its normal counterpart moved away from the
		// background, which is what a terminal's "bright" variant is.
		ansibrighten(t.VeryMuted),
		ansibrighten(t.Error),
		ansibrighten(t.Success),
		ansibrighten(t.Warning),
		ansibrighten(t.Info),
		ansibrighten(t.Accent),
		ansibrighten(t.Secondary),
		ansibrighten(t.Text),
	}
}

// ansiBrightenMix is how far a bright ANSI slot is moved away from the plain
// one. A large move makes the two variants easy to tell apart at a glance,
// which is the whole point of a bright colour.
const ansiBrightenMix = 0.35

// ansibrighten moves c away from the reading extreme of the active background:
// toward white on a dark terminal, toward black on a light one.
//
// A nil colour returns nil, which leaves the terminal default in place. That is
// the right answer for a theme that does not define a slot.
func ansibrighten(c color.Color) color.Color {
	if c == nil {
		return nil
	}
	target := float64(255)
	if !isDarkBackground() {
		target = 0
	}
	r, g, b, _ := c.RGBA()
	mix := func(v uint32) uint8 {
		cur := float64(v >> 8)
		cur += (target - cur) * ansiBrightenMix
		if cur < 0 {
			cur = 0
		}
		if cur > 255 {
			cur = 255
		}
		return uint8(cur)
	}
	return color.RGBA{R: mix(r), G: mix(g), B: mix(b), A: 0xff}
}

// ---------------------------------------------------------------------------
// Cursor and screen control
// ---------------------------------------------------------------------------

// StripCursorControl removes the escape sequences that move the cursor, erase
// regions of the screen, or change terminal modes. Programs such as `git push`,
// `cargo build` and `npm install` emit all of them to animate progress bars.
// Replayed as text inside a viewport, those sequences corrupt the render state
// rather than animating anything.
//
// Preserved: SGR (colour and style) sequences, OSC hyperlinks, printable text.
// Stripped: CSI cursor movement, erase and scroll, save/restore cursor, DEC
// private mode set and reset, and the two-byte ESC save/restore sequences.
//
// Bare carriage returns are resolved too: the text after the last \r on a line
// replaces the text before it. This is what turns a progress bar that repaints
// itself into the single final state of that bar.
//
// This is not exactly what a real terminal shows. A terminal overwrites cell by
// cell, so `Downloading...\rDone` leaves `Doneding...` behind. Replacing the
// line instead keeps the transcript readable, which is the point of the
// transform. The one exception is a \r with nothing after it, which moves the
// cursor and writes nothing: there the text before it is kept, or a program that
// ends its line with a bare \r would lose that line entirely.
func StripCursorControl(s string) string {
	if !strings.ContainsRune(s, 0x1b) && !strings.ContainsRune(s, '\r') {
		return s
	}

	var buf strings.Builder
	buf.Grow(len(s))

	parser := ansi.GetParser()
	defer ansi.PutParser(parser)

	var state byte
	for len(s) > 0 {
		parser.Reset()
		seq, _, n, newState := ansi.DecodeSequence(s, state, parser)

		if ansi.HasCsiPrefix(seq) {
			switch parser.Command() & 0xff {
			case 'm':
				// SGR: colours and styles. Keep.
				buf.WriteString(seq)
			case 'h', 'l':
				// Mode set/reset. Never renders, so it is always safe to drop,
				// with or without the DEC private "?" prefix.
			case 'A', 'B', 'C', 'D', // cursor up, down, forward, back
				'E', 'F', // cursor next, previous line
				'G',      // cursor to column
				'H', 'f', // cursor position
				'J',      // erase display
				'K',      // erase line
				'S', 'T', // scroll up, down
				's', 'u': // save, restore cursor
				// Cursor and screen control. Drop.
			default:
				// Unknown. Keep it, so an unrecognised but harmless sequence
				// never turns into data loss.
				buf.WriteString(seq)
			}
		} else if ansi.HasEscPrefix(seq) && len(seq) == 2 {
			switch seq[1] {
			case '7', '8':
				// DEC save, restore cursor. Drop.
			default:
				buf.WriteString(seq)
			}
		} else {
			buf.WriteString(seq)
		}

		s = s[n:]
		state = newState
	}

	result := buf.String()
	if strings.ContainsRune(result, '\r') {
		result = resolveCarriageReturns(result)
	}
	return result
}

// resolveCarriageReturns keeps the text written after the last \r on each line.
// A \r with nothing after it is a cursor move with no write, so the text before
// it survives and the \r itself is dropped.
func resolveCarriageReturns(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		idx := strings.LastIndexByte(line, '\r')
		if idx < 0 {
			continue
		}
		if idx == len(line)-1 {
			lines[i] = line[:idx]
			continue
		}
		lines[i] = line[idx+1:]
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Colour remapping
// ---------------------------------------------------------------------------

// RemapANSI16 rewrites the basic 16-colour SGR codes in s to explicit 24-bit
// values from palette.
//
// Programs emit `\x1b[31m` and leave the choice of red to the terminal. Inside
// the TUI that default is chosen for ordinary terminal text, not for text drawn
// on this application's background, so it is often unreadable. Rewriting the
// code to truecolor keeps the output legible whatever the terminal is configured
// to do.
//
// Extended colours (`38`/`48`/`58` with `;5;n` or `;2;r;g;b`) and non-colour
// attributes (bold, italic, underline) pass through untouched, as do the default
// colour resets 39, 49 and 59.
func RemapANSI16(s string, palette [16]color.Color) string {
	if !strings.ContainsRune(s, 0x1b) {
		return s
	}

	var buf strings.Builder
	buf.Grow(len(s))

	parser := ansi.GetParser()
	defer ansi.PutParser(parser)

	var state byte
	for len(s) > 0 {
		parser.Reset()
		seq, _, n, newState := ansi.DecodeSequence(s, state, parser)

		if ansi.HasCsiPrefix(seq) && parser.Command() == 'm' {
			remapSGR(parser.Params(), palette, &buf)
		} else {
			buf.WriteString(seq)
		}

		s = s[n:]
		state = newState
	}

	return buf.String()
}

// remapSGR writes one SGR sequence back out, replacing 16-colour params with
// their truecolor equivalents from palette.
func remapSGR(params ansi.Params, palette [16]color.Color, buf *strings.Builder) {
	buf.WriteString("\x1b[")

	first := true
	// sep writes the parameter separator, unless this is the first parameter.
	sep := func() {
		if !first {
			buf.WriteByte(';')
		}
		first = false
	}

	for i := 0; i < len(params); i++ {
		p := params[i].Param(0)

		switch {
		// An extended-colour introducer swallows the params after it, so they
		// are copied whole. Reading them as separate attributes would corrupt
		// the sequence.
		case p == 38 || p == 48 || p == 58:
			sep()
			buf.WriteString(strconv.Itoa(p))
			i += copyExtendedColor(params, i, buf)

		case p >= 30 && p <= 37:
			sep()
			writeTruecolor(buf, 38, palette[p-30])
		case p >= 90 && p <= 97:
			sep()
			writeTruecolor(buf, 38, palette[8+p-90])
		case p >= 40 && p <= 47:
			sep()
			writeTruecolor(buf, 48, palette[p-40])
		case p >= 100 && p <= 107:
			sep()
			writeTruecolor(buf, 48, palette[8+p-100])

		default:
			sep()
			buf.WriteString(strconv.Itoa(p))
		}
	}

	buf.WriteByte('m')
}

// copyExtendedColor writes the tail of an extended-colour sequence — the `5;n`
// or `2;r;g;b` that follows an introducer — and returns how many params it
// consumed. The leading separator is written here because the caller has
// already written the introducer.
func copyExtendedColor(params ansi.Params, i int, buf *strings.Builder) int {
	if i+1 >= len(params) {
		return 0
	}
	sub := params[i+1].Param(0)
	buf.WriteByte(';')
	buf.WriteString(strconv.Itoa(sub))

	switch sub {
	case 5: // 256-colour: 38;5;n — one more param.
		if i+2 < len(params) {
			buf.WriteByte(';')
			buf.WriteString(strconv.Itoa(params[i+2].Param(0)))
			return 2
		}
		return 1
	case 2: // truecolour: 38;2;r;g;b — three more params.
		used := 1
		for j := 2; j <= 4 && i+j < len(params); j++ {
			buf.WriteByte(';')
			buf.WriteString(strconv.Itoa(params[i+j].Param(0)))
			used++
		}
		return used
	default:
		return 1
	}
}

// writeTruecolor appends `introducer;2;r;g;b` to buf. A nil colour emits only
// the introducer, which asks the terminal for its own default.
func writeTruecolor(buf *strings.Builder, introducer int, c color.Color) {
	buf.WriteString(strconv.Itoa(introducer))
	if c == nil {
		return
	}
	r, g, b, _ := c.RGBA()
	buf.WriteString(";2;")
	buf.WriteString(strconv.Itoa(int(r >> 8)))
	buf.WriteByte(';')
	buf.WriteString(strconv.Itoa(int(g >> 8)))
	buf.WriteByte(';')
	buf.WriteString(strconv.Itoa(int(b >> 8)))
}

// ---------------------------------------------------------------------------
// Full pipeline
// ---------------------------------------------------------------------------

// CompleteANSIOnly returns s with a trailing, unterminated escape sequence
// removed.
//
// Output arrives in whatever sized pieces the program chose to write, so a
// stream boundary can land in the middle of a colour code. Drawn as-is the
// partial code is printed literally for one frame — a flash of `[38;2;255;0`
// in the middle of the transcript — and then disappears when the rest arrives.
// Dropping the partial tail removes the flash at the cost of showing that one
// colour a frame late, which is invisible.
//
// This matters only while a command is still producing output. A finished
// buffer is complete by definition, so the scan finds a ground state at the end
// and returns s untouched.
func CompleteANSIOnly(s string) string {
	if !strings.ContainsRune(s, 0x1b) {
		return s
	}
	return cutAtGround(s)
}

// cutAtGround walks the escape sequences and returns everything up to the last
// one that ends with the parser back in its ground state.
func cutAtGround(s string) string {
	p := ansi.GetParser()
	defer ansi.PutParser(p)

	var state byte
	rest := s
	cut := 0

	for len(rest) > 0 {
		p.Reset()
		_, _, n, newState := ansi.DecodeSequence(rest, state, p)
		if n == 0 {
			break
		}
		rest = rest[n:]
		state = newState
		if state == parser.GroundState {
			cut = len(s) - len(rest)
		}
	}

	return s[:cut]
}

// CompleteANSIFrom returns s with a leading fragment of an escape sequence
// removed.
//
// It is the counterpart of CompleteANSIOnly for the other end of a window. A
// block that shows only the tail of a very long line cuts at an arbitrary byte,
// and that byte can be the middle of a colour code; the half before the cut
// would otherwise be printed literally.
//
// A fragment is recognised by its shape, because the caller cannot say whether
// the window starts mid-sequence: an optional introducer, then parameter bytes,
// then a final byte. Text that does not have that shape is returned unchanged,
// so ordinary content is never eaten. A string that begins with ESC is left
// alone; that is a sequence that starts cleanly, and CompleteANSIOnly is the
// function that judges it.
func CompleteANSIFrom(s string) string {
	if s == "" || s[0] == 0x1b {
		return s
	}

	i := 0
	if isSequenceIntroducer(s[0]) {
		i++
	}
	for i < len(s) && isSequenceParam(s[i]) {
		i++
	}
	// A single leading character is ordinary text. A fragment always carries at
	// least the introducer or one parameter before its final byte.
	if i > 0 && i < len(s) && isSequenceFinal(s[i]) {
		return s[i+1:]
	}
	return s
}

// isSequenceIntroducer reports whether b opens an escape sequence body.
func isSequenceIntroducer(b byte) bool {
	switch b {
	case '[', ']', 'P', '^', '_', '(', ')':
		return true
	}
	return false
}

// isSequenceParam reports whether b is one of the parameter bytes 0x30-0x3F.
func isSequenceParam(b byte) bool { return b >= 0x30 && b <= 0x3f }

// isSequenceFinal reports whether b is one of the final bytes 0x40-0x7E, which
// is what terminates a CSI, OSC or other escape sequence.
func isSequenceFinal(b byte) bool { return b >= 0x40 && b <= 0x7e }

// NormalizeOutput prepares raw shell output for drawing in the scrollback. It is
// the whole pipeline in one call: StripCursorControl, then RemapANSI16, then a
// downsample to whatever colour depth the terminal can actually show.
//
// The order matters. Cursor control has to go first, because a removed
// `\x1b[K` can unbalance the style state that the remap then rewrites. The
// downsample has to go last, so it sees the final truecolor values.
//
// Calling this once per frame on a growing buffer is affordable: the two passes
// are a single sequential scan each, and the shell block only ever renders its
// visible window rather than the whole transcript.
func NormalizeOutput(s string) string {
	if s == "" {
		return s
	}
	s = CompleteANSIOnly(s)
	s = StripCursorControl(s)
	s = RemapANSI16(s, ANSIPalette(GetTheme()))

	// Below truecolor the 24-bit codes the remap just wrote are not all
	// displayable. colorprofile.Writer picks the nearest colour the terminal
	// has, and drops colour entirely when the terminal has none.
	if p := terminalColorProfile(); p < colorprofile.TrueColor {
		var b strings.Builder
		w := &colorprofile.Writer{Forward: &b, Profile: p}
		if _, err := w.WriteString(s); err == nil {
			return b.String()
		}
	}
	return s
}

// StripForContext returns s with every escape sequence removed. This is the
// form that goes to the language model: the model reads text, and escape
// sequences are noise it can only misread.
func StripForContext(s string) string {
	return ansi.Strip(s)
}

// ---------------------------------------------------------------------------
// Cleanup helpers
// ---------------------------------------------------------------------------

// TrimTrailingResets removes trailing whitespace and the bare reset sequences
// that programs append after their last line of output.
//
// The reset sits between the content and the newline, so trimming only "\n"
// misses it: `\x1b[0m\n` is what `task` and several other tools emit, and the
// bytes in between are exactly what the loop below walks back over.
func TrimTrailingResets(s string) string {
	for {
		trimmed := strings.TrimRight(s, " \t\r\n")
		trimmed = strings.TrimSuffix(trimmed, "\x1b[0m")
		if trimmed == s {
			return s
		}
		s = trimmed
	}
}

// FirstLines returns the first count lines of s, or all of s when it has fewer.
func FirstLines(s string, count int) string {
	if count <= 0 {
		return ""
	}
	end := 0
	for i := range count {
		next := strings.IndexByte(s[end:], '\n')
		if next < 0 {
			return s
		}
		end += next
		if i == count-1 {
			return s[:end]
		}
		end++
	}
	return s
}

// LastLines returns the last count lines of s, or all of s when it has fewer.
func LastLines(s string, count int) string {
	if count <= 0 {
		return ""
	}
	start := len(s)
	for range count {
		previous := strings.LastIndexByte(s[:start], '\n')
		if previous < 0 {
			return s
		}
		start = previous
	}
	return s[start+1:]
}
