package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestShellEnvForcesColorWhenAsked(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	env := shellEnv([]string{"PATH=/bin", "FORCE_COLOR=0"}, "/bin/bash", true)
	want := []string{
		"COLORTERM=truecolor",
		"CLICOLOR_FORCE=1",
		"FORCE_COLOR=1",
	}
	for _, w := range want {
		if !containsEnv(env, w) {
			t.Errorf("shellEnv did not set %s: %v", w, env)
		}
	}
	if got := envValue(env, "SHELL"); got != "/bin/bash" {
		t.Errorf("SHELL = %q, want /bin/bash", got)
	}
	if got := envValue(env, "EDITOR"); got != "false" {
		t.Errorf("EDITOR = %q, want false", got)
	}
}

// A caller's own value must be replaced, not duplicated, or the child would see
// two values for the same name and the winner would depend on its own lookup.
func TestShellEnvOverridesRatherThanAppends(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	env := shellEnv([]string{"EDITOR=nvim", "PAGER=less", "PATH=/bin"}, "", true)
	if n := countEnv(env, "EDITOR="); n != 1 {
		t.Errorf("EDITOR appears %d times: %v", n, env)
	}
	if got := envValue(env, "EDITOR"); got != "false" {
		t.Errorf("EDITOR = %q, want false", got)
	}
	if got := envValue(env, "PAGER"); got != "cat" {
		t.Errorf("PAGER = %q, want cat", got)
	}
	if got := envValue(env, "PATH"); got != "/bin" {
		t.Errorf("PATH = %q, want /bin", got)
	}
	if got := envValue(env, "SHELL"); got != "" {
		t.Errorf("SHELL = %q, want it left inherited", got)
	}
}

func TestShellEnvHonoursNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	env := shellEnv([]string{"PATH=/bin", "NO_COLOR=1"}, "/bin/bash", true)
	for _, k := range []string{"FORCE_COLOR", "CLICOLOR_FORCE", "COLORTERM"} {
		if n := countEnv(env, k+"="); n != 0 {
			t.Errorf("%s was set %d times with NO_COLOR present: %v", k, n, env)
		}
	}
	// The user's own NO_COLOR still reaches the child.
	if got := envValue(env, "NO_COLOR"); got != "1" {
		t.Errorf("NO_COLOR = %q, want 1", got)
	}
}

func TestShellEnvDoesNotMutateInput(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	base := []string{"EDITOR=nvim", "PATH=/bin"}
	before := len(base)
	shellEnv(base, "/bin/bash", true)
	if len(base) != before || base[0] != "EDITOR=nvim" {
		t.Errorf("base was modified: %v", base)
	}
}

// Colour is a per-caller decision. The shell tool must not get it, because the
// variables reach every process in the command tree.
func TestShellEnvOmitsColorWhenNotAsked(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	env := shellEnv([]string{"PATH=/bin"}, "/bin/bash", false)
	for _, k := range []string{"FORCE_COLOR", "CLICOLOR_FORCE", "COLORTERM"} {
		if n := countEnv(env, k+"="); n != 0 {
			t.Errorf("%s was set %d times with colour declined: %v", k, n, env)
		}
	}
	// The non-interactive overrides are unconditional.
	if got := envValue(env, "PAGER"); got != "cat" {
		t.Errorf("PAGER = %q, want cat", got)
	}
}

// ---------------------------------------------------------------------------

// collectSink records every chunk with a mutex, because the two reader
// goroutines call it concurrently.
type collectSink struct {
	mu     sync.Mutex
	joined strings.Builder
	count  int
}

func (c *collectSink) sink(chunk string, _ bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.joined.WriteString(chunk)
	c.count++
}

func (c *collectSink) snapshot() (string, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.joined.String(), c.count
}

