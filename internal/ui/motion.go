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

	// layoutGlide is how long a layout section takes to move or resize.
	layoutGlide = 180 * time.Millisecond

	// shakeDuration is how long the composer shakes after rejected input.
	shakeDuration = 380 * time.Millisecond

	// toastHold is how long a toast stays up; toastFade is its transition.
	toastHold = 1600 * time.Millisecond
	toastFade = 160 * time.Millisecond

	// shimmerSweep is how long the highlight takes to cross the activity
	// phrase, and shimmerRest the pause before it crosses again.
	shimmerSweep = 1100 * time.Millisecond
	shimmerRest  = 900 * time.Millisecond

	// shimmerWidth is the width of the highlight band in columns.
	shimmerWidth = 8

	// motionFPS is the frame rate of transitions and layout animation. It is
	// Bubble Tea's own maximum by default, so no frame is drawn for nothing.
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
}

// surfaceCount is the number of modal surfaces.
const surfaceCount = 13

// surfaceSpecs lists the modal surfaces in z-order: a later one is drawn over
// an earlier one.
var surfaceSpecs = [surfaceCount]surfaceSpec{
	// Slash and @ autocomplete popup. It completes what the user types, so
	// the composer stays at full brightness behind it.
	{
		open: func(m *AppModel) bool {
			ic, ok := m.input.(*InputComponent)
			return ok && ic.PopupVisible()
		},
		render: func(m *AppModel) (string, string) {
			ic, _ := m.input.(*InputComponent)
			return ic.RenderPopupBox(m.width, m.height), ""
		},
	},
	{
		open:   func(m *AppModel) bool { return m.state == stateModelSelector && m.modelSelector != nil },
		render: func(m *AppModel) (string, string) { return m.modelSelector.RenderOverlay(), "" },
		scrim:  true,
	},
	{
		open: func(m *AppModel) bool {
			return m.state == stateThinkingLevelSelector && m.thinkingLevelSelector != nil
		},
		render: func(m *AppModel) (string, string) { return m.thinkingLevelSelector.RenderOverlay(), "" },
		scrim:  true,
	},
	{
		open:   func(m *AppModel) bool { return m.state == stateModelModeSelector && m.modelModeSelector != nil },
		render: func(m *AppModel) (string, string) { return m.modelModeSelector.RenderOverlay(), "" },
		scrim:  true,
	},
	{
		open:   func(m *AppModel) bool { return m.killSubagentOpen() },
		render: func(m *AppModel) (string, string) { return m.killSubagentSelector.RenderOverlay(), "" },
		scrim:  true,
	},
	{
		open:   func(m *AppModel) bool { return m.subagentRunPickerOpen() },
		render: func(m *AppModel) (string, string) { return m.subagentRunPicker.RenderOverlay(), "" },
		scrim:  true,
	},
	// The theme selector previews each theme on the conversation behind it,
	// so that conversation must keep its true colours.
	{
		open:   func(m *AppModel) bool { return m.state == stateThemeSelector && m.themeSelector != nil },
		render: func(m *AppModel) (string, string) { return m.themeSelector.RenderOverlay(), "" },
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
		open:   func(m *AppModel) bool { return m.sheet != nil },
		render: func(m *AppModel) (string, string) { return m.renderSheet(), "" },
		scrim:  true,
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

	// toast is the notice in the corner of the screen.
	toast     anim.Presence
	toastText string

	// anims are all of the values above, for anim.Update.
	anims []anim.Animation
}

// newMotionState returns the animation state at rest.
func newMotionState() *motionState {
	fxs := &motionState{
		scrim: anim.NewValue(0, anim.Duration(scrimFade), anim.FPS(motionFPS)),
		shake: anim.NewValue(1, anim.Duration(shakeDuration), anim.Ease(anim.Linear), anim.FPS(motionFPS)),
		toast: anim.NewPresence(anim.Duration(toastFade), anim.FPS(motionFPS)),
	}
	fxs.anims = append(fxs.anims, &fxs.scrim, &fxs.shake, &fxs.toast)
	for i := range fxs.surfaces {
		fxs.surfaces[i].shown = anim.NewValue(0, anim.Duration(surfaceFade), anim.FPS(motionFPS))
		fxs.anims = append(fxs.anims, &fxs.surfaces[i].shown)
	}
	return fxs
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
// open, and returns the commands that run the transitions it started.
//
// Update calls it with animate set after each message. View calls it too,
// without animate, for the surfaces that changed with no Update in between;
// View cannot return a command, so those jump to their end state.
func (m *AppModel) syncSurfaces(animate bool) tea.Cmd {
	fxs := m.fxState()
	var cmds []tea.Cmd
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
		cmds = append(cmds, moveValue(&a.shown, open, animate && !spec.still))
	}
	if scrim != fxs.scrimOn {
		fxs.scrimOn = scrim
		cmds = append(cmds, moveValue(&fxs.scrim, scrim, animate))
	}
	return tea.Batch(cmds...)
}

// moveValue sends v to 1 when on and to 0 when not, as an animation or at
// once.
func moveValue(v *anim.Value, on, animate bool) tea.Cmd {
	target := 0.0
	if on {
		target = 1
	}
	if animate {
		return v.To(target)
	}
	v.Set(target)
	return nil
}

// updateMotion passes msg to every animation and returns their commands. Each
// animation ignores the messages that are not its own.
func (m *AppModel) updateMotion(msg tea.Msg) tea.Cmd {
	return anim.Update(msg, m.fxState().anims...)
}

// bloomPackage is the import path that every bloom package starts with.
const bloomPackage = "bloomfx.dev/bloom"

// isMotionMsg reports whether msg belongs to bloom: an animation frame, a
// layout animation tick, or one of its timers.
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
	fxs := m.fxState()
	moving := fxs.animating()
	return tea.Batch(m.root().Update(msg), m.updateMotion(msg), fxs.settled(moving))
}

