package ui

import (
	"image"
	"image/color"
	"math"
	"reflect"
	"strings"
	"time"

	"bloomfx.dev/bloom"
	"bloomfx.dev/bloom/anim"
	"bloomfx.dev/bloom/canvas"
	"bloomfx.dev/bloom/fx"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"

	"github.com/mark3labs/kit/internal/ui/style"
)

// --------------------------------------------------------------------------
// Motion: transitions and effects drawn by the bloom compositor
// --------------------------------------------------------------------------
//
// Kit components render to ANSI strings. View hands those strings to a
// bloom.Root, which lays them out, draws modal surfaces over them as layers
// and applies effects to cells. This file holds the animation state and the
// effects; layout.go holds the layout tree.
//
// The animated values here do not schedule their own frames. Update sets a
// target on a value, and the shared frame clock moves every value that has
// one (see frameclock.go and advanceFrame). So one loop draws the frames of a
// dialog, the dimming behind it and the spinner, where each had its own.
//
// Two rules keep the effects safe:
//
//   - No effect here reads the terminal's real background colour. Kit knows
//     only whether the background is dark or light, so an effect that wrote
//     an explicit background into unstyled cells would paint a rectangle of
//     the wrong colour. The effects below change colours that a cell already
//     has, and blend toward a colour taken from the theme.
//   - No effect recolours an image cell. In the Kitty placeholder protocol
//     the foreground colour of a cell is the image id, so a blended colour
//     would point the terminal at a different image.

const (
	// surfaceFade is how long a modal surface takes to appear or go.
	surfaceFade = 150 * time.Millisecond

	// surfaceRise is how many rows a surface travels upward as it appears.
	surfaceRise = 2

	// scrimFade is how long the screen takes to dim behind a modal surface.
	scrimFade = 180 * time.Millisecond

	// scrimAmount is how far the dimmed screen moves toward the background.
	scrimAmount = 0.45

	// surfaceLeave is how long a modal surface takes to go. It goes faster
	// than it came: the user has finished with it.
	surfaceLeave = 100 * time.Millisecond

	// layoutGlide is about how long a layout section takes to move or resize.
	// The section moves as a spring, see layoutSpring, so this is the time
	// after which it is within a small part of a row of its place.
	layoutGlide = 180 * time.Millisecond

	// layoutSpring is the angular frequency of the spring that moves a layout
	// section. Its damping is 1, so a section never goes past its place. A
	// spring keeps its speed when its target changes on the way: a composer
	// that grows one more row while it grows does not stop and start again.
	layoutSpring = 30.0

	// shakeDuration is how long the composer shakes after rejected input.
	shakeDuration = 380 * time.Millisecond

	// toastHold is how long a toast stays up; toastFade is its transition.
	toastHold = 1600 * time.Millisecond
	toastFade = 160 * time.Millisecond

	// paneFade is how long the transcript of another pane takes to appear,
	// and paneShift how many columns it travels sideways on the way.
	paneFade  = 160 * time.Millisecond
	paneShift = 4

	// swapFade is how long the composer section takes to appear when what
	// it holds changes: the composer, a prompt, or the bar of a child pane.
	swapFade = 140 * time.Millisecond

	// entryFade is how long a new item of the transcript takes to appear.
	entryFade = 180 * time.Millisecond

	// entryMax is the most new items that one Update can add for them to
	// fade in. More than that is not a conversation that moves on: it is a
	// session that loads, and it appears at once.
	entryMax = 3

	// scrollbarFade is how long the scrollbar of the transcript takes to
	// appear or go.
	scrollbarFade = 140 * time.Millisecond

	// toastMax is the most toasts on screen at one time. The oldest one goes
	// to make room for one more.
	toastMax = 4

	// toastMaxWidth is the most columns of text in a toast. Longer text is
	// cut: a notice that needs more room belongs in the transcript.
	toastMaxWidth = 56

	// shimmerSweep is how long the highlight takes to cross the activity
	// phrase, and shimmerRest the pause before it crosses again.
	shimmerSweep = 1100 * time.Millisecond
	shimmerRest  = 900 * time.Millisecond

	// shimmerWidth is the width of the highlight band in columns.
	shimmerWidth = 8

	// motionFPS is the frame rate of layout animation, which the bloom root
	// schedules. It is Bubble Tea's own maximum by default, so no frame is
	// drawn for nothing. Transitions run on the frame clock, at the same
	// rate: see frameClockFastFPS.
	motionFPS = 60
)

// region is one rendered piece of the frame: a chrome section or a modal box.
// It lets bloom place a string that a Kit component rendered, as it is.
type region string

// View implements bloom.Child.
func (r region) View() string { return string(r) }

// surfaceSpec describes one modal surface that View draws over the layout.
type surfaceSpec struct {
	// open reports whether the surface is on screen.
	open func(m *AppModel) bool

	// render draws the surface as a bare box and returns its vertical anchor
	// ("top-center", "bottom-center" or "" for the centre). It is called
	// only while the surface is open.
	render func(m *AppModel) (box, anchor string)

	// scrim dims the screen beneath the surface. Surfaces that exist to be
	// read against the conversation leave it off.
	scrim bool

	// still makes the surface appear and go at once, with no transition. A
	// surface that holds an image needs it: an image placed directly on the
	// screen does not travel with the cells of a box that moves.
	still bool

	// steady makes the surface fade in its place, with no travel. A surface
	// that finds what the mouse points at from where its box belongs needs
	// it: a click while the box was a row lower would land on another line.
	steady bool

	// tail lets the surface glide from one height to another, such as when
	// a filter leaves a list with fewer rows. It is the number of rows at
	// the foot of the box that stay whole on the way (see boxAtHeight). A
	// surface with no tail takes each new size at once.
	tail int
}

