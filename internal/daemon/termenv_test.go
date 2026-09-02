package daemon

import (
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/clipboard"
)

// envValue returns the value of key in an environment slice, and whether it
// is present at all. A key must appear at most once: a duplicate makes the
// child's view of its own environment depend on which lookup it uses.
// clip is the daemon-controlled variable set for every session child, as
// spawnPickDir supplies it.
func clip(path string) map[string]string {
	return map[string]string{clipboard.RemoteClipboardEnv: path}
}

func envValue(t *testing.T, env []string, key string) (string, bool) {
	t.Helper()
	var value string
	found := 0
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k == key {
			value = v
			found++
		}
	}
	if found > 1 {
		t.Fatalf("%s appears %d times in the child environment", key, found)
	}
	return value, found == 1
}

func TestChildEnvDescribesTheClientTerminal(t *testing.T) {
	// The daemon's own terminal is the wrong one to describe: it is not
	// where the session is seen.
	base := []string{"TERM=dumb", "HOME=/home/kit", "PATH=/usr/bin"}
	info := TerminalInfo{Term: "xterm-kitty", ColorTerm: "truecolor", Background: "#1E1E2E"}

	env := childEnv(base, info, clip("/tmp/clip-1"))

	for key, want := range map[string]string{
		"TERM":                       "xterm-kitty",
		"COLORTERM":                  "truecolor",
		RemoteBackgroundEnv:          "#1e1e2e",
		RemoteSessionEnv:             "1",
		clipboard.RemoteClipboardEnv: "/tmp/clip-1",
	} {
		got, ok := envValue(t, env, key)
		if !ok {
			t.Errorf("%s missing from the child environment", key)
			continue
		}
		if got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	// Everything the client said nothing about is left alone.
	if got, _ := envValue(t, env, "HOME"); got != "/home/kit" {
		t.Errorf("HOME = %q, want the daemon's own value", got)
	}
}

func TestChildEnvKeepsDaemonTerminalWhenClientReportsNone(t *testing.T) {
	// A client too old to describe its terminal must not cost the child a
	// working TERM.
	base := []string{"TERM=screen-256color", "COLORTERM=truecolor"}

	env := childEnv(base, TerminalInfo{}, clip("/tmp/clip-2"))

	if got, _ := envValue(t, env, "TERM"); got != "screen-256color" {
		t.Errorf("TERM = %q, want the daemon's own value", got)
	}
	if got, _ := envValue(t, env, "COLORTERM"); got != "truecolor" {
		t.Errorf("COLORTERM = %q, want the daemon's own value", got)
	}
	if _, ok := envValue(t, env, RemoteBackgroundEnv); ok {
		t.Error("a client that reported no background must leave the child free to ask")
	}
}

func TestChildEnvDropsTheDaemonColorTermWithTheDaemonTerm(t *testing.T) {
	// The daemon runs under a truecolor terminal; the client does not.
	// Keeping COLORTERM would promise the client's terminal a colour depth
	// it never claimed — the same misreport this whole path exists to fix,
	// pointed the other way.
	base := []string{"TERM=xterm-ghostty", "COLORTERM=truecolor"}
	info := TerminalInfo{Term: "xterm-256color"}

	env := childEnv(base, info, clip("/tmp/clip-3"))

	if got, _ := envValue(t, env, "TERM"); got != "xterm-256color" {
		t.Errorf("TERM = %q, want the client's", got)
	}
	if got, ok := envValue(t, env, "COLORTERM"); ok {
		t.Errorf("COLORTERM = %q, want it dropped with the daemon's TERM", got)
	}
}

func TestChildEnvFallsBackWhenNobodyNamesATerm(t *testing.T) {
	// A daemon started by systemd has no TERM at all.
	env := childEnv([]string{"HOME=/home/kit"}, TerminalInfo{}, clip("/tmp/clip-4"))

	if got, _ := envValue(t, env, "TERM"); got != fallbackTerm {
		t.Errorf("TERM = %q, want %q", got, fallbackTerm)
	}
}

func TestChildEnvRejectsAnUnparseableBackground(t *testing.T) {
	// The value is planted in a child's environment, so it is never passed
	// through unchecked.
	for _, bad := range []string{"red", "#12345", "#gggggg", "#1e1e2e; rm -rf /", ""} {
		env := childEnv(nil, TerminalInfo{Background: bad}, clip("/tmp/clip-5"))
		if got, ok := envValue(t, env, RemoteBackgroundEnv); ok {
			t.Errorf("background %q reached the child as %q", bad, got)
		}
	}
}

func TestChildEnvPassesAnUnansweredQueryThrough(t *testing.T) {
	// "Asked and got nothing" is worth forwarding: it stops the child
	// spending its own startup on a question already known to go
	// unanswered.
	env := childEnv(nil, TerminalInfo{Background: BackgroundUnknown}, clip("/tmp/clip-6"))

	got, ok := envValue(t, env, RemoteBackgroundEnv)
	if !ok || got != BackgroundUnknown {
		t.Errorf("%s = %q (present %v), want %q", RemoteBackgroundEnv, got, ok, BackgroundUnknown)
	}
	if !BackgroundIsDark(got) {
		t.Error("an unanswered query must read as dark, matching lipgloss")
	}
}

func TestChildEnvHasNoDuplicateKeys(t *testing.T) {
	base := []string{"TERM=dumb", "COLORTERM=truecolor", RemoteSessionEnv + "=0",
		clipboard.RemoteClipboardEnv + "=/stale", RemoteBackgroundEnv + "=#ffffff"}
	info := TerminalInfo{Term: "xterm-kitty", ColorTerm: "truecolor", Background: "#1e1e2e"}

	env := childEnv(base, info, clip("/tmp/clip-7"))

	seen := map[string]bool{}
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if seen[k] {
			t.Errorf("%s appears more than once", k)
		}
		seen[k] = true
	}
	// The stale values are the ones that lost.
	if got, _ := envValue(t, env, RemoteBackgroundEnv); got != "#1e1e2e" {
		t.Errorf("%s = %q, want the client's value", RemoteBackgroundEnv, got)
	}
	if got, _ := envValue(t, env, clipboard.RemoteClipboardEnv); got != "/tmp/clip-7" {
		t.Errorf("%s = %q, want this session's file", clipboard.RemoteClipboardEnv, got)
	}
}

