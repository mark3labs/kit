package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	kit "github.com/mark3labs/kit/pkg/kit"
)

func TestSubagentStatusBarStatuses(t *testing.T) {
	for _, tc := range []struct{ status, glyph string }{
		{"starting", "●"}, {"running", "●"}, {"completed", "✓"},
		{"failed", "✗"}, {"timed_out", "✗"}, {"stopped", "■"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			c := &inspectorTestController{runs: []kit.SubagentRun{{Agent: "explore", Status: tc.status, FinishedAt: time.Now()}}}
			m := &AppModel{appCtrl: c, width: 80}
			row := m.renderSubagentStatusBar()
			if !strings.Contains(row, tc.glyph) || !strings.Contains(row, "explore") {
				t.Fatalf("unexpected row: %q", row)
			}
			for width := 1; width < 40; width++ {
				m.width = width
				if got := lipgloss.Width(m.renderSubagentStatusBar()); got > width {
					t.Fatalf("width %d exceeds %d", got, width)
				}
			}
		})
	}
}

func TestSubagentStatusBarBlinksWithoutMovingLabels(t *testing.T) {
	c := &inspectorTestController{runs: []kit.SubagentRun{{Agent: "explore", Status: "running"}}}
	m := &AppModel{appCtrl: c, width: 80}
	on := m.renderSubagentStatusBar()
	m.frames.frame = FrameClockFPS / 2
	off := m.renderSubagentStatusBar()
	if strings.Contains(off, "●") || !strings.Contains(on, "●") {
		t.Fatal("active light did not blink")
	}
	if lipgloss.Width(on) != lipgloss.Width(off) {
		t.Fatal("blink changed row width")
	}
	m.frames.frame = FrameClockFPS
	if m.renderSubagentStatusBar() != on {
		t.Fatal("light did not return to its on state")
	}
}

func TestSubagentStatusBarExpiresAndReservesLine(t *testing.T) {
	c := &inspectorTestController{}
	m, _, _ := newTestAppModel(c)
	m = sendMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30})
	idleHeight := m.scrollList.height
	c.runs = []kit.SubagentRun{{Status: "running"}}
	m.subagentStatusCheckedAt = time.Time{}
	m.syncSubagentStatusBar()
	m.distributeHeight()
	if m.scrollList.height != idleHeight-1 {
		t.Fatalf("height=%d, want %d", m.scrollList.height, idleHeight-1)
	}
	if !strings.Contains(m.renderSubagentStatusBar(), "agent") {
		t.Fatal("missing fallback name")
	}
	m.state = stateWorking
	m.distributeHeight()
	if m.scrollList.height != idleHeight-2 {
		t.Fatalf("height=%d, want %d", m.scrollList.height, idleHeight-2)
	}
	c.runs = []kit.SubagentRun{{Status: "completed", FinishedAt: time.Now().Add(-11 * time.Second)}}
	m.subagentStatusCheckedAt = time.Time{}
	if row := m.renderSubagentStatusBar(); row != "" {
		t.Fatalf("expired row: %q", row)
	}
	m.layoutDirty = false
	m.syncSubagentStatusBar()
	if !m.layoutDirty {
		t.Fatal("row disappearance did not update layout")
	}
}
