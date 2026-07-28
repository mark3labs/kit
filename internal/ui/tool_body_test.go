package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/ui/style"
)

// Tool bodies are nested content: they sit beneath a tool header whose marker
// occupies column 0. The block contract (block_contract_test.go) pins where
// the header starts and that nothing overflows; these tests pin the interior,
// which the contract test cannot see because every body type satisfies it in
// its own way.

// bodyWidthFor is the width a body renderer receives for a given terminal
// width, matching what RenderToolMessage passes.
func bodyWidthFor(termWidth int) int { return termWidth - 8 }

// bodyIndentColumn returns the number of literal spaces a rendered body line
// carries before its styling begins.
//
// This deliberately measures the un-stripped string. A tool body's panel is
// indented with plain spaces and then painted, so the panel's left edge is the
// last plain space before the first escape sequence. Measuring the stripped
// text instead would report the panel's interior padding (and, for the read
// and write bodies, their line-number gutters) as though it were indentation.
func bodyIndentColumn(rendered string) int {
	for line := range strings.SplitSeq(rendered, "\n") {
		if strings.TrimSpace(stripBlockAnsi(line)) == "" {
			continue
		}
		return len(line) - len(strings.TrimLeft(line, " "))
	}
	return -1
}

// toolBodyCases covers every renderer reachable from renderToolBody, with
// input shaped the way the real tools emit it.
var toolBodyCases = []struct {
	name   string
	tool   string
	args   string
	result string
}{
	{
		name: "bash", tool: "bash", args: `{"command":"ls -la"}`,
		result: "total 24\n-rw-r--r--  1 user staff 1024 main.go\nSTDERR:\nls: nope: No such file\nExit code: 1",
	},
	{
		name: "bash/truncating", tool: "bash", args: `{"command":"yes"}`,
		result: strings.Repeat("a line of output\n", 40),
	},
	{
		name: "ls", tool: "ls", args: `{"path":"/tmp"}`,
		result: strings.Repeat("some/path/to/file.go\n", 30),
	},
	{
		name: "grep", tool: "grep", args: `{"pattern":"func"}`,
		result: strings.Repeat("main.go:12: func main() {}\n", 5),
	},
	{
		name: "find", tool: "find", args: `{"pattern":"*.go"}`,
		result: strings.Repeat("internal/ui/model.go\n", 5),
	},
	{
		name: "read", tool: "read", args: `{"path":"main.go"}`,
		result: "1: package main\n2: \n3: func main() {}\n",
	},
	{
		name: "write", tool: "write",
		args:   `{"path":"a.go","content":"package main\n\nfunc main() {}\n"}`,
		result: "written",
	},
	{
		name: "subagent", tool: "subagent", args: `{"agent":"explore"}`,
		result: "Subagent completed successfully in 12s\n\nResult:\nFound three call sites\n",
	},
	{
		name: "subagent/failed", tool: "subagent", args: `{"agent":"explore"}`,
		result: "Subagent failed\n\nError:\ncontext deadline exceeded\n",
	},
	{
		name: "edit", tool: "edit",
		args:   `{"path":"a.go","edits":[{"old_text":"one","new_text":"two"}]}`,
		result: "edited",
	},
}

// TestToolBodyStartsAtContentColumn pins every tool body to the same left
// edge. Each renderer used to apply its own two-space literal, which meant a
// change to one of them silently produced a body that no longer lined up with
// the rest.
func TestToolBodyStartsAtContentColumn(t *testing.T) {
	for _, w := range []int{40, 80, 120} {
		for _, tc := range toolBodyCases {
			t.Run(tc.name+"/w"+itoa(w), func(t *testing.T) {
				got := renderToolBody(tc.tool, tc.args, tc.result, bodyWidthFor(w))
				if got == "" {
					t.Skip("renderer declined this input")
				}
				if col := bodyIndentColumn(got); col != style.ContentOffset {
					t.Errorf("body panel starts at column %d, want %d\nfirst line: %q",
						col, style.ContentOffset, strings.SplitN(stripBlockAnsi(got), "\n", 2)[0])
				}
			})
		}
	}
}

// TestToolBodyFitsWidth guards the interior directly. RenderToolMessage's
// output is already covered by the block contract, but a body that overflows
// its own budget while the surrounding block stays inside the terminal would
// pass that test and still look wrong.
func TestToolBodyFitsWidth(t *testing.T) {
	for _, w := range []int{40, 80, 120} {
		budget := bodyWidthFor(w)
		for _, tc := range toolBodyCases {
			t.Run(tc.name+"/w"+itoa(w), func(t *testing.T) {
				got := renderToolBody(tc.tool, tc.args, tc.result, budget)
				if got == "" {
					t.Skip("renderer declined this input")
				}
				if widest := widestColumn(got); widest > budget {
					t.Errorf("body overflows its budget: widest %d > %d\n%s",
						widest, budget, firstOverflowingLine(got, budget))
				}
			})
		}
	}
}