// surfaceID names the layer of one modal surface, by its place in
// surfaceSpecs. Layout animation follows the layer by it.
type surfaceID int

// boxRegion draws a modal box in a layer whose height is changing. It has the
// size of the box, and at each height on the way it draws the box at that
// height.
type boxRegion struct {
	bloom.NoView

	box  string
	tail int
}

// Measure implements bloom.Measurer.
func (b boxRegion) Measure(bloom.Size) bloom.Size {
	return bloom.Size{W: lipgloss.Width(b.box), H: sectionRows(b.box)}
}

// Layout implements bloom.Layouter.
func (b boxRegion) Layout(ctx bloom.Context) bloom.Child {
	return region(boxAtHeight(b.box, ctx.Height, b.tail))
}

// surfaceCount is the number of modal surfaces.
const surfaceCount = 13

// surfaceSpecs lists the modal surfaces in z-order: a later one is drawn over
// an earlier one.
var surfaceSpecs = [surfaceCount]surfaceSpec{
	// Slash and @ autocomplete popup. It completes what the user types, so
	// the composer stays at full brightness behind it, and it sits on the
	// composer: the eye is on the text that it completes.
	{
		open: func(m *AppModel) bool {
			ic, ok := m.input.(*InputComponent)
			return ok && ic.PopupVisible()
		},
		render: func(m *AppModel) (string, string) {
			ic, _ := m.input.(*InputComponent)
			return ic.RenderPopupBox(m.width, m.height), anchorAboveInput
		},
		tail: popupListTail,
	},
	{
		open:   func(m *AppModel) bool { return m.state == stateModelSelector && m.modelSelector != nil },
		render: func(m *AppModel) (string, string) { return m.modelSelector.RenderOverlay(), "" },
		scrim:  true,
		tail:   popupListTail,
	},
	{
		open: func(m *AppModel) bool {
			return m.state == stateThinkingLevelSelector && m.thinkingLevelSelector != nil
		},
		render: func(m *AppModel) (string, string) { return m.thinkingLevelSelector.RenderOverlay(), "" },
		scrim:  true,
		tail:   popupListTail,
	},
	{
		open:   func(m *AppModel) bool { return m.state == stateModelModeSelector && m.modelModeSelector != nil },
		render: func(m *AppModel) (string, string) { return m.modelModeSelector.RenderOverlay(), "" },
		scrim:  true,
		tail:   popupListTail,
	},
	{
		open:   func(m *AppModel) bool { return m.killSubagentOpen() },
		render: func(m *AppModel) (string, string) { return m.killSubagentSelector.RenderOverlay(), "" },
		scrim:  true,
		tail:   popupListTail,
	},
	{
		open:   func(m *AppModel) bool { return m.subagentRunPickerOpen() },
		render: func(m *AppModel) (string, string) { return m.subagentRunPicker.RenderOverlay(), "" },
		scrim:  true,
		tail:   popupListTail,
	},
	// The theme selector previews each theme on the conversation behind it,
	// so that conversation must keep its true colours.
	{
		open:   func(m *AppModel) bool { return m.state == stateThemeSelector && m.themeSelector != nil },
		render: func(m *AppModel) (string, string) { return m.themeSelector.RenderOverlay(), "" },
		tail:   popupListTail,
	},
	{
		open:   func(m *AppModel) bool { return m.state == stateProviderConnect && m.providerConnect != nil },
		render: func(m *AppModel) (string, string) { return m.providerConnect.RenderOverlay(), "" },
		scrim:  true,
	},
	{
		open:   func(m *AppModel) bool { return m.state == stateTreeSelector && m.treeSelector != nil },
		render: func(m *AppModel) (string, string) { return m.treeSelector.RenderOverlay(), "" },
		scrim:  true,
	},
	{
		open:   func(m *AppModel) bool { return m.state == stateSessionSelector && m.sessionSelector != nil },
		render: func(m *AppModel) (string, string) { return m.sessionSelector.RenderOverlay(), "" },
		scrim:  true,
	},
	// Extension overlays and the message inspector. Honours the caller's
	// vertical anchor.
	{
		open:   func(m *AppModel) bool { return m.state == stateOverlay && m.overlay != nil },
		render: func(m *AppModel) (string, string) { return m.overlay.Render(), m.overlay.Anchor() },
		scrim:  true,
	},
	{
		open:   func(m *AppModel) bool { return m.imagePreview != nil },
		render: func(m *AppModel) (string, string) { return m.renderImagePreview(), "" },
		scrim:  true,
		still:  true,
	},
	{
		// A sheet takes mouse selection in its body; see renderSheet.
		open:   func(m *AppModel) bool { return m.sheet != nil },
		render: func(m *AppModel) (string, string) { return m.renderSheet(), "" },
		scrim:  true,
		steady: true,
	},
}

// surfaceAnim is the animation state of one modal surface.
type surfaceAnim struct {
	// shown runs from 0, hidden, to 1, fully shown.
	shown anim.Value

	// open is the state the surface was last seen in.
	open bool

	// box and anchor are what was drawn last. They outlive the surface for
	// as long as it takes to animate out: by then the component that drew
	// the box is gone.
	box    string
	anchor string
}

// toastID names one toast, for the timer that takes it down and for the
// layout animation that closes the gap it leaves.
type toastID uint64

// toast is one notice in the corner of the screen.
type toast struct {
	id   toastID
	text string

	// key names what the toast is about. A notice with the key of a toast
	// that is up takes the place of its text: see showToastFor.
	key string

	// shown runs from 0, out of sight, to 1, in its place.
	shown anim.Value

	// timer is the number of the timer that takes the toast down. A toast
	// that is shown again gets a new timer, and the old one then does nothing.
	timer uint64

	// leaving is true from the moment its time is up.
	leaving bool
}

