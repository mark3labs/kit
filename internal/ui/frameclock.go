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
// Transitions run on this clock too (see motion.go). A transition is short
// and moves cells, so it wants more frames than a spinner does: while one
// runs, the clock adds a frame between each two beats, which makes 60 frames
// a second. The beats stay 30 a second, so nothing that counts them changes
// speed. Each animated value used to schedule its own frames; a dialog that
// opened while the agent worked then ran three loops, out of phase, for one
// animation.
//
// One loop is not on this clock: layout animation. The bloom root schedules
// the frames of a section that glides, and it has no way to take them from
// another clock. A glide is short, so that loop is seldom awake. The root
// asks for a frame after each message that it hears, to look for a glide, so
// the ticks of this clock do not go to it while the layout is at rest: see
// AppModel.rootHears. Each tick would otherwise be drawn twice.
//
// While the terminal does not have focus the clock ticks 10 times a second.
// Each tick then stands for three beats, so that the time that animations
// count stays true. An animation that nobody looks at does not have to be
// smooth, and a session that works in the background for an hour costs a
// third of the frames.
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

// frameClockFastFPS is the rate of the clock while a transition runs: a frame
// on each beat, and one between each two beats.
const frameClockFastFPS = 2 * FrameClockFPS

// frameClockUnfocusedFPS is the rate of the clock while the terminal does not
// have focus. It divides FrameClockFPS, so each tick stands for a whole
// number of beats.
const frameClockUnfocusedFPS = 10

// frameTickMsg is one tick of the shared animation clock.
//
// generation identifies the run that scheduled the tick, so a beat left over
// from a clock that has since stopped is discarded rather than starting a
// second loop alongside the current one. frame is the monotonic frame number,
// assigned when the tick is scheduled, which is what animations subdivide to
// derive their own rate.
type frameTickMsg struct {
	generation uint64
	frame      int

	// between marks a frame between two beats. Only transitions move on it:
	// it is not a beat, and frame is the number of the beat before it.
	between bool

	// skipped is the number of beats before this one that had no tick of
	// their own, because the terminal does not have focus. The tick stands
	// for those beats too.
	skipped int
}

// due reports whether an animation that advances once every n clock frames
// should advance on this tick. n <= 1 advances on every beat.
//
// A tick that stands for several beats is due when one of them is. The
// animation advances once for it, not once for each beat: it runs slower while
// nobody looks, and it does not jump.
func (t frameTickMsg) due(n int) bool {
	if t.between {
		return false
	}
	if n <= 1 {
		return true
	}
	// The last multiple of n at or before this beat is one of the beats that
	// the tick stands for.
	return t.frame/n*n >= t.frame-t.skipped
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

	// fast adds a frame between each two beats. AppModel sets it while a
	// transition runs. It takes effect on the next tick that is scheduled:
	// the tick in flight keeps its time.
	fast bool

	// blurred is true while the terminal does not have focus.
	blurred bool

	// between is true when the tick accepted last was a frame between two
	// beats. The next tick is then the beat that it came before.
	between bool
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
	c.between = false
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
	c.between = t.between
	return true
}

// next returns the tick that follows the one accepted last, and how long
// from now it is due.
func (c *frameClock) next() (frameTickMsg, time.Duration) {
	t := frameTickMsg{generation: c.generation, frame: c.frame + 1}
	switch {
	case c.between:
		// The beat that the frame in between came before. It is due half a
		// beat later whatever the clock does next, so the beats keep time.
		return t, time.Second / frameClockFastFPS
	case c.blurred:
		t.skipped = FrameClockFPS/frameClockUnfocusedFPS - 1
		t.frame += t.skipped
		return t, time.Second / frameClockUnfocusedFPS
	case c.fast:
		t.frame, t.between = c.frame, true
		return t, time.Second / frameClockFastFPS
	}
	return t, time.Second / FrameClockFPS
}

// tick schedules the next beat. The frame number is assigned here, at schedule
// time, so the sequence stays monotonic and is not affected by how long the
// receiving update takes.
func (c *frameClock) tick() tea.Cmd {
	next, after := c.next()
	return tea.Tick(after, func(_ time.Time) tea.Msg {
		return next
	})
}
