package ui

import (
	"fmt"
	"strings"
	"testing"
)

// numberedResult returns a tool result of n lines, "line01" to "lineNN".
func numberedResult(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line%02d\n", i)
	}
	return b.String()
}

// shownLines counts how many lines of a numberedResult(n) a rendered tool
// block shows.
func shownLines(rendered string, n int) int {
	plain := stripBlockAnsi(rendered)
	shown := 0
	for i := 1; i <= n; i++ {
		if strings.Contains(plain, fmt.Sprintf("line%02d", i)) {
			shown++
		}
	}
	return shown
}

// limitRenderer returns a tool renderer lookup that sets MaxBodyLines only.
func limitRenderer(lines int) func(string) *ToolRendererData {
	return func(string) *ToolRendererData { return &ToolRendererData{MaxBodyLines: lines} }
}

// TestMaxBodyLinesLimitsTheBuiltinBody checks that MaxBodyLines replaces the
// line limit of a builtin body renderer.
func TestMaxBodyLinesLimitsTheBuiltinBody(t *testing.T) {
	r := newMessageRenderer(100, false)
	args := `{"command":"seq 40"}`

	if got := shownLines(r.RenderToolMessage("shell", args, numberedResult(40), false).Content, 40); got != maxBashLines {
		t.Fatalf("without a renderer the shell body shows %d lines, want %d", got, maxBashLines)
	}
	r.getToolRenderer = limitRenderer(6)
	if got := shownLines(r.RenderToolMessage("shell", args, numberedResult(40), false).Content, 40); got != 6 {
		t.Errorf("with MaxBodyLines 6 the shell body shows %d lines", got)
	}
}

// TestMaxBodyLinesLimitsThePlainBody checks MaxBodyLines for the plain
// rendering, which shows 10 lines. A tool without a builtin renderer, such as
// an MCP tool, and every error result get this rendering.
func TestMaxBodyLinesLimitsThePlainBody(t *testing.T) {
	for _, isError := range []bool{false, true} {
		r := newMessageRenderer(100, false)
		if got := shownLines(r.RenderToolMessage("srv__query", "{}", numberedResult(40), isError).Content, 40); got != 10 {
			t.Fatalf("error=%v: without a renderer the plain body shows %d lines, want 10", isError, got)
		}
		r.getToolRenderer = limitRenderer(6)
		if got := shownLines(r.RenderToolMessage("srv__query", "{}", numberedResult(40), isError).Content, 40); got != 6 {
			t.Errorf("error=%v: with MaxBodyLines 6 the plain body shows %d lines", isError, got)
		}
	}
}

// TestMaxBodyLinesLeavesRenderBodyAlone checks that MaxBodyLines applies only
// to a body that Kit renders itself, not to the result of RenderBody.
func TestMaxBodyLinesLeavesRenderBodyAlone(t *testing.T) {
	r := newMessageRenderer(100, false)
	r.getToolRenderer = func(string) *ToolRendererData {
		return &ToolRendererData{
			MaxBodyLines: 2,
			RenderBody:   func(result string, isError bool, width int) string { return result },
		}
	}
	if got := shownLines(r.RenderToolMessage("srv__query", "{}", numberedResult(5), false).Content, 5); got != 5 {
		t.Errorf("a RenderBody result shows %d of 5 lines, want all", got)
	}
}
