package ui

import (
	"testing"

	"charm.land/lipgloss/v2"
)

// TestFillWidgetActivityAndRestore checks activity rows and transcript anchoring.
func TestFillWidgetActivityAndRestore(t *testing.T) {
	for _, nav := range []bool{false, true} {
		m, _, _ := newTestAppModel(nil)
		m.input = NewInputComponent(80, nil)
		m.state = stateWorking
		m.distributeHeight()
		free := m.scrollList.height
		if m.chrome.activity == "" {
			t.Fatal("missing activity row")
		}
		m.scrollList.SetItems(makeItems(30, 3))
		if nav {
			m.state = stateMessageNav
			m.navReturnState = stateWorking
			m.scrollList.SetSelectedIndex(4)
			m.scrollList.autoScroll = false
		}
		m.getWidgets = func(slot string) []WidgetData {
			if slot == "below" {
				return []WidgetData{{ID: "fill", FillHeight: true, Text: "fallback"}}
			}
			return nil
		}
		m.distributeHeight()
		if m.scrollList.height != 0 || lipgloss.Height(m.chrome.below) != free {
			t.Fatal("fill did not account for activity")
		}
		if m.scrollList.View() != "" {
			t.Fatal("hidden transcript rendered")
		}
		if got := m.transcriptPlacements(nil, -1); len(got) != 0 {
			t.Fatal("hidden transcript placed graphics")
		}
		m.getWidgets = nil
		m.distributeHeight()
		if m.scrollList.height != free {
			t.Fatal("transcript height not restored")
		}
		if nav {
			if m.scrollList.offsetIdx > 4 {
				t.Fatal("selected message not visible")
			}
		} else {
			idx, line := m.scrollList.bottomOffset()
			if m.scrollList.offsetIdx != idx || m.scrollList.offsetLine != line {
				t.Fatal("auto-scroll bottom not restored")
			}
		}
	}
}

// TestSizedWidgetPrecedenceAndAnimation checks sized callbacks and clock demand.
func TestSizedWidgetPrecedenceAndAnimation(t *testing.T) {
	m := newTestModel()
	w := WidgetData{FillHeight: true, RefreshHz: 10, Render: func(int) string { return "natural" }, RenderSized: func(w, h int) string { return "sized" }}
	m.getWidgets = func(string) []WidgetData { return []WidgetData{w} }
	if !m.animatedWidgets() {
		t.Fatal("sized renderer must keep frame clock running")
	}
	got, _ := m.resolveWidgetContentSized(w, "k", 80, 2)
	if got != "sized\n " {
		t.Fatalf("sized output=%q", got)
	}
	got, _ = m.resolveWidgetContent(w, "header", 80)
	if got != "natural" {
		t.Fatalf("header must ignore fill fields: %q", got)
	}
}
