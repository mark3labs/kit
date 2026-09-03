package main

import (
	"maps"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/pkg/extensions/test"
)

const popupTermFile = "popup-terminal.go"

// loadPopupTerm loads the extension and returns a harness whose context is
// configured for a deterministic, non-executing run.
func loadPopupTerm(t *testing.T, opts map[string]string) *test.Harness {
	t.Helper()
	h := test.New(t)
	h.LoadFile(popupTermFile)
	mc := h.Context()
	mc.SessionID = "test-session"
	mc.CWD = "/tmp/kit-term-test"
	mc.Interactive = true
	maps.Copy(mc.Options, opts)
	return h
}

// runTerm invokes the /term command with the given arguments.
func runTerm(t *testing.T, h *test.Harness, args string) (string, error) {
	t.Helper()
	for _, cmd := range h.RegisteredCommands() {
		if cmd.Name == "term" {
			return cmd.Execute(args, h.Context().ToContext())
		}
	}
	t.Fatalf("command /term is not registered")
	return "", nil
}

// allPrints joins every output channel so assertions do not have to guess
// which one a message went to.
func allPrints(h *test.Harness) string {
	mc := h.Context()
	var b strings.Builder
	for _, group := range [][]string{mc.Prints, mc.PrintInfos, mc.PrintErrors} {
		for _, s := range group {
			b.WriteString(s)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestPopupTerminal_RegistersCommand(t *testing.T) {
	h := loadPopupTerm(t, nil)
	test.AssertCommandRegistered(t, h, "term")

	for _, cmd := range h.RegisteredCommands() {
		if cmd.Name != "term" {
			continue
		}
		if cmd.Description == "" {
			t.Error("command /term has an empty description")
		}
		if cmd.Execute == nil {
			t.Error("command /term has a nil Execute")
		}
		if cmd.Complete == nil {
			t.Error("command /term has no completion function")
		}
	}
}

func TestPopupTerminal_RegistersShortcut(t *testing.T) {
	h := loadPopupTerm(t, nil)
	found := false
	for _, sc := range h.Runner().RegisteredShortcuts() {
		if sc.Def.Key == "ctrl+alt+t" {
			found = true
			if sc.Def.Description == "" {
				t.Error("shortcut ctrl+alt+t has an empty description")
			}
			if sc.Handler == nil {
				t.Error("shortcut ctrl+alt+t has a nil handler")
			}
		}
	}
	if !found {
		t.Errorf("shortcut ctrl+alt+t is not registered; got %+v", h.Runner().RegisteredShortcuts())
	}
}

func TestPopupTerminal_RegistersOptions(t *testing.T) {
	h := loadPopupTerm(t, nil)
	want := map[string]string{
		"terminal-multiplexer":  "auto",
		"terminal-shell":        "",
		"terminal-kill-on-exit": "true",
		"terminal-dry-run":      "false",
	}
	got := make(map[string]string)
	for _, o := range h.Runner().RegisteredOptions() {
		got[o.Name] = o.Default
		if o.Description == "" {
			t.Errorf("option %q has an empty description", o.Name)
		}
	}
	for name, def := range want {
		v, ok := got[name]
		if !ok {
			t.Errorf("option %q is not registered", name)
			continue
		}
		if v != def {
			t.Errorf("option %q default = %q, want %q", name, v, def)
		}
	}
}

func TestPopupTerminal_CompletionSuggestsSubcommands(t *testing.T) {
	h := loadPopupTerm(t, nil)
	var complete func(string, extensions.Context) []string
	for _, cmd := range h.RegisteredCommands() {
		if cmd.Name == "term" {
			complete = cmd.Complete
		}
	}
	if complete == nil {
		t.Fatal("no completion function")
	}
	ctx := h.Context().ToContext()

	all := complete("", ctx)
	if len(all) != 2 {
		t.Errorf("empty prefix returned %v, want 2 suggestions", all)
	}

	k := complete("k", ctx)
	if len(k) != 1 || k[0] != "kill" {
		t.Errorf("prefix \"k\" returned %v, want [kill]", k)
	}

	none := complete("zzz", ctx)
	if len(none) != 0 {
		t.Errorf("prefix \"zzz\" returned %v, want no suggestions", none)
	}
}

// ---------------------------------------------------------------------------
// Command construction (dry run — nothing is executed)
// ---------------------------------------------------------------------------

func TestPopupTerminal_DryRunTmuxCommand(t *testing.T) {
	h := loadPopupTerm(t, map[string]string{
		"terminal-multiplexer": "tmux",
		"terminal-shell":       "/bin/bash",
		"terminal-dry-run":     "true",
	})
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}

	if _, err := runTerm(t, h, ""); err != nil {
		t.Fatalf("/term returned an error: %v", err)
	}

	out := allPrints(h)
	// "-A" is what makes the toggle idempotent: attach if the session
	// exists, create it otherwise.
	want := `tmux new-session -A -s kit-test-session -c /tmp/kit-term-test /bin/bash`
	if !strings.Contains(out, want) {
		t.Errorf("dry run output does not contain the expected command.\ngot:\n%s\nwant substring:\n%s", out, want)
	}
	if !strings.Contains(out, "dry run") {
		t.Errorf("dry run output is not labelled as a dry run:\n%s", out)
	}
}

func TestPopupTerminal_DryRunDoesNotSuspendTUI(t *testing.T) {
	// The mock's SuspendTUI runs its callback for real, so a dry run that
	// still called it would spawn a shell during the test. Proving the
	// process is not spawned is the point of this test: the mock CWD does
	// not exist, so exec.Cmd.Run would fail on chdir and the command would
	// report an error.
	h := loadPopupTerm(t, map[string]string{
		"terminal-multiplexer": "none",
		"terminal-shell":       "/nonexistent/shell",
		"terminal-dry-run":     "true",
	})
	msg, err := runTerm(t, h, "")
	if err != nil {
		t.Fatalf("dry run should not fail, got: %v", err)
	}
	if msg != "" {
		t.Errorf("dry run returned %q, want an empty status line", msg)
	}
	if strings.Contains(allPrints(h), "Terminal exited") {
		t.Error("dry run executed the command")
	}
}

func TestPopupTerminal_DryRunNoMultiplexer(t *testing.T) {
	h := loadPopupTerm(t, map[string]string{
		"terminal-multiplexer": "none",
		"terminal-shell":       "/bin/sh",
		"terminal-dry-run":     "true",
	})
	if _, err := runTerm(t, h, ""); err != nil {
		t.Fatalf("/term returned an error: %v", err)
	}
	out := allPrints(h)
	if !strings.Contains(out, "backend:  none") {
		t.Errorf("expected backend none in:\n%s", out)
	}
	if !strings.Contains(out, "command:  /bin/sh") {
		t.Errorf("expected a bare shell command in:\n%s", out)
	}
}

func TestPopupTerminal_SessionNameIsSanitized(t *testing.T) {
	// tmux reads "." and ":" as window and pane separators, so a raw
	// session ID must never reach it.
	cases := []struct {
		sessionID string
		want      string
	}{
		{"test-session", "kit-test-session"},
		{"2026-09-02T11.20.47", "kit-2026-09-02T11-20-47"},
		{"a/b:c.d", "kit-a-b-c-d"},
		{"01JABCDEF0123456789", "kit-01JABCDEF0123456789"},
	}

	for _, tc := range cases {
		t.Run(tc.sessionID, func(t *testing.T) {
			h := loadPopupTerm(t, map[string]string{
				"terminal-multiplexer": "none",
				"terminal-dry-run":     "true",
			})
			h.Context().SessionID = tc.sessionID

			if _, err := runTerm(t, h, ""); err != nil {
				t.Fatalf("/term returned an error: %v", err)
			}
			out := allPrints(h)
			if !strings.Contains(out, "session:  "+tc.want) {
				t.Errorf("session name for %q not %q:\n%s", tc.sessionID, tc.want, out)
			}
			if strings.ContainsAny(tc.want, ".:/") {
				t.Fatalf("test case %q is itself unsafe", tc.want)
			}
		})
	}
}

func TestPopupTerminal_NoSessionIDStillGetsAName(t *testing.T) {
	h := loadPopupTerm(t, map[string]string{
		"terminal-multiplexer": "none",
		"terminal-dry-run":     "true",
	})
	h.Context().SessionID = ""

	if _, err := runTerm(t, h, ""); err != nil {
		t.Fatalf("/term returned an error: %v", err)
	}
	out := allPrints(h)
	// Falls back to the PID so two --no-session instances do not collide.
	if !strings.Contains(out, "session:  kit-p") {
		t.Errorf("expected a pid-derived session name in:\n%s", out)
	}
}

func TestPopupTerminal_ShellFallsBackToEnv(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/fish")
	h := loadPopupTerm(t, map[string]string{
		"terminal-multiplexer": "none",
		"terminal-dry-run":     "true",
	})
	if _, err := runTerm(t, h, ""); err != nil {
		t.Fatalf("/term returned an error: %v", err)
	}
	if !strings.Contains(allPrints(h), "/usr/bin/fish") {
		t.Errorf("expected $SHELL to be used:\n%s", allPrints(h))
	}
}

// ---------------------------------------------------------------------------
// Errors and subcommands
// ---------------------------------------------------------------------------

func TestPopupTerminal_UnknownMultiplexerIsRejected(t *testing.T) {
	h := loadPopupTerm(t, map[string]string{
		"terminal-multiplexer": "screen",
		"terminal-dry-run":     "true",
	})
	_, err := runTerm(t, h, "")
	if err == nil {
		t.Fatal("expected an error for an unknown multiplexer")
	}
	if !strings.Contains(err.Error(), "screen") {
		t.Errorf("error should name the bad value, got: %v", err)
	}
}

func TestPopupTerminal_MissingMultiplexerIsReported(t *testing.T) {
	if _, err := exec.LookPath("abduco"); err == nil {
		t.Skip("abduco is installed, cannot test the missing-binary path")
	}
	h := loadPopupTerm(t, map[string]string{
		"terminal-multiplexer": "abduco",
		"terminal-dry-run":     "true",
	})
	_, err := runTerm(t, h, "")
	if err == nil {
		t.Fatal("expected an error when the multiplexer is not installed")
	}
	if !strings.Contains(err.Error(), "PATH") {
		t.Errorf("error should mention PATH, got: %v", err)
	}
}

func TestPopupTerminal_UnknownSubcommandIsRejected(t *testing.T) {
	h := loadPopupTerm(t, nil)
	_, err := runTerm(t, h, "frobnicate")
	if err == nil {
		t.Fatal("expected an error for an unknown subcommand")
	}
	if !strings.Contains(err.Error(), "usage:") {
		t.Errorf("error should show usage, got: %v", err)
	}
}

func TestPopupTerminal_StatusReportsState(t *testing.T) {
	h := loadPopupTerm(t, map[string]string{
		"terminal-multiplexer": "none",
		"terminal-shell":       "/bin/sh",
	})
	if _, err := runTerm(t, h, "status"); err != nil {
		t.Fatalf("/term status returned an error: %v", err)
	}
	out := allPrints(h)
	for _, want := range []string{"backend:", "session:", "shell:", "state:"} {
		if !strings.Contains(out, want) {
			t.Errorf("status output is missing %q:\n%s", want, out)
		}
	}
	// With no multiplexer the user should be told persistence is off.
	if !strings.Contains(out, "does not persist") {
		t.Errorf("status should warn that the shell does not persist:\n%s", out)
	}
}

func TestPopupTerminal_KillWithoutMultiplexerFails(t *testing.T) {
	h := loadPopupTerm(t, map[string]string{"terminal-multiplexer": "none"})
	_, err := runTerm(t, h, "kill")
	if err == nil {
		t.Fatal("expected an error killing with no multiplexer")
	}
}

func TestPopupTerminal_NonInteractiveIsRefused(t *testing.T) {
	h := loadPopupTerm(t, map[string]string{"terminal-multiplexer": "none"})
	h.Context().Interactive = false

	_, err := runTerm(t, h, "")
	if err == nil {
		t.Fatal("expected /term to refuse to run outside the TUI")
	}
	if !strings.Contains(err.Error(), "interactive") {
		t.Errorf("error should mention the interactive TUI, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func TestPopupTerminal_ShutdownWithoutAttachIsSafe(t *testing.T) {
	// Nothing was ever attached, so shutdown must not try to kill anything.
	h := loadPopupTerm(t, map[string]string{"terminal-multiplexer": "tmux"})

	if _, err := h.Emit(extensions.SessionStartEvent{SessionID: "test-session"}); err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	if _, err := h.Emit(extensions.SessionShutdownEvent{}); err != nil {
		t.Fatalf("SessionShutdown: %v", err)
	}
}

func TestPopupTerminal_StatusBarClearedWhenNothingRunning(t *testing.T) {
	h := loadPopupTerm(t, map[string]string{"terminal-multiplexer": "none"})
	if _, err := h.Emit(extensions.SessionStartEvent{SessionID: "test-session"}); err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	if _, ok := h.Context().StatusEntries["popup-terminal"]; ok {
		t.Error("status marker set even though no terminal is running")
	}
}

// ---------------------------------------------------------------------------
// Integration against real tmux
// ---------------------------------------------------------------------------

// tmuxAvailable reports whether a usable tmux server can be reached.
func tmuxAvailable(t *testing.T) bool {
	t.Helper()
	if testing.Short() {
		return false
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return false
	}
	return true
}

// TestPopupTerminal_TmuxLifecycle drives the liveness probe, the status
// report and the kill path against a real tmux server. It creates the session
// out-of-band (attaching needs a tty, which a test does not have) and then
// asks the extension to observe and tear it down.
func TestPopupTerminal_TmuxLifecycle(t *testing.T) {
	if !tmuxAvailable(t) {
		t.Skip("tmux is not available (or -short)")
	}

	const sessionID = "itest-lifecycle"
	const tmuxName = "kit-itest-lifecycle"

	// Ensure a clean slate, and clean up even if the test fails.
	_ = exec.Command("tmux", "kill-session", "-t", "="+tmuxName).Run()
	t.Cleanup(func() {
		_ = exec.Command("tmux", "kill-session", "-t", "="+tmuxName).Run()
	})

	h := loadPopupTerm(t, map[string]string{"terminal-multiplexer": "tmux"})
	h.Context().SessionID = sessionID

	// 1. Nothing running yet.
	if _, err := runTerm(t, h, "status"); err != nil {
		t.Fatalf("/term status: %v", err)
	}
	if !strings.Contains(allPrints(h), "not started") {
		t.Errorf("expected \"not started\" before creation:\n%s", allPrints(h))
	}
	if _, ok := h.Context().StatusEntries["popup-terminal"]; ok {
		t.Error("status marker set before the session exists")
	}

	// 2. Create the session the way /term would, but detached so it works
	//    without a tty.
	out, err := exec.Command("tmux", "new-session", "-d", "-s", tmuxName, "sh", "-c", "sleep 300").CombinedOutput()
	if err != nil {
		t.Skipf("cannot start a tmux session in this environment: %v: %s", err, out)
	}

	// 3. The extension must now see it as running, and mark the status bar.
	h2 := loadPopupTerm(t, map[string]string{"terminal-multiplexer": "tmux"})
	h2.Context().SessionID = sessionID
	if _, err := runTerm(t, h2, "status"); err != nil {
		t.Fatalf("/term status: %v", err)
	}
	if !strings.Contains(allPrints(h2), "running (detached)") {
		t.Errorf("expected the live session to be reported as running:\n%s", allPrints(h2))
	}
	entry, ok := h2.Context().StatusEntries["popup-terminal"]
	if !ok {
		t.Error("status marker not set while the terminal is running")
	} else if entry.Text != "term" {
		t.Errorf("status text = %q, want \"term\"", entry.Text)
	}

	// 4. Kill it through the extension.
	msg, err := runTerm(t, h2, "kill")
	if err != nil {
		t.Fatalf("/term kill: %v", err)
	}
	if !strings.Contains(msg, tmuxName) {
		t.Errorf("kill message %q should name the session", msg)
	}

	// 5. tmux must agree that it is gone.
	if exec.Command("tmux", "has-session", "-t", "="+tmuxName).Run() == nil {
		t.Fatal("tmux session still exists after /term kill")
	}
	if _, ok := h2.Context().StatusEntries["popup-terminal"]; ok {
		t.Error("status marker still set after the session was killed")
	}
}

// TestPopupTerminal_TmuxExactNameMatch guards the "=" prefix on the tmux
// target. Without it tmux does prefix matching and a longer session name
// would satisfy the liveness probe for a shorter one — and, worse, /term kill
// would kill the wrong shell.
func TestPopupTerminal_TmuxExactNameMatch(t *testing.T) {
	if !tmuxAvailable(t) {
		t.Skip("tmux is not available (or -short)")
	}

	const decoy = "kit-itest-prefix-longer"
	_ = exec.Command("tmux", "kill-session", "-t", "="+decoy).Run()
	t.Cleanup(func() {
		_ = exec.Command("tmux", "kill-session", "-t", "="+decoy).Run()
	})

	out, err := exec.Command("tmux", "new-session", "-d", "-s", decoy, "sh", "-c", "sleep 300").CombinedOutput()
	if err != nil {
		t.Skipf("cannot start a tmux session in this environment: %v: %s", err, out)
	}

	// This session ID yields "kit-itest-prefix", a strict prefix of the decoy.
	h := loadPopupTerm(t, map[string]string{"terminal-multiplexer": "tmux"})
	h.Context().SessionID = "itest-prefix"

	if _, err := runTerm(t, h, "status"); err != nil {
		t.Fatalf("/term status: %v", err)
	}
	if strings.Contains(allPrints(h), "running (detached)") {
		t.Error("liveness probe matched a session by prefix; the \"=\" exact-match prefix is missing")
	}

	if _, err := runTerm(t, h, "kill"); err == nil {
		t.Error("/term kill matched a session by prefix and would have killed the wrong shell")
	}

	if exec.Command("tmux", "has-session", "-t", "="+decoy).Run() != nil {
		t.Fatal("the unrelated tmux session was killed")
	}
}

// TestPopupTerminal_TmuxEnvIsUnnested verifies that $TMUX is stripped from
// the child environment, which is what lets the popup work while Kit itself
// runs inside tmux.
func TestPopupTerminal_TmuxEnvIsUnnested(t *testing.T) {
	if !tmuxAvailable(t) {
		t.Skip("tmux is not available (or -short)")
	}
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")

	const sessionID = "itest-nested"
	const tmuxName = "kit-itest-nested"
	_ = exec.Command("tmux", "kill-session", "-t", "="+tmuxName).Run()
	t.Cleanup(func() {
		_ = exec.Command("tmux", "kill-session", "-t", "="+tmuxName).Run()
	})

	// Reproduce exactly what the extension does: the same argv and the same
	// filtered environment, run detached so no tty is needed. If $TMUX
	// leaked through, tmux would refuse with "sessions should be nested
	// with care".
	env := make([]string, 0)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "TMUX=") {
			continue
		}
		env = append(env, kv)
	}
	cmd := exec.Command("tmux", "new-session", "-d", "-s", tmuxName, "sh", "-c", "sleep 300")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot start a tmux session in this environment: %v: %s", err, out)
	}

	h := loadPopupTerm(t, map[string]string{"terminal-multiplexer": "tmux"})
	h.Context().SessionID = sessionID
	if _, err := runTerm(t, h, "status"); err != nil {
		t.Fatalf("/term status: %v", err)
	}
	if !strings.Contains(allPrints(h), "running (detached)") {
		t.Errorf("session did not start with $TMUX stripped:\n%s", allPrints(h))
	}
}
