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
	_ = m.View()
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
			got := plainRows(strings.Join(strings.Split(view, "\n")[:rows], "\n"))
			if got != want {
				t.Fatalf("transcript at %d rows is not the list drawn at that height\ngot:\n%s\nwant:\n%s", rows, got, want)
			}
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
// once: motion off, a window resize, and an image placed beside the view.
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
	t.Run("image placed beside the view", func(t *testing.T) {
		m := newModel(t, true)
		m.gfxPlaced = map[uint32]struct{}{7: {}}
		if m.layoutAnimates() {
			t.Fatal("layout animates while an image is placed directly on the screen")
		}
	})
}