// motionState is the animation state that AppModel keeps between frames.
type motionState struct {
	surfaces [surfaceCount]surfaceAnim

	// scrim is how far the screen is dimmed behind a modal surface, from 0
	// to 1. It is one value for the whole screen, not one for each surface,
	// so that the dimming holds steady while one dialog replaces another.
	scrim   anim.Value
	scrimOn bool

	// shake runs from 0 to 1 while the composer shakes, and rests at 1.
	shake anim.Value

	// bar is how far the scrollbar of the transcript is shown, from 0 to 1.
	// barOn is the state it was last seen in: see syncScrollbar.
	bar   anim.Value
	barOn bool

	// pane runs from 0 to 1 while the transcript of another pane appears,
	// and rests at 1. paneFrom is the side that the transcript comes from:
	// 1 for the right, -1 for the left.
	pane     anim.Value
	paneFrom int

	// swap runs from 0 to 1 while the composer section appears with new
	// content, and rests at 1. inputKind is what the section held when it
	// was last seen: see syncInputKind.
	swap      anim.Value
	inputKind any

	// entries are the items of the transcript that are new, while they
	// fade in.
	entries []*entry

	// toasts are the notices in the corner of the screen, the oldest first.
	toasts   []*toast
	toastSeq uint64

	// pending are the commands of the toasts that notify showed during the
	// Update in hand. Update sends them out with its own.
	pending []tea.Cmd

	// now is the clock that the values run on. Tests replace it.
	now func() time.Time
}

// newMotionState returns the animation state at rest.
func newMotionState() *motionState {
	fxs := &motionState{
		scrim: anim.NewValue(0),
		shake: anim.NewValue(1),
		bar:   anim.NewValue(0),
		pane:  anim.NewValue(1),
		swap:  anim.NewValue(1),
		now:   time.Now,
	}
	for i := range fxs.surfaces {
		fxs.surfaces[i].shown = anim.NewValue(0)
	}
	return fxs
}

// each calls fn with every animated value.
func (fxs *motionState) each(fn func(v *anim.Value)) {
	fn(&fxs.scrim)
	fn(&fxs.shake)
	fn(&fxs.bar)
	fn(&fxs.pane)
	fn(&fxs.swap)
	for _, e := range fxs.entries {
		fn(&e.shown)
	}
	for i := range fxs.surfaces {
		fn(&fxs.surfaces[i].shown)
	}
	for _, t := range fxs.toasts {
		fn(&t.shown)
	}
}

// move sends v to target over d, as an animation or at once. It schedules
// nothing: the frame clock moves the value, and Update wakes the clock after
// each message.
//
// A value keeps the duration that it was made with, so a new value takes its
// place, at the same address, and starts from where the old one was.
func (fxs *motionState) move(v *anim.Value, target float64, d time.Duration, animate bool, opts ...anim.Option) {
	if !animate {
		v.Set(target)
		return
	}
	*v = anim.NewValue(v.Float(), append([]anim.Option{anim.Duration(d)}, opts...)...)
	v.SetTarget(target, fxs.now())
}

// advance moves every value to where it is at this time, and lets go of the
// toasts that have gone.
func (fxs *motionState) advance() {
	now := fxs.now()
	fxs.each(func(v *anim.Value) { v.Advance(now) })
	fxs.pruneToasts()
	fxs.pruneEntries()
}

// fxState returns the animation state, which it makes on first use. Tests
// build an AppModel without the constructor, so nothing may assume that the
// constructor ran.
func (m *AppModel) fxState() *motionState {
	if m.fx == nil {
		m.fx = newMotionState()
	}
	return m.fx
}

// root returns the bloom root that lays out and draws the frame, which it
// makes on first use. One root lives as long as the model: it holds the
// render caches and the positions that layout animation moves from.
func (m *AppModel) root() *bloom.Root {
	if m.ui == nil {
		fg, bg := fxColors()
		m.ui = bloom.New(
			bloom.WithColors(fg, bg),
			bloom.WithFPS(motionFPS),
			// Kit has its own selection and its own mouse handling.
			bloom.WithCopyFlash(false),
			bloom.WithClickDelay(false),
		)
	}
	return m.ui
}

// motionOn reports whether transitions and layout animation run.
//
// The user can turn motion off, and a terminal below 256 colours cannot show
// a colour blend as motion: the steps collapse onto a few colours and strobe.
func (m *AppModel) motionOn() bool {
	return m.motionEnabled && style.SupportsSmoothAnimation()
}

// fxColors returns the colours that effects blend with: the default text
// colour and the colour that stands for the background.
//
// They are estimates. A theme names a text colour, and may name a
// background; without one the background is taken as black or white from the
// terminal's reported brightness.
func fxColors() (fg, bg color.RGBA) {
	theme := style.GetTheme()
	dark := style.IsDarkBackground()

	fg = color.RGBA{R: 0xdd, G: 0xdd, B: 0xdd, A: 0xff}
	bg = color.RGBA{A: 0xff}
	if !dark {
		fg = color.RGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff}
		bg = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
	if solidColor(theme.Text) {
		fg = rgba(theme.Text)
	}
	if solidColor(theme.Background) {
		bg = rgba(theme.Background)
	}
	return fg, bg
}

// solidColor reports whether c is a colour that can be blended.
func solidColor(c color.Color) bool {
	if c == nil {
		return false
	}
	if _, none := c.(lipgloss.NoColor); none {
		return false
	}
	return true
}

