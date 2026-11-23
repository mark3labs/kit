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

	// regionCount is one more than the last section.
	regionCount
)

// inputFallbackRows is the height kept for an input that renders nothing:
// the composer bar's padding, one row of text, and its padding again.
const inputFallbackRows = 3

// streamMinRows is the least number of rows that the transcript keeps while a
// section that can be left out is still on screen. See layoutTree.
const streamMinRows = 4

// The order in which sections leave a frame that is too short for them all:
// the lowest number goes first. The composer, the activity row, the warning
// and the status bar have no number, because they do not leave.
const (
	dropBelow = iota + 1
	dropAbove
	dropFooter
	dropHeader
	dropSeparator
	dropSubagents
	dropQueued
)

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
	return bloom.Size{W: w, H: sectionRows(string(s))}
}

// sectionRows returns the number of rows that a rendered section takes: its
// lines, or none when it is empty.
func sectionRows(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
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
	// wrap returns the node for the list drawn at a height of rows, with the
	// transitions that run on the transcript. See AppModel.transcriptChild.
	wrap func(view string, rows int) bloom.Child
}

// Layout implements bloom.Layouter. It does not change the list.
func (s *streamRegion) Layout(ctx bloom.Context) bloom.Child {
	if s.list == nil || ctx.Height <= 0 || ctx.Height >= layoutUnbounded || ctx.Height == s.list.height {
		return s.wrapped(s.view, s.listRows())
	}
	return s.wrapped(s.list.ViewAtHeight(ctx.Height), ctx.Height)
}

// listRows returns the height of the list, or zero without one.
func (s *streamRegion) listRows() int {
	if s.list == nil {
		return 0
	}
	return s.list.height
}

// wrapped returns the node for view, the list drawn at a height of rows.
func (s *streamRegion) wrapped(view string, rows int) bloom.Child {
	if s.wrap == nil {
		return section(view)
	}
	return s.wrap(view, rows)
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

	// fill is true when a fill widget takes the rows of the transcript. No
	// section then leaves a short frame: see layoutTree.
	fill bool
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
	c.fill = m.hasFillWidget(above) || m.hasFillWidget(below)

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
//
// A frame can be too short for every section. The sections then leave it one
// at a time, in the order of the drop constants, until the rest fit with
// streamMinRows rows for the transcript. The composer, the activity row, the
// warning and the status bar always keep their rows. Without this order every
// section would lose rows in proportion to its height, the composer included,
// and the transcript would get none.
func (m *AppModel) layoutTree(c *chromeCache, stream bloom.Child, animate bool) bloom.Child {
	// An input that renders nothing still keeps its rows, so the transcript
	// does not jump when the composer comes back.
	inputRows := inputFallbackRows
	if c.input != "" {
		inputRows = sectionRows(c.input)
	}

	// The rows that must stay. If the frame cannot hold even those, there is
	// no order that helps, and the sections share the rows as they can. Fill
	// widgets stand in for the transcript, so they do not give way to it.
	kept := sectionRows(c.warning) + sectionRows(c.activity) + inputRows + sectionRows(c.status)
	ordered := !c.fill && !m.fillActive && kept <= m.height
	streamMin := min(streamMinRows, m.height-kept)

	place := func(id regionID, slot bloom.SlotNode) bloom.Child {
		slot = slot.ID(id)
		if animate {
			slot = slot.Animate(anim.Spring(layoutSpring, 1))
		}
		return slot
	}
	// keep is a section that holds its rows in a short frame.
	keep := func(id regionID, child bloom.Child, rows int) bloom.Child {
		slot := bloom.Auto(child)
		if ordered {
			slot = slot.Min(rows)
		}
		return place(id, slot)
	}
	// drop is a section that leaves a short frame, in the order of its number.
	drop := func(id regionID, s string, order int) bloom.Child {
		slot := bloom.Auto(section(s))
		if ordered {
			slot = slot.Min(sectionRows(s)).Collapsible(order)
		}
		return place(id, slot)
	}

	// The transcript takes the rows that are left. Fill widgets take those
	// rows in its place: their sections in c already hold them.
	streamSlot := bloom.Fill(stream)
	switch {
	case m.fillActive:
		streamSlot = bloom.Fixed(0, nil)
	case ordered:
		streamSlot = streamSlot.Min(streamMin)
	}

	inputSlot := keep(regionInput, m.composerChild(c.input), inputRows)
	if c.input == "" {
		inputSlot = place(regionInput, bloom.Fixed(inputFallbackRows, nil))
	}

	return bloom.Column(
		drop(regionHeader, c.header, dropHeader),
		place(regionStream, streamSlot),
		keep(regionWarning, section(c.warning), sectionRows(c.warning)),
		drop(regionSeparator, c.separator, dropSeparator),
		drop(regionAbove, c.above, dropAbove),
		drop(regionQueued, c.queued, dropQueued),
		drop(regionSubagents, c.subagents, dropSubagents),
		keep(regionActivity, m.activityChild(c.activity), sectionRows(c.activity)),
		inputSlot,
		drop(regionBelow, c.below, dropBelow),
		keep(regionStatus, section(c.status), sectionRows(c.status)),
		drop(regionFooter, c.footer, dropFooter),
	)
}

// hasFillWidget reports whether one of widgets takes the rows of the
// transcript at the width of the frame.
func (m *AppModel) hasFillWidget(widgets []WidgetData) bool {
	for _, w := range widgets {
		if w.FillHeight && widgetContentWidth(m.width, w.NoBorder) > 0 {
			return true
		}
	}
	return false
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

// noteSectionRects records where the frame drew each section, and reports
// whether one of them is in another place than in the frame before.
func (m *AppModel) noteSectionRects(frame *bloom.Frame) (moved bool) {
	if frame == nil {
		return false
	}
	for id := regionHeader; id < regionCount; id++ {
		r, _ := frame.Rect(id)
		if r != m.sectionRects[id] {
			m.sectionRects[id] = r
			moved = true
		}
	}
	return moved
}

// layoutAnimates reports whether sections glide to their new place in the
// frame about to be drawn.
//
// They jump when the window was resized, since a resize is not a change to
// animate, and when the change moves an image that is placed directly on the
// screen. Such an image is painted beside the view at a fixed row, and would
// be left behind by a section that moved under it. See imagesStayPut.
//
// It reads the chrome of the frame, so View calls it after it has rendered
// the sections.
func (m *AppModel) layoutAnimates() bool {
	return m.motionOn() && !m.layoutJump && m.imagesStayPut()
}