// TestToolPanelsShareGeometry verifies the renderers that draw a filled panel
// produce panels of identical width. They are stacked in a single transcript,
// so a one-column difference between them is visible as a ragged right edge
// even though each is individually within budget.
func TestToolPanelsShareGeometry(t *testing.T) {
	const w = 80
	budget := bodyWidthFor(w)

	panels := map[string]string{
		"bash":     renderToolBody("bash", `{"command":"ls"}`, strings.Repeat("x", 200), budget),
		"ls":       renderToolBody("ls", `{"path":"/tmp"}`, strings.Repeat("x", 200), budget),
		"grep":     renderToolBody("grep", `{"pattern":"x"}`, strings.Repeat("x", 200), budget),
		"subagent": renderToolBody("subagent", `{"agent":"a"}`, strings.Repeat("x", 200), budget),
	}

	widths := map[string]int{}
	for name, rendered := range panels {
		if rendered == "" {
			t.Fatalf("%s produced no body", name)
		}
		widths[name] = widestColumn(rendered)
	}

	var first string
	for name, got := range widths {
		if first == "" {
			first = name
			continue
		}
		if got != widths[first] {
			t.Errorf("panel widths differ: %s=%d, %s=%d", first, widths[first], name, got)
		}
	}
}

// TestBashStripsOutputTags verifies the tagged stdout/stderr form is unwrapped
// rather than shown as literal markup.
//
// Kit's builtin bash tool emits the STDERR:/Exit code: form, not this one, so
// nothing in-tree produces it — but the error path already stripped these tags
// via parseBashOutput, which meant a third-party MCP tool named "bash" would
// have its markup hidden when it failed and shown raw when it succeeded.
func TestBashStripsOutputTags(t *testing.T) {
	got := stripBlockAnsi(renderToolBody("bash", `{"command":"x"}`,
		"<stdout>\nhello\n</stdout>\n<stderr>\nbad\n</stderr>", 70))

	for _, tag := range []string{"<stdout>", "</stdout>", "<stderr>", "</stderr>"} {
		if strings.Contains(got, tag) {
			t.Errorf("rendered body still contains %s:\n%s", tag, got)
		}
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("stdout content was dropped:\n%s", got)
	}
}

// TestBashReportsExitCodeInCaption verifies the exit-code line is lifted out
// of the output and into the caption rather than rendered as a body line.
func TestBashReportsExitCodeInCaption(t *testing.T) {
	got := stripBlockAnsi(renderToolBody("bash", `{"command":"false"}`,
		"some output\nExit code: 3", 70))

	if strings.Contains(got, "Exit code: 3") {
		t.Errorf("raw exit-code line leaked into the body:\n%s", got)
	}
	if !strings.Contains(got, "exit code 3") {
		t.Errorf("caption is missing the exit code:\n%s", got)
	}
}

// TestToolBodySurvivesNarrowWidth guards the clamp. renderBashBody used to
// compute a clamped width and then render with the unclamped one, which goes
// negative below the indent and makes lipgloss emit nothing at all.
func TestToolBodySurvivesNarrowWidth(t *testing.T) {
	for _, w := range []int{0, 1, 2, 5, 10} {
		for _, tc := range toolBodyCases {
			t.Run(tc.name+"/w"+itoa(w), func(t *testing.T) {
				// Must not panic, and must not silently render nothing when
				// there was content to show.
				got := renderToolBody(tc.tool, tc.args, tc.result, w)
				if strings.TrimSpace(stripBlockAnsi(got)) == "" && got != "" {
					t.Errorf("rendered only whitespace at width %d", w)
				}
			})
		}
	}
}

// diffArgs builds edit-tool arguments for a single before/after replacement.
// The payload is marshalled rather than concatenated: the text under test
// contains tabs and quotes, which are not legal raw inside a JSON string, and
// hand-escaping them silently produced arguments the renderer rejected.
func diffArgs(t *testing.T, before, after string) string {
	t.Helper()
	payload := map[string]any{
		"path": "a.go",
		"edits": []map[string]string{
			{"old_text": before, "new_text": after},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshalling edit args: %v", err)
	}
	return string(encoded)
}

// TestDiffFallsBackToUnified verifies the side-by-side layout gives way to a
// single column when there is not room for two.
//
// The panels have minimum widths, so forcing them into a narrow terminal
// produced rows wider than the screen — 43 columns inside a 40-column
// terminal. That wraps in the emulator rather than in lipgloss, which is
// exactly the class of bug the block contract exists to prevent.
func TestDiffFallsBackToUnified(t *testing.T) {
	args := diffArgs(t,
		"func hello() string {\n\treturn \"old value\"\n}",
		"func hello() string {\n\treturn \"new value\"\n}",
	)

	t.Run("narrow renders one column", func(t *testing.T) {
		got := stripBlockAnsi(renderToolBody("edit", args, "edited", bodyWidthFor(40)))
		if got == "" {
			t.Fatal("diff produced no body")
		}
		if strings.Contains(got, "│") {
			t.Errorf("expected a unified diff at 40 columns, got side-by-side:\n%s", got)
		}
		// Both sides of the change must still be present.
		if !strings.Contains(got, "old value") || !strings.Contains(got, "new value") {
			t.Errorf("unified diff dropped one side of the change:\n%s", got)
		}
	})

	t.Run("wide keeps side by side", func(t *testing.T) {
		got := stripBlockAnsi(renderToolBody("edit", args, "edited", bodyWidthFor(100)))
		if !strings.Contains(got, "│") {
			t.Errorf("expected side-by-side at 100 columns, got:\n%s", got)
		}
	})
}
