package ui

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strings"
	"testing"
	"time"

	"bloomfx.dev/bloom"
	"bloomfx.dev/bloom/anim"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi/kitty"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/ui/commands"
	"github.com/mark3labs/kit/internal/ui/style"
)

// enableMotion turns motion on for m and gives the terminal the colours that
// motion needs, for the length of the test.
func enableMotion(t *testing.T, m *AppModel) {
	t.Helper()
	dark, profile := style.IsDarkBackground(), style.ColorProfile()
	t.Cleanup(func() { style.SetTerminalCapabilities(dark, profile) })
	style.SetTerminalCapabilities(true, colorprofile.TrueColor)
	m.motionEnabled = true
}

// limitColors gives the terminal a profile that cannot show a colour blend,
// for the length of the test.
func limitColors(t *testing.T) {
	t.Helper()
	dark, profile := style.IsDarkBackground(), style.ColorProfile()
	t.Cleanup(func() { style.SetTerminalCapabilities(dark, profile) })
	style.SetTerminalCapabilities(true, colorprofile.ANSI)
}

// openTestSheet opens a sheet with a title that a test can find in the frame.
func openTestSheet(m *AppModel) {
	m.openSheet("Motion test sheet", "", 40, func(int, helpStyles) []string {
		return []string{"body line one", "body line two"}
	})
}

// openSurface returns the animation state of the one surface that is open.
func openSurface(t *testing.T, m *AppModel) *surfaceAnim {
	t.Helper()
	fxs := m.fxState()
	var found *surfaceAnim
	for i := range surfaceSpecs {
		if surfaceSpecs[i].open(m) {
			if found != nil {
				t.Fatal("more than one surface is open")
			}
			found = &fxs.surfaces[i]
		}
	}
	if found == nil {
		t.Fatal("no surface is open")
	}
	return found
}

// rowOf returns the screen row of the first row of view that contains text,
// or -1.
func rowOf(view, text string) int {
	for i, row := range strings.Split(stripAnsi(view), "\n") {
		if strings.Contains(row, text) {
			return i
		}
	}
	return -1
}

// TestSurfaceCountMatchesSpecs keeps the list of surfaces complete: a surface
// with no open function would stop the frame.
func TestSurfaceCountMatchesSpecs(t *testing.T) {
	for i, spec := range surfaceSpecs {
		if spec.open == nil || spec.render == nil {
			t.Fatalf("surface %d of %d has no open or render function", i, surfaceCount)
		}
	}
}

// TestSurfaceJumpsWithoutMotion checks that a modal surface is fully drawn in
// its place on the first frame, and gone on the first frame after it closes,
// when motion is off.
func TestSurfaceJumpsWithoutMotion(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)

	openTestSheet(m)
	if _, cmd := m.Update(noopMsg{}); cmd != nil {
		t.Fatal("a surface that opens without motion must not schedule frames")
	}
	view := m.View().Content
	if rowOf(view, "Motion test sheet") < 0 {
		t.Fatalf("the sheet is not in the frame\n%s", stripAnsi(view))
	}
	if a := openSurface(t, m); a.shown.Float() != 1 || a.shown.Animating() {
		t.Fatalf("shown = %v (animating %v), want 1 at rest", a.shown.Float(), a.shown.Animating())
	}

	m.sheet = nil
	m.Update(noopMsg{})
	if view := m.View().Content; rowOf(view, "Motion test sheet") >= 0 {
		t.Fatalf("the sheet is still in the frame after it closed\n%s", stripAnsi(view))
	}
	if m.surfacesVisible() {
		t.Fatal("a surface is still visible after it closed")
	}
}

// TestSurfaceOpensWithTransition checks that a surface that opens with motion
// on schedules frames, starts below its place, and ends in it.
func TestSurfaceOpensWithTransition(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)

	m.themeSelector = NewThemeSelector("", m.width, m.height)
	m.state = stateThemeSelector
	if _, cmd := m.Update(noopMsg{}); cmd == nil {
		t.Fatal("a surface that opens with motion must schedule its first frame")
	}
	a := openSurface(t, m)
	if !a.shown.Animating() || a.shown.Target() != 1 {
		t.Fatalf("animating=%v target=%v, want an animation toward 1", a.shown.Animating(), a.shown.Target())
	}

	// The top border of the selector is the only rounded corner on screen.
	a.shown.Set(1)
	settled := rowOf(m.View().Content, "╭")
	if settled < 0 {
		t.Fatal("the selector is not in the frame at rest")
	}

	a.shown.Set(0)
	view := m.View().Content
	if got := rowOf(view, "╭"); got != settled+surfaceRise {
		t.Fatalf("top border at the start = %d, want %d rows below %d", got, surfaceRise, settled)
	}
	if got := lipgloss.Height(view); got != m.height {
		t.Fatalf("frame height during the transition = %d, want %d", got, m.height)
	}
}

// TestSheetFadesInPlace checks that a sheet does not travel as it appears. A
// sheet finds the line under the mouse from where its box belongs, so a box
// drawn a row lower would give a click to another line.
func TestSheetFadesInPlace(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)

	openTestSheet(m)
	if _, cmd := m.Update(noopMsg{}); cmd == nil {
		t.Fatal("a sheet that opens with motion must schedule its first frame")
	}
	a := openSurface(t, m)

	a.shown.Set(1)
	settled := m.View().Content
	row := rowOf(settled, "body line one")
	if row < 0 {
		t.Fatal("the sheet body is not in the frame at rest")
	}
	bodyY := m.sheet.bodyY

	for _, progress := range []float64{0, 0.3, 0.7} {
		a.shown.Set(progress)
		view := m.View().Content
		if got := rowOf(view, "body line one"); got != row {
			t.Fatalf("at progress %v the body is on row %d, want row %d as at rest", progress, got, row)
		}
		if m.sheet.bodyY != bodyY {
			t.Fatalf("at progress %v the hit-test origin is %d, want %d", progress, m.sheet.bodyY, bodyY)
		}
		// The sheet still has a transition: it is not yet as it is at rest.
		if view == settled {
			t.Fatalf("at progress %v the sheet is drawn as at rest: it has no transition", progress)
		}
	}
}

