package ui

import (
	"fmt"
	"strings"
	"testing"
)

// TestResolveWidgetContentRenderWins verifies that a Render callback takes
// precedence over Text and that its output is used verbatim (no wrapping,
// no re-styling) — this is what makes arbitrary extension UI possible.
func TestResolveWidgetContentRenderWins(t *testing.T) {
	raw := "\x1b[1;35m▐ custom ▌\x1b[0m"
	w := WidgetData{
		Text:   "should be ignored",
		Render: func(width int) string { return raw },
	}

	got, ok := newTestModel().resolveWidgetContent(w, "k", 80)
	if !ok {
		t.Fatal("expected content")
	}
	if got != raw {
		t.Errorf("Render output must be verbatim.\n got: %q\nwant: %q", got, raw)
	}
}

// TestResolveWidgetContentRenderWidth verifies the width handed to Render is
// the real content column, not the container width.
func TestResolveWidgetContentRenderWidth(t *testing.T) {
	for _, tc := range []struct {
		name      string
		noBorder  bool
		container int
	}{
		{"bordered", false, 80},
		{"borderless", true, 80},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var seen int
			w := WidgetData{
				NoBorder: tc.noBorder,
				Render: func(width int) string {
					seen = width
					return "x"
				},
			}
			if _, ok := newTestModel().resolveWidgetContent(w, "k", tc.container); !ok {
				t.Fatal("expected content")
			}
			want := widgetContentWidth(tc.container, tc.noBorder)
			if seen != want {
				t.Errorf("Render got width %d, want %d", seen, want)
			}
			if seen >= tc.container {
				t.Errorf("width %d must be less than container %d", seen, tc.container)
			}
		})
	}
}

// TestResolveWidgetContentEmptyHides verifies an empty Render result hides the
// widget rather than drawing an empty bordered block.
func TestResolveWidgetContentEmptyHides(t *testing.T) {
	w := WidgetData{Render: func(int) string { return "" }}
	if _, ok := newTestModel().resolveWidgetContent(w, "k", 80); ok {
		t.Error("empty Render output should hide the widget")
	}

	if _, ok := newTestModel().resolveWidgetContent(WidgetData{}, "k", 80); ok {
		t.Error("empty Text should hide the widget")
	}
}

// TestResolveWidgetContentPanicIsContained verifies a panicking extension
// widget degrades to hidden instead of taking down the render loop.
func TestResolveWidgetContentPanicIsContained(t *testing.T) {
	w := WidgetData{Render: func(int) string { panic("extension bug") }}

	got, ok := newTestModel().resolveWidgetContent(w, "k", 80)
	if ok || got != "" {
		t.Errorf("panicking Render should hide the widget, got %q ok=%v", got, ok)
	}
}

// TestResolveWidgetContentTinyWidth verifies Render is skipped when there is
// no usable column to draw into.
func TestResolveWidgetContentTinyWidth(t *testing.T) {
	called := false
	w := WidgetData{Render: func(int) string { called = true; return "x" }}

	if _, ok := newTestModel().resolveWidgetContent(w, "k", 2); ok {
		t.Error("expected widget to be hidden at tiny width")
	}
	if called {
		t.Error("Render should not be called when width is unusable")
	}
}

// TestResolveWidgetContentMarkdown verifies the Markdown path still styles
// Text, and that Render bypasses markdown entirely.
func TestResolveWidgetContentMarkdown(t *testing.T) {
	md, ok := newTestModel().resolveWidgetContent(WidgetData{Text: "# Head\n\n**bold**", Markdown: true}, "k", 80)
	if !ok {
		t.Fatal("expected content")
	}
	if !strings.Contains(md, "\x1b[") {
		t.Errorf("markdown should be ANSI-styled, got %q", md)
	}
	if strings.Contains(md, "**") {
		t.Errorf("bold markers should be consumed, got %q", md)
	}

	plain, ok := newTestModel().resolveWidgetContent(WidgetData{Text: "# Head", Markdown: false}, "k", 80)
	if !ok || plain != "# Head" {
		t.Errorf("non-markdown Text should pass through, got %q", plain)
	}

	// Markdown must be ignored when Render is set.
	both, ok := newTestModel().resolveWidgetContent(WidgetData{
		Text:     "# Head",
		Markdown: true,
		Render:   func(int) string { return "**literal**" },
	}, "k", 80)
	if !ok || both != "**literal**" {
		t.Errorf("Render must bypass markdown, got %q", both)
	}
}