// rgba returns c as a plain RGBA value. Effects are compared with == to skip
// frames that did not change, so the colours they hold must be comparable
// whatever type the theme used for them.
func rgba(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

// syncRootColors gives the root the colours of the active theme. Call it when
// the theme or the terminal's brightness changes.
func (m *AppModel) syncRootColors() {
	fg, bg := fxColors()
	root := m.root()
	root.Update(tea.ForegroundColorMsg{Color: fg})
	root.Update(tea.BackgroundColorMsg{Color: bg})
}

// --------------------------------------------------------------------------
// Modal surfaces
// --------------------------------------------------------------------------

// syncSurfaces brings the animation state in step with which surfaces are
// open, and starts the transitions of those that changed.
//
// Update calls it with animate set after each message, and then wakes the
// frame clock. View calls it too, without animate, for the surfaces that
// changed with no Update in between: nothing would wake the clock for those,
// so they jump to their end state.
func (m *AppModel) syncSurfaces(animate bool) {
	fxs := m.fxState()
	scrim := false
	for i := range surfaceSpecs {
		spec, a := &surfaceSpecs[i], &fxs.surfaces[i]
		open := spec.open(m)
		if open && spec.scrim {
			scrim = true
		}
		if open == a.open {
			continue
		}
		a.open = open
		// A surface goes faster than it came.
		d := surfaceLeave
		if open {
			d = surfaceFade
		}
		fxs.move(&a.shown, onOff(open), d, animate && !spec.still)
	}
	if scrim != fxs.scrimOn {
		fxs.scrimOn = scrim
		fxs.move(&fxs.scrim, onOff(scrim), scrimFade, animate)
	}
}

// onOff returns the target of a value that is 1 when on and 0 when not.
func onOff(on bool) float64 {
	if on {
		return 1
	}
	return 0
}

// bloomPackage is the import path that every bloom package starts with.
const bloomPackage = "bloomfx.dev/bloom"

// isMotionMsg reports whether msg belongs to bloom: a layout animation tick,
// or one of its timers.
//
// Some of those types are not exported, so they are known by the package
// that declares them. They arrive up to 60 times a second while something
// moves, and they mean nothing to the rest of the model; see updateMotionMsg.
func isMotionMsg(msg tea.Msg) bool {
	if msg == nil {
		return false
	}
	t := reflect.TypeOf(msg)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return strings.HasPrefix(t.PkgPath(), bloomPackage)
}

// motionSettledMsg follows the frame on which the last running transition
// came to rest.
//
// View can find work that only an Update can start: once a modal surface has
// gone, the images that were hidden behind it are due to be placed again, and
// placing them takes a command. The frame that ends a transition is drawn
// after the Update that ended it, so that Update cannot know. This message
// gives the model one more Update, after that frame.
type motionSettledMsg struct{}

// updateMotionMsg handles a message that belongs to bloom, and nothing else
// does.
//
// The model's own Update is skipped for it. That Update treats a message it
// does not know as a reason to flush the buffered stream chunks, which is
// right for an event and wrong for an animation frame: 60 frames a second
// would undo the coalescing that keeps a long response cheap to draw.
func (m *AppModel) updateMotionMsg(msg tea.Msg) tea.Cmd {
	return m.root().Update(msg)
}

// settled returns the command that sends motionSettledMsg, if a transition
// was moving before the frame in hand and none is moving now.
func (fxs *motionState) settled(moving bool) tea.Cmd {
	if !moving || fxs.animating() {
		return nil
	}
	return func() tea.Msg { return motionSettledMsg{} }
}

// animating reports whether a transition or feedback effect is running. The
// frame clock runs fast for as long as it is.
func (fxs *motionState) animating() bool {
	moving := false
	fxs.each(func(v *anim.Value) { moving = moving || v.Animating() })
	return moving
}

// surfacesVisible reports whether a modal surface is drawn, open or on its
// way out. Content that is painted beside the view, such as a directly placed
// image, stays hidden for as long as this is true.
func (m *AppModel) surfacesVisible() bool {
	if m.fx == nil {
		return false
	}
	for i := range m.fx.surfaces {
		if a := &m.fx.surfaces[i]; a.open || a.shown.Animating() || a.shown.Float() > 0 {
			return true
		}
	}
	return false
}

// surfaceLayers returns a layer for each modal surface on screen, in z-order,
// and how far the screen beneath them is dimmed. With glide set, a surface
// that has a tail glides from one height to another.
func (m *AppModel) surfaceLayers(glide bool) (layers []bloom.LayerNode, scrim float64) {
	// A surface that changed without an Update in between has no transition
	// to run; put it in its end state.
	m.syncSurfaces(false)

	fxs := m.fxState()
	fg, bg := fxColors()
	for i := range surfaceSpecs {
		spec, a := &surfaceSpecs[i], &fxs.surfaces[i]
		if a.open {
			a.box, a.anchor = spec.render(m)
			a.box = trimTrailingBlankLines(a.box)
		}
		t := min(max(a.shown.Float(), 0), 1)
		if !a.open && t <= 0 && !a.shown.Animating() {
			a.box = "" // gone: let the box go
		}
		if strings.TrimSpace(a.box) == "" {
			continue
		}
		// A surface that is still or steady keeps the place that its own
		// code works out, so it does not glide.
		resize := glide && spec.tail > 0 && !spec.still && !spec.steady
		var box bloom.Child = region(a.box)
		if resize {
			box = boxRegion{box: a.box, tail: spec.tail}
		}
		layer := bloom.Layer(box)
		if a.anchor == anchorAboveInput {
			// Against the composer, wherever the layout put it in this
			// frame. A box with no room above the composer goes below it.
			layer = layer.AnchorTo(regionInput, bloom.Top)
		} else {
			rect, ok := overlayRect(a.box, a.anchor, m.width, m.height)
			if !ok {
				continue
			}
			layer = layer.Place(rect)
		}
		if resize {
			layer = layer.ID(surfaceID(i)).Animate(anim.Spring(layoutSpring, 1))
		}
		switch {
		case t >= 1:
		case spec.steady || !a.open:
			// A surface that goes fades where it is. It does not travel back
			// down: the rise says where a dialog came from, and a dialog that
			// the user closed has nowhere to go to.
			layer = layer.Transition(t, fadeIn{Fg: fg, From: bg})
		default:
			layer = layer.Transition(t, fadeIn{Fg: fg, From: bg}, rise{Rows: surfaceRise})
		}
		layers = append(layers, layer)
	}

	// The dimming needs enough colours to read as a tint. It does not need
	// motion: with motion off it is simply there or not.
	if style.SupportsSmoothAnimation() {
		scrim = min(max(fxs.scrim.Float(), 0), 1)
	}
	return layers, scrim
}

// --------------------------------------------------------------------------
// Scrollbar
// --------------------------------------------------------------------------

// syncScrollbar brings the scrollbar in step with the transcript: it is shown
// while the user has scrolled the transcript away from its end. A transcript
// that follows the conversation has no scrollbar, because its place is known
// and its last column then stays free for the text.
//
// The test is whether the transcript follows, not whether its last line is in
// view. A section that grows below the transcript hides its last lines for a
// moment, and the user did not scroll then.
//
// Update calls it with animate set after each message, as it does
// syncSurfaces. View calls it without, for a change that no Update saw.
func (m *AppModel) syncScrollbar(animate bool) {
	fxs := m.fxState()
	on := m.scrollList != nil && m.scrollList.height > 0 && !m.scrollList.autoScroll
	if on == fxs.barOn {
		return
	}
	fxs.barOn = on
	fxs.move(&fxs.bar, onOff(on), scrollbarFade, animate)
}

// streamChild returns the transcript for the layout: view, the scroll list
// drawn at a height of rows, with its transitions and its scrollbar.
func (m *AppModel) streamChild(view string, rows int) bloom.Child {
	return m.withScrollbar(m.transcriptChild(view, rows), rows)
}

// withScrollbar returns the transcript with its scrollbar, when it has one.
// stream is the transcript drawn at a height of rows.
//
// The scrollbar is a layer over the last column of the transcript. It takes
// no column from the text: only the cells of its thumb cover what is beneath
// them. It shows where the view is and takes no mouse events; the wheel and
// the keys scroll.
//
// The thumb is for the rows that are drawn. While a layout animation runs,
// these are not the rows of the list's own viewport: the transcript passes
// through the heights between, each with its own first line (see
// ScrollList.atHeight). A thumb from the list's own numbers would show the
// end of the glide from its first frame, and would go at once when the
// transcript is on its way to a height that holds all of it.
func (m *AppModel) withScrollbar(stream bloom.Child, rows int) bloom.Child {
	t := min(max(m.fxState().bar.Float(), 0), 1)
	if t <= 0 || m.scrollList == nil || rows <= 0 {
		return stream
	}
	var total, visible, offset int
	m.scrollList.atHeight(rows, func() { total, visible, offset = m.scrollList.scrollExtent() })
	if total <= visible {
		return stream
	}
	thumb := lipgloss.NewStyle().Foreground(style.GetTheme().Muted)
	bar := bloom.Scrollbar(total, visible, offset).Style(bloom.ScrollbarStyle{ThumbStyle: thumb})
	// The layer is as tall as the transcript: its height is cut to the
	// overlay that it is in. It is transparent, so its track hides nothing.
	layer := bloom.Layer(bar).
		Anchor(bloom.End, bloom.Start).
		Size(1, layoutUnbounded).
		Transparent()
	if t < 1 {
		fg, bg := fxColors()
		layer = layer.Transition(t, fadeIn{Fg: fg, From: bg})
	}
	return bloom.Overlay(stream, layer)
}

// --------------------------------------------------------------------------
// Toast
// --------------------------------------------------------------------------

// toastExpiredMsg says that the time of a toast is up. timer is the number of
// the timer that sent it: a toast that was shown again since then has a new
// timer, and is not taken down by the one before it.
type toastExpiredMsg struct {
	id    toastID
	timer uint64
}

// showToast puts a short notice in the corner of the screen for a moment. It
// returns the command that takes it down again.
//
// Toasts stack: a new one goes below those that are up. A notice that says
// what a toast on screen says does not add one, it keeps that toast up for
// longer, so a key that the user holds down does not fill the corner.
//
// With motion on a toast slides in and out. With motion off it is there on
// the next frame and gone when its time is up. One timer takes it down in
// both cases.
func (m *AppModel) showToast(text string) tea.Cmd {
	return m.showToastFor(text, text)
}

// showToastFor is showToast for a notice about one thing that changes, such
// as the thinking level. key names the thing. While a toast with that key is
// up, a new notice takes the place of its text, so a user who steps through
// five levels sees one toast that changes, not five.
func (m *AppModel) showToastFor(key, text string) tea.Cmd {
	fxs := m.fxState()
	// A toast is one row. It does not wrap, and it leaves the transcript
	// some room beside it.
	text = strings.Join(strings.Fields(text), " ")
	text = ansi.Truncate(text, max(min(toastMaxWidth, m.width-8), 8), "…")

	var t *toast
	for _, up := range fxs.toasts {
		if up.key == key {
			t = up
		}
	}
	if t != nil {
		t.text, t.leaving = text, false
	} else {
		fxs.toastSeq++
		t = &toast{id: toastID(fxs.toastSeq), key: key, text: text, shown: anim.NewValue(0)}
		fxs.toasts = append(fxs.toasts, t)
		if over := len(fxs.toasts) - toastMax; over > 0 {
			fxs.toasts = fxs.toasts[over:]
		}
	}
	fxs.move(&t.shown, 1, toastFade, m.motionOn())

	t.timer++
	expired := toastExpiredMsg{id: t.id, timer: t.timer}
	return tea.Tick(toastHold, func(time.Time) tea.Msg { return expired })
}

// notify shows a toast from any place in an Update, such as a handler that
// returns no command. Update sends the command of the toast out with its own.
// key is as for showToastFor; with no key, the text is the key.
func (m *AppModel) notify(key, text string) {
	if key == "" {
		key = text
	}
	cmd := m.showToastFor(key, text)
	fxs := m.fxState()
	fxs.pending = append(fxs.pending, cmd)
}

// takePending returns the commands that notify kept, and lets go of them.
func (fxs *motionState) takePending() tea.Cmd {
	if len(fxs.pending) == 0 {
		return nil
	}
	cmd := tea.Batch(fxs.pending...)
	fxs.pending = nil
	return cmd
}

// expireToast takes down the toast that msg is the timer of, if that timer
// is still the one that the toast waits for.
func (m *AppModel) expireToast(msg toastExpiredMsg) {
	fxs := m.fxState()
	for _, t := range fxs.toasts {
		if t.id == msg.id && t.timer == msg.timer {
			t.leaving = true
			fxs.move(&t.shown, 0, toastFade, m.motionOn())
		}
	}
	fxs.pruneToasts()
}

// pruneToasts lets go of the toasts that have gone out of sight.
func (fxs *motionState) pruneToasts() {
	kept := fxs.toasts[:0]
	for _, t := range fxs.toasts {
		if t.leaving && !t.shown.Animating() && t.shown.Float() <= 0 {
			continue
		}
		kept = append(kept, t)
	}
	clear(fxs.toasts[len(kept):])
	fxs.toasts = kept
}

// toastBox draws the box of one toast.
func toastBox(text string) string {
	// The border is the muted text colour, not a border colour: the theme's
	// border colours are for dividers and are too close to the background
	// to set a notice apart from the conversation beneath it.
	theme := style.GetTheme()
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Muted).
		Foreground(theme.Text).
		Padding(0, 1).
		Render(text)
}