// TestSurfaceAnimatesOutAfterClose checks that a closed surface stays drawn,
// from the box it drew last, until its transition has run. For the images
// that stay hidden for that time, see TestDirectImageHidesWhileSurfaceLeaves.
func TestSurfaceAnimatesOutAfterClose(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)

	openTestSheet(m)
	m.Update(noopMsg{})
	a := openSurface(t, m)
	a.shown.Set(1)
	_ = m.View()

	m.sheet = nil
	if _, cmd := m.Update(noopMsg{}); cmd == nil {
		t.Fatal("a surface that closes with motion must schedule its first frame")
	}
	a.shown.Set(0.5) // halfway out
	if rowOf(m.View().Content, "Motion test sheet") < 0 {
		t.Fatal("the sheet left the frame before its transition had run")
	}
	if !m.surfacesVisible() {
		t.Fatal("surfacesVisible = false while a surface is on its way out")
	}

	a.shown.Set(0)
	if rowOf(m.View().Content, "Motion test sheet") >= 0 {
		t.Fatal("the sheet is still in the frame after its transition")
	}
	if a.box != "" {
		t.Fatal("the box of a surface that has gone is still kept")
	}
	if m.surfacesVisible() {
		t.Fatal("surfacesVisible = true after the surface has gone")
	}
}

// TestSurfaceLeavesInPlace checks that a surface that closes fades where it
// is. It must not travel back down the rows that it rose through, and it goes
// in less time than it came.
func TestSurfaceLeavesInPlace(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)
	fxs := m.fxState()
	now := time.Unix(1000, 0)
	fxs.now = func() time.Time { return now }

	m.themeSelector = NewThemeSelector("", m.width, m.height)
	m.state = stateThemeSelector
	m.Update(noopMsg{})
	a := openSurface(t, m)
	a.shown.Set(1)
	settled := rowOf(m.View().Content, "╭")

	m.themeSelector, m.state = nil, stateInput
	m.Update(noopMsg{})
	if !a.shown.Animating() || a.shown.Target() != 0 {
		t.Fatalf("animating=%v target=%v, want an animation toward 0", a.shown.Animating(), a.shown.Target())
	}
	if !m.frames.fast {
		t.Fatal("the frame clock is not fast while a surface goes")
	}

	// Halfway through the time that a surface takes to go, it is still in
	// the frame, on the rows it was on.
	now = now.Add(surfaceLeave / 2)
	fxs.advance()
	if got := rowOf(m.View().Content, "╭"); got != settled {
		t.Fatalf("top border on row %d while the surface goes, want row %d as at rest", got, settled)
	}

	// It is gone after surfaceLeave, which is before a surface that opens
	// has arrived.
	if surfaceLeave >= surfaceFade {
		t.Fatalf("a surface takes %v to go and %v to come: it must go faster", surfaceLeave, surfaceFade)
	}
	now = now.Add(surfaceLeave / 2)
	fxs.advance()
	if a.shown.Animating() || a.shown.Float() != 0 {
		t.Fatalf("shown = %v (animating %v) after surfaceLeave, want 0 at rest", a.shown.Float(), a.shown.Animating())
	}
	if rowOf(m.View().Content, "╭") >= 0 {
		t.Fatal("the surface is still in the frame after it went")
	}
}

// TestTransitionsShareTheFrameClock checks that a transition starts no loop
// of its own. When the clock already runs for the spinner, a surface that
// opens makes the clock fast and schedules nothing more; when the transition
// ends, the clock goes back to its own rate and keeps running.
func TestTransitionsShareTheFrameClock(t *testing.T) {
	m, stream, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)
	fxs := m.fxState()
	now := time.Unix(1000, 0)
	fxs.now = func() time.Time { return now }

	stream.spinning = true
	if m.wakeFrameClock() == nil || !m.frames.running {
		t.Fatal("the spinner did not start the frame clock")
	}
	if m.frames.fast {
		t.Fatal("the clock is fast with no transition")
	}

	openTestSheet(m)
	_, cmd := m.Update(noopMsg{})
	for _, msg := range flattenCmd(cmd) {
		if _, ok := msg.(frameTickMsg); ok {
			t.Fatal("a surface that opened while the clock ran scheduled a second tick")
		}
		if isMotionMsg(msg) {
			t.Fatalf("a surface that opened scheduled a frame of its own: %T", msg)
		}
	}
	if !m.frames.fast {
		t.Fatal("the clock did not go fast for the transition")
	}

	// The tick in flight is a beat, as it was scheduled before the surface
	// opened. The model answers it with a frame between two beats.
	m.Update(frameTickMsg{generation: m.frames.generation, frame: m.frames.frame + 1})
	if next, _ := m.frames.next(); !next.between {
		t.Fatal("a fast clock did not follow a beat with a frame in between")
	}

	// The transition ends: the clock stays up for the spinner, at its rate.
	now = now.Add(time.Second)
	m.Update(frameTickMsg{generation: m.frames.generation, frame: m.frames.frame, between: true})
	if fxs.animating() {
		t.Fatal("the transition did not end")
	}
	if !m.frames.running || m.frames.fast {
		t.Fatalf("running=%v fast=%v after the transition, want a slow clock that still runs", m.frames.running, m.frames.fast)
	}
}

// popupModel returns a model whose autocomplete popup shows n commands.
func popupModel(t *testing.T, n int) (*AppModel, *InputComponent) {
	t.Helper()
	m, _, _ := newTestAppModel(&stubAppController{})
	ic := NewInputComponent(80, nil)
	m.input = ic
	setPopupCommands(ic, n)
	return m, ic
}

// setPopupCommands makes the autocomplete popup of ic show n commands.
func setPopupCommands(ic *InputComponent, n int) {
	ic.showPopup = true
	ic.selected = 0
	ic.filtered = ic.filtered[:0]
	for i := range n {
		ic.filtered = append(ic.filtered, FuzzyMatch{Command: &commands.SlashCommand{
			Name:        fmt.Sprintf("/command-%d", i),
			Description: "a test command",
		}})
	}
}

// TestBoxAtHeight checks a box drawn at a height that is not its own. It is
// closed at every height: the top border stays, and so do the rows at its
// foot.
func TestBoxAtHeight(t *testing.T) {
	box := strings.Join([]string{"top", "pad", "title", "item 1", "item 2", "item 3", "blank", "footer", "pad", "bottom"}, "\n")
	const tail = 4
	foot := "blank\nfooter\npad\nbottom"

	if got := boxAtHeight(box, 10, tail); got != box {
		t.Fatalf("a box at its own height changed:\n%s", got)
	}
	if got := boxAtHeight(box, layoutUnbounded, tail); got != box {
		t.Fatal("a box asked for its natural size changed")
	}
	if got := boxAtHeight(box, 7, 0); got != box {
		t.Fatal("a box with no tail changed")
	}

	shorter := boxAtHeight(box, 7, tail)
	if want := "top\npad\ntitle\n" + foot; shorter != want {
		t.Fatalf("box at 7 rows:\n%s\nwant:\n%s", shorter, want)
	}
	taller := boxAtHeight(box, 12, tail)
	if want := "top\npad\ntitle\nitem 1\nitem 2\nitem 3\npad\npad\n" + foot; taller != want {
		t.Fatalf("box at 12 rows:\n%s\nwant:\n%s", taller, want)
	}
	// Too few rows even for the foot: the top border stays, and the layer
	// cuts off what does not fit.
	if got := boxAtHeight(box, 2, tail); got != "top\n"+foot {
		t.Fatalf("box at 2 rows:\n%s", got)
	}
}

