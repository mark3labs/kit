package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestRenderToolMessage_ClipsSingleHugeLine guards against MCP tools that
// return minified JSON on one line. The fallback renderer capped only the
// line count, so a 600 KB single-line result was kept whole in the
// transcript and made every frame slow.
func TestRenderToolMessage_ClipsSingleHugeLine(t *testing.T) {
	const width = 100
	huge := `{"connections":[` + strings.Repeat(`{"address":"0xAd552A648C74D49E10027AB8a618A3ad4901c5bE","chainId":14},`, 10000) + `]}`

	r := newMessageRenderer(width, false)
	msg := r.RenderToolMessage("lifi__get-connections", `{}`, huge, false)

	if len(msg.Content) > 10*1024 {
		t.Fatalf("rendered content is %d bytes; want the huge line clipped", len(msg.Content))
	}
	for i, line := range strings.Split(msg.Content, "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("line %d is %d cells wide; want <= %d", i, w, width)
		}
	}
}
