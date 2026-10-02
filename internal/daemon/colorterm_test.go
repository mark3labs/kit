package daemon

import (
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/mark3labs/kit/internal/ui/termgfx"
)

func TestColorTermFor(t *testing.T) {
	cases := []struct {
		name     string
		current  string
		detected colorprofile.Profile
		mux      string
		want     string
	}{
		{"existing value is kept", "24bit", colorprofile.ANSI256, "", "24bit"},
		{"existing value is not upgraded", "yes", colorprofile.ANSI, termgfx.MultiplexerZellij, "yes"},
		{"detected truecolor is reported", "", colorprofile.TrueColor, "", "truecolor"},
		{"zellij with 256 colours", "", colorprofile.ANSI256, termgfx.MultiplexerZellij, "truecolor"},
		{"zellij with only 16 colours", "", colorprofile.ANSI, termgfx.MultiplexerZellij, ""},
		{"zellij under NO_COLOR", "", colorprofile.ASCII, termgfx.MultiplexerZellij, ""},
		{"plain 256-colour terminal", "", colorprofile.ANSI256, "", ""},
		{"tmux without RGB", "", colorprofile.ANSI256, termgfx.MultiplexerTmux, ""},
		{"no tty", "", colorprofile.NoTTY, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := colorTermFor(tc.current, tc.detected, tc.mux); got != tc.want {
				t.Errorf("colorTermFor(%q, %v, %q) = %q, want %q",
					tc.current, tc.detected, tc.mux, got, tc.want)
			}
		})
	}
}

// TestZellijClientGivesTheChildTrueColor is the full chain for the session
// that rendered catppuccin surfaces as pink: a client in zellij with
// TERM=xterm-256color and no COLORTERM. The child must resolve TrueColor,
// not ANSI256.
func TestZellijClientGivesTheChildTrueColor(t *testing.T) {
	info := TerminalInfo{
		Term:        "xterm-256color",
		ColorTerm:   colorTermFor("", colorprofile.ANSI256, termgfx.MultiplexerZellij),
		Multiplexer: termgfx.MultiplexerZellij,
	}
	env := childEnv([]string{"PATH=/usr/bin"}, info, clip("/tmp/clip-ct"))

	if got, _ := envValue(t, env, "COLORTERM"); got != "truecolor" {
		t.Fatalf("COLORTERM = %q, want truecolor", got)
	}
	if p := colorprofile.Env(env); p != colorprofile.TrueColor {
		t.Fatalf("child colour profile = %v, want TrueColor", p)
	}
}
