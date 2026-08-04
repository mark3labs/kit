package extensions

import "testing"

func TestParseThemeHex(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		r, g, b int
		ok      bool
	}{
		{"six digit", "#89b4fa", 0x89, 0xb4, 0xfa, true},
		{"no hash", "89b4fa", 0x89, 0xb4, 0xfa, true},
		{"three digit", "#abc", 0xaa, 0xbb, 0xcc, true},
		{"uppercase", "#FF0055", 0xff, 0x00, 0x55, true},
		{"whitespace", "  #000000  ", 0, 0, 0, true},
		{"white", "#ffffff", 255, 255, 255, true},
		{"empty", "", 0, 0, 0, false},
		{"bare hash", "#", 0, 0, 0, false},
		{"too short", "#ab", 0, 0, 0, false},
		{"too long", "#aabbccdd", 0, 0, 0, false},
		{"non hex", "#gghhii", 0, 0, 0, false},
		{"named color", "red", 0, 0, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, g, b, ok := parseThemeHex(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if r != tc.r || g != tc.g || b != tc.b {
				t.Fatalf("got (%d,%d,%d), want (%d,%d,%d)", r, g, b, tc.r, tc.g, tc.b)
			}
		})
	}
}

func TestThemeColorsANSI(t *testing.T) {
	th := ThemeColors{Accent: "#89b4fa"}

	got := th.ANSI(th.Accent, "hi")
	want := "\033[38;2;137;180;250mhi\033[0m"
	if got != want {
		t.Fatalf("ANSI = %q, want %q", got, want)
	}

	gotBold := th.ANSIBold(th.Accent, "hi")
	wantBold := "\033[1;38;2;137;180;250mhi\033[0m"
	if gotBold != wantBold {
		t.Fatalf("ANSIBold = %q, want %q", gotBold, wantBold)
	}
}

// A zero-valued ThemeColors is what a non-TUI host yields. Painting with it
// must produce clean text, not escape-code noise in a log or a pipe.
func TestThemeColorsANSIUnsetDegradesToPlain(t *testing.T) {
	var th ThemeColors
	if got := th.ANSI(th.Accent, "hi"); got != "hi" {
		t.Fatalf("ANSI with unset color = %q, want %q", got, "hi")
	}
	if got := th.ANSI("not-a-color", "hi"); got != "hi" {
		t.Fatalf("ANSI with junk color = %q, want %q", got, "hi")
	}
	// Bold carries no color information, so it survives an unset color.
	if got := th.ANSIBold(th.Accent, "hi"); got != "\033[1mhi\033[0m" {
		t.Fatalf("ANSIBold with unset color = %q", got)
	}
}

// GetTheme must be safe to call on a Context that was never wired by a host,
// which is the shape headless and test callers see.
func TestGetThemeDefaultIsNonNil(t *testing.T) {
	ctx := normalizeContext(Context{})
	if ctx.GetTheme == nil {
		t.Fatal("GetTheme was not defaulted")
	}
	th := ctx.GetTheme()
	if th.Accent != "" {
		t.Fatalf("default theme should be zero-valued, got Accent=%q", th.Accent)
	}
	if got := th.ANSI(th.Accent, "x"); got != "x" {
		t.Fatalf("painting with default theme = %q, want %q", got, "x")
	}
}
