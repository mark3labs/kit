package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"bloomfx.dev/bloom"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// testClock is a clock that a test moves by hand.
type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time          { return c.now }
func (c *testClock) advance(d time.Duration) { c.now = c.now.Add(d) }

// fillTranscript adds enough messages to the transcript to fill any viewport.
func fillTranscript(m *AppModel) {
	for i := range 40 {
		m.printSystemMessage(fmt.Sprintf("transcript message %d", i))
	}
}

// TestSectionMeasure checks that a section is measured the way Kit measures a
// section: by its lines, with nothing for an empty one.
func TestSectionMeasure(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want bloom.Size
	}{
		{"", bloom.Size{}},
		{"one", bloom.Size{W: 40, H: 1}},
		{"one\ntwo\nthree", bloom.Size{W: 40, H: 3}},
		{"\x1b[1mbold\x1b[0m\nrow", bloom.Size{W: 40, H: 2}},
	} {
		if got := section(tc.s).Measure(bloom.Size{W: 40, H: 10}); got != tc.want {
			t.Errorf("Measure(%q) = %+v, want %+v", tc.s, got, tc.want)
		}
	}
	// In a space with no limit a section reports its own width.
	if got := section("abc\nabcdef").Measure(bloom.Size{W: layoutUnbounded, H: layoutUnbounded}); got != (bloom.Size{W: 6, H: 2}) {
		t.Errorf("natural size = %+v, want 6x2", got)
	}
}

// TestLayoutGivesTranscriptTheRemainingRows checks the solved layout against
// the sum that it replaces: the transcript gets the rows of the screen that
// the other sections do not take.
func TestLayoutGivesTranscriptTheRemainingRows(t *testing.T) {
	for _, chrome := range []bool{false, true} {
		t.Run(fmt.Sprintf("chrome=%v", chrome), func(t *testing.T) {
			m, _, _ := newTestAppModel(&stubAppController{})
			m.input = NewInputComponent(80, nil)
			if chrome {
				m.getHeader = func() *WidgetData { return &WidgetData{Text: "header\nsecond row"} }
				m.getFooter = func() *WidgetData { return &WidgetData{Text: "footer"} }
				m.getWidgets = func(slot string) []WidgetData {
					return []WidgetData{{ID: slot, Text: "widget in " + slot}}
				}
				m.queuedMessages = []string{"queued message"}
				m.ctrlCPressedOnce = true
				m.input.(*InputComponent).textarea.SetValue("one\ntwo\nthree")
			}
			m.distributeHeight()

			c := m.chrome
			used := 0
			for _, s := range []string{c.header, c.warning, c.separator, c.above, c.queued, c.subagents, c.activity, c.input, c.below, c.status, c.footer} {
				if s != "" {
					used += lipgloss.Height(s)
				}
			}
			if chrome && (c.header == "" || c.footer == "" || c.above == "" || c.below == "" || c.queued == "" || c.warning == "") {
				t.Fatalf("a section that the test set up rendered nothing: %+v", c)
			}
			if want := m.height - used; m.scrollList.height != want {
				t.Fatalf("transcript height = %d, want %d (screen %d, other sections %d)", m.scrollList.height, want, m.height, used)
			}

			view := m.View().Content
			if got := lipgloss.Height(view); got != m.height {
				t.Fatalf("frame height = %d, want %d", got, m.height)
			}
		})
	}
}

// TestShortFrameDropsSectionsInOrder checks a frame that is too short for all
// of its sections. The sections that can be left out go one at a time, the
// widgets first, until the transcript has streamMinRows rows. The composer
// and the status bar keep every row.
func TestShortFrameDropsSectionsInOrder(t *testing.T) {
	newModel := func(height int) *AppModel {
		m, _, _ := newTestAppModel(&stubAppController{})
		m.input = NewInputComponent(80, nil)
		m.height = height
		m.getHeader = func() *WidgetData { return &WidgetData{Text: "header one\nheader two"} }
		m.getFooter = func() *WidgetData { return &WidgetData{Text: "footer"} }
		m.getWidgets = func(slot string) []WidgetData {
			return []WidgetData{{ID: slot, Text: "widget " + slot + " 1\nwidget " + slot + " 2\nwidget " + slot + " 3"}}
		}
		m.queuedMessages = []string{"queued message"}
		m.layoutDirty = true
		return m
	}
	rows := func(m *AppModel, id regionID) int { return regionRows(m.ui.Frame(), id) }

	// A tall frame holds everything.
	m := newModel(40)
	view := m.View().Content
	for _, id := range []regionID{regionHeader, regionAbove, regionQueued, regionBelow, regionFooter} {
		if rows(m, id) == 0 {
			t.Fatalf("section %d has no rows in a tall frame\n%s", id, stripAnsi(view))
		}
	}
	all := m.height - m.scrollList.height // the rows of every section but the transcript

	// One row too few for all of them: only the first in the order goes.
	m = newModel(all + streamMinRows - 1)
	view = m.View().Content
	if rows(m, regionBelow) != 0 {
		t.Fatalf("the widgets below the composer did not leave a short frame\n%s", stripAnsi(view))
	}
	for _, id := range []regionID{regionHeader, regionAbove, regionQueued, regionFooter} {
		if rows(m, id) == 0 {
			t.Fatalf("section %d left the frame before its turn\n%s", id, stripAnsi(view))
		}
	}
	if m.scrollList.height < streamMinRows || rows(m, regionStream) != m.scrollList.height {
		t.Fatalf("transcript has %d rows (list %d), want at least %d", rows(m, regionStream), m.scrollList.height, streamMinRows)
	}

	// A frame with room for little more than the composer: every section
	// that can go has gone, and the composer and status bar are whole.
	m = newModel(streamMinRows + 6)
	view = m.View().Content
	for _, id := range []regionID{regionHeader, regionAbove, regionBelow, regionFooter} {
		if rows(m, id) != 0 {
			t.Fatalf("section %d is still in a frame of %d rows\n%s", id, m.height, stripAnsi(view))
		}
	}
	if got, want := rows(m, regionInput), sectionRows(m.chrome.input); got != want {
		t.Fatalf("composer has %d rows, want all %d", got, want)
	}
	if got := rows(m, regionStatus); got != 1 {
		t.Fatalf("status bar has %d rows, want 1", got)
	}
	if rows(m, regionStream) < streamMinRows {
		t.Fatalf("transcript has %d rows, want at least %d\n%s", rows(m, regionStream), streamMinRows, stripAnsi(view))
	}
	if got := lipgloss.Height(view); got != m.height {
		t.Fatalf("frame height = %d, want %d", got, m.height)
	}
}