// newTestModel returns a minimal AppModel usable for widget-resolution tests.
func newTestModel() *AppModel { return &AppModel{} }

// TestWidgetFrameDivisor checks the mapping from a requested rate to a
// subdivision of the fixed shared clock.
func TestWidgetFrameDivisor(t *testing.T) {
	for _, tc := range []struct{ hz, want int }{
		{0, 1}, {-5, 1}, {30, 1}, {60, 1},
		{15, 2}, {10, 3}, {6, 5}, {5, 6}, {1, 30},
	} {
		if got := widgetFrameDivisor(tc.hz); got != tc.want {
			t.Errorf("widgetFrameDivisor(%d) = %d, want %d", tc.hz, got, tc.want)
		}
	}
}

// TestAnimatedWidgetRateLimited verifies an animated widget's Render is
// entered at its requested subdivision of the clock, not on every frame.
func TestAnimatedWidgetRateLimited(t *testing.T) {
	m := newTestModel()
	calls := 0
	w := WidgetData{
		RefreshHz: 10, // divisor 3 at 30fps
		Render:    func(int) string { calls++; return fmt.Sprintf("f%d", calls) },
	}

	// Simulate 30 clock frames, resolving twice per frame the way
	// distributeHeight + View do.
	for f := range 30 {
		m.frames.frame = f
		m.renderEpoch++
		m.resolveWidgetContent(w, "w", 80)
		m.resolveWidgetContent(w, "w", 80)
	}

	// 30 frames / divisor 3 = 10 distinct beats.
	if calls != 10 {
		t.Errorf("Render called %d times over 30 frames at 10Hz, want 10", calls)
	}
}

// TestStaticWidgetConsistentWithinFrame verifies a time-varying static widget
// returns identical content to the measure and paint passes of one frame,
// while still updating on the next frame.
func TestStaticWidgetConsistentWithinFrame(t *testing.T) {
	m := newTestModel()
	calls := 0
	w := WidgetData{Render: func(int) string { calls++; return fmt.Sprintf("v%d", calls) }}

	m.renderEpoch++
	measure, _ := m.resolveWidgetContent(w, "w", 80)
	paint, _ := m.resolveWidgetContent(w, "w", 80)
	if measure != paint {
		t.Errorf("measure %q != paint %q within one frame", measure, paint)
	}
	if calls != 1 {
		t.Errorf("Render called %d times in one frame, want 1", calls)
	}

	m.renderEpoch++
	next, _ := m.resolveWidgetContent(w, "w", 80)
	if next == paint {
		t.Errorf("static widget did not refresh on the next frame: %q", next)
	}
}

// TestWidgetCacheInvalidatedByResize verifies a width change re-renders even
// when the widget is not otherwise due.
func TestWidgetCacheInvalidatedByResize(t *testing.T) {
	m := newTestModel()
	var widths []int
	w := WidgetData{
		RefreshHz: 1, // divisor 30 — would otherwise not be due again
		Render:    func(width int) string { widths = append(widths, width); return "x" },
	}

	m.resolveWidgetContent(w, "w", 80)
	m.resolveWidgetContent(w, "w", 80) // cached
	m.resolveWidgetContent(w, "w", 120)

	if len(widths) != 2 {
		t.Fatalf("Render called %d times, want 2 (initial + resize)", len(widths))
	}
	if widths[0] == widths[1] {
		t.Errorf("expected distinct widths, got %v", widths)
	}
}

// TestAnimatedWidgetsDetection verifies only an explicit RefreshHz opt-in
// holds the shared animation clock open.
func TestAnimatedWidgetsDetection(t *testing.T) {
	render := func(int) string { return "x" }

	static := &AppModel{getWidgets: func(string) []WidgetData {
		return []WidgetData{{Render: render}}
	}}
	if static.animatedWidgets() {
		t.Error("a widget without RefreshHz must not hold the clock open")
	}

	animated := &AppModel{getWidgets: func(placement string) []WidgetData {
		if placement == "above" {
			return []WidgetData{{Render: render, RefreshHz: 8}}
		}
		return nil
	}}
	if !animated.animatedWidgets() {
		t.Error("a widget with RefreshHz must hold the clock open")
	}

	footer := &AppModel{getFooter: func() *WidgetData {
		return &WidgetData{Render: render, RefreshHz: 4}
	}}
	if !footer.animatedWidgets() {
		t.Error("an animated footer must hold the clock open")
	}

	if (&AppModel{}).animatedWidgets() {
		t.Error("no providers must not hold the clock open")
	}
}