// toastLayer returns the layer that draws the toasts, if one is on screen.
//
// The toasts are a column against the right edge. Each has an ID, so that
// with glide set, when one goes the toasts below it glide up into its place.
func (m *AppModel) toastLayer(glide bool) (bloom.LayerNode, bool) {
	fxs := m.fxState()
	if len(fxs.toasts) == 0 {
		return bloom.LayerNode{}, false
	}
	fg, bg := fxColors()
	rows := make([]bloom.Child, 0, len(fxs.toasts))
	for _, t := range fxs.toasts {
		var box bloom.Child = region(toastBox(t.text))
		if p := min(max(t.shown.Float(), 0), 1); p < 1 {
			box = bloom.Transition(p, box, fadeIn{Fg: fg, From: bg}, fx.Slide{From: bloom.Right})
		}
		slot := bloom.Auto(box).ID(t.id).Align(bloom.End)
		if glide {
			slot = slot.Animate(anim.Spring(layoutSpring, 1))
		}
		rows = append(rows, slot)
	}
	layer := bloom.Layer(bloom.Column(rows...)).
		Anchor(bloom.End, bloom.Start).
		Offset(-1, 0)
	return layer, true
}

// --------------------------------------------------------------------------
// Feedback
// --------------------------------------------------------------------------