// TestShortFrameKeepsFillWidgets checks that a fill widget does not leave a
// short frame: it stands in for the transcript, so it has nothing to give way
// to.
func TestShortFrameKeepsFillWidgets(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	m.height = 8
	m.getWidgets = func(slot string) []WidgetData {
		if slot != "above" {
			return nil
		}
		return []WidgetData{{ID: "fill", Text: "fill widget", FillHeight: true}}
	}
	m.layoutDirty = true
	view := m.View().Content
	if !m.fillActive {
		t.Fatal("the fill widget is not active")
	}
	if regionRows(m.ui.Frame(), regionAbove) == 0 || rowOf(view, "fill widget") < 0 {
		t.Fatalf("the fill widget left a short frame\n%s", stripAnsi(view))
	}
}

// lastColumn returns the last cell of each of the first rows of a frame.
func lastColumn(view string, rows int) string {
	var b strings.Builder
	for _, line := range strings.Split(stripAnsi(view), "\n")[:rows] {
		cells := []rune(line)
		if len(cells) < 80 {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(cells[len(cells)-1])
	}
	return b.String()
}

// TestScrollbarShowsWhereTheViewIs checks the scrollbar of the transcript. It
// is not there while the transcript follows the conversation. When the user
// scrolls away it is in the last column, its thumb moves with the view, and
// it takes no column from the text.
func TestScrollbarShowsWhereTheViewIs(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	fillTranscript(m)
	m.layoutDirty = true
	following := m.View().Content
	rows := m.scrollList.height
	if strings.Contains(lastColumn(following, rows), "┃") {
		t.Fatalf("a transcript that follows the conversation has a scrollbar\n%s", stripAnsi(following))
	}

	// The user scrolls up to the start.
	m.scrollList.autoScroll = false
	m.scrollList.GotoTop()
	m.Update(noopMsg{})
	top := m.View().Content
	col := lastColumn(top, rows)
	if !strings.HasPrefix(col, "┃") {
		t.Fatalf("at the start of the transcript the thumb is not at the top of the last column: %q", col)
	}
	if strings.HasSuffix(col, "┃") {
		t.Fatalf("the thumb fills the track: %q", col)
	}
	if got := lipgloss.Height(top); got != m.height {
		t.Fatalf("frame height with a scrollbar = %d, want %d", got, m.height)
	}
	// Nothing but the last column changed: the text keeps its width.
	m.scrollList.autoScroll = true
	m.Update(noopMsg{})
	plain := strings.Split(stripAnsi(m.View().Content), "\n")
	for i, line := range strings.Split(stripAnsi(top), "\n")[:rows] {
		a, b := []rune(line), []rune(plain[i])
		if len(a) == 80 {
			a = a[:79]
		}
		if len(b) == 80 {
			b = b[:79]
		}
		if strings.TrimRight(string(a), " ") != strings.TrimRight(string(b), " ") {
			t.Fatalf("row %d changed outside the last column:\n%q\n%q", i, string(a), string(b))
		}
	}

	// Scrolled to the end, and still not following: the thumb is at the
	// bottom of the track.
	m.scrollList.autoScroll = false
	m.scrollList.GotoBottom()
	m.Update(noopMsg{})
	col = lastColumn(m.View().Content, rows)
	if !strings.HasSuffix(col, "┃") || strings.HasPrefix(col, "┃") {
		t.Fatalf("at the end of the transcript the thumb is not at the bottom of the last column: %q", col)
	}
}

// TestScrollExtentEstimatesUnseenItems checks the numbers behind the
// scrollbar. Items that were never drawn count as the mean of those that
// were, nothing is rendered to measure them, and a list at its end has its
// view exactly at the end of the content.
func TestScrollExtentEstimatesUnseenItems(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	fillTranscript(m)
	m.layoutDirty = true
	_ = m.View()
	list := m.scrollList

	total, visible, offset := list.scrollExtent()
	if visible != list.height {
		t.Fatalf("visible = %d, want the height of the list, %d", visible, list.height)
	}
	if offset+visible != total {
		t.Fatalf("a list at its end has offset %d + visible %d != total %d", offset, visible, total)
	}

	// Forget every height but those of the items in view.
	known := len(list.heightCache)
	clear(list.heightCache)
	_ = list.View()
	if len(list.heightCache) >= known || len(list.heightCache) == 0 {
		t.Fatalf("the list knows %d heights after it drew one view, had %d: the test checks nothing", len(list.heightCache), known)
	}
	before := len(list.heightCache)
	estimate, _, offset := list.scrollExtent()
	if len(list.heightCache) != before {
		t.Fatal("scrollExtent measured items that were not in view")
	}
	if estimate != total {
		t.Fatalf("estimated total = %d, want %d: every message of this transcript has the same height", estimate, total)
	}
	if offset+visible != estimate {
		t.Fatalf("with estimates, a list at its end has offset %d + visible %d != total %d", offset, visible, estimate)
	}
}

// TestViewRecordsWhereSectionsWent checks the rows that the mouse handlers
// and the image placement read: where the transcript starts, and where the
// input starts.
func TestViewRecordsWhereSectionsWent(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	m.getHeader = func() *WidgetData { return &WidgetData{Text: "header\nsecond row"} }
	m.input.(*InputComponent).textarea.SetValue("text in the composer")
	m.layoutDirty = true

	view := m.View().Content
	header := lipgloss.Height(m.chrome.header)
	if m.scrollbackYOffset != header || m.lastHeaderHeight != header {
		t.Fatalf("transcript starts at %d (header height %d), want %d", m.scrollbackYOffset, m.lastHeaderHeight, header)
	}
	// The composer has a row of padding above its text.
	textRow := rowOf(view, "text in the composer")
	if textRow < 0 {
		t.Fatalf("the composer text is not in the frame\n%s", stripAnsi(view))
	}
	if m.inputYOffset > textRow || textRow >= m.inputYOffset+lipgloss.Height(m.chrome.input) {
		t.Fatalf("input section at row %d (height %d) does not hold the composer text on row %d",
			m.inputYOffset, lipgloss.Height(m.chrome.input), textRow)
	}
	if want := m.height - lipgloss.Height(m.chrome.input) - lipgloss.Height(m.chrome.status); m.inputYOffset != want {
		t.Fatalf("input section at row %d, want %d", m.inputYOffset, want)
	}
}

// TestLayoutKeepsRowsForAnInputThatRendersNothing checks that an input with
// no view keeps the rows of a composer, so that the transcript does not jump
// when the composer comes back.
func TestLayoutKeepsRowsForAnInputThatRendersNothing(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{}) // a stub input renders ""
	m.distributeHeight()
	status := lipgloss.Height(m.chrome.status)
	if want := m.height - inputFallbackRows - status; m.scrollList.height != want {
		t.Fatalf("transcript height = %d, want %d", m.scrollList.height, want)
	}
	if got := lipgloss.Height(m.View().Content); got != m.height {
		t.Fatalf("frame height = %d, want %d", got, m.height)
	}
}

