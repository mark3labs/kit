package core

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// toolNameComparison matches a comparison between a *ToolName field and a
// quoted literal, in either operand order:
//
//	h.ToolName == "bash"    tc.ToolName != "shell"    "bash" == h.ToolName
//
// The captured literal is whichever side is quoted.
var toolNameComparison = regexp.MustCompile(
	`(?:[\w.]*ToolName\s*(?:==|!=)\s*"([^"]*)")|(?:"([^"]*)"\s*(?:==|!=)\s*[\w.]*ToolName)`)

// stripLineComment removes a trailing // comment so that comment prose cannot
// satisfy the ShellToolName exception below. A line such as
//
//	if h.ToolName == "bash" { // unlike h.ToolName == "shell"
//
// would otherwise look like it compares against both names. Quoted strings are
// tracked so a // inside a literal (a URL, say) is not mistaken for a comment.
func stripLineComment(line string) string {
	var inString bool
	var quote rune
	prev := rune(0)
	for i, r := range line {
		switch {
		case inString:
			if r == quote && prev != '\\' {
				inString = false
			}
		case r == '"' || r == '`' || r == '\'':
			inString = true
			quote = r
		case r == '/' && prev == '/':
			return line[:i-1]
		}
		prev = r
	}
	return line
}

// comparedToolNames returns every quoted literal that line compares against a
// ToolName field, in either operand order. Trailing comments are ignored.
func comparedToolNames(line string) []string {
	var out []string
	for _, m := range toolNameComparison.FindAllStringSubmatch(stripLineComment(line), -1) {
		// Exactly one of the two capture groups is set per match.
		if m[1] != "" {
			out = append(out, m[1])
		} else if m[2] != "" {
			out = append(out, m[2])
		}
	}
	return out
}

// flagsLegacyToolNameComparison reports whether line compares a live ToolName
// against the shell tool's earlier name without also comparing it against the
// current name. The exception is scoped to ToolName comparisons specifically,
// so an unrelated `|| label == "shell"` on the same line does not excuse it.
func flagsLegacyToolNameComparison(line string) bool {
	names := comparedToolNames(line)
	var sawLegacy, sawCurrent bool
	for _, n := range names {
		switch n {
		case LegacyShellToolName:
			sawLegacy = true
		case ShellToolName:
			sawCurrent = true
		}
	}
	return sawLegacy && !sawCurrent
}

// TestFlagsLegacyToolNameComparison covers the matcher used by the docs scan
// below, including both operand orders and the scoped "shell" exception.
func TestFlagsLegacyToolNameComparison(t *testing.T) {
	cases := []struct {
		line string
		flag bool
	}{
		// Plain comparisons against the earlier name — always dead code.
		{`    if h.ToolName == "bash" {`, true},
		{`    if h.ToolName != "bash" {`, true},
		{`    if "bash" == h.ToolName {`, true},
		{`    if "bash" != tc.ToolName {`, true},
		{`if e.ToolName=="bash"{`, true},

		// The defensive both-spellings form, in either order.
		{`if tc.ToolName != "shell" && tc.ToolName != "bash" {`, false},
		{`if tc.ToolName == "bash" || tc.ToolName == "shell" {`, false},
		{`if "shell" == h.ToolName || "bash" == h.ToolName {`, false},

		// A "shell" literal that is not a ToolName comparison must not excuse it.
		{`if h.ToolName == "bash" || label == "shell" {`, true},
		{`if h.ToolName == "bash" { log("shell") }`, true},

		// Nor may comment prose: only real code counts toward the exception.
		{`if h.ToolName == "bash" { // unlike h.ToolName == "shell"`, true},
		{`// h.ToolName == "shell" is right, this is not:`, false},
		{`if h.ToolName == "bash" { // TODO: should be "shell"`, true},

		// A // inside a string literal is not a comment.
		{`if h.ToolName == "bash" && url == "http://x" || h.ToolName == "shell" {`, false},

		// Correct and unrelated lines.
		{`    if h.ToolName == "shell" {`, false},
		{`    if h.ToolName == "read" {`, false},
		{`    if e.ToolName == "subagent" {`, false},
		{`    ToolName:    "bash",`, false}, // renderer registration, normalized
		{`list, _ := kit.FilterCoreToolNames(nil, []string{"bash", "write"})`, false},
		{`shell: "bash"`, false}, // the shell binary, not the tool name
		{``, false},
	}
	for _, c := range cases {
		if got := flagsLegacyToolNameComparison(c.line); got != c.flag {
			t.Errorf("flagsLegacyToolNameComparison(%q) = %v, want %v", c.line, got, c.flag)
		}
	}
}

// TestDocsDoNotCompareLiveToolNameToLegacyShellName keeps doc examples honest
// about the name the shell tool reports at runtime.
//
// NormalizeCoreToolName accepts LegacyShellToolName wherever a *user names* the
// tool — CoreToolList, include/exclude flags, SetActiveTools, tool renderers —
// so "bash" is correct in those positions. It is not applied to the ToolName
// carried by events and hooks: hooks read the live fantasy.ToolInfo.Name, which
// is always ShellToolName. An example comparing that field to "bash" is dead
// code, and two such examples shipped for months before a reviewer caught them.
//
// A comparison is allowed when the same line also compares ToolName against
// ShellToolName, which is the defensive form used by examples that must cope
// with sessions recorded by older builds.
func TestDocsDoNotCompareLiveToolNameToLegacyShellName(t *testing.T) {
	repoRoot := filepath.Join("..", "..")

	// Directories holding prose and example code that users copy from.
	roots := []string{
		filepath.Join(repoRoot, "www", "pages"),
		filepath.Join(repoRoot, "skills"),
		filepath.Join(repoRoot, "examples"),
		filepath.Join(repoRoot, "pkg", "extensions", "test"),
	}

	var scanned int
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			continue // docs not present in this checkout
		}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil //nolint:nilerr // skip unreadable entries, don't abort
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".md" && ext != ".go" {
				return nil
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			scanned++
			for i, line := range strings.Split(string(body), "\n") {
				if !flagsLegacyToolNameComparison(line) {
					continue
				}
				rel, relErr := filepath.Rel(repoRoot, path)
				if relErr != nil {
					rel = path
				}
				t.Errorf("%s:%d compares a live ToolName to %q, which never matches "+
					"(the shell tool reports %q); use %q or compare against both names:\n\t%s",
					rel, i+1, LegacyShellToolName, ShellToolName, ShellToolName,
					strings.TrimSpace(line))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}

	if scanned == 0 {
		t.Skip("no documentation or example files found in this checkout")
	}
}
