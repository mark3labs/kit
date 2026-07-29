package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// --------------------------------------------------------------------------
// Shared animation clock
// --------------------------------------------------------------------------

// The frame clock is the single heartbeat every animation in the TUI runs on.
//
// Each animation used to own its tick loop: the startup logo scheduled one at
// 30fps, the activity spinner another at 14fps, and each reimplemented the
// same generation guard to stop a restarted animation from running two loops
// at double speed. That does not scale, for two reasons.
//
// The first is cost. BubbleTea has no dirty-region tracking, so every tick —
// whatever it animates, however small — costs a full View() pass over the
// whole app. Independent loops therefore compose additively: a 14fps spinner
// beside a 30fps logo is 44 renders a second, not 30, and the two are not even
// in phase, so the work arrives in an irregular stutter. On one clock the same
// two animations cost 30, and a third costs nothing further.
//
// The second is correctness. The generation guard is subtle and was written
// three times. Here it is written once, in accept, and every animation
// inherits it.
//
// Animations do not schedule anything. They advance when handed a frame and
// report, through AppModel.wantsFrames, whether they still want more. The
// clock runs only while something does, so an idle session costs nothing.
//
// Deliberately out of scope: the 16ms stream-flush ticks. Those look like an
// animation loop but are a coalescing window whose timing is tied to when
// provider chunks arrive, not to the display. Folding them in here would make
// streaming latency depend on the phase of the render clock.

// FrameClockFPS is the rate the shared animation clock ticks at.
//
// 30 is the ceiling worth paying for: it is the rate at which a colour
// gradient sweeping across a cell reads as motion rather than as a sequence of
// steps, and it is already the rate the logo animation was tuned against.
// Animations that want to run slower subdivide it rather than asking for a
// second clock — see frameTickMsg.due.
const FrameClockFPS = 30

// How often each animation advances, counted in clock frames.
//
// The activity spinner ran at 14fps when it owned its loop, which is not an
// integer divisor of 30. It is subdivided to 15 instead: a 14-frame bounce now
// takes 0.93s rather than 1.0s, which is not a difference anyone can see on a
// bouncing dot, and it buys exact, driftless subdivision with no float maths
// and no accumulator.
const (
	frameEveryLogo    = 1 // 30fps
	frameEverySpinner = 2 // 15fps
)

// frameTickMsg is one beat of the shared animation clock.
//
// generation identifies the run that scheduled the tick, so a beat left over
// from a clock that has since stopped is discarded rather than starting a
// second loop alongside the current one. frame is the monotonic frame number,
// assigned when the tick is scheduled, which is what animations subdivide to
// derive their own rate.
type frameTickMsg struct {
	generation uint64
	frame      int
}

// due reports whether an animation that advances once every n clock frames
// should advance on this tick. n <= 1 advances on every frame.
func (t frameTickMsg) due(n int) bool {
	if n <= 1 {
		return true
	}
	return t.frame%n == 0
}

// frameClock schedules the shared animation heartbeat.
//
// The zero value is a stopped clock, which is the correct state for a session
// with nothing animating.
type frameClock struct {
	// running is whether a tick is currently in flight. It is the guard that
	// makes start idempotent, so waking the clock from several places in the
	// same update cannot produce two concurrent loops.
	running bool

	// generation identifies the current run. Bumped on every start and stop so
	// that any tick still in flight from a previous run fails accept.
	generation uint64

	// frame is the monotonic frame counter for the current run, reset at each
	// start so subdivision phase is predictable from the moment motion begins.
	frame int
}

// start wakes the clock, returning the command that schedules its first frame.
// It returns nil when the clock is already running, which makes it safe to
// call after every message without tracking whether motion was already
// underway.
func (c *frameClock) start() tea.Cmd {
	if c.running {
		return nil
	}
	c.running = true
	c.generation++
	c.frame = 0
	return c.tick()
}

// stop halts the clock and invalidates any tick still in flight, so the loop
// dies rather than being handed to animations that no longer want it.
func (c *frameClock) stop() {
	if !c.running {
		return
	}
	c.running = false
	c.generation++
}

// accept reports whether a tick belongs to the current run, adopting its frame
// number when it does. A tick from a superseded run is dropped.
func (c *frameClock) accept(t frameTickMsg) bool {
	if !c.running || t.generation != c.generation {
		return false
	}
	c.frame = t.frame
	return true
}

// tick schedules the next beat. The frame number is assigned here, at schedule
// time, so the sequence stays monotonic and is not affected by how long the
// receiving update takes.
func (c *frameClock) tick() tea.Cmd {
	next := frameTickMsg{generation: c.generation, frame: c.frame + 1}
	return tea.Tick(time.Second/FrameClockFPS, func(_ time.Time) tea.Msg {
		return next
	})
}