// TestScrollListViewAtHeight checks that drawing the list at another height
// leaves the list as it was, follows the bottom while the list does, and
// keeps the first visible line when the user scrolled up.
func TestScrollListViewAtHeight(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	fillTranscript(m)
	list := m.scrollList
	list.SetHeight(10)
	list.GotoBottom()

	full := strings.Split(list.View(), "\n")
	idx, line := list.offsetIdx, list.offsetLine

	taller := strings.Split(list.ViewAtHeight(14), "\n")
	if len(taller) != 14 {
		t.Fatalf("ViewAtHeight(14) drew %d rows", len(taller))
	}
	if got := strings.Join(taller[4:], "\n"); got != strings.Join(full, "\n") {
		t.Fatal("a taller view that follows the bottom must end with the rows of the list's own view")
	}
	shorter := strings.Split(list.ViewAtHeight(6), "\n")
	if got := strings.Join(shorter, "\n"); got != strings.Join(full[4:], "\n") {
		t.Fatal("a shorter view that follows the bottom must be the last rows of the list's own view")
	}
	if list.height != 10 || list.offsetIdx != idx || list.offsetLine != line {
		t.Fatalf("ViewAtHeight changed the list: height=%d offset=%d/%d", list.height, list.offsetIdx, list.offsetLine)
	}

	// Scrolled up: the first visible line stays where it is.
	list.ScrollBy(-12)
	list.autoScroll = false
	full = strings.Split(list.View(), "\n")
	shorter = strings.Split(list.ViewAtHeight(6), "\n")
	if got := strings.Join(shorter, "\n"); got != strings.Join(full[:6], "\n") {
		t.Fatal("a shorter view of a list scrolled up must be the first rows of the list's own view")
	}
}

