package core

import (
	"reflect"
	"strings"
	"testing"
)

// These tests cover the pure part of the shell tool: argument-vector
// construction, the name and descriptions derived from the configured shell,
// and the tool-name mapping. Nothing here starts a process; the tests that do
// live in shell_exec_test.go and shell_streaming_test.go.

func TestNormalizeShell(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"unset uses the default", nil, []string{"bash"}},
		{"empty uses the default", []string{}, []string{"bash"}},
		{"a shell is kept as written", []string{"/bin/dash"}, []string{"/bin/dash"}},
		{"leading arguments are kept as written", []string{"busybox", "ash"}, []string{"busybox", "ash"}},
	}
	for _, c := range cases {
		if got := normalizeShell(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: normalizeShell(%#v) = %#v, want %#v", c.name, c.in, got, c.want)
		}
	}
}

func TestResolveShell_DefaultIsUnchangedBehaviour(t *testing.T) {
	r, err := resolveShell(nil)
	if err != nil {
		t.Fatalf("resolveShell(nil): %v", err)
	}
	if !reflect.DeepEqual(r.argv, []string{"bash"}) {
		t.Errorf("argv = %#v, want [bash]", r.argv)
	}
	if r.shellPath == "" {
		t.Error("shellPath is empty")
	}
	if got := r.commandArgs("echo hello"); !reflect.DeepEqual(got, []string{"bash", "-c", "echo hello"}) {
		t.Errorf("commandArgs = %#v, want [bash -c echo hello]", got)
	}
}

func TestCommandArgs(t *testing.T) {
	r, err := resolveShell([]string{"busybox", "ash"})
	if err != nil {
		t.Fatalf("resolveShell: %v", err)
	}
	got := r.commandArgs("make all")
	if !reflect.DeepEqual(got, []string{"busybox", "ash", "-c", "make all"}) {
		t.Errorf("commandArgs = %#v, want [busybox ash -c make all]", got)
	}
}

func TestResolveShell_CommandIsOneArgument(t *testing.T) {
	r, err := resolveShell([]string{"/bin/dash"})
	if err != nil {
		t.Fatalf("resolveShell: %v", err)
	}
	// A command with spaces, quotes and a semicolon must arrive as a single
	// argument. Splitting or re-quoting it would change what the shell runs.
	command := `echo "a b"; echo 'c d'`
	got := r.commandArgs(command)
	if len(got) != 3 {
		t.Fatalf("commandArgs produced %d arguments, want 3: %#v", len(got), got)
	}
	if got[2] != command {
		t.Errorf("command argument = %q, want %q", got[2], command)
	}
}

func TestResolveShell_UnresolvableNameIsNotAnError(t *testing.T) {
	r, err := resolveShell([]string{"no-such-shell-9d3f"})
	if err != nil {
		t.Fatalf("a PATH failure must not be an error, got %v", err)
	}
	if r.shellPath != "no-such-shell-9d3f" {
		t.Errorf("shellPath = %q, want the unresolved name", r.shellPath)
	}
}

func TestResolveShell_EmptyElementRejected(t *testing.T) {
	if _, err := resolveShell([]string{"bash", ""}); err != errEmptyShellElement {
		t.Errorf("resolveShell([bash, \"\"]) error = %v, want errEmptyShellElement", err)
	}
	if _, err := resolveShell([]string{""}); err != errEmptyShellElement {
		t.Errorf("resolveShell([\"\"]) error = %v, want errEmptyShellElement", err)
	}
}

