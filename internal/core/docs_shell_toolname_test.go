package core

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// legacyToolNameEquality matches a doc example that tests a live tool name for
// equality with the shell tool's earlier name, e.g. `h.ToolName == "bash"`.
var legacyToolNameEquality = regexp.MustCompile(`ToolName\s*(==|!=)\s*"` + LegacyShellToolName + `"`)

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
// A comparison is allowed when the same line also names ShellToolName, which is
// the defensive `!= "shell" && != "bash"` form used by examples that must cope
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
				if !legacyToolNameEquality.MatchString(line) {
					continue
				}
				if strings.Contains(line, `"`+ShellToolName+`"`) {
					continue // defensive form naming both spellings
				}
				rel, relErr := filepath.Rel(repoRoot, path)
				if relErr != nil {
					rel = path
				}
				t.Errorf("%s:%d compares a live ToolName to %q, which never matches "+
					"(the shell tool reports %q); use %q or name both spellings:\n\t%s",
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