// TestLayoutAnimationGlides checks a section that grows with motion on. The
// frame keeps its size, the transcript's section passes through the heights
// between, the scroll list has its final height from the start, and the
// transcript is drawn at each height in between so that no gap opens.
func TestLayoutAnimationGlides(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)
	clock := &testClock{now: time.Unix(1000, 0)}
	m.ui = bloom.New(bloom.WithClock(clock.Now), bloom.WithFPS(motionFPS))
	fillTranscript(m)
	m.layoutDirty = true
	atRest := m.View().Content
	before := m.scrollList.height

	// A tall section appears between the transcript and the composer.
	m.getWidgets = func(slot string) []WidgetData {
		if slot != "above" {
			return nil
		}
		return []WidgetData{{ID: "tall", Text: "w1\nw2\nw3\nw4\nw5\nw6"}}
	}
	m.layoutDirty = true
	_ = m.View()
	target := m.scrollList.height
	if target >= before-4 {
		t.Fatalf("the widget took %d rows, want at least 5", before-target)
	}

	streamRows := func() int { return regionRows(m.ui.Frame(), regionStream) }
	if got := streamRows(); got != before {
		t.Fatalf("first frame of the glide: transcript section has %d rows, want the %d it had", got, before)
	}

	seenBetween := false
	for range 40 {
		clock.advance(layoutGlide / 20)
		view := m.View().Content
		rows := streamRows()
		if got := lipgloss.Height(view); got != m.height {
			t.Fatalf("frame height during the glide = %d, want %d", got, m.height)
		}
		if m.scrollList.height != target {
			t.Fatalf("scroll list height changed during the glide: %d, want %d", m.scrollList.height, target)
		}
		if rows < target || rows > before {
			t.Fatalf("transcript section has %d rows, outside %d..%d", rows, target, before)
		}
		if rows > target && rows < before {
			seenBetween = true
			// The transcript is drawn at the height of its section now.
			want := plainRows(m.scrollList.ViewAtHeight(rows))
			got := transcriptRows(view, rows)
			if got != want {
				t.Fatalf("transcript at %d rows is not the list drawn at that height\ngot:\n%s\nwant:\n%s", rows, got, want)
			}
		}
		// The text does not move while its section shrinks: each frame, the
		// last one included, shows the first rows of the frame at rest. A
		// frame that showed other lines would make the text jump.
		if got, want := transcriptRows(view, rows), transcriptRows(atRest, rows); got != want {
			t.Fatalf("transcript at %d rows moved\ngot:\n%s\nwant:\n%s", rows, got, want)
		}
	}
	if !seenBetween {
		t.Fatal("the transcript section jumped: no frame had a height in between")
	}
	if got := streamRows(); got != target {
		t.Fatalf("after the glide the transcript section has %d rows, want %d", got, target)
	}
}

// TestLayoutJumps checks the cases in which sections must take their place at
// once: motion off and a window resize. For an image placed beside the view,
// see TestLayoutJumpsWhenAnImageMoves.
func TestLayoutJumps(t *testing.T) {
	grow := func(m *AppModel) {
		m.getWidgets = func(slot string) []WidgetData {
			if slot != "above" {
				return nil
			}
			return []WidgetData{{ID: "tall", Text: "w1\nw2\nw3\nw4\nw5\nw6"}}
		}
		m.layoutDirty = true
	}
	newModel := func(t *testing.T, motion bool) *AppModel {
		t.Helper()
		m, _, _ := newTestAppModel(&stubAppController{})
		m.input = NewInputComponent(80, nil)
		if motion {
			enableMotion(t, m)
		}
		clock := &testClock{now: time.Unix(1000, 0)}
		m.ui = bloom.New(bloom.WithClock(clock.Now))
		fillTranscript(m)
		m.layoutDirty = true
		_ = m.View()
		return m
	}
	settled := func(t *testing.T, m *AppModel) {
		t.Helper()
		_ = m.View()
		if got := regionRows(m.ui.Frame(), regionStream); got != m.scrollList.height {
			t.Fatalf("transcript section has %d rows, want %d at once", got, m.scrollList.height)
		}
	}

	t.Run("motion off", func(t *testing.T) {
		m := newModel(t, false)
		grow(m)
		settled(t, m)
	})
	t.Run("window resize", func(t *testing.T) {
		m := newModel(t, true)
		grow(m)
		m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
		settled(t, m)
		if m.layoutJump {
			t.Fatal("the jump outlived the frame that it was for")
		}
	})
}

