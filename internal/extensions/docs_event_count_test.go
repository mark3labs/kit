package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestDocumentedEventCountMatchesTable keeps the prose event totals in the
// docs honest.
//
// Both pages open with "Kit provides N lifecycle events" / "Extensions can
// hook into N lifecycle events" and then enumerate them in a table. The number
// has drifted out of step three times as events were added (24, 27 and 30 were
// each stale at some point), because nothing tied the sentence to the list
// beneath it. This asserts the two agree.
func TestDocumentedEventCountMatchesTable(t *testing.T) {
	repoRoot := filepath.Join("..", "..")

	capabilities := filepath.Join(repoRoot, "www", "pages", "extensions", "capabilities.md")
	skill := filepath.Join(repoRoot, "skills", "kit-extensions", "SKILL.md")

	// The table lives in capabilities.md and is the source of truth.
	capsBody, err := os.ReadFile(capabilities)
	if err != nil {
		t.Skipf("docs not present in this checkout: %v", fmt.Errorf("read %s: %w", capabilities, err))
	}

	want := countEventTableRows(string(capsBody))
	if want == 0 {
		t.Fatal("found no `| `On...`` table rows in capabilities.md; has the table format changed?")
	}

	for _, tc := range []struct {
		path string
		re   *regexp.Regexp
	}{
		{capabilities, regexp.MustCompile(`hook into (\d+) lifecycle events`)},
		{skill, regexp.MustCompile(`Kit provides (\d+) lifecycle events`)},
	} {
		body, err := os.ReadFile(tc.path)
		if err != nil {
			t.Errorf("%v", fmt.Errorf("read %s: %w", tc.path, err))
			continue
		}
		m := tc.re.FindSubmatch(body)
		if m == nil {
			t.Errorf("%s: no lifecycle-event count sentence matching %q", filepath.Base(tc.path), tc.re)
			continue
		}
		got, err := strconv.Atoi(string(m[1]))
		if err != nil {
			t.Errorf("%s: unparsable count %q", filepath.Base(tc.path), m[1])
			continue
		}
		if got != want {
			t.Errorf("%s claims %d lifecycle events, but the capabilities table lists %d",
				filepath.Base(tc.path), got, want)
		}
	}
}

// countEventTableRows counts the leading `| `OnX“ rows of the lifecycle table,
// stopping at the blank line that ends it so later comparison tables are not
// counted.
func countEventTableRows(body string) int {
	var n int
	var started bool
	for line := range strings.SplitSeq(body, "\n") {
		switch {
		case strings.HasPrefix(line, "| `On"):
			started = true
			n++
		case started && strings.TrimSpace(line) == "":
			return n
		}
	}
	return n
}