// TestAutocompletePopupSitsOnTheComposer checks that the / popup is against
// the top edge of the composer, at the left of the screen, and not in the
// middle of it.
func TestAutocompletePopupSitsOnTheComposer(t *testing.T) {
	m, _ := popupModel(t, 3)
	m.layoutDirty = true
	view := m.View().Content

	rows := strings.Split(stripAnsi(view), "\n")
	top, bottom := rowOf(view, "╭"), rowOf(view, "╰")
	if top < 0 || bottom < 0 {
		t.Fatalf("the popup is not in the frame\n%s", stripAnsi(view))
	}
	if bottom != m.inputYOffset-1 {
		t.Fatalf("the popup ends on row %d, want row %d, just above the composer on row %d",
			bottom, m.inputYOffset-1, m.inputYOffset)
	}
	if !strings.HasPrefix(rows[top], "╭") {
		t.Fatalf("the popup is not at the left of the screen: %q", rows[top])
	}
	if lipgloss.Height(view) != m.height {
		t.Fatalf("frame height with the popup = %d, want %d", lipgloss.Height(view), m.height)
	}

	// A composer that grows takes the popup up with it.
	m.input.(*InputComponent).textarea.SetValue("one\ntwo\nthree\nfour")
	m.layoutDirty = true
	view = m.View().Content
	if got := rowOf(view, "╰"); got != m.inputYOffset-1 || got >= bottom {
		t.Fatalf("after the composer grew the popup ends on row %d, want row %d", got, m.inputYOffset-1)
	}
}

// TestPopupGlidesBetweenHeights checks a popup whose list gets shorter with
// motion on. The box passes through the heights between, it stays on the
// composer, and on every frame it is a closed box with its footer.
func TestPopupGlidesBetweenHeights(t *testing.T) {
	m, ic := popupModel(t, 6)
	enableMotion(t, m)
	clock := &testClock{now: time.Unix(1000, 0)}
	m.ui = bloom.New(bloom.WithClock(clock.Now), bloom.WithFPS(motionFPS))
	m.layoutDirty = true
	m.Update(noopMsg{})
	openSurface(t, m).shown.Set(1)
	first := m.View().Content
	tall := rowOf(first, "╰") - rowOf(first, "╭") + 1

	setPopupCommands(ic, 1)
	seenBetween := false
	short := 0
	for range 40 {
		view := m.View().Content
		clock.advance(layoutGlide / 20)
		top, bottom := rowOf(view, "╭"), rowOf(view, "╰")
		if bottom != m.inputYOffset-1 {
			t.Fatalf("the popup left the composer while it changed height: it ends on row %d, want %d\n%s",
				bottom, m.inputYOffset-1, stripAnsi(view))
		}
		if rowOf(view, "esc dismiss") != bottom-2 {
			t.Fatalf("the footer is not at the foot of the box\n%s", stripAnsi(view))
		}
		short = bottom - top + 1
		if short > tall {
			t.Fatalf("the popup has %d rows, more than the %d it had", short, tall)
		}
		if short < tall && short > tall-5 {
			seenBetween = true
		}
	}
	if !seenBetween {
		t.Fatal("the popup jumped: no frame had a height in between")
	}
	if short != tall-5 {
		t.Fatalf("the popup ended at %d rows, want %d", short, tall-5)
	}

	// With motion off the box takes its new height on the next frame.
	m.motionEnabled = false
	setPopupCommands(ic, 6)
	view := m.View().Content
	if got := rowOf(view, "╰") - rowOf(view, "╭") + 1; got != tall {
		t.Fatalf("with motion off the popup has %d rows on the next frame, want %d", got, tall)
	}
}

// TestScrimDimsScreenBehindModal checks that the conversation is dimmed
// behind a modal surface, that the theme selector leaves it as it is, and
// that a terminal with few colours gets no dimming at all.
func TestScrimDimsScreenBehindModal(t *testing.T) {
	newModel := func(t *testing.T) (*AppModel, string) {
		t.Helper()
		m, _, _ := newTestAppModel(&stubAppController{})
		m.input = NewInputComponent(80, nil)
		enableMotion(t, m)
		m.printSystemMessage("a line of the conversation")
		return m, m.View().Content
	}
	// conversation returns the frame row that holds the conversation line,
	// with its colours.
	conversation := func(t *testing.T, view string) string {
		t.Helper()
		for row := range strings.SplitSeq(view, "\n") {
			if strings.Contains(stripAnsi(row), "a line of the conversation") {
				return row
			}
		}
		t.Fatalf("the conversation line is not in the frame\n%s", stripAnsi(view))
		return ""
	}

	t.Run("a modal dims the screen", func(t *testing.T) {
		m, before := newModel(t)
		openTestSheet(m)
		m.Update(noopMsg{})
		m.fxState().scrim.Set(1)
		if conversation(t, m.View().Content) == conversation(t, before) {
			t.Fatal("the conversation has the same colours behind the sheet")
		}
	})

	t.Run("the theme selector does not", func(t *testing.T) {
		m, before := newModel(t)
		m.themeSelector = NewThemeSelector("", m.width, m.height)
		m.state = stateThemeSelector
		m.Update(noopMsg{})
		if m.fxState().scrimOn {
			t.Fatal("the theme selector dims the conversation that it previews themes on")
		}
		if conversation(t, m.View().Content) != conversation(t, before) {
			t.Fatal("the conversation changed colour behind the theme selector")
		}
	})

	t.Run("a terminal with few colours gets none", func(t *testing.T) {
		m, _ := newModel(t)
		limitColors(t)
		before := m.View().Content
		openTestSheet(m)
		m.Update(noopMsg{})
		if conversation(t, m.View().Content) != conversation(t, before) {
			t.Fatal("the conversation was dimmed on a terminal that cannot show the blend")
		}
	})
}

// imageRow returns a row of Kitty placeholder cells for image id, as
// imagepreview draws them, followed by plain coloured text.
func imageRow(id color.RGBA, text string) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(id).Render(
		string(kitty.Placeholder) + string(kitty.Diacritic(0)) + string(kitty.Diacritic(0)) +
			string(kitty.Placeholder) + string(kitty.Diacritic(0)) + string(kitty.Diacritic(1)),
	))
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ff8800")).Render(text))
	return b.String()
}