// directImageGlideModel returns a model with motion on, an image placed
// directly in its transcript, and a root on a clock that the test moves,
// after its first frame.
func directImageGlideModel(t *testing.T) (*AppModel, *testClock) {
	t.Helper()
	m := newDirectImageModel(t)
	enableMotion(t, m)
	clock := &testClock{now: time.Unix(1000, 0)}
	m.ui = bloom.New(bloom.WithClock(clock.Now), bloom.WithFPS(motionFPS))
	_ = m.View()
	if len(m.gfxSpots) != 1 || !strings.Contains(m.gfxPlacement, "a=p") {
		t.Fatalf("the image is not placed: %d spot(s), placement %q", len(m.gfxSpots), m.gfxPlacement)
	}
	return m, clock
}

// TestLayoutGlidesAroundAnImageThatStaysPut checks a layout change that does
// not move an image placed directly on the screen. A section that appears
// below the transcript covers its last rows, and the image, further up, is
// where it was. The sections then glide, and the image is at the same place,
// with the same rows drawn, on every frame.
func TestLayoutGlidesAroundAnImageThatStaysPut(t *testing.T) {
	m, clock := directImageGlideModel(t)
	before := m.scrollList.height
	spot := m.gfxSpots[0]

	m.getWidgets = tallWidget
	m.layoutDirty = true
	_ = m.View()
	target := m.scrollList.height
	if target >= before {
		t.Fatalf("the widget took no rows from the transcript (%d, was %d)", target, before)
	}
	streamRows := func() int { return regionRows(m.ui.Frame(), regionStream) }
	if got := streamRows(); got != before {
		t.Fatalf("first frame after the change: transcript section has %d rows, want the %d it had: the layout jumped", got, before)
	}

	seenBetween := false
	for range 40 {
		clock.advance(layoutGlide / 20)
		_ = m.View()
		if rows := streamRows(); rows > target && rows < before {
			seenBetween = true
		}
		if len(m.gfxSpots) != 1 || m.gfxSpots[0] != spot {
			t.Fatalf("the image moved during the glide: %+v, was %+v", m.gfxSpots, spot)
		}
	}
	if !seenBetween {
		t.Fatal("the transcript section jumped: no frame had a height in between")
	}
	if got := streamRows(); got != target {
		t.Fatalf("after the glide the transcript section has %d rows, want %d", got, target)
	}
}

// TestLayoutJumpsWhenAnImageMoves checks the layout changes that do move an
// image placed directly on the screen, or that change which of its rows are
// in view. The image cannot travel with a section, so the sections take
// their place at once and the image is placed there.
func TestLayoutJumpsWhenAnImageMoves(t *testing.T) {
	settled := func(t *testing.T, m *AppModel) {
		t.Helper()
		_ = m.View()
		if got := regionRows(m.ui.Frame(), regionStream); got != m.scrollList.height {
			t.Fatalf("transcript section has %d rows, want %d at once", got, m.scrollList.height)
		}
	}

	t.Run("a header moves the transcript down", func(t *testing.T) {
		m, _ := directImageGlideModel(t)
		spot := m.gfxSpots[0]
		m.getHeader = func() *WidgetData { return &WidgetData{Text: "header\nsecond row"} }
		m.layoutDirty = true
		settled(t, m)
		if len(m.gfxSpots) != 1 || m.gfxSpots[0].row != spot.row+lipgloss.Height(m.chrome.header) {
			t.Fatalf("the image is at %+v, want it %d rows below row %d", m.gfxSpots, lipgloss.Height(m.chrome.header), spot.row)
		}
	})

	t.Run("a section covers part of the image", func(t *testing.T) {
		m, _ := directImageGlideModel(t)
		spot := m.gfxSpots[0]
		// A widget that leaves the transcript the rows down to the middle of
		// the image. It is not so tall that the frame has to drop it.
		keep := spot.row + 2
		if keep < streamMinRows {
			t.Fatalf("the image is on row %d: the transcript cannot be cut there", spot.row)
		}
		rows := m.scrollList.height - keep
		m.getWidgets = func(slot string) []WidgetData {
			if slot != "above" {
				return nil
			}
			return []WidgetData{{ID: "tall", NoBorder: true, Text: strings.TrimSuffix(strings.Repeat("w\n", rows), "\n")}}
		}
		m.layoutDirty = true
		settled(t, m)
		if len(m.gfxSpots) == 1 && m.gfxSpots[0] == spot {
			t.Fatalf("the widget left the image whole (%d transcript rows): the test checks nothing", m.scrollList.height)
		}
	})

	t.Run("no image, the same change glides", func(t *testing.T) {
		m, _ := glideModel(t, nil)
		before := m.scrollList.height
		m.getHeader = func() *WidgetData { return &WidgetData{Text: "header\nsecond row"} }
		m.layoutDirty = true
		_ = m.View()
		if got := regionRows(m.ui.Frame(), regionStream); got != before {
			t.Fatalf("without an image the transcript section has %d rows on the first frame, want the %d it had", got, before)
		}
	})
}

