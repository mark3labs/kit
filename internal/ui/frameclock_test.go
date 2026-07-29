package ui

import (
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/ui/style"
)

// A stopped clock is the zero value, so an AppModel with nothing animating
// costs nothing without anyone having to initialise anything.
func TestFrameClockZeroValueIsStopped(t *testing.T) {
	var c frameClock
	if c.running {
		t.Error("the zero value should be a stopped clock")
	}
	if c.accept(frameTickMsg{frame: 1}) {
		t.Error("a stopped clock should not accept beats")
	}
}

// start must be idempotent. It is called after every message, so a second call
// while running has to be free — otherwise the app would accumulate a new tick
// loop per keystroke and the animation would accelerate without bound.
func TestFrameClockStartIsIdempotent(t *testing.T) {
	var c frameClock

	first := c.start()
	if first == nil {
		t.Fatal("starting a stopped clock should schedule a beat")
	}
	gen := c.generation

	for range 10 {
		if cmd := c.start(); cmd != nil {
			t.Fatal("starting a running clock should not schedule a second loop")
		}
	}
	if c.generation != gen {
		t.Errorf("redundant starts disturbed the generation: %d, want %d", c.generation, gen)
	}
}

// A beat scheduled before a stop must not be honoured after a restart. This is
// the guard that used to be reimplemented per animation; it lives here now, so
// every animation inherits it and none can get it wrong.
func TestFrameClockRejectsSupersededBeats(t *testing.T) {
	var c frameClock

	c.start()
	stale := frameTickMsg{generation: c.generation, frame: c.frame + 1}

	c.stop()
	if c.accept(stale) {
		t.Error("a stopped clock accepted a beat")
	}

	c.start()
	if c.accept(stale) {
		t.Error("a restarted clock accepted a beat from the previous run")
	}
	if c.frame != 0 {
		t.Errorf("a rejected beat advanced the frame counter to %d", c.frame)
	}

	live := frameTickMsg{generation: c.generation, frame: c.frame + 1}
	if !c.accept(live) {
		t.Error("the clock rejected a beat from its own run")
	}
	if c.frame != 1 {
		t.Errorf("frame is %d after one accepted beat, want 1", c.frame)
	}
}

// Frame numbers are assigned when a beat is scheduled, so the sequence stays
// monotonic regardless of how long the receiving update takes.
func TestFrameClockFrameNumbersAreMonotonic(t *testing.T) {
	var c frameClock
	c.start()

	for want := 1; want <= 5; want++ {
		if !c.accept(frameTickMsg{generation: c.generation, frame: c.frame + 1}) {
			t.Fatalf("beat %d was rejected", want)
		}
		if c.frame != want {
			t.Fatalf("frame is %d, want %d", c.frame, want)
		}
	}

	// Restarting resets the phase, so a subdivided animation always begins on
	// a predictable beat rather than wherever the previous run happened to
	// leave the counter.
	c.stop()
	c.start()
	if c.frame != 0 {
		t.Errorf("frame is %d after a restart, want 0", c.frame)
	}
}

// due is how an animation asks for a rate slower than the clock's. Getting it
// wrong is silent — the animation just runs at the wrong speed — so the
// arithmetic is pinned here.
func TestFrameTickDue(t *testing.T) {
	// A divisor of 1 or less means every beat.
	for _, every := range []int{0, 1} {
		for frame := range 5 {
			if !(frameTickMsg{frame: frame}).due(every) {
				t.Errorf("due(%d) was false on frame %d; every beat was expected", every, frame)
			}
		}
	}

	// The spinner's subdivision: every second beat, starting on the second.
	got := 0
	for frame := 1; frame <= FrameClockFPS; frame++ {
		if (frameTickMsg{frame: frame}).due(frameEverySpinner) {
			got++
		}
	}
	if want := FrameClockFPS / frameEverySpinner; got != want {
		t.Errorf("spinner was due %d times in a clock second, want %d", got, want)
	}
}

// The logo's frame counts are durations only once multiplied by the clock's
// rate, so the two are coupled. Retuning the clock without revisiting the
// animation would silently change how long the flourish lasts — at 10fps the
// splash would sit under a sweep for five seconds — so the resulting duration
// is pinned here rather than the rate, which is the thing anyone actually
// cares about.
func TestLogoAnimationDurationStaysAFlourish(t *testing.T) {
	if frameEveryLogo != 1 {
		t.Fatalf("the logo is expected to run at the clock's full rate, not every %d frames", frameEveryLogo)
	}

	d := time.Duration(style.KitLogoAnimationFrames) * (time.Second / FrameClockFPS)

	// Long enough to read as a deliberate gesture rather than a flicker, short
	// enough that it is over before anyone could want to skip it.
	const minDuration = 750 * time.Millisecond
	const maxDuration = 2500 * time.Millisecond

	if d < minDuration || d > maxDuration {
		t.Errorf("the startup animation runs for %v at %dfps, outside the %v–%v a flourish should last",
			d, FrameClockFPS, minDuration, maxDuration)
	}
}