// cellsOf draws a rendered frame into cells, so that a test can read the
// colour of one cell.
func cellsOf(frame string, w, h int) uv.ScreenBuffer {
	buf := uv.NewScreenBuffer(w, h)
	uv.NewStyledString(frame).Draw(&buf, image.Rect(0, 0, w, h))
	return buf
}

// sameColor reports whether two colours are the same colour.
func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

// TestEffectsLeaveImageCellsAlone checks that the colour effects do not
// change the foreground of a Kitty placeholder cell. That colour is the image
// id: a blended one names another image.
//
// Each effect is given a row of two placeholder cells and a caption, and must
// reach all of it. The caption proves that: at least one of its cells changes
// colour. Then each placeholder cell is read back on its own.
func TestEffectsLeaveImageCellsAlone(t *testing.T) {
	const (
		width        = 20
		placeholders = 2
		captionEnd   = placeholders + len("caption")
	)
	id := color.RGBA{R: 0, G: 1, B: 42, A: 0xff}
	row := imageRow(id, "caption")
	plain := cellsOf(bloom.Render(width, 1, region(row)), width, 1)
	fg, bg := color.RGBA{R: 0xdd, G: 0xdd, B: 0xdd, A: 0xff}, color.RGBA{A: 0xff}

	// The highlight is in the middle of its band at column 2: the band then
	// covers the two placeholder cells and the start of the caption.
	const shimmerAt = 2.0
	shimmerPhase := (shimmerAt + shimmerWidth/2.0) / float64(captionEnd+shimmerWidth)

	for name, effect := range map[string]bloom.Child{
		"dimScreen": bloom.Transition(1, region(row), dimScreen{Fg: fg, To: bg, Amount: 0.5}),
		"fadeIn":    bloom.Transition(0.4, region(row), fadeIn{Fg: fg, From: bg}),
		"fadeRows":  bloom.Transition(0.4, region(row), fadeRows{Start: 0, End: 1, Fg: fg, From: bg}),
		"shimmer": bloom.Transition(shimmerPhase, region(row),
			phraseShimmer{Start: 0, End: captionEnd, Fg: fg, Color: color.RGBA{R: 0xff, A: 0xff}}),
	} {
		t.Run(name, func(t *testing.T) {
			got := cellsOf(bloom.Render(width, 1, effect), width, 1)

			// The effect reached the cells next to the image.
			reached := false
			for x := placeholders; x < placeholders+2; x++ {
				if !sameColor(got.CellAt(x, 0).Style.Fg, plain.CellAt(x, 0).Style.Fg) {
					reached = true
				}
			}
			if !reached {
				t.Fatal("the effect did not change the caption beside the image: the test checks nothing")
			}

			for x := range placeholders {
				c := got.CellAt(x, 0)
				if !strings.HasPrefix(c.Content, string(kitty.Placeholder)) {
					t.Fatalf("cell %d is %q, want a placeholder cell", x, c.Content)
				}
				if c.Content != plain.CellAt(x, 0).Content {
					t.Fatalf("cell %d changed: %q, want %q", x, c.Content, plain.CellAt(x, 0).Content)
				}
				if !sameColor(c.Style.Fg, id) {
					t.Fatalf("placeholder cell %d has foreground %v, want the image id %v", x, c.Style.Fg, id)
				}
			}
		})
	}
}

// TestEffectsGiveNoBackgroundToPlainCells checks the rule that makes the
// effects safe without the terminal's real background colour: a cell that has
// no background of its own gets none.
func TestEffectsGiveNoBackgroundToPlainCells(t *testing.T) {
	row := lipgloss.NewStyle().Bold(true).Render("Thinking") + "  and plain text"
	fg, bg := color.RGBA{R: 0xdd, G: 0xdd, B: 0xdd, A: 0xff}, color.RGBA{A: 0xff}

	for name, effect := range map[string]bloom.Child{
		"dimScreen": bloom.Transition(1, region(row), dimScreen{Fg: fg, To: bg, Amount: 0.5}),
		"fadeIn":    bloom.Transition(0.4, region(row), fadeIn{Fg: fg, From: bg}),
		"fadeRows":  bloom.Transition(0.4, region(row), fadeRows{Start: 0, End: 1, Fg: fg, From: bg}),
		"shimmer":   bloom.Transition(0.5, region(row), phraseShimmer{Start: 0, End: 8, Fg: fg, Color: color.RGBA{R: 0xff, A: 0xff}}),
	} {
		t.Run(name, func(t *testing.T) {
			got := bloom.Render(30, 1, effect)
			if strings.Contains(got, "48;") {
				t.Fatalf("the effect set a background colour: %q", got)
			}
			if stripAnsi(got) != stripAnsi(bloom.Render(30, 1, region(row))) {
				t.Fatalf("the effect changed the text: %q", stripAnsi(got))
			}
		})
	}
}

// TestPhraseShimmerStaysInsideThePhrase checks that the highlight colours
// only the columns of the phrase.
func TestPhraseShimmerStaysInsideThePhrase(t *testing.T) {
	row := "abcdefgh" + "ijklmnop"
	fg := color.RGBA{R: 0xdd, G: 0xdd, B: 0xdd, A: 0xff}
	got := bloom.Render(16, 1, bloom.Transition(0.5, region(row),
		phraseShimmer{Start: 0, End: 8, Fg: fg, Color: color.RGBA{R: 0xff, A: 0xff}}))
	if !strings.Contains(got, "38;2;") {
		t.Fatalf("no cell was coloured: %q", got)
	}
	// The rest of the row follows a reset, with no colour of its own.
	if !strings.HasSuffix(got, "\x1b[mijklmnop") {
		t.Fatalf("the highlight reached past the phrase: %q", got)
	}
}