// tallWidget returns a widget source with one six-row widget above the
// composer.
func tallWidget(slot string) []WidgetData {
	if slot != "above" {
		return nil
	}
	return []WidgetData{{ID: "tall", Text: "w1\nw2\nw3\nw4\nw5\nw6"}}
}

// transcriptRows returns the first rows of a frame as plain text.
func transcriptRows(view string, rows int) string {
	return plainRows(strings.Join(strings.Split(view, "\n")[:rows], "\n"))
}

// glideModel returns a model with motion on, a full transcript and a root on
// a clock that the test moves, after its first frame.
func glideModel(t *testing.T, widgets func(string) []WidgetData) (*AppModel, *testClock) {
	t.Helper()
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	enableMotion(t, m)
	clock := &testClock{now: time.Unix(1000, 0)}
	m.ui = bloom.New(bloom.WithClock(clock.Now), bloom.WithFPS(motionFPS))
	fillTranscript(m)
	m.getWidgets = widgets
	m.layoutDirty = true
	_ = m.View()
	return m, clock
}

// glideUntil draws frames until the transcript section has a number of rows
// that ok accepts, and returns that number and the frame.
func glideUntil(t *testing.T, m *AppModel, clock *testClock, ok func(rows int) bool) (int, string) {
	t.Helper()
	for range 40 {
		clock.advance(layoutGlide / 20)
		view := m.View().Content
		if rows := regionRows(m.ui.Frame(), regionStream); ok(rows) {
			return rows, view
		}
	}
	t.Fatal("no frame had the transcript at a height in between")
	return 0, ""
}

// TestMouseHitTestFollowsTheDrawnTranscript checks a click on the transcript
// while its section shrinks. The transcript on screen then has more rows than
// the scroll list has. A click on one of the rows that the list's own
// viewport does not have must land on the line that is drawn there.
func TestMouseHitTestFollowsTheDrawnTranscript(t *testing.T) {
	m, clock := glideModel(t, nil)
	before := m.scrollList.height

	// The text stays where it is while the section shrinks, so each row
	// shows the line that it shows at rest.
	type place struct{ item, line int }
	atRest := make([]place, before)
	for y := range atRest {
		atRest[y].item, atRest[y].line = m.scrollList.getItemAndLineAtY(y)
	}

	m.getWidgets = tallWidget
	m.layoutDirty = true
	_ = m.View()
	target := m.scrollList.height

	rows, _ := glideUntil(t, m, clock, func(rows int) bool { return rows > target && rows < before })
	if _, got := m.currentScrollbackBounds(); got != rows {
		t.Fatalf("hit-test height = %d, want the %d rows on screen (the list has %d)", got, rows, target)
	}

	// A click on the last row that is drawn. It is below the list's own
	// viewport, which has only the rows that the layout will settle on.
	want := atRest[rows-1]
	if want.item < 0 {
		t.Fatalf("row %d shows no line at rest: the test needs a line there", rows-1)
	}
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 4, Y: rows - 1})
	sel := m.scrollList.sel
	if !sel.MouseDown {
		t.Fatal("the click on the last drawn row of the transcript started no selection")
	}
	if sel.MouseDownItemIdx != want.item || sel.MouseDownLineIdx != want.line {
		t.Fatalf("the click landed on item %d line %d, want item %d line %d (the line under the pointer)",
			sel.MouseDownItemIdx, sel.MouseDownLineIdx, want.item, want.line)
	}
	if m.scrollList.height != target {
		t.Fatalf("the hit test changed the height of the list: %d, want %d", m.scrollList.height, target)
	}
}

