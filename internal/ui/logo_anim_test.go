package ui

import (
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/ui/style"
)

// splashOf returns the rendered splash currently held in the scrollback.
func splashOf(t *testing.T, m *AppModel) string {
	t.Helper()
	if len(m.messages) == 0 {
		t.Fatal("no splash in the scrollback")
	}
	return m.messages[0].Render(m.width)
}

// The splash must be rendered from the animation's first frame, not from the
// resting logo, or the animation would open with a visible jump.
func TestSplashStartsAtInitialFrame(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.AddStartupMessageToScrollList()

	if m.splashItem == nil {
		t.Fatal("splash item was not retained for animation")
	}
	if m.logoFrame != m.initialLogoFrame() {
		t.Errorf("splash rendered at frame %d, want %d", m.logoFrame, m.initialLogoFrame())
	}

	want := m.renderSplash(m.initialLogoFrame())
	if got := splashOf(t, m); got != want {
		t.Error("splash content does not match its initial frame")
	}
}

// Ticks advance the animation in place: the item keeps its ID and its height,
// because ScrollList caches height by ID and would otherwise be left stale.
func TestLogoAnimationAdvancesInPlace(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.AddStartupMessageToScrollList()

	// Drive the animation directly, independent of whether the test host's
	// terminal profile would have started it.
	m.logoAnimGeneration++
	gen := m.logoAnimGeneration
	m.logoFrame = 0

	id := m.messages[0].ID()
	height := strings.Count(splashOf(t, m), "\n")

	for range 5 {
		if cmd := m.advanceLogoAnimation(gen); cmd == nil {
			t.Fatal("animation stopped early")
		}
	}

	if m.logoFrame != 5 {
		t.Errorf("frame is %d after 5 ticks, want 5", m.logoFrame)
	}
	if got := m.messages[0].ID(); got != id {
		t.Error("animation replaced the splash item instead of repainting it")
	}
	if got := strings.Count(splashOf(t, m), "\n"); got != height {
		t.Errorf("splash height changed mid-animation: %d, want %d", got, height)
	}
	if len(m.messages) != 1 {
		t.Errorf("animation appended %d extra items", len(m.messages)-1)
	}
}

// The animation must stop on its own and leave the resting logo behind.
func TestLogoAnimationSettles(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.AddStartupMessageToScrollList()

	m.logoAnimGeneration++
	gen := m.logoAnimGeneration
	m.logoFrame = 0

	ticks := 0
	for m.advanceLogoAnimation(gen) != nil {
		ticks++
		if ticks > style.KitLogoAnimationFrames+10 {
			t.Fatal("animation never stopped")
		}
	}

	if m.splashItem != nil {
		t.Error("splash item should be released once the animation settles")
	}
	// A late tick from the same run must be harmless now.
	if cmd := m.advanceLogoAnimation(gen); cmd != nil {
		t.Error("a tick after settling should not reschedule")
	}

	rest := m.renderSplash(style.KitLogoAnimationFrames)
	if got := splashOf(t, m); got != rest {
		t.Error("the settled splash is not the resting logo")
	}
}

// A tick from a superseded run must be dropped, or two tick loops would run
// concurrently and the animation would play at double speed.
func TestLogoAnimationIgnoresStaleTicks(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.AddStartupMessageToScrollList()

	m.logoAnimGeneration++
	stale := m.logoAnimGeneration
	m.logoFrame = 0

	// A newer run supersedes the old one.
	m.logoAnimGeneration++

	if cmd := m.advanceLogoAnimation(stale); cmd != nil {
		t.Error("a stale tick should not reschedule")
	}
	if m.logoFrame != 0 {
		t.Errorf("a stale tick advanced the animation to frame %d", m.logoFrame)
	}
}

// The tick is handled ahead of the modal routing, so a prompt or overlay
// opening mid-animation cannot swallow it and strand the logo mid-sweep.
func TestLogoTickSurvivesModalState(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.AddStartupMessageToScrollList()

	m.logoAnimGeneration++
	gen := m.logoAnimGeneration
	m.logoFrame = 0

	// stateOverlay would return early from Update if the tick were handled
	// in the main switch.
	m.state = stateOverlay

	updated, cmd := m.Update(logoAnimTickMsg{generation: gen})
	m = updated.(*AppModel)

	if cmd == nil {
		t.Fatal("tick was swallowed by the modal routing")
	}
	if m.logoFrame != 1 {
		t.Errorf("frame is %d, want 1", m.logoFrame)
	}
}

// Extensions can suppress the splash entirely; nothing should be scheduled.
func TestNoAnimationWhenStartupMessageHidden(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.getUIVisibility = func() *UIVisibility {
		return &UIVisibility{HideStartupMessage: true}
	}

	m.AddStartupMessageToScrollList()

	if len(m.messages) != 0 {
		t.Fatalf("expected no splash, got %d messages", len(m.messages))
	}
	if m.splashItem != nil {
		t.Error("no splash item should be retained")
	}
	if cmd := m.startLogoAnimation(); cmd != nil {
		t.Error("no animation should be scheduled without a splash")
	}
}

// Below the width the block art needs, the logo degrades to a plain "KIT" and
// there is nothing to animate.
func TestNoAnimationWhenTooNarrow(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.width = 10

	m.AddStartupMessageToScrollList()

	if m.logoAnimationSupported() {
		t.Error("animation should be disabled at a width the logo cannot fit")
	}
	if m.initialLogoFrame() != style.KitLogoAnimationFrames {
		t.Error("a non-animating splash should render at the resting frame")
	}
	if cmd := m.startLogoAnimation(); cmd != nil {
		t.Error("no animation should be scheduled when the logo does not fit")
	}
}