// shakeComposer shakes the composer once, to say that input was rejected. It
// does nothing while motion is off.
func (m *AppModel) shakeComposer() {
	if !m.motionOn() {
		return
	}
	fxs := m.fxState()
	fxs.shake.Set(0)
	fxs.move(&fxs.shake, 1, shakeDuration, true, anim.Ease(anim.Linear))
}

// composerChild returns the composer section for the layout: fading in if
// what it holds has just changed, and shaking if it is.
func (m *AppModel) composerChild(view string) bloom.Child {
	fxs := m.fxState()
	var child bloom.Child = section(view)
	if fxs.swap.Animating() {
		fg, bg := fxColors()
		child = bloom.Transition(fxs.swap.Float(), child, fadeIn{Fg: fg, From: bg})
	}
	if fxs.shake.Animating() {
		child = bloom.Transition(fxs.shake.Float(), child, fx.Shake{Amount: 2, Times: 3})
	}
	return child
}

// inputKind says what the composer section holds: the composer itself, a
// prompt, or the bar of a child pane. Two prompts are two kinds, so a prompt
// that follows another one fades in too.
type inputKind struct {
	prompt *promptOverlay
	child  bool
}

// syncInputKind starts the fade of the composer section when what it holds
// has changed. The section glides to its new height by itself; without the
// fade, the new content would be there in full on the first frame of that
// glide, cut off by the old height.
func (m *AppModel) syncInputKind(animate bool) {
	var kind inputKind
	switch {
	case m.state == statePrompt && m.prompt != nil:
		kind.prompt = m.prompt
	case m.childView != nil && m.child != nil:
		kind.child = true
	}
	fxs := m.fxState()
	if fxs.inputKind == nil {
		// The first frame: there is nothing that it changed from.
		fxs.inputKind = kind
		return
	}
	if fxs.inputKind == kind {
		return
	}
	fxs.inputKind = kind
	if animate {
		fxs.swap.Set(0)
		fxs.move(&fxs.swap, 1, swapFade, true)
	}
}

// --------------------------------------------------------------------------
// Transcript
// --------------------------------------------------------------------------