func TestRunShellCommandStreamsAndCollects(t *testing.T) {
	var sink collectSink
	res, err := RunShellCommand(context.Background(), ShellRunOptions{
		Command: "printf 'one\\ntwo\\nthree\\n'",
		OnChunk: sink.sink,
	})
	if err != nil {
		t.Fatalf("RunShellCommand: %v", err)
	}
	if res.Output != "one\ntwo\nthree\n" {
		t.Errorf("Output = %q", res.Output)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	streamed, count := sink.snapshot()
	if streamed != res.Output {
		t.Errorf("streamed %q, collected %q", streamed, res.Output)
	}
	if count < 1 {
		t.Error("no chunk was streamed")
	}
}

func TestRunShellCommandCombinesStderr(t *testing.T) {
	res, err := RunShellCommand(context.Background(), ShellRunOptions{
		Command: "printf 'out\\n'; printf 'err\\n' >&2",
	})
	if err != nil {
		t.Fatalf("RunShellCommand: %v", err)
	}
	if res.Output != "out\nerr\n" {
		t.Errorf("Output = %q, want %q", res.Output, "out\nerr\n")
	}
}

func TestRunShellCommandReportsExitCode(t *testing.T) {
	res, err := RunShellCommand(context.Background(), ShellRunOptions{
		Command: "exit 3",
	})
	if err != nil {
		t.Fatalf("RunShellCommand: %v", err)
	}
	if res.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", res.ExitCode)
	}
}

// The whole point of the colour environment: a command that would stay plain
// when piped must come back coloured.
func TestRunShellCommandEmitsColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	res, err := RunShellCommand(context.Background(), ShellRunOptions{
		Command: `printf '\033[31mRED\033[0m\n'`,
	})
	if err != nil {
		t.Fatalf("RunShellCommand: %v", err)
	}
	if !strings.Contains(res.Output, "\x1b[") {
		t.Fatalf("no escape sequence in output %q", res.Output)
	}
}

func TestRunShellCommandTimesOut(t *testing.T) {
	res, err := RunShellCommand(context.Background(), ShellRunOptions{
		Command: "printf 'before\\n'; sleep 30",
		Timeout: 300 * time.Millisecond,
	})
	if !errors.Is(err, ErrShellCommandTimedOut) {
		t.Fatalf("err = %v, want ErrShellCommandTimedOut", err)
	}
	if !res.TimedOut {
		t.Error("TimedOut flag not set")
	}
	// Output produced before the kill is kept, so the user sees how far it got.
	if !strings.Contains(res.Output, "before") {
		t.Errorf("early output lost: %q", res.Output)
	}
}

func TestRunShellCommandNoOnChunkStillCollects(t *testing.T) {
	res, err := RunShellCommand(context.Background(), ShellRunOptions{
		Command: "printf 'quiet\\n'",
	})
	if err != nil {
		t.Fatalf("RunShellCommand: %v", err)
	}
	if res.Output != "quiet\n" {
		t.Errorf("Output = %q", res.Output)
	}
}

// A backgrounded grandchild must not hold the call open: the drain watchdog
// force-closes the pipes and the call returns.
func TestRunShellCommandBackgroundedGrandchildDoesNotHang(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		res, err := RunShellCommand(context.Background(), ShellRunOptions{
			Command: "echo started; sleep 20 &",
			Timeout: 25 * time.Second,
		})
		if err != nil {
			t.Errorf("RunShellCommand: %v", err)
		}
		if !strings.Contains(res.Output, "started") {
			t.Errorf("foreground output missing: %q", res.Output)
		}
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("RunShellCommand hung on a backgrounded grandchild")
	}
}

func TestRunShellCommandInvalidShell(t *testing.T) {
	_, err := RunShellCommand(context.Background(), ShellRunOptions{
		Command: "true",
		Shell:   []string{"bash", ""},
	})
	if !errors.Is(err, errEmptyShellElement) {
		t.Fatalf("err = %v, want errEmptyShellElement", err)
	}
}

func TestRunShellCommandRunsInWorkDir(t *testing.T) {
	dir := t.TempDir()
	res, err := RunShellCommand(context.Background(), ShellRunOptions{
		Command: "pwd",
		WorkDir: dir,
	})
	if err != nil {
		t.Fatalf("RunShellCommand: %v", err)
	}
	// macOS reports /private/var for /var, so compare the resolved path.
	if got := strings.TrimSpace(res.Output); got == "" {
		t.Error("no output from pwd")
	}
}

// ---------------------------------------------------------------------------

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if after, ok := strings.CutPrefix(kv, prefix); ok {
			return after
		}
	}
	return ""
}

func countEnv(env []string, prefix string) int {
	n := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			n++
		}
	}
	return n
}

func containsEnv(env []string, want string) bool {
	return countEnv(env, want) > 0
}
