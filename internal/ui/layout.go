package ui

import (
	"strings"

	"bloomfx.dev/bloom"
	"bloomfx.dev/bloom/anim"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/ui/style"
)

// --------------------------------------------------------------------------
// Layout: the stacked sections of the frame
// --------------------------------------------------------------------------
//
// The frame is a column of sections, top to bottom:
//
//	header         extension header (0 rows if not set)
//	stream         the transcript: every row that the others leave
//	warning        "ctrl+c again to quit" while idle (0 or 1 row)
//	separator      0 rows unless it has something to say
//	above widgets  extension widgets
//	queued msgs    queued and steering messages
//	subagents      background and subagent status
//	activity row   1 row while the agent works, 0 when idle
//	input          the composer, or the prompt that replaces it
//	below widgets  extension widgets
//	status bar     1 row
//	footer         extension footer (0 rows if not set)
//
// Each section is a string that a Kit component rendered. bloom gives each
// its natural height and the transcript the rest, places them, and reports
// where each one went. Nothing here adds heights by hand: the layout tree is
// the single description of the frame, used both to size the transcript
// (bloom.Solve, in distributeHeight) and to draw it (bloom.Root, in View).

// regionID names a section of the layout. View and the mouse handlers look a
// section's rectangle up by it, and layout animation follows a section by it
// from one frame to the next.
type regionID int

const (
	regionHeader regionID = iota + 1
	regionStream
	regionWarning
	regionSeparator
	regionAbove
	regionQueued
	regionSubagents
	regionActivity
	regionInput
	regionBelow
	regionStatus
	regionFooter
)

// inputFallbackRows is the height kept for an input that renders nothing:
// the composer bar's padding, one row of text, and its padding again.
const inputFallbackRows = 3

// layoutUnbounded is the least size that bloom gives for a space with no
// limit. A section asked to measure itself in such a space reports its own
// width.
const layoutUnbounded = 1 << 20

// section is one section of the layout: a string that a Kit component
// rendered.
//
// It reports its own size, so that bloom does not have to parse the string
// into cells only to measure it. That matters for an extension widget that
// draws a full framebuffer on every frame. The height is the number of lines,
// which is how Kit has always measured a section.
type section string

// View implements bloom.Child.
func (s section) View() string { return string(s) }

// Measure implements bloom.Measurer.
func (s section) Measure(avail bloom.Size) bloom.Size {
	if s == "" {
		return bloom.Size{}
	}
	w := avail.W
	if w >= layoutUnbounded {
		w = lipgloss.Width(string(s))
	}
	return bloom.Size{W: w, H: strings.Count(string(s), "\n") + 1}
}

// streamRegion draws the transcript in a section whose height is changing.
//
// The scroll list is sized to the height that the layout settles on. While a
// layout animation runs, the transcript's section passes through the heights
// between, and bloom asks for a layout at each one; the list is then drawn at
// that height, so the transcript follows the section's edge and no gap opens
// beside it.
type streamRegion struct {
	bloom.NoView

	list *ScrollList
	// view is the list drawn at its own height, rendered once for the frame.
	view string
}

// Layout implements bloom.Layouter. It does not change the list.
func (s *streamRegion) Layout(ctx bloom.Context) bloom.Child {
	if s.list == nil || ctx.Height <= 0 || ctx.Height >= layoutUnbounded || ctx.Height == s.list.height {
		return section(s.view)
	}
	return section(s.list.ViewAtHeight(ctx.Height))
}

// chromeCache holds the rendered sections of one frame, other than the
// transcript. distributeHeight fills it when the layout changed, and View
// reuses the strings within the same frame instead of rendering the chrome a
// second time.
type chromeCache struct {
	valid     bool
	header    string
	footer    string
	warning   string
	separator string
	above     string
	below     string
	queued    string
	activity  string
	subagents string
	input     string
	status    string
}