func TestChildEnvKeepsMalformedEntries(t *testing.T) {
	// An environment entry without '=' is not ours to interpret.
	env := childEnv([]string{"NOTANASSIGNMENT"}, TerminalInfo{}, clip("/tmp/clip-8"))
	if !slices.Contains(env, "NOTANASSIGNMENT") {
		t.Error("a malformed environment entry was dropped")
	}
}

func TestBackgroundIsDark(t *testing.T) {
	cases := map[string]bool{
		"#1e1e2e":         true,  // catppuccin mocha
		"#000000":         true,  // black
		"#eff1f5":         false, // catppuccin latte
		"#ffffff":         false, // white
		"#808080":         false, // exactly at the boundary: lightness 0.502
		"":                true,  // nothing said: lipgloss's own default
		"nonsense":        true,
		BackgroundUnknown: true,
	}
	for input, want := range cases {
		if got := BackgroundIsDark(input); got != want {
			t.Errorf("BackgroundIsDark(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestTerminalInfoRoundTrip(t *testing.T) {
	info := TerminalInfo{Term: "xterm-kitty", ColorTerm: "truecolor", Background: "#1e1e2e"}
	payload, err := EncodeTerminalInfo(info)
	if err != nil {
		t.Fatalf("EncodeTerminalInfo: %v", err)
	}
	got, err := DecodeTerminalInfo(payload)
	if err != nil {
		t.Fatalf("DecodeTerminalInfo: %v", err)
	}
	if got != info {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, info)
	}
}

func TestDecodeTerminalInfoRejectsGarbage(t *testing.T) {
	if _, err := DecodeTerminalInfo([]byte("not json")); err == nil {
		t.Fatal("DecodeTerminalInfo accepted a non-JSON payload")
	}
}

// TestTerminalIsForgottenWithTheConnection pins the lifetime: wire ids are
// per connection and get reused, so a departed client's terminal must not
// describe the next client that lands on the same id.
func TestTerminalIsForgottenWithTheConnection(t *testing.T) {
	conns := newConnSet()
	conn := conns.addLocal(nil)

	conns.setTerminal(conn.id, TerminalInfo{Term: "xterm-kitty"})
	if got := conns.terminalFor(conn.id); got.Term != "xterm-kitty" {
		t.Fatalf("terminalFor = %+v, want the client's terminal", got)
	}

	conns.remove(conn.id)
	if got := conns.terminalFor(conn.id); got != (TerminalInfo{}) {
		t.Fatalf("terminalFor after the client left = %+v, want the zero value", got)
	}
}

// TestSetTerminalForAMissingConnection covers a TERMINAL frame that races
// the client's disconnect: there is nothing to record it against, and
// inventing an entry would leave the set holding a connection that is gone.
func TestSetTerminalForAMissingConnection(t *testing.T) {
	conns := newConnSet()
	conns.setTerminal(999, TerminalInfo{Term: "xterm-kitty"})
	if got := conns.terminalFor(999); got != (TerminalInfo{}) {
		t.Fatalf("terminalFor = %+v, want the zero value", got)
	}
}
