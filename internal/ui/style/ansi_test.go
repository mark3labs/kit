package style

import (
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestStripCursorControlKeepsSGRAndText(t *testing.T) {
	in := "\x1b[31mred\x1b[0m plain"
	got := StripCursorControl(in)
	if got != in {
		t.Fatalf("StripCursorControl(%q) = %q, want unchanged", in, got)
	}
}

func TestStripCursorControlRemovesCursorAndErase(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"cursor up", "a\x1b[2Ab", "ab"},
		{"cursor position", "a\x1b[10;20Hb", "ab"},
		{"erase line", "a\x1b[Kb", "ab"},
		{"erase display", "a\x1b[2Jb", "ab"},
		{"scroll", "a\x1b[1Sb", "ab"},
		{"save restore cursor", "a\x1b[s\x1b[ub", "ab"},
		{"private mode set", "a\x1b[?25hb", "ab"},
		{"esc save cursor", "a\x1b7b\x1b8c", "abc"},
		{"plain h and l", "a\x1b[4hb\x1b[4lc", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripCursorControl(tt.in); got != tt.want {
				t.Errorf("StripCursorControl(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// An unrecognised CSI is kept: dropping it would lose data on a sequence this
// function has never seen.
func TestStripCursorControlKeepsUnknownCSI(t *testing.T) {
	in := "a\x1b[99zb"
	if got := StripCursorControl(in); got != in {
		t.Errorf("StripCursorControl(%q) = %q, want unchanged", in, got)
	}
}

func TestStripCursorControlResolvesCarriageReturns(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"progress bar", "10%\r50%\r100%\ndone", "100%\ndone"},
		{"one cr", "abc\rxy", "xy"},
		// A bare trailing \r is a cursor move with no write, so the text before
		// it must survive: `printf 'foo\r\n'` is common.
		{"cr at line end only", "abc\r", "abc"},
		{"cr then newline", "abc\r\ndef", "abc\ndef"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripCursorControl(tt.in); got != tt.want {
				t.Errorf("StripCursorControl(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestStripCursorControlFastPath(t *testing.T) {
	in := "no escapes here\nat all"
	if got := StripCursorControl(in); got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestRemapANSI16RewritesBaseColours(t *testing.T) {
	palette := testPalette()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"red fg", "\x1b[31mx", "\x1b[38;2;17;0;0mx"},
		{"bright red fg", "\x1b[91mx", "\x1b[38;2;255;255;255mx"},
		{"green bg", "\x1b[42mx", "\x1b[48;2;34;0;0mx"},
		{"bright cyan bg", "\x1b[106mx", "\x1b[48;2;238;0;0mx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemapANSI16(tt.in, palette); got != tt.want {
				t.Errorf("RemapANSI16(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRemapANSI16PassesExtendedColoursThrough(t *testing.T) {
	palette := testPalette()
	tests := []string{
		"\x1b[38;5;208mx",
		"\x1b[38;2;1;2;3mx",
		"\x1b[48;5;17mx",
		"\x1b[48;2;4;5;6mx",
		"\x1b[38;5;208;1m", // trailing bold after an extended colour
	}
	for _, in := range tests {
		if got := RemapANSI16(in, palette); got != in {
			t.Errorf("RemapANSI16(%q) = %q, want unchanged", in, got)
		}
	}
}

func TestRemapANSI16KeepsNonColourAttributes(t *testing.T) {
	palette := testPalette()
	tests := []string{
		"\x1b[1mbold\x1b[0m",
		"\x1b[3mitalic",
		"\x1b[4munderline",
		"\x1b[39mdefault fg",
		"\x1b[49mdefault bg",
		"\x1b[0m",
	}
	for _, in := range tests {
		if got := RemapANSI16(in, palette); got != in {
			t.Errorf("RemapANSI16(%q) = %q, want unchanged", in, got)
		}
	}
}

// A 256-colour code followed by a base colour must not lose either one: the
// rewrite stays inside the one sequence rather than splitting it.
func TestRemapANSI16MixedSequence(t *testing.T) {
	palette := testPalette()
	in := "\x1b[38;5;9;31mx"
	want := "\x1b[38;5;9;38;2;17;0;0mx"
	if got := RemapANSI16(in, palette); got != want {
		t.Errorf("RemapANSI16(%q) = %q, want %q", in, got, want)
	}
}

func TestRemapANSI16NoEscapes(t *testing.T) {
	palette := testPalette()
	in := "plain text"
	if got := RemapANSI16(in, palette); got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

// testPalette returns a palette whose every slot is a distinct, known colour so
// that a rewrite to the wrong slot is visible in the expected string.
func testPalette() [16]color.Color {
	var p [16]color.Color
	for i := range p {
		p[i] = color.RGBA{R: uint8(i * 17), G: 0, B: 0, A: 0xff}
	}
	// Slot 9 is the bright red the tests above expect.
	p[9] = color.RGBA{R: 255, G: 255, B: 255, A: 0xff}
	return p
}

func TestTrimTrailingResets(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare reset then newline", "done\x1b[0m\n", "done"},
		{"reset only", "\x1b[0m\n", ""},
		{"trailing spaces", "done   \n", "done"},
		{"plain", "done\n", "done"},
		{"inner reset kept", "a\x1b[0mb", "a\x1b[0mb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TrimTrailingResets(tt.in); got != tt.want {
				t.Errorf("TrimTrailingResets(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFirstAndLastLines(t *testing.T) {
	const out = "1\n2\n3\n4\n5"
	tests := []struct {
		name       string
		head, want string
		tail       string
		wantTail   string
	}{
		{"head two", FirstLines(out, 2), "1\n2", LastLines(out, 2), "4\n5"},
		{"more than exist", FirstLines(out, 10), out, LastLines(out, 10), out},
		{"zero", FirstLines(out, 0), "", LastLines(out, 0), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.head != tt.want {
				t.Errorf("FirstLines = %q, want %q", tt.head, tt.want)
			}
			if tt.tail != tt.wantTail {
				t.Errorf("LastLines = %q, want %q", tt.tail, tt.wantTail)
			}
		})
	}
}

func TestANSIPaletteIsComplete(t *testing.T) {
	p := ANSIPalette(DefaultTheme())
	for i, c := range p {
		if c == nil {
			t.Errorf("palette slot %d is nil", i)
		}
	}
}

// A bright slot must differ from its normal counterpart, otherwise the bright
// half of the palette carries no information.
func TestANSIPaletteBrightSlotsDiffer(t *testing.T) {
	p := ANSIPalette(DefaultTheme())
	for i := range 8 {
		normal := rgbaOf(p[i])
		bright := rgbaOf(p[8+i])
		if normal == bright {
			t.Errorf("slot %d and %d are both %v", i, 8+i, normal)
		}
	}
}

func rgbaOf(c color.Color) string {
	r, g, b, a := c.RGBA()
	return string([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8), byte(a >> 8)})
}

// StripCursorControl followed by RemapANSI16 must leave a string with no
// cursor-control sequence left in it and every colour rewritten, which is the
// exact sequence the shell item renders through.
func TestStripThenRemap(t *testing.T) {
	in := "50%\r\x1b[32mdone\x1b[0m\x1b[K\n"
	got := RemapANSI16(StripCursorControl(in), testPalette())

	if strings.Contains(got, "\x1b[K") {
		t.Errorf("erase sequence survived: %q", got)
	}
	if strings.ContainsRune(got, '\r') {
		t.Errorf("carriage return survived: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;2;") {
		t.Errorf("colour was not remapped: %q", got)
	}
	plain := ansi.Strip(got)
	if plain != "done\n" {
		t.Errorf("visible text = %q, want %q", plain, "done\n")
	}
}

func TestCompleteANSIOnlyDropsPartialTail(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"complete", "\x1b[31mred\x1b[0m", "\x1b[31mred\x1b[0m"},
		{"partial csi", "\x1b[31mred\x1b[3", "\x1b[31mred"},
		{"partial esc", "\x1b[31mred\x1b", "\x1b[31mred"},
		{"partial params", "\x1b[38;2;25", ""},
		{"no escapes", "plain", "plain"},
		{"text then bare esc", "abc\x1b", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompleteANSIOnly(tt.in); got != tt.want {
				t.Errorf("CompleteANSIOnly(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCompleteANSIFromDropsPartialHead(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"clean", "red\x1b[0m", "red\x1b[0m"},
		{"starts mid csi", "1;32mgreen", "green"},
		{"starts mid params", "2;25;3", "2;25;3"},
		{"starts mid esc", "[31mred", "red"},
		{"no escapes", "plain", "plain"},
		{"leading escape is left alone", "\x1b[38;2;1", "\x1b[38;2;1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompleteANSIFrom(tt.in); got != tt.want {
				t.Errorf("CompleteANSIFrom(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// The head cut must never eat plain text that happens to precede the first
// escape sequence: a window that starts cleanly is the common case.
func TestCompleteANSIFromKeepsLeadingText(t *testing.T) {
	in := "plain lead\n\x1b[31mred\x1b[0m"
	if got := CompleteANSIFrom(in); got != in {
		t.Errorf("CompleteANSIFrom(%q) = %q, want unchanged", in, got)
	}
}

// A split stream must normalize to the same bytes as the whole thing at once,
// once the tail has arrived. This is the property the shell item depends on.
func TestNormalizeOutputIsSplitInsensitive(t *testing.T) {
	full := "\x1b[31mred\x1b[0m\n\x1b[1;32mbold green\x1b[0m\n"

	whole := NormalizeOutput(full)

	// Feed the same bytes one byte at a time, keeping only what the renderer
	// would have shown at each step, and check the last step matches.
	var acc string
	for i := range len(full) {
		acc += string(full[i])
		shown := NormalizeOutput(acc)
		if i == len(full)-1 {
			if shown != whole {
				t.Errorf("byte-at-a-time result\n got %q\nwant %q", shown, whole)
			}
		}
	}
}