// settled returns the command that sends motionSettledMsg, if a transition
// was moving before the message in hand and none is moving now.
func (fxs *motionState) settled(moving bool) tea.Cmd {
	if !moving || fxs.animating() {
		return nil
	}
	return func() tea.Msg { return motionSettledMsg{} }
}

// animating reports whether a transition or feedback effect is running.
func (fxs *motionState) animating() bool {
	if fxs.scrim.Animating() || fxs.shake.Animating() || fxs.toast.Animating() {
		return true
	}
	for i := range fxs.surfaces {
		if fxs.surfaces[i].shown.Animating() {
			return true
		}
	}
	return false
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
// and how far the screen beneath them is dimmed.
func (m *AppModel) surfaceLayers() (layers []bloom.LayerNode, scrim float64) {
	// A surface that changed without an Update in between has no transition
	// to run; put it in its end state.
	_ = m.syncSurfaces(false)

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
		rect, ok := overlayRect(a.box, a.anchor, m.width, m.height)
		if !ok {
			continue
		}
		layer := bloom.Layer(region(a.box)).Place(rect)
		if t < 1 {
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
// Toast
// --------------------------------------------------------------------------

// showToast puts a short notice in the corner of the screen for a moment. It
// returns the command that shows and then hides it.
func (m *AppModel) showToast(text string) tea.Cmd {
	fxs := m.fxState()
	fade := toastFade
	if !m.motionOn() {
		fade = time.Millisecond
	}
	// A new presence, in the same place: the hide timer of the toast before
	// it then belongs to nothing, and cannot hide this one early.
	fxs.toast = anim.NewPresence(anim.Duration(fade), anim.FPS(motionFPS))
	fxs.toastText = text
	return fxs.toast.ShowFor(toastHold)
}

// toastLayer returns the layer that draws the toast, if one is on screen.
func (m *AppModel) toastLayer() (bloom.LayerNode, bool) {
	fxs := m.fxState()
	if fxs.toastText == "" || !fxs.toast.Visible() {
		return bloom.LayerNode{}, false
	}
	theme := style.GetTheme()
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.MutedBorder).
		Foreground(theme.Text).
		Padding(0, 1).
		Render(fxs.toastText)

	fg, bg := fxColors()
	layer := bloom.Layer(region(box)).
		Anchor(bloom.End, bloom.Start).
		Offset(-1, 0)
	if t := fxs.toast.Progress(); t < 1 {
		layer = layer.Transition(t, fadeIn{Fg: fg, From: bg}, fx.Slide{From: bloom.Right})
	}
	return layer, true
}

// --------------------------------------------------------------------------
// Feedback
// --------------------------------------------------------------------------

// shakeComposer shakes the composer once, to say that input was rejected. It
// returns the command that runs the shake, or nil while motion is off.
func (m *AppModel) shakeComposer() tea.Cmd {
	if !m.motionOn() {
		return nil
	}
	return m.fxState().shake.Play(0, 1)
}

// composerChild returns the composer for the layout, shaking if it is.
func (m *AppModel) composerChild(view string) bloom.Child {
	fxs := m.fxState()
	if !fxs.shake.Animating() {
		return section(view)
	}
	return bloom.Transition(fxs.shake.Float(), section(view), fx.Shake{Amount: 2, Times: 3})
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
		if isImageCell(c) {
			continue
		}
		if c.Style.Bg != nil {
			c.Style.Bg = s.Mix(e.From, c.Style.Bg, t)
		}
		if blankCell(c) {
			continue
		}
		fg := c.Style.Fg
		if fg == nil {
			fg = e.Fg
		}
		c.Style.Fg = s.Mix(e.From, fg, t)
	}
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