// TestActivityPhraseColumns checks that the activity row records where its
// phrase is, and forgets it when the row is not drawn.
func TestActivityPhraseColumns(t *testing.T) {
	m, stream := newTestAppModelWithRealStream(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	m.width = 80
	m.state = stateWorking
	stream.spinning = true

	row := m.renderActivityRow()
	if row == "" {
		t.Fatal("no activity row while working")
	}
	plain := stripAnsi(row)
	if m.activityPhraseEnd <= m.activityPhraseStart {
		t.Fatalf("phrase columns = %d..%d, want a range", m.activityPhraseStart, m.activityPhraseEnd)
	}
	if got := string([]rune(plain)[m.activityPhraseStart:m.activityPhraseEnd]); got != stream.ActivityPhrase() {
		t.Fatalf("the phrase columns hold %q, want %q", got, stream.ActivityPhrase())
	}

	m.state = stateInput
	stream.spinning = false
	if m.renderActivityRow() != "" || m.activityPhraseEnd != 0 {
		t.Fatalf("idle row kept phrase columns %d..%d", m.activityPhraseStart, m.activityPhraseEnd)
	}
}

// toastRow returns the screen row of the toast text in the frame, or -1.
func toastRow(m *AppModel) int {
	return rowOf(m.View().Content, "Copied to the test")
}

// TestToastShowsAndHides checks a toast with motion off. It is drawn in the
// top right corner on the first frame, no animation runs for it, and one
// timer takes it down.
func TestToastShowsAndHides(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)

	if cmd := m.showToast("Copied to the test"); cmd == nil {
		t.Fatal("showToast returned no command: nothing would take the toast down")
	}
	fxs := m.fxState()
	if fxs.animating() {
		t.Fatal("a toast shown with motion off started an animation")
	}
	if cmd := m.wakeFrameClock(); cmd != nil {
		t.Fatal("a toast shown with motion off woke the frame clock")
	}

	// The toast must be there already, in its place.
	view := m.View().Content
	row := rowOf(view, "Copied to the test")
	if row != 1 {
		t.Fatalf("toast text on row %d, want row 1 (inside its border)\n%s", row, stripAnsi(view))
	}
	line := strings.Split(stripAnsi(view), "\n")[row]
	if !strings.HasSuffix(strings.TrimRight(line, " "), "│") || lipgloss.Width(strings.TrimRight(line, " ")) != m.width-1 {
		t.Fatalf("toast is not against the right edge: %q", line)
	}
	if got := lipgloss.Height(view); got != m.height {
		t.Fatalf("frame height with a toast = %d, want %d", got, m.height)
	}

	top := fxs.toasts[0]
	if _, cmd := m.Update(toastExpiredMsg{id: top.id, timer: top.timer}); cmd != nil {
		t.Fatal("the timer of a toast started more work")
	}
	if toastRow(m) >= 0 {
		t.Fatal("the toast is still drawn after its time was up")
	}
	if len(fxs.toasts) != 0 {
		t.Fatalf("%d toast(s) kept after the last one went", len(fxs.toasts))
	}
}

// TestToastTimerBelongsToItsToast checks that a timer takes down only its
// own toast, that the timer of a toast that was shown again does nothing, and
// that no toast timer flushes the buffered stream chunks as an event would.
func TestToastTimerBelongsToItsToast(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	fxs := m.fxState()

	m.showToast("first toast")
	first := *fxs.toasts[0]
	m.showToast("Copied to the test")
	second := *fxs.toasts[1]
	if first.id == second.id {
		t.Fatal("two toasts have the same number")
	}

	m.pendingStreamChunks = []pendingStreamChunk{{role: "assistant", content: "buffered"}}
	m.Update(toastExpiredMsg{id: first.id, timer: first.timer})
	if toastRow(m) < 0 {
		t.Fatal("the timer of the first toast took down the second")
	}
	if rowOf(m.View().Content, "first toast") >= 0 {
		t.Fatal("the first toast is still drawn after its own timer")
	}
	if len(m.pendingStreamChunks) != 1 {
		t.Fatal("the timer of a toast flushed the buffered stream chunks")
	}

	// The same notice again keeps the toast up: its first timer is void.
	m.showToast("Copied to the test")
	if len(fxs.toasts) != 1 {
		t.Fatalf("the same notice twice made %d toasts, want 1", len(fxs.toasts))
	}
	m.Update(toastExpiredMsg{id: second.id, timer: second.timer})
	if toastRow(m) < 0 {
		t.Fatal("the first timer of a toast took it down after it was shown again")
	}
	again := fxs.toasts[0]
	m.Update(toastExpiredMsg{id: again.id, timer: again.timer})
	if toastRow(m) >= 0 {
		t.Fatal("the toast is still drawn after its own timer")
	}
}

// TestToastsStack checks that toasts with different text are drawn one below
// the other, each against the right edge, and that no more than toastMax are
// up at one time.
func TestToastsStack(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)

	m.showToast("short")
	m.showToast("a much longer notice")
	view := m.View().Content
	first, second := rowOf(view, "short"), rowOf(view, "a much longer notice")
	if first != 1 || second != 4 {
		t.Fatalf("toast rows = %d and %d, want 1 and 4\n%s", first, second, stripAnsi(view))
	}
	for _, row := range []int{first, second} {
		line := strings.TrimRight(strings.Split(stripAnsi(view), "\n")[row], " ")
		if !strings.HasSuffix(line, "│") || lipgloss.Width(line) != m.width-1 {
			t.Fatalf("toast on row %d is not against the right edge: %q", row, line)
		}
	}

	for i := range toastMax + 2 {
		m.showToast(strings.Repeat("x", i+1))
	}
	if got := len(m.fxState().toasts); got != toastMax {
		t.Fatalf("%d toasts are up, want no more than %d", got, toastMax)
	}
	if rowOf(m.View().Content, "short") >= 0 {
		t.Fatal("the oldest toast did not make room")
	}
}

// TestToastForOneThingChangesInPlace checks a notice about one thing that
// changes. A new notice with the key of a toast that is up takes the place of
// its text, and the timer of the old text does not take the toast down.
func TestToastForOneThingChangesInPlace(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	fxs := m.fxState()

	m.showToastFor("level", "Level: low")
	first := *fxs.toasts[0]
	m.showToast("another notice")
	m.showToastFor("level", "Level: high")
	if len(fxs.toasts) != 2 {
		t.Fatalf("%d toasts are up, want 2", len(fxs.toasts))
	}
	view := m.View().Content
	if rowOf(view, "Level: low") >= 0 || rowOf(view, "Level: high") != 1 {
		t.Fatalf("the toast did not change its text in place\n%s", stripAnsi(view))
	}
	m.Update(toastExpiredMsg{id: first.id, timer: first.timer})
	if rowOf(m.View().Content, "Level: high") < 0 {
		t.Fatal("the timer of the old text took the toast down")
	}
}

// TestToastIsOneShortRow checks that a toast with long text, or with text of
// more than one line, is one row that fits the screen.
func TestToastIsOneShortRow(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)

	m.showToast("first line\nsecond line " + strings.Repeat("long ", 40))
	text := m.fxState().toasts[0].text
	if strings.Contains(text, "\n") || !strings.HasPrefix(text, "first line second line") {
		t.Fatalf("toast text = %q, want one line", text)
	}
	if w := lipgloss.Width(text); w > toastMaxWidth || !strings.HasSuffix(text, "…") {
		t.Fatalf("toast text is %d columns (%q), want it cut at %d", w, text, toastMaxWidth)
	}
	if got := lipgloss.Height(m.View().Content); got != m.height {
		t.Fatalf("frame height with a long toast = %d, want %d", got, m.height)
	}
}

