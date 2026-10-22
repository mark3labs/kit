package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// TestFillWidgetLayout checks row allocation, frame bounds and hidden mouse input.
func TestFillWidgetLayout(t *testing.T) {
	for _, chrome := range []bool{false, true} {
		for _, output := range []string{"", "short", strings.Repeat("long\n", 100)} {
			t.Run(fmt.Sprintf("chrome=%v/output=%d", chrome, len(output)), func(t *testing.T) {
				m, _, _ := newTestAppModel(nil)
				m.input = NewInputComponent(80, nil)
				if chrome {
					m.getHeader = func() *WidgetData { return &WidgetData{Text: "header"} }
					m.getFooter = func() *WidgetData { return &WidgetData{Text: "footer"} }
					m.queuedMessages = []string{"queued message"}
					m.ctrlCPressedOnce = true
					m.input.(*InputComponent).textarea.SetValue("one\ntwo\nthree")
				}
				m.distributeHeight()
				want := m.scrollList.height
				seen, calls := 0, 0
				m.getWidgets = func(slot string) []WidgetData {
					if slot != "above" {
						return nil
					}
					return []WidgetData{{ID: "fill", FillHeight: true, RenderSized: func(w, h int) string {
						seen = h
						calls++
						return output
					}}}
				}
				m.layoutDirty = true
				view := m.View().Content
				if seen != want || m.scrollList.height != 0 {
					t.Fatalf("height=%d scroll=%d, want %d/0", seen, m.scrollList.height, want)
				}
				if got := lipgloss.Height(view); got != m.height {
					t.Fatalf("frame height=%d want %d\n%s", got, m.height, view)
				}
				if !strings.Contains(view, m.chrome.input) {
					t.Fatal("input missing")
				}
				wantInputY := m.lastHeaderHeight + lipgloss.Height(m.chrome.above)
				if chrome {
					wantInputY += 1 + lipgloss.Height(m.renderSeparator()) + lipgloss.Height(m.chrome.queued)
				}
				if m.inputYOffset != wantInputY {
					t.Fatalf("input Y=%d want %d", m.inputYOffset, wantInputY)
				}
				if got := lipgloss.Height(m.chrome.above); got != want {
					t.Fatalf("widget height=%d want %d", got, want)
				}
				before := calls
				_ = m.View()
				if calls != before+1 {
					t.Fatal("next frame must render fill widget with assigned height")
				}
				m = sendMsg(m, tea.MouseWheelMsg{Button: tea.MouseWheelUp})
				m = sendMsg(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: 1})
				if !m.scrollList.autoScroll || m.scrollList.IsMouseDown() {
					t.Fatal("hidden transcript handled mouse input")
				}
				m.getWidgets = nil
				m.layoutDirty = true
				_ = m.View()
				if m.scrollList.height != want {
					t.Fatalf("restored height=%d want %d", m.scrollList.height, want)
				}
			})
		}
	}
}

// TestFillWidgetSplitAndHeightCache checks shared rows and height-only cache misses.
func TestFillWidgetSplitAndHeightCache(t *testing.T) {
	m, _, _ := newTestAppModel(nil)
	m.input = NewInputComponent(80, nil)
	m.distributeHeight()
	free := m.scrollList.height
	calls := 0
	sizes := map[string]int{}
	m.getWidgets = func(slot string) []WidgetData {
		return []WidgetData{{ID: slot, FillHeight: true, RefreshHz: 1, RenderSized: func(w, h int) string { calls++; sizes[slot] = h; return "x" }}}
	}
	m.distributeHeight()
	if sizes["above"] != (free+1)/2 || sizes["below"] != free/2 {
		t.Fatalf("split=%v free=%d", sizes, free)
	}
	m.distributeHeight()
	if calls != 2 {
		t.Fatalf("cache missed: calls=%d", calls)
	}
	m.height += 2
	m.distributeHeight()
	if calls != 4 {
		t.Fatalf("height-only resize reused stale cache: calls=%d", calls)
	}
	m.height = 1
	m.distributeHeight()
	if calls != 4 || m.chrome.above != "" || m.chrome.below != "" {
		t.Fatal("zero-height fill was rendered")
	}
}