// enterPane starts the transition of a transcript that has just taken the
// place of another one: the pane of a subagent, or the parent pane again.
// from is the side that it comes from: 1 for the right, -1 for the left.
func (m *AppModel) enterPane(from int) {
	if !m.motionOn() {
		return
	}
	fxs := m.fxState()
	fxs.paneFrom = from
	fxs.pane.Set(0)
	fxs.move(&fxs.pane, 1, paneFade, true)
	// The items of the pane are not new. They come in with the pane.
	fxs.entries = nil
}

// entry is one item of the transcript that fades in.
type entry struct {
	id string
	// shown runs from 0, the colour of the background, to 1.
	shown anim.Value
}

// transcriptMark says where the transcript ended before an Update, so that
// the items that the Update added can be told from those that were there.
type transcriptMark struct {
	pane *conversationPane
	n    int
	last string
}

// markTranscript returns the mark of the transcript on screen.
func (m *AppModel) markTranscript() transcriptMark {
	mark := transcriptMark{pane: m.conversationPane}
	if m.conversationPane == nil {
		return mark
	}
	mark.n = len(m.messages)
	if mark.n > 0 && m.messages[mark.n-1] != nil {
		mark.last = m.messages[mark.n-1].ID()
	}
	return mark
}

// noteNewItems starts the fade of the items that were added to the end of
// the transcript since mark was taken.
//
// Only a few items at the end count. A transcript that was cleared, that got
// an item in its middle, or that got a whole session at once is drawn as it
// is, and so is the transcript of another pane.
func (m *AppModel) noteNewItems(mark transcriptMark) {
	if !m.motionOn() || m.conversationPane == nil || m.conversationPane != mark.pane {
		return
	}
	added := len(m.messages) - mark.n
	if added <= 0 || added > entryMax {
		return
	}
	if mark.n > 0 && (m.messages[mark.n-1] == nil || m.messages[mark.n-1].ID() != mark.last) {
		return
	}
	fxs := m.fxState()
	for _, item := range m.messages[mark.n:] {
		if item == nil {
			continue
		}
		e := &entry{id: item.ID(), shown: anim.NewValue(0)}
		fxs.move(&e.shown, 1, entryFade, true)
		fxs.entries = append(fxs.entries, e)
	}
}

// pruneEntries lets go of the items that have finished their fade.
func (fxs *motionState) pruneEntries() {
	kept := fxs.entries[:0]
	for _, e := range fxs.entries {
		if e.shown.Animating() {
			kept = append(kept, e)
		}
	}
	clear(fxs.entries[len(kept):])
	fxs.entries = kept
}

// transcriptChild returns the transcript for the layout. view is the scroll
// list drawn at a height of rows.
//
// Two transitions can run on it. A pane that has just come on screen fades in
// and travels a few columns from the side it came from. An item that is new
// fades in on the rows that it has in this view. The rows are those of the
// list at this height: while a layout animation runs, the transcript is
// drawn at heights that are not the list's own, and an item is then on other
// rows (see ScrollList.atHeight).
func (m *AppModel) transcriptChild(view string, rows int) bloom.Child {
	var child bloom.Child = section(view)
	if m.fx == nil || m.scrollList == nil {
		return child
	}
	fxs := m.fx
	fg, bg := fxColors()

	if len(fxs.entries) > 0 {
		var visible []VisibleItem
		if rows == m.scrollList.height {
			visible = m.scrollList.VisibleItems()
		} else {
			m.scrollList.atHeight(rows, func() { visible = m.scrollList.VisibleItems() })
		}
		for _, e := range fxs.entries {
			for _, v := range visible {
				if v.Item == nil || v.Item.ID() != e.id {
					continue
				}
				child = bloom.Transition(min(max(e.shown.Float(), 0), 1), child,
					fadeRows{Start: v.Row, End: v.Row + v.Height, Fg: fg, From: bg})
			}
		}
	}

	if fxs.pane.Animating() {
		effects := []fx.Effect{fadeIn{Fg: fg, From: bg}}
		// An image that is placed directly on the screen does not travel
		// with the cells, so a pane that shows one fades where it is.
		//
		// The test is the view that is about to be drawn, with the rule that
		// places the images after it. The placements of the last frame are
		// not the test: on the first frame of a pane they are those of the
		// pane that was there before.
		if len(m.transcriptPlacements(1)) == 0 {
			effects = append(effects, shift{Cols: paneShift * fxs.paneFrom})
		}
		child = bloom.Transition(min(max(fxs.pane.Float(), 0), 1), child, effects...)
	}
	return child
}

// activityChild returns the activity row for the layout. While the agent
// works, a highlight crosses the phrase that says what it is doing.
func (m *AppModel) activityChild(view string) bloom.Child {
	if view == "" || !m.motionOn() || m.activityPhraseEnd <= m.activityPhraseStart || m.turnStartedAt.IsZero() {
		return section(view)
	}
	cycle := shimmerSweep + shimmerRest
	phase := float64(time.Since(m.turnStartedAt)%cycle) / float64(shimmerSweep)
	if phase >= 1 {
		return section(view) // at rest between two sweeps
	}
	fg, _ := fxColors()
	return bloom.Transition(phase, section(view), phraseShimmer{
		Start: m.activityPhraseStart,
		End:   m.activityPhraseEnd,
		Fg:    fg,
		Color: rgba(style.GetTheme().Primary),
	})
}

// --------------------------------------------------------------------------
// Effects
// --------------------------------------------------------------------------

// isImageCell reports whether c shows part of an image through the Kitty
// placeholder protocol. Its foreground colour is the image id.
func isImageCell(c *uv.Cell) bool {
	return strings.HasPrefix(c.Content, string(kitty.Placeholder))
}