// renderChrome renders every section of the frame other than the transcript,
// at its natural height. Fill widgets take no rows here: distributeHeight
// sizes them from the rows that are left.
//
// It also returns the widget snapshots that it rendered, so that measurement
// and the fill pass agree even if an extension updates its widgets meanwhile.
func (m *AppModel) renderChrome() (c chromeCache, above, below []WidgetData) {
	vis := m.uiVis()
	c.valid = true

	// The separator only materialises when it has something to say, so an
	// idle screen carries no rule between transcript and composer.
	if !vis.HideSeparator {
		c.separator = m.renderSeparator()
	}

	c.queued = m.renderQueuedMessages()

	// Live activity sits immediately above the composer so the eye finds it
	// in the same place it is about to type. It costs zero rows when idle.
	c.activity = m.renderActivityRow()
	c.subagents = m.renderBackgroundStatus()

	// When a prompt is active, it replaces the input area for consistency
	// (it appears in the same position as the input). renderInput includes
	// the editor interceptor's Render wrapper, so the height that is measured
	// is the height that is drawn.
	if m.state == statePrompt && m.prompt != nil {
		c.input = m.prompt.Render()
	} else {
		c.input = m.renderInput()
	}

	if m.getWidgets != nil {
		above = m.getWidgets("above")
		below = m.getWidgets("below")
	}
	c.above = m.renderWidgets("above", above, nil)
	c.below = m.renderWidgets("below", below, nil)

	c.header = m.renderHeaderFooter("header", m.getHeader)
	c.footer = m.renderHeaderFooter("footer", m.getFooter)

	// Confirmation prompts for destructive keys live in the activity row
	// while the agent is working, since that row is already showing the
	// interrupt affordance and is where the eye is. When idle there is no
	// activity row, so the prompt gets its own row instead.
	if m.ctrlCPressedOnce && !m.agentWorking() {
		c.warning = lipgloss.NewStyle().
			Foreground(style.GetTheme().Warning).
			Render(strings.Repeat(" ", style.ContentOffset) + "ctrl+c again to quit")
	}

	if !vis.HideStatusBar {
		c.status = m.renderStatusBar()
	}
	return c, above, below
}

// layoutTree returns the frame's layout: the chrome in c around the
// transcript.
//
// stream is the transcript; nil stands for it while only sizes are wanted.
// With animate set, a section that changes size or place glides there.
//
// Every section is always in the tree, at zero rows when it has nothing to
// show. A section that came and went from the tree could not be animated:
// layout animation follows a node from one frame to the next by its ID.
func (m *AppModel) layoutTree(c *chromeCache, stream bloom.Child, animate bool) bloom.Child {
	place := func(id regionID, slot bloom.SlotNode) bloom.Child {
		slot = slot.ID(id)
		if animate {
			slot = slot.Animate(anim.Duration(layoutGlide))
		}
		return slot
	}
	auto := func(id regionID, s string) bloom.Child {
		return place(id, bloom.Auto(section(s)))
	}

	// The transcript takes the rows that are left. Fill widgets take those
	// rows in its place: their sections in c already hold them.
	streamSlot := bloom.Fill(stream)
	if m.fillActive {
		streamSlot = bloom.Fixed(0, nil)
	}

	// An input that renders nothing still keeps its rows, so the transcript
	// does not jump when the composer comes back.
	inputSlot := bloom.Auto(m.composerChild(c.input))
	if c.input == "" {
		inputSlot = bloom.Fixed(inputFallbackRows, nil)
	}

	return bloom.Column(
		auto(regionHeader, c.header),
		place(regionStream, streamSlot),
		auto(regionWarning, c.warning),
		auto(regionSeparator, c.separator),
		auto(regionAbove, c.above),
		auto(regionQueued, c.queued),
		auto(regionSubagents, c.subagents),
		place(regionActivity, bloom.Auto(m.activityChild(c.activity))),
		place(regionInput, inputSlot),
		auto(regionBelow, c.below),
		auto(regionStatus, c.status),
		auto(regionFooter, c.footer),
	)
}

// regionRows returns the height of a section in a solved or drawn frame, or
// zero when the section has no rows in it.
func regionRows(frame *bloom.Frame, id regionID) int {
	if frame == nil {
		return 0
	}
	if r, ok := frame.Rect(id); ok {
		return r.Dy()
	}
	return 0
}

// layoutAnimates reports whether sections glide to their new place in the
// frame about to be drawn.
//
// They jump when the window was resized, since a resize is not a change to
// animate, and while an image is placed directly on the screen. Such an image
// is painted beside the view at a fixed row, and would be left behind by a
// section that moved under it.
func (m *AppModel) layoutAnimates() bool {
	return m.motionOn() && !m.layoutJump && len(m.gfxPlaced) == 0
}
