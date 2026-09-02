package daemon

import (
	"fmt"
	"image/color"
	"maps"
	"strconv"
	"strings"
)

// The environment a session's child renders against.
//
// A session child talks to a PTY, and a PTY has no capabilities of its
// own: it reports no colour depth and answers no background-colour query.
// Left alone the child therefore describes the DAEMON's environment —
// under a service manager, no terminal at all — and a 24-bit theme is
// rendered through whatever fallback that produces. Quantising to 256
// colours does not dim a palette evenly: every colour moves to the nearest
// cell of a coarse cube, so the near-black surfaces a dark theme is built
// from land on saturated cube corners and the session appears in colours
// no theme defines.
//
// The client knows all of this about its own terminal, so it sends it
// (FrameTerminal) and the daemon plants it in the child's environment.
// This applies to local sessions as much as remote ones: a daemon started
// from one terminal, or by systemd, outlives the terminal that started it.

const (
	// RemoteSessionEnv marks a child as running inside a daemon session.
	RemoteSessionEnv = "KIT_REMOTE_SESSION"

	// RemoteBackgroundEnv holds the CLIENT terminal's background colour as
	// "#rrggbb", or BackgroundUnknown when that terminal was asked and did
	// not answer. The child prefers it over its own OSC query, which would
	// otherwise have to cross the PTY — and, for a remote session, the
	// network — before the first frame is drawn, and which the client,
	// being the side that owns the terminal, can answer without any of it.
	RemoteBackgroundEnv = "KIT_REMOTE_BACKGROUND"

	// fallbackTerm is the TERM a child gets when neither the client nor
	// the daemon names one. It claims no more than every terminal
	// emulator in practical use provides.
	fallbackTerm = "xterm-256color"
)

// childEnv builds the environment for a session's child: the daemon's own
// environment, with the daemon's per-session variables and the client's
// terminal description layered on top.
//
// own carries the variables the daemon controls (clipboard file, cwd file,
// owner marker). They always win over an inherited value of the same name,
// so a stale one from the daemon's own environment cannot leak into a
// session.
//
// Anything the client did not report is left as the daemon has it, so a
// client too old to send a TerminalInfo keeps the previous behaviour
// instead of losing a working TERM.
func childEnv(base []string, info TerminalInfo, own map[string]string) []string {
	overrides := make(map[string]string, len(own)+4)
	maps.Copy(overrides, own)
	overrides[RemoteSessionEnv] = "1"
	if info.Term != "" {
		overrides["TERM"] = info.Term
	}
	if info.ColorTerm != "" {
		overrides["COLORTERM"] = info.ColorTerm
	}
	if bg := backgroundEnvValue(info.Background); bg != "" {
		overrides[RemoteBackgroundEnv] = bg
	}

	// A client that describes its terminal but reports no COLORTERM has
	// none. Keeping the daemon's would claim a colour depth the user's
	// terminal never promised — the same misreport in the other
	// direction — so it is dropped along with the TERM it belonged to.
	dropColorTerm := info.Term != "" && info.ColorTerm == ""

	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			out = append(out, kv)
			continue
		}
		if _, replaced := overrides[k]; replaced {
			continue // the override wins; appended below
		}
		if k == "COLORTERM" && dropColorTerm {
			continue
		}
		// A background left over from the daemon's own environment
		// describes a terminal that is not this client's.
		if k == RemoteBackgroundEnv {
			continue
		}
		out = append(out, kv)
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	if !hasEnv(out, "TERM") {
		out = append(out, "TERM="+fallbackTerm)
	}
	return out
}

// hasEnv reports whether key is present in an environment slice.
func hasEnv(env []string, key string) bool {
	for _, kv := range env {
		if k, _, ok := strings.Cut(kv, "="); ok && k == key {
			return true
		}
	}
	return false
}

// backgroundEnvValue turns a reported background into the value the child
// reads: a normalized hex colour, or BackgroundUnknown when the client
// asked its terminal and got nothing back. Anything else yields "", which
// leaves the variable unset and the child free to ask for itself.
func backgroundEnvValue(reported string) string {
	if strings.EqualFold(strings.TrimSpace(reported), BackgroundUnknown) {
		return BackgroundUnknown
	}
	return normalizeHexColor(reported)
}

// normalizeHexColor validates a "#rrggbb" string and returns it lowercased,
// or "" when it is not one. The value is planted in a child's environment,
// so it is never passed through unchecked.
func normalizeHexColor(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) != 7 || s[0] != '#' {
		return ""
	}
	if _, err := strconv.ParseUint(s[1:], 16, 32); err != nil {
		return ""
	}
	return s
}

// HexColor renders a colour as "#rrggbb". color.Color reports 16-bit
// premultiplied channels, so each is scaled back to 8 bits. A nil colour
// yields "", which callers read as "the terminal did not say".
func HexColor(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// BackgroundIsDark reports whether a reported background reads as dark.
//
// The test is HSL lightness below one half, which is the test lipgloss
// applies to a background it queried itself. Sharing the rule means a
// session started from this environment variable and one started from a
// live query never disagree about which half of a theme to render. A value
// that names no colour — BackgroundUnknown among them — reports dark,
// matching lipgloss's own answer for a terminal that will not say.
func BackgroundIsDark(hex string) bool {
	hex = normalizeHexColor(hex)
	if hex == "" {
		return true
	}
	v, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return true
	}
	r := float64((v>>16)&0xff) / 255
	g := float64((v>>8)&0xff) / 255
	b := float64(v&0xff) / 255
	lightness := (max(r, g, b) + min(r, g, b)) / 2
	return lightness < 0.5
}
