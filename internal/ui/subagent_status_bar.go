package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/ui/style"
	kit "github.com/mark3labs/kit/pkg/kit"
)

const subagentStatusRetention = 10 * time.Second

func (m *AppModel) subagentStatusRuns() []kit.SubagentRun {
	now := time.Now()
	if now.Sub(m.subagentStatusCheckedAt) >= time.Second {
		m.subagentStatusCheckedAt = now
		m.subagentStatusCache = nil
		if c, ok := m.appCtrl.(subagentInspectorController); ok {
			for _, r := range c.SubagentRuns() {
				r.Events = nil // Do not retain child transcripts in the status cache.
				m.subagentStatusCache = append(m.subagentStatusCache, r)
			}
		}
	}
	var out []kit.SubagentRun
	for _, r := range m.subagentStatusCache {
		if r.Status == "starting" || r.Status == "running" || (!r.FinishedAt.IsZero() && now.Sub(r.FinishedAt) < subagentStatusRetention) {
			out = append(out, r)
		}
	}
	return out
}
func (m *AppModel) hasVisibleSubagentRuns() bool { return len(m.subagentStatusRuns()) > 0 }

func (m *AppModel) syncSubagentStatusBar() {
	present := m.width > 0 && m.hasVisibleSubagentRuns()
	if present != m.lastSubagentStatusPresent {
		m.layoutDirty = true
		m.lastSubagentStatusPresent = present
	}
}

// renderSubagentStatusBar renders compact status for active and recent child runs.
func (m *AppModel) renderSubagentStatusBar() string {
	runs := m.subagentStatusRuns()
	if m.width <= 0 || len(runs) == 0 {
		return ""
	}
	theme := style.GetTheme()
	var labels []string
	for _, r := range runs {
		glyph, color := "✓", theme.Success
		switch r.Status {
		case "starting", "running":
			glyph, color = "●", theme.Success
			// Alternate on and off every half second without changing the row width.
			if (m.frames.frame/(FrameClockFPS/2))%2 != 0 {
				glyph = " "
			}
		case "failed", "timed_out":
			glyph, color = "✗", theme.Error
		case "stopped":
			glyph, color = "■", theme.Muted
		}
		name := strings.TrimSpace(r.Agent)
		if name == "" {
			name = "agent"
		}
		name = strings.Join(strings.Fields(name), " ")
		labels = append(labels, lipgloss.NewStyle().Foreground(color).Render(glyph)+" "+name)
	}
	line := " Subagents: " + strings.Join(labels, "  ")
	hint := "ctrl+alt+a runs"
	if lipgloss.Width(line)+lipgloss.Width(hint)+2 <= m.width {
		line += strings.Repeat(" ", m.width-lipgloss.Width(line)-lipgloss.Width(hint)) + hint
	} else {
		line = truncateLine(line, m.width)
	}
	return line
}
