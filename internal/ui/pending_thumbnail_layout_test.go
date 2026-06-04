package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uicore "github.com/mark3labs/kit/internal/ui/core"
)

// drainCmds runs a tea.Cmd chain back through m.Update like the BubbleTea
// event loop, expanding batches, until no further messages are produced.
func drainCmds(t *testing.T, m *AppModel, cmd tea.Cmd) *AppModel {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for i := 0; i < 50 && len(queue) > 0; i++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		updated, nc := m.Update(msg)
		m = updated.(*AppModel)
		_ = m.View()
		if nc != nil {
			queue = append(queue, nc)
		}
	}
	return m
}

func measuredInputHeight(m *AppModel) int {
	rendered := m.renderInput()
	if rendered == "" {
		return 0
	}
	return strings.Count(rendered, "\n") + 1
}

// TestPendingThumbnailTriggersLayoutRecompute is a regression test for the bug
// where a pasted image's async half-block preview rendered but was clipped off
// the bottom of the screen: the thumbnail arrives via thumbnailReadyMsg after
// distributeHeight already measured the input region without it. The parent
// must mark the layout dirty so the (now taller) input is re-measured.
func TestPendingThumbnailTriggersLayoutRecompute(t *testing.T) {
	real := NewInputComponent(80, nil)
	m, _, _ := newTestAppModel(nil)
	m.input = real
	m = sendMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	heightBefore := measuredInputHeight(m)

	updated, cmd := m.Update(clipboardImageMsg{image: &uicore.ImageAttachment{
		Data:      makeTestPNG(t, 16, 16),
		MediaType: "image/png",
	}})
	m = updated.(*AppModel)
	_ = m.View()
	m = drainCmds(t, m, cmd)

	heightAfter := measuredInputHeight(m)
	if heightAfter <= heightBefore {
		t.Errorf("input region should grow to fit the thumbnail (before=%d after=%d)", heightBefore, heightAfter)
	}

	if !strings.Contains(m.View().Content, "▀") {
		t.Error("parent View should contain the half-block thumbnail (was clipped or not rendered)")
	}
}