// TestClickAndDragWhileTranscriptGrows checks a click and a drag on the
// transcript while its section grows and new content arrives. The transcript
// on screen then has fewer rows than the scroll list has, and the new content
// put the list at the bottom of its own, taller viewport: the rows on screen
// start at a later line than the list's own viewport does.
//
// The click stops the list following the bottom. The list must go on from
// the line that is at the top of the screen: the next frame shows the same
// rows, and a drag along the row of the click stays on the line of the click.
func TestClickAndDragWhileTranscriptGrows(t *testing.T) {
	m, clock := glideModel(t, tallWidget)
	before := m.scrollList.height

	// The widget goes: the transcript gets its rows.
	m.getWidgets = nil
	m.layoutDirty = true
	_ = m.View()
	target := m.scrollList.height
	if target < before+5 {
		t.Fatalf("the transcript got %d rows, want at least 5", target-before)
	}
	rows, _ := glideUntil(t, m, clock, func(rows int) bool { return rows > before && rows < target-1 })

	// New content arrives while the section grows. The list follows it to
	// the bottom of its own viewport, and the screen shows the last lines.
	m.printSystemMessage("a line that streamed in")
	view := m.View().Content
	if got := regionRows(m.ui.Frame(), regionStream); got != rows {
		t.Fatalf("the transcript has %d rows with no time passed, want %d", got, rows)
	}
	shown := transcriptRows(view, rows)
	wantItem := len(m.scrollList.items) - 1
	wantLine := m.scrollList.itemHeight(wantItem) - 1
	own := transcriptRows(m.scrollList.View(), rows)
	if own == shown {
		t.Fatal("the screen shows the rows of the list's own viewport: the test checks nothing")
	}

	// A click on the last row that is drawn: the last line of the transcript.
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 4, Y: rows - 1})
	sel := m.scrollList.sel
	if !sel.MouseDown || sel.MouseDownItemIdx != wantItem || sel.MouseDownLineIdx != wantLine {
		t.Fatalf("the click landed on item %d line %d (down=%v), want item %d line %d",
			sel.MouseDownItemIdx, sel.MouseDownLineIdx, sel.MouseDown, wantItem, wantLine)
	}
	if m.scrollList.autoScroll {
		t.Fatal("the list still follows the bottom after the click")
	}

	// No time has passed: the next frame must show the rows that were
	// clicked on, not the rows of the list's own, taller viewport.
	view = m.View().Content
	if got := regionRows(m.ui.Frame(), regionStream); got != rows {
		t.Fatalf("the transcript has %d rows with no time passed, want %d", got, rows)
	}
	if got := transcriptRows(view, rows); got != shown {
		t.Fatalf("the transcript jumped after the click\nbefore:\n%s\nafter:\n%s", shown, got)
	}

	// A drag along the same row stays on the line of the click.
	m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: 12, Y: rows - 1})
	sel = m.scrollList.sel
	if sel.DragItemIdx != wantItem || sel.DragLineIdx != wantLine {
		t.Fatalf("the drag is on item %d line %d, want item %d line %d (the line of the click)",
			sel.DragItemIdx, sel.DragLineIdx, wantItem, wantLine)
	}

	// The animation runs to its end. The transcript fills its section: the
	// list never shows blank rows below its last line.
	for range 40 {
		clock.advance(layoutGlide / 20)
		if got := lipgloss.Height(m.View().Content); got != m.height {
			t.Fatalf("frame height = %d, want %d", got, m.height)
		}
	}
	if got := regionRows(m.ui.Frame(), regionStream); got != target {
		t.Fatalf("after the animation the transcript has %d rows, want %d", got, target)
	}
	if m.scrollList.anchored {
		t.Fatal("the list still has an anchor after the animation")
	}
	if item, line := m.scrollList.getItemAndLineAtY(target - 1); item != wantItem || line != wantLine {
		t.Fatalf("the last row shows item %d line %d, want the last line of the transcript (item %d line %d)",
			item, line, wantItem, wantLine)
	}
}

// TestViewAtHeightJoinsUpAfterAHeightChange checks the frames between two
// heights of a list that has not moved since its height changed. They start
// from the line that the list showed before the change, so the first of them
// is the view from before and the last is the list's own view.
func TestViewAtHeightJoinsUpAfterAHeightChange(t *testing.T) {
	newList := func(height int) *ScrollList {
		m, _, _ := newTestAppModel(&stubAppController{})
		fillTranscript(m)
		m.scrollList.SetHeight(height)
		m.scrollList.GotoBottom()
		m.scrollList.releaseAnchor()
		return m.scrollList
	}
	rowsOf := func(view string) []string { return strings.Split(view, "\n") }

	// The viewport shrinks: the first line stays, the last rows are covered.
	t.Run("shrinks", func(t *testing.T) {
		list := newList(14)
		before := rowsOf(list.View())
		list.SetHeight(10)
		for h := 14; h >= 10; h-- {
			if got := list.ViewAtHeight(h); got != strings.Join(before[:h], "\n") {
				t.Fatalf("at %d rows the list does not show the first %d rows from before", h, h)
			}
		}
	})

	// The viewport grows: the last line stays at the bottom.
	t.Run("grows", func(t *testing.T) {
		list := newList(10)
		before := list.View()
		list.SetHeight(14)
		after := rowsOf(list.View())
		if got := list.ViewAtHeight(10); got != before {
			t.Fatal("at the old height the list does not show the view from before")
		}
		for h := 10; h <= 14; h++ {
			if got := list.ViewAtHeight(h); got != strings.Join(after[14-h:], "\n") {
				t.Fatalf("at %d rows the list does not end with the last %d rows of its own view", h, h)
			}
		}
	})

	// A list that was scrolled away and back is where the height change
	// left it, but the user put it there: it is drawn from where it is.
	t.Run("scrolled away and back", func(t *testing.T) {
		list := newList(10)
		list.SetHeight(14)
		list.autoScroll = false
		idx, line := list.offsetIdx, list.offsetLine
		own := rowsOf(list.View())

		list.ScrollBy(-1)
		list.ScrollBy(1)
		if list.offsetIdx != idx || list.offsetLine != line {
			t.Fatalf("the list is at %d/%d after a scroll up and down, want %d/%d: the test needs it back",
				list.offsetIdx, list.offsetLine, idx, line)
		}
		if list.anchorHolds() {
			t.Fatal("the anchor holds again after the list was scrolled away and back")
		}
		if got := list.ViewAtHeight(10); got != strings.Join(own[:10], "\n") {
			t.Fatal("a list that was scrolled is not drawn from its own first line")
		}
	})

	// Each method that puts the list somewhere ends the anchor.
	for name, move := range map[string]func(*ScrollList){
		"ScrollBy":      func(l *ScrollList) { l.ScrollBy(-2) },
		"GotoTop":       func(l *ScrollList) { l.GotoTop() },
		"GotoBottom":    func(l *ScrollList) { l.GotoBottom() },
		"EnsureVisible": func(l *ScrollList) { l.EnsureVisible(3) },
	} {
		t.Run(name+" releases the anchor", func(t *testing.T) {
			list := newList(10)
			list.SetHeight(14)
			if !list.anchorHolds() {
				t.Fatal("no anchor after a height change: the test checks nothing")
			}
			move(list)
			if list.anchored {
				t.Fatalf("%s left the anchor set", name)
			}
		})
	}

	// A list that moved after the change is drawn from where it is.
	t.Run("moved since", func(t *testing.T) {
		list := newList(14)
		list.SetHeight(10)
		list.autoScroll = false
		list.ScrollBy(-3)
		own := rowsOf(list.View())
		if list.anchorHolds() {
			t.Fatal("the anchor still holds after the list was scrolled")
		}
		if got := rowsOf(list.ViewAtHeight(12)); strings.Join(got[:10], "\n") != strings.Join(own, "\n") {
			t.Fatal("a list that was scrolled is not drawn from its own first line")
		}
	})
}

