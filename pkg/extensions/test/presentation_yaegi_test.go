package test

import (
	"testing"

	"github.com/mark3labs/kit/pkg/extensions"
)

// TestPresentationExample checks what the presentation example sets up.
// Every tool gets a line limit, and a successful MCP tool result gets one
// line. The chrome is hidden, but the startup block keeps the model line.
func TestPresentationExample(t *testing.T) {
	harness := New(t)
	harness.LoadFile("../../../examples/extensions/presentation.go")
	runner := harness.Runner()

	read := runner.GetToolRenderer("read")
	if read == nil || read.MaxBodyLines != 6 || read.RenderBody != nil {
		t.Fatalf("renderer for read = %+v, want MaxBodyLines 6 and no RenderBody", read)
	}
	mcp := runner.GetToolRenderer("github__search_issues")
	if mcp == nil || mcp.MaxBodyLines != 6 || mcp.RenderBody == nil {
		t.Fatalf("renderer for an MCP tool = %+v, want MaxBodyLines 6 and a RenderBody", mcp)
	}

	if _, err := harness.Emit(extensions.SessionStartEvent{}); err != nil {
		t.Fatal(err)
	}
	vis := harness.Context().UIVisibility
	if vis == nil || !vis.HideLogo || !vis.HideStatusBar || !vis.HideSeparator || !vis.HideMCPStartupMessages || vis.HideStartupMessage {
		t.Errorf("visibility = %+v, want the logo, status bar, separator and MCP startup messages hidden, and the startup block kept", vis)
	}

	for result, want := range map[string]string{
		"a\nb\nc\n": "(3 lines of output hidden)",
		"a":         "(1 line of output hidden)",
		" \n":       "",
	} {
		if got := mcp.RenderBody(result, false, 80); got != want {
			t.Errorf("RenderBody(%q) = %q, want %q", result, got, want)
		}
	}
	// An error falls back to Kit's rendering, so it shows the result.
	if got := mcp.RenderBody("boom", true, 80); got != "" {
		t.Errorf("RenderBody for an error = %q, want the fallback", got)
	}

	harness.Context().Options["presentation-mcp-output"] = "full"
	if _, err := harness.Emit(extensions.SessionStartEvent{}); err != nil {
		t.Fatal(err)
	}
	if got := mcp.RenderBody("a\nb\n", false, 80); got != "" {
		t.Errorf("RenderBody with the option set to full = %q, want the fallback", got)
	}
}