// TestNotifyReturnsItsTimerFromUpdate checks a toast that a handler shows
// without a command of its own to return. The command that takes the toast
// down must leave with the commands of that Update, and only once.
func TestNotifyReturnsItsTimerFromUpdate(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	before := len(m.messages)

	_, cmd := m.Update(app.ExtensionPrintEvent{Level: "toast", Text: "Saved by the extension"})
	if cmd == nil {
		t.Fatal("Update returned no command for a toast: nothing would take it down")
	}
	if len(m.messages) != before {
		t.Fatalf("a toast added %d message(s) to the transcript", len(m.messages)-before)
	}
	if rowOf(m.View().Content, "Saved by the extension") != 1 {
		t.Fatalf("the toast of an extension is not on screen\n%s", stripAnsi(m.View().Content))
	}
	if m.fxState().pending != nil {
		t.Fatal("the command of the toast is still kept after Update returned it")
	}
	if _, cmd := m.Update(noopMsg{}); cmd != nil {
		t.Fatal("a later Update returned the command of the toast again")
	}
}

// TestRejectedCommandShakesComposer checks that a command that cannot be done
// shakes the composer, as a prompt that could not be sent does.
func TestRejectedCommandShakesComposer(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)

	m.isReasoningModel = false
	m.handleThinkingCommand("high")
	if !m.fxState().shake.Animating() {
		t.Fatal("/thinking on a model that cannot reason did not shake the composer")
	}

	m.fxState().shake.Set(1)
	if cmd := m.handleCopyCommand(); cmd == nil {
		t.Fatal("/copy with nothing to copy returned no command: its toast would stay up")
	}
	if !m.fxState().shake.Animating() {
		t.Fatal("/copy with nothing to copy did not shake the composer")
	}
	if got := len(m.fxState().toasts); got != 1 {
		t.Fatalf("/copy with nothing to copy showed %d toasts, want 1", got)
	}
}

// TestToastSlidesInWithMotion checks that a toast with motion on starts out
// of sight, moves on the frame clock, ends in the same place as one with
// motion off, and slides out again when its time is up.
func TestToastSlidesInWithMotion(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)
	fxs := m.fxState()
	now := time.Unix(1000, 0)
	fxs.now = func() time.Time { return now }

	m.showToast("Copied to the test")
	if toastRow(m) >= 0 {
		t.Fatal("the toast is in its place before its first frame: it has no transition")
	}
	if !fxs.animating() || m.wakeFrameClock() == nil {
		t.Fatal("a toast shown with motion on did not wake the frame clock")
	}
	now = now.Add(time.Second)
	fxs.advance()
	if got := toastRow(m); got != 1 {
		t.Fatalf("toast text on row %d after its transition, want row 1", got)
	}

	top := fxs.toasts[0]
	m.Update(toastExpiredMsg{id: top.id, timer: top.timer})
	if !fxs.animating() || len(fxs.toasts) != 1 {
		t.Fatal("a toast whose time is up did not start to slide out")
	}
	now = now.Add(time.Second)
	fxs.advance()
	if len(fxs.toasts) != 0 || toastRow(m) >= 0 {
		t.Fatal("the toast is still there after it slid out")
	}
}

// TestCopyCommandShowsToast checks that /copy confirms with a toast and adds
// nothing to the transcript.
func TestCopyCommandShowsToast(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	m.printAssistantMessage("an answer to copy")
	before := len(m.messages)

	if cmd := m.handleCopyCommand(); cmd == nil {
		t.Fatal("/copy returned no command")
	}
	if len(m.messages) != before {
		t.Fatalf("/copy added %d message(s) to the transcript", len(m.messages)-before)
	}
	toasts := m.fxState().toasts
	if len(toasts) != 1 || !strings.Contains(toasts[0].text, "Copied last assistant message") {
		t.Fatalf("toasts = %v, want the copy notice", toasts)
	}
}

// TestShakeComposerNeedsMotion checks that the composer shakes only when
// motion is on, and that a shaking composer keeps the frame its size.
func TestShakeComposerNeedsMotion(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)

	m.shakeComposer()
	if m.fxState().shake.Animating() || m.wakeFrameClock() != nil {
		t.Fatal("the composer shook with motion off")
	}

	enableMotion(t, m)
	m.shakeComposer()
	if !m.fxState().shake.Animating() {
		t.Fatal("the composer did not shake with motion on")
	}
	if m.wakeFrameClock() == nil {
		t.Fatal("the shake did not wake the frame clock")
	}
	view := m.View().Content
	if got := lipgloss.Height(view); got != m.height {
		t.Fatalf("frame height during the shake = %d, want %d", got, m.height)
	}
	if rowOf(view, "test-model") < 0 {
		t.Fatalf("the status bar left the frame during the shake\n%s", stripAnsi(view))
	}
}

// TestUpdatePassesEveryMessageToTheRoot checks the wiring that layout
// animation depends on: the root hears the terminal's messages.
func TestUpdatePassesEveryMessageToTheRoot(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	if m.ui != nil {
		t.Fatal("the root exists before first use")
	}
	m.Update(tea.FocusMsg{})
	if m.ui == nil {
		t.Fatal("Update did not reach the root")
	}
}

// flattenCmd runs cmd and returns the messages that it and the commands
// batched in it produce. A tick blocks for as long as it is set for.
func flattenCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			msgs = append(msgs, flattenCmd(c)...)
		}
		return msgs
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

// TestIsMotionMsg checks that the messages of bloom are told apart from the
// model's own, the one that is not exported included.
func TestIsMotionMsg(t *testing.T) {
	if !isMotionMsg(anim.FrameMsg{}) {
		t.Fatal("a bloom animation frame is not a motion message")
	}
	for _, msg := range []tea.Msg{nil, noopMsg{}, tea.KeyPressMsg{}, frameTickMsg{}, motionSettledMsg{}} {
		if isMotionMsg(msg) {
			t.Fatalf("%T is taken for a motion message", msg)
		}
	}

	// The tick of a layout animation has a type that bloom does not export.
	// Get a real one: a root that has an animated section asks for a tick.
	root := bloom.New()
	root.Render(20, 4, bloom.Column(bloom.Fill(region("a")).ID(regionStream).Animate()))
	cmd := root.Update(noopMsg{})
	if cmd == nil {
		t.Fatal("a root with an animated section asked for no tick")
	}
	msgs := flattenCmd(cmd)
	if len(msgs) != 1 || !isMotionMsg(msgs[0]) {
		t.Fatalf("layout tick %T is not taken for a motion message", msgs)
	}
}