// TestStopFollowingAt checks what a click does to the viewport of a list that
// follows the bottom: nothing at the list's own height, and at another height
// the list goes on from the line that is at the top of the screen.
func TestStopFollowingAt(t *testing.T) {
	newList := func() *ScrollList {
		m, _, _ := newTestAppModel(&stubAppController{})
		fillTranscript(m)
		m.scrollList.SetHeight(10)
		m.scrollList.GotoBottom()
		m.scrollList.releaseAnchor()
		return m.scrollList
	}

	t.Run("own height", func(t *testing.T) {
		list := newList()
		idx, line := list.offsetIdx, list.offsetLine
		list.stopFollowingAt(10)
		if list.autoScroll || list.anchored || list.offsetIdx != idx || list.offsetLine != line {
			t.Fatalf("follow=%v anchored=%v offset=%d/%d, want a list stopped at %d/%d",
				list.autoScroll, list.anchored, list.offsetIdx, list.offsetLine, idx, line)
		}
	})

	// The list is drawn with 14 rows on its way down to 10. The rows on
	// screen stay: the list's own viewport is the first 10 of them.
	t.Run("drawn taller", func(t *testing.T) {
		list := newList()
		drawn := strings.Split(list.ViewAtHeight(14), "\n")
		list.stopFollowingAt(14)
		if got := list.ViewAtHeight(14); got != strings.Join(drawn, "\n") {
			t.Fatal("the rows on screen changed when the list stopped following the bottom")
		}
		if got := list.View(); got != strings.Join(drawn[:10], "\n") {
			t.Fatal("the list's own viewport is not the first rows of what was on screen")
		}
	})

	// The list is drawn with 6 rows on its way up to 10. The rows on screen
	// stay for now; the list's own viewport ends with them, since it shows
	// no blank rows below the last line.
	t.Run("drawn shorter", func(t *testing.T) {
		list := newList()
		drawn := list.ViewAtHeight(6)
		own := list.View()
		list.stopFollowingAt(6)
		if got := list.ViewAtHeight(6); got != drawn {
			t.Fatal("the rows on screen changed when the list stopped following the bottom")
		}
		if got := list.View(); got != own {
			t.Fatal("the list's own viewport moved: it must still end at the last line")
		}
		// A scroll ends the anchor: the list is drawn from where it is.
		list.ScrollBy(-1)
		if list.anchorHolds() {
			t.Fatal("the anchor still holds after a scroll")
		}
	})
}

// TestScrollbarGoesWhenTheUserIsBackAtTheEnd checks the two page keys. Page
// Up stops the transcript from following the conversation and shows the
// scrollbar. Page Down to the end starts the following again, and the
// scrollbar goes.
func TestScrollbarGoesWhenTheUserIsBackAtTheEnd(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	m.input = NewInputComponent(80, nil)
	fillTranscript(m)
	m.layoutDirty = true
	_ = m.View()
	rows := m.scrollList.height

	m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyPgUp})
	if m.scrollList.autoScroll {
		t.Fatal("the transcript still follows the conversation after Page Up")
	}
	if !strings.Contains(lastColumn(m.View().Content, rows), "┃") {
		t.Fatal("no scrollbar after Page Up")
	}

	m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	if !m.scrollList.AtBottom() {
		t.Fatal("one Page Down did not bring the transcript back to its end")
	}
	if !m.scrollList.autoScroll {
		t.Fatal("the transcript does not follow the conversation after Page Down to its end")
	}
	if strings.Contains(lastColumn(m.View().Content, rows), "┃") {
		t.Fatal("the scrollbar is still there at the end of the transcript")
	}
}