func TestShellDisplayName(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"bash"}, "bash"},
		{[]string{"/bin/dash"}, "/bin/dash"},
		{[]string{"busybox", "ash"}, "busybox ash"},
		{[]string{"env", "-i", "/bin/dash"}, "env -i /bin/dash"},
	}
	for _, c := range cases {
		if got := shellDisplayName(c.in); got != c.want {
			t.Errorf("shellDisplayName(%#v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestShellToolDescription_DialectNote(t *testing.T) {
	// bash: no dialect note at all.
	d := shellToolDescription(nil)
	if !strings.Contains(d, "bash") {
		t.Errorf("default description does not name bash: %q", d)
	}
	if strings.Contains(d, "IMPORTANT") {
		t.Errorf("default description carries a dialect note: %q", d)
	}

	// A POSIX shell: name it and ask for POSIX syntax.
	d = shellToolDescription([]string{"/bin/dash"})
	if !strings.Contains(d, "/bin/dash") {
		t.Errorf("description does not name the shell: %q", d)
	}
	if !strings.Contains(d, "POSIX sh syntax only") {
		t.Errorf("description of a POSIX shell does not ask for POSIX syntax: %q", d)
	}

	// A shell that is not POSIX: name it, and claim no dialect for it.
	d = shellToolDescription([]string{"/usr/bin/fish"})
	if !strings.Contains(d, "/usr/bin/fish") {
		t.Errorf("description does not name the shell: %q", d)
	}
	if strings.Contains(d, "POSIX") {
		t.Errorf("description instructs POSIX syntax for a shell that is not POSIX: %q", d)
	}
}

func TestShellCommandParamDescription(t *testing.T) {
	if got := shellCommandParamDescription(nil); got != "Command to execute with bash" {
		t.Errorf("default = %q", got)
	}
	if got := shellCommandParamDescription([]string{"/bin/dash"}); got != "Command to execute with /bin/dash (POSIX sh syntax)" {
		t.Errorf("dash = %q", got)
	}
	if got := shellCommandParamDescription([]string{"/usr/bin/fish"}); got != "Command to execute with /usr/bin/fish" {
		t.Errorf("fish = %q", got)
	}
}

func TestShellDialectClassification(t *testing.T) {
	if !isBashShell([]string{"/usr/local/bin/bash"}) {
		t.Error("bash by absolute path should classify as bash")
	}
	if !isPosixShell([]string{"busybox", "ash"}) {
		t.Error("busybox ash should classify as POSIX")
	}
	for _, s := range []string{"zsh", "fish", "nu", "elvish", "pwsh"} {
		if isPosixShell([]string{s}) {
			t.Errorf("%s should not classify as POSIX", s)
		}
	}
}

func TestShellTool_NameDoesNotVaryWithTheShell(t *testing.T) {
	for _, shell := range [][]string{nil, {"bash"}, {"/bin/dash"}, {"busybox", "ash"}} {
		tool := NewShellTool(WithShell(shell))
		if got := tool.Info().Name; got != ShellToolName {
			t.Errorf("shell %#v: tool name = %q, want %q", shell, got, ShellToolName)
		}
	}
}

func TestNormalizeCoreToolName(t *testing.T) {
	if got := NormalizeCoreToolName(LegacyShellToolName); got != ShellToolName {
		t.Errorf("NormalizeCoreToolName(%q) = %q, want %q", LegacyShellToolName, got, ShellToolName)
	}
	for _, n := range []string{ShellToolName, "read", "write", "edit", "grep", "find", "ls", "subagent", "unknown"} {
		if got := NormalizeCoreToolName(n); got != n {
			t.Errorf("NormalizeCoreToolName(%q) = %q, want it unchanged", n, got)
		}
	}
}

func TestListedTools_AcceptsTheEarlierName(t *testing.T) {
	if got := len(ListedTools([]string{LegacyShellToolName})); got != 1 {
		t.Errorf("ListedTools([bash]) produced %d tools, want 1", got)
	}
	if got := ListedTools([]string{LegacyShellToolName}); len(got) == 1 && got[0].Info().Name != ShellToolName {
		t.Errorf("ListedTools([bash])[0] = %q, want %q", got[0].Info().Name, ShellToolName)
	}
	// An unknown name is skipped rather than dereferenced.
	if got := len(ListedTools([]string{"no-such-tool"})); got != 0 {
		t.Errorf("ListedTools([no-such-tool]) produced %d tools, want 0", got)
	}
}

func TestApplyOptions_ShellIsPerConfiguration(t *testing.T) {
	// Two configurations in one process must not observe each other. The
	// resolution path reads no package-level state, which is what makes this
	// hold for two Kit instances in one process.
	a := ApplyOptions([]ToolOption{WithShell([]string{"sh"})})
	b := ApplyOptions(nil)
	if !reflect.DeepEqual(a.Shell, []string{"sh"}) {
		t.Errorf("configured shell = %#v", a.Shell)
	}
	if b.Shell != nil {
		t.Errorf("unconfigured shell = %#v, want nil", b.Shell)
	}
	ra, _ := resolveShell(a.Shell)
	rb, _ := resolveShell(b.Shell)
	if ra.argv[0] != "sh" || rb.argv[0] != "bash" {
		t.Errorf("argv0 = %q and %q, want sh and bash", ra.argv[0], rb.argv[0])
	}
}

func TestWithShell_EmptyLeavesTheDefault(t *testing.T) {
	cfg := ApplyOptions([]ToolOption{WithShell(nil), WithShell([]string{})})
	if cfg.Shell != nil {
		t.Errorf("Shell = %#v, want nil so that the default applies", cfg.Shell)
	}
}

func TestResolveShell_LauncherVectorLeavesSHELLAlone(t *testing.T) {
	// For ["busybox", "ash"] KIT cannot know which element is the shell;
	// advertising the busybox binary as SHELL would hand child programs a
	// dispatcher, not a shell. The resolution reports no path, and the
	// execution paths then leave SHELL inherited.
	r, err := resolveShell([]string{"busybox", "ash"})
	if err != nil {
		t.Fatalf("resolveShell: %v", err)
	}
	if r.shellPath != "" {
		t.Errorf("shellPath = %q, want empty for a launcher vector", r.shellPath)
	}
}

func TestShellCommandArgs_MatchesTheToolConstruction(t *testing.T) {
	args, shellPath, err := ShellCommandArgs([]string{"busybox", "ash"}, "make all")
	if err != nil {
		t.Fatalf("ShellCommandArgs: %v", err)
	}
	if !reflect.DeepEqual(args, []string{"busybox", "ash", "-c", "make all"}) {
		t.Errorf("args = %#v", args)
	}
	if shellPath != "" {
		t.Errorf("shellPath = %q, want empty for a launcher vector", shellPath)
	}
	if _, _, err := ShellCommandArgs([]string{""}, "true"); err == nil {
		t.Error("an empty element must be rejected")
	}
}