// TestMotionMsgDoesNotFlushStream checks that an animation frame leaves the
// buffered stream chunks alone. An event flushes them; 60 frames a second
// would undo the coalescing.
func TestMotionMsgDoesNotFlushStream(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	m.pendingStreamChunks = []pendingStreamChunk{{role: "assistant", content: "buffered"}}

	m.Update(anim.FrameMsg{})
	if len(m.pendingStreamChunks) != 1 {
		t.Fatal("a bloom animation frame flushed the buffered stream chunks")
	}
	// A tick of the frame clock, on which the transitions run.
	m.frames.start()
	m.Update(frameTickMsg{generation: m.frames.generation, frame: 1})
	if len(m.pendingStreamChunks) != 1 {
		t.Fatal("a tick of the frame clock flushed the buffered stream chunks")
	}
	m.Update(motionSettledMsg{})
	if len(m.pendingStreamChunks) != 1 {
		t.Fatal("the settled message flushed the buffered stream chunks")
	}
	m.Update(noopMsg{})
	if len(m.pendingStreamChunks) != 0 {
		t.Fatal("an event did not flush the buffered stream chunks: the test checks nothing")
	}
}

// TestSettledFollowsTheLastTransition checks that the model gets one more
// Update after the frame on which the last transition came to rest.
func TestSettledFollowsTheLastTransition(t *testing.T) {
	fxs := newMotionState()
	if fxs.settled(false) != nil {
		t.Fatal("settled with nothing that moved")
	}
	fxs.shake.Set(0)
	fxs.move(&fxs.shake, 1, shakeDuration, true)
	if !fxs.animating() {
		t.Fatal("the shake did not start")
	}
	if fxs.settled(true) != nil {
		t.Fatal("settled while the shake runs")
	}
	fxs.shake.Set(1)
	msgs := flattenCmd(fxs.settled(true))
	if len(msgs) != 1 {
		t.Fatalf("settled sent %d messages, want 1", len(msgs))
	}
	if _, ok := msgs[0].(motionSettledMsg); !ok {
		t.Fatalf("settled sent %T", msgs[0])
	}
}

// TestSurfaceTransitionRunsThroughUpdate runs a real transition: the commands
// that Update returns are run, and their messages given back to Update, until
// none is left. The surface must end fully shown, and the model must be told
// that motion settled.
func TestSurfaceTransitionRunsThroughUpdate(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)
	_ = m.View()

	openTestSheet(m)
	_, cmd := m.Update(noopMsg{})
	a := openSurface(t, m)

	pending := flattenCmd(cmd)
	settled := false
	deadline := time.Now().Add(5 * time.Second)
	for len(pending) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("the transition did not end: %d messages still pending, shown=%v", len(pending), a.shown.Float())
		}
		msg := pending[0]
		pending = pending[1:]
		if _, ok := msg.(motionSettledMsg); ok {
			settled = true
		}
		_, cmd := m.Update(msg)
		if got := lipgloss.Height(m.View().Content); got != m.height {
			t.Fatalf("frame height during the transition = %d, want %d", got, m.height)
		}
		pending = append(pending, flattenCmd(cmd)...)
	}
	if a.shown.Float() != 1 || a.shown.Animating() {
		t.Fatalf("shown = %v (animating %v), want 1 at rest", a.shown.Float(), a.shown.Animating())
	}
	if !settled {
		t.Fatal("no motionSettledMsg followed the transition")
	}
}

// TestDirectImageHidesWhileSurfaceLeaves checks an image that is placed
// directly on the screen through a modal that opens and closes. The image has
// a placement before the modal, none while the modal is open or on its way
// out, and one again when the modal has gone.
func TestDirectImageHidesWhileSurfaceLeaves(t *testing.T) {
	m := newDirectImageModel(t)
	enableMotion(t, m)
	placed := func() bool {
		_ = m.View()
		return strings.Contains(m.gfxPlacement, "a=p")
	}

	if !placed() {
		t.Fatalf("no placement for a visible transcript image: %q", m.gfxPlacement)
	}

	openTestSheet(m)
	m.Update(noopMsg{})
	a := openSurface(t, m)
	a.shown.Set(1)
	if placed() {
		t.Fatalf("the image is placed under an open sheet: %q", m.gfxPlacement)
	}

	m.sheet = nil
	m.Update(noopMsg{})
	a.shown.Set(0.5) // halfway out: the sheet is closed, and still drawn
	if m.modalActive() {
		t.Fatal("the model still has a modal: the test would not reach the surface check")
	}
	if placed() {
		t.Fatalf("the image is placed under a sheet on its way out: %q", m.gfxPlacement)
	}

	a.shown.Set(0)
	if !placed() {
		t.Fatalf("the image did not come back after the sheet had gone: %q", m.gfxPlacement)
	}
}

// changedRows returns the rows of frame a that are not drawn as in frame b.
func changedRows(a, b string) []int {
	var rows []int
	other := strings.Split(b, "\n")
	for i, row := range strings.Split(a, "\n") {
		if i >= len(other) || row != other[i] {
			rows = append(rows, i)
		}
	}
	return rows
}

// TestNewTranscriptItemFadesIn checks an item that an Update adds to the end
// of the transcript with motion on. It fades in on the rows that it has, the
// rest of the frame is drawn as at rest, and the fade ends on the frame clock.
func TestNewTranscriptItemFadesIn(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)
	fxs := m.fxState()
	now := time.Unix(1000, 0)
	fxs.now = func() time.Time { return now }
	fillTranscript(m)
	m.layoutDirty = true
	_ = m.View()

	m.Update(app.ExtensionPrintEvent{Level: "info", Text: "the new notice"})
	if len(fxs.entries) != 1 || !fxs.entries[0].shown.Animating() {
		t.Fatalf("%d item(s) fade in after one was added, want 1", len(fxs.entries))
	}
	if !m.frames.fast {
		t.Fatal("the frame clock is not fast while an item fades in")
	}
	fading := m.View().Content
	row := rowOf(fading, "the new notice")
	if row < 0 {
		t.Fatalf("the new item is not in the frame\n%s", stripAnsi(fading))
	}

	now = now.Add(time.Second)
	fxs.advance()
	if len(fxs.entries) != 0 {
		t.Fatal("the item is still kept after its fade")
	}
	atRest := m.View().Content
	if stripAnsi(fading) != stripAnsi(atRest) {
		t.Fatal("the fade changed the text of the frame")
	}
	changed := changedRows(fading, atRest)
	if !slices.Contains(changed, row) {
		t.Fatalf("the row of the new item was drawn as at rest: it did not fade (changed rows %v, item on row %d)", changed, row)
	}
	for _, r := range changed {
		if r < row-2 || r > row+2 {
			t.Fatalf("row %d changed during the fade of the item on row %d: the fade reached another item", r, row)
		}
	}
}