// dimScreen moves the colours of every cell toward To, which stands for the
// background, so that what is drawn over the screen stands out. Fg is the
// colour of text that has none of its own.
//
// Unlike fx.Dim it gives no background to a cell that has none: see the rules
// at the top of this file.
type dimScreen struct {
	Fg, To color.RGBA
	Amount float64
}

// Apply implements fx.Effect.
func (e dimScreen) Apply(s *canvas.Surface, t float64) {
	f := e.Amount * t
	if f <= 0 {
		return
	}
	for _, c := range s.Cells() {
		if isImageCell(c) {
			continue
		}
		if c.Style.Bg != nil {
			c.Style.Bg = s.Mix(c.Style.Bg, e.To, f)
		}
		// A blank cell with no attributes shows no foreground. Leaving it
		// unstyled keeps the dimmed frame as short as the frame it dims.
		if blankCell(c) {
			continue
		}
		fg := c.Style.Fg
		if fg == nil {
			fg = e.Fg
		}
		c.Style.Fg = s.Mix(fg, e.To, f)
	}
}

// blankCell reports whether c is a space that draws nothing in its
// foreground colour: one with no underline and no attribute such as reverse.
func blankCell(c *uv.Cell) bool {
	return (c.Content == " " || c.Content == "") && c.Style.Fg == nil &&
		c.Style.Attrs == 0 && c.Style.Underline == 0
}

// fadeIn brings a node's colours in from From, which stands for the
// background, as t runs from 0 to 1. Fg is the colour of text that has none of
// its own. At 1 the node is left as it is.
type fadeIn struct {
	Fg, From color.RGBA
}

// Apply implements fx.Effect.
func (e fadeIn) Apply(s *canvas.Surface, t float64) {
	if t >= 1 {
		return
	}
	t = max(t, 0)
	for _, c := range s.Cells() {
		e.cell(s, c, t)
	}
}

// cell brings the colours of one cell in from the background.
func (e fadeIn) cell(s *canvas.Surface, c *uv.Cell, t float64) {
	if c == nil || isImageCell(c) {
		return
	}
	if c.Style.Bg != nil {
		c.Style.Bg = s.Mix(e.From, c.Style.Bg, t)
	}
	if blankCell(c) {
		return
	}
	fg := c.Style.Fg
	if fg == nil {
		fg = e.Fg
	}
	c.Style.Fg = s.Mix(e.From, fg, t)
}

// fadeRows is fadeIn for the rows from Start up to End of a node. The rest of
// the node is left as it is. It brings in one item of the transcript, which
// is a string that holds every item in view.
type fadeRows struct {
	Start, End int
	Fg, From   color.RGBA
}

// Apply implements fx.Effect.
func (e fadeRows) Apply(s *canvas.Surface, t float64) {
	if t >= 1 {
		return
	}
	t = max(t, 0)
	fade := fadeIn{Fg: e.Fg, From: e.From}
	for y := max(e.Start, 0); y < min(e.End, s.Height()); y++ {
		for x := range s.Width() {
			fade.cell(s, s.Cell(x, y), t)
		}
	}
}

// shift moves a node sideways into its place, from Cols columns to its right
// as t runs from 0 to 1, or from its left when Cols is less than zero. Like
// rise, it is a slide for a short distance.
type shift struct {
	Cols int
}

// Apply implements fx.Effect.
func (e shift) Apply(s *canvas.Surface, t float64) {
	if t >= 1 {
		return
	}
	dx := int(math.Round(float64(e.Cols) * (1 - max(t, 0))))
	s.Offset = s.Offset.Add(image.Pt(dx, 0))
}

// rise moves a node up into its place from Rows below it as t runs from 0 to
// 1. It is fx.Slide for a short distance: a dialog that crossed half the
// screen each time it opened would be tiring.
type rise struct {
	Rows int
}

// Apply implements fx.Effect.
func (e rise) Apply(s *canvas.Surface, t float64) {
	if t >= 1 {
		return
	}
	dy := int(math.Round(float64(e.Rows) * (1 - max(t, 0))))
	s.Offset = s.Offset.Add(image.Pt(0, dy))
}

// phraseShimmer sweeps a highlight across the columns from Start up to End of
// a row, and leaves the rest of the row alone. t is the position of the
// highlight, from 0, just before the first column, to 1, just past the last.
//
// It moves the colour of the text toward Color and leaves the background of
// each cell as it is, so it reads as a glint on the letters. fx.Shimmer tints
// the background too, which suits a placeholder bar but would put a block of
// colour behind text. Fg is the colour of text that has none of its own.
type phraseShimmer struct {
	Start, End int
	Fg, Color  color.RGBA
}

// shimmerStrength is how far the text at the middle of the highlight moves
// toward its colour.
const shimmerStrength = 0.85

// Apply implements fx.Effect.
func (e phraseShimmer) Apply(s *canvas.Surface, t float64) {
	end := min(e.End, s.Width())
	if e.Start >= end || t <= 0 || t >= 1 {
		return
	}
	half := float64(shimmerWidth) / 2
	// The middle of the highlight travels from half a band before the first
	// column to half a band past the last, so it enters and leaves smoothly.
	center := float64(e.Start) - half + t*(float64(end-e.Start)+float64(shimmerWidth))
	for y := range s.Height() {
		for x := e.Start; x < end; x++ {
			d := math.Abs(float64(x) + 0.5 - center)
			if d >= half {
				continue
			}
			c := s.Cell(x, y)
			if c == nil || c.Width == 0 || isImageCell(c) {
				continue
			}
			fg := c.Style.Fg
			if fg == nil {
				fg = e.Fg
			}
			// Strongest in the middle, fading to nothing at the edges.
			k := 1 - d/half
			c.Style.Fg = s.Mix(fg, e.Color, shimmerStrength*k*k*(3-2*k))
		}
	}
}
