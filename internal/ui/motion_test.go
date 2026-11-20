package ui

import (
	"image"
	"image/color"
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
	if fxs.animating() || fxs.toast.Visible() {
		t.Fatal("a toast shown with motion off started an animation")
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

	if _, cmd := m.Update(toastExpiredMsg{gen: fxs.toastGen}); cmd != nil {
		t.Fatal("the timer of a toast started more work")
	}
	if toastRow(m) >= 0 {
		t.Fatal("the toast is still drawn after its time was up")
	}
}

// TestToastTimerBelongsToItsToast checks that the timer of a toast that was
// replaced does not take down the toast that replaced it, and does not flush
// the buffered stream chunks as an event would.
func TestToastTimerBelongsToItsToast(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	fxs := m.fxState()

	m.showToast("first toast")
	first := fxs.toastGen
	m.showToast("Copied to the test")
	second := fxs.toastGen
	if first == second {
		t.Fatal("two toasts have the same number")
	}

	m.pendingStreamChunks = []pendingStreamChunk{{role: "assistant", content: "buffered"}}
	m.Update(toastExpiredMsg{gen: first})
	if toastRow(m) < 0 {
		t.Fatal("the timer of the first toast took down the second")
	}
	if len(m.pendingStreamChunks) != 1 {
		t.Fatal("the timer of a toast flushed the buffered stream chunks")
	}

	m.Update(toastExpiredMsg{gen: second})
	if toastRow(m) >= 0 {
		t.Fatal("the second toast is still drawn after its own timer")
	}
}

// TestToastSlidesInWithMotion checks that a toast with motion on starts out
// of sight and ends in the same place as one with motion off.
func TestToastSlidesInWithMotion(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)

	m.showToast("Copied to the test")
	if toastRow(m) >= 0 {
		t.Fatal("the toast is in its place before its first frame: it has no transition")
	}
	fxs := m.fxState()
	now := time.Now()
	fxs.toast.ShowAt(now)
	fxs.toast.Advance(now.Add(time.Second))
	if got := toastRow(m); got != 1 {
		t.Fatalf("toast text on row %d after its transition, want row 1", got)
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
	if got := m.fxState().toastText; !strings.Contains(got, "Copied last assistant message") {
		t.Fatalf("toast = %q, want the copy notice", got)
	}
}

// TestShakeComposerNeedsMotion checks that the composer shakes only when
// motion is on, and that a shaking composer keeps the frame its size.
func TestShakeComposerNeedsMotion(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)

	if cmd := m.shakeComposer(); cmd != nil {
		t.Fatal("the composer shook with motion off")
	}

	enableMotion(t, m)
	if cmd := m.shakeComposer(); cmd == nil {
		t.Fatal("the composer did not shake with motion on")
	}
	if !m.fxState().shake.Animating() {
		t.Fatal("the shake is not running")
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
		t.Fatal("an animation frame is not a motion message")
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
		t.Fatal("an animation frame flushed the buffered stream chunks")
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
	cmd := fxs.shake.Play(0, 1)
	if cmd == nil || !fxs.animating() {
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