// TestNewItemFadesOnItsRowsDuringAGlide checks the rows of a new item while
// the transcript is drawn at a height that is not the list's own, as a layout
// animation draws it. The fade must be on the rows that the item has at that
// height.
func TestNewItemFadesOnItsRowsDuringAGlide(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)
	fillTranscript(m)
	m.layoutDirty = true
	_ = m.View()
	m.Update(app.ExtensionPrintEvent{Level: "info", Text: "the new notice"})
	_ = m.View()

	list := m.scrollList
	for _, rows := range []int{list.height, list.height - 3, list.height + 2} {
		view := list.ViewAtHeight(rows)
		plain := bloom.Render(m.width, rows, section(view))
		fading := bloom.Render(m.width, rows, m.transcriptChild(view, rows))
		row := rowOf(plain, "the new notice")
		if row < 0 {
			t.Fatalf("at %d rows the new item is not in view\n%s", rows, stripAnsi(plain))
		}
		changed := changedRows(fading, plain)
		if !slices.Contains(changed, row) {
			t.Fatalf("at %d rows the item on row %d did not fade (changed rows %v)", rows, row, changed)
		}
		for _, r := range changed {
			if r < row-2 || r > row+2 {
				t.Fatalf("at %d rows, row %d changed for the item on row %d", rows, r, row)
			}
		}
	}
}

// TestOnlyAFewNewItemsFade checks the changes to the transcript that are not
// a conversation that moves on. A session that loads, a transcript that was
// cleared and an item with motion off are drawn as they are.
func TestOnlyAFewNewItemsFade(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	fxs := m.fxState()

	// Motion off.
	mark := m.markTranscript()
	m.printSystemMessage("with motion off")
	m.noteNewItems(mark)
	if len(fxs.entries) != 0 {
		t.Fatal("an item fades in with motion off")
	}

	enableMotion(t, m)

	// Many items at once.
	mark = m.markTranscript()
	for i := range entryMax + 1 {
		m.printSystemMessage(fmt.Sprintf("loaded message %d", i))
	}
	m.noteNewItems(mark)
	if len(fxs.entries) != 0 {
		t.Fatalf("%d items fade in after %d were added at once", len(fxs.entries), entryMax+1)
	}

	// A transcript that was replaced and is as long as before plus one.
	mark = m.markTranscript()
	m.messages = append([]MessageItem{}, m.messages[1:]...)
	m.printSystemMessage("one")
	m.printSystemMessage("two")
	m.noteNewItems(mark)
	if len(fxs.entries) != 0 {
		t.Fatal("an item fades in although the transcript before it changed")
	}

	// The usual case, to show that the checks above can fail.
	mark = m.markTranscript()
	m.printSystemMessage("a new message")
	m.noteNewItems(mark)
	if len(fxs.entries) != 1 {
		t.Fatalf("%d items fade in after one was added, want 1", len(fxs.entries))
	}
}

// rootTicks returns the messages that cmd produces and that belong to bloom:
// the ticks that the root answers a message with.
func rootTicks(cmd tea.Cmd) []tea.Msg {
	var ticks []tea.Msg
	for _, msg := range flattenCmd(cmd) {
		if isMotionMsg(msg) {
			ticks = append(ticks, msg)
		}
	}
	return ticks
}

// TestRootDoesNotHearTheTicksOfTheClock checks that a tick of the frame clock
// is not answered by a tick of the root while the layout is at rest. Each
// answer is one more frame, so the spinner would be drawn twice for every
// beat. The root must hear a tick again when a section has moved, and when
// the clock stops, so that a glide is never left without frames.
func TestRootDoesNotHearTheTicksOfTheClock(t *testing.T) {
	m, stream, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)
	now := time.Unix(1000, 0)
	m.fxState().now = func() time.Time { return now }
	stream.spinning = true
	m.layoutDirty = true

	// The first frames: the root gets its sections, and the layout settles.
	m.Update(noopMsg{})
	_ = m.View()
	_ = m.View()
	if m.layoutMoved {
		t.Fatal("the layout still moves on the second frame of a screen at rest")
	}
	if !m.frames.running {
		t.Fatal("the spinner did not start the frame clock")
	}
	beat := func() frameTickMsg {
		return frameTickMsg{generation: m.frames.generation, frame: m.frames.frame + 1}
	}
	// heard sends msg and reports whether the root answered it. A root asks
	// for one tick at a time, so its answer is given back to it: the next
	// message can then be answered too.
	heard := func(msg tea.Msg) bool {
		t.Helper()
		_, cmd := m.Update(msg)
		ticks := rootTicks(cmd)
		for _, tick := range ticks {
			m.Update(tick)
		}
		_ = m.View()
		return len(ticks) > 0
	}

	// The root answers an event: this is what the checks below must not see.
	if !heard(noopMsg{}) {
		t.Fatal("the root did not answer an event: the test checks nothing")
	}

	for _, msg := range []tea.Msg{
		beat(),
		app.StreamChunkEvent{Content: "text"},
		streamAppendFlushMsg{},
		app.ToolOutputEvent{ToolCallID: "t", Chunk: "out"},
	} {
		if heard(msg) {
			t.Fatalf("%T was answered by a tick of the root while the clock runs and the layout is at rest", msg)
		}
	}

	// A section in a new place: the next tick goes to the root.
	m.layoutMoved = true
	if !heard(beat()) {
		t.Fatal("after a section moved, the root did not hear the tick of the clock")
	}
	if m.layoutMoved {
		t.Fatal("the frame of a layout at rest left layoutMoved set")
	}

	// The clock stops with this tick: the root hears it. The text that the
	// chunk added has finished its fade by then, so nothing else keeps the
	// clock up.
	now = now.Add(time.Second)
	stream.spinning = false
	if !heard(beat()) {
		t.Fatal("the root did not hear the last tick of the clock")
	}
	if m.frames.running {
		t.Fatal("the clock still runs with nothing to animate")
	}

	// With the clock stopped the root hears a chunk too.
	if !heard(app.StreamChunkEvent{Content: "text"}) {
		t.Fatal("the root did not hear a chunk while the clock is stopped")
	}
}
