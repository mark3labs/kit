package ui

import (
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/ui/style"
)

// --------------------------------------------------------------------------
// Scrollback repaint on theme change
// --------------------------------------------------------------------------
//
// Scrollback items memoize their styled output, which bakes ANSI color codes
// into a string. Without invalidation those strings outlive the theme they
// were built for, leaving history in the old palette while the chrome around
// it switches. These tests pin the invalidation contract.

// withTheme applies a theme for the duration of a test and restores the
// previous one afterwards. The active theme is global, so leaking one would
// make unrelated tests order-dependent.
//
// The themes used in these tests are chosen for having genuinely different
// muted/text palettes: presets that leave a color unset inherit it from the
// default theme, and comparing two themes that happen to share a color would
// make the assertions vacuous.
func withTheme(t *testing.T, name string) {
	t.Helper()
	prev := style.GetTheme()
	if err := style.ApplyThemeWithoutSave(name); err != nil {
		t.Fatalf("ApplyThemeWithoutSave(%q): %v", name, err)
	}
	t.Cleanup(func() { style.SetTheme(prev) })
}

func TestThemeGenerationAdvancesOnChange(t *testing.T) {
	withTheme(t, "kitt")
	before := style.ThemeGeneration()

	withTheme(t, "catppuccin")
	after := style.ThemeGeneration()

	if after == before {
		t.Fatalf("ThemeGeneration did not advance across a theme change (%d)", before)
	}
}

// A themed item must re-run its render closure after a theme switch, and must
// not re-run it while the theme is unchanged.
func TestThemedMessageItemRepaintsOncePerThemeChange(t *testing.T) {
	withTheme(t, "kitt")

	calls := 0
	item := NewThemedMessageItem("id", "assistant", "raw", func() string {
		calls++
		return "render-" + string(rune('a'+calls-1))
	})

	if got := item.Render(80); got != "render-a" {
		t.Fatalf("first render = %q, want %q", got, "render-a")
	}
	// Repeated draws at the same theme must be served from cache.
	item.Render(80)
	item.Render(80)
	if calls != 1 {
		t.Fatalf("render closure ran %d times without a theme change, want 1", calls)
	}

	withTheme(t, "catppuccin")

	if got := item.Render(80); got != "render-b" {
		t.Fatalf("render after theme change = %q, want %q", got, "render-b")
	}
	item.Render(80)
	if calls != 2 {
		t.Fatalf("render closure ran %d times across one theme change, want 2", calls)
	}
}

// Static items are the opt-out: content whose colors the caller owns, or which
// isn't styled text at all (image previews). They must stay put.
func TestStyledMessageItemIgnoresThemeChange(t *testing.T) {
	withTheme(t, "kitt")

	item := NewStyledMessageItem("id", "user", "raw", "fixed content")
	if got := item.Render(80); got != "fixed content" {
		t.Fatalf("render = %q, want %q", got, "fixed content")
	}

	withTheme(t, "catppuccin")

	if got := item.Render(80); got != "fixed content" {
		t.Fatalf("static item changed after theme switch: %q", got)
	}
}

// Invalidate covers the non-theme reason to repaint: the closure's own inputs
// moved. The logo animation drives its frames this way.
func TestThemedMessageItemInvalidateForcesRepaint(t *testing.T) {
	withTheme(t, "kitt")

	frame := 0
	item := NewThemedMessageItem("id", "logo", "raw", func() string {
		return "frame-" + string(rune('0'+frame))
	})

	if got := item.Render(80); got != "frame-0" {
		t.Fatalf("render = %q, want frame-0", got)
	}

	frame = 1
	if got := item.Render(80); got != "frame-0" {
		t.Fatalf("render = %q, want the cached frame-0 before Invalidate", got)
	}

	item.Invalidate()
	if got := item.Render(80); got != "frame-1" {
		t.Fatalf("render after Invalidate = %q, want frame-1", got)
	}
}

// Streaming items keep their own caches, keyed on width and completion state.
// A theme change has to cut through those too.
func TestStreamingMessageItemRepaintsOnThemeChange(t *testing.T) {
	withTheme(t, "kitt")

	// A reasoning block is the streaming item that actually paints color:
	// muted italic body plus a "Thought" label.
	item := NewStreamingMessageItem("id", "reasoning", "test-model")
	item.AppendChunk("thinking hard")
	item.MarkComplete()

	before := item.Render(80)
	if before == "" {
		t.Fatal("streaming item rendered empty")
	}
	if again := item.Render(80); again != before {
		t.Fatal("streaming item did not serve a stable render from cache")
	}

	withTheme(t, "catppuccin")

	if after := item.Render(80); after == before {
		t.Fatal("streaming item kept its old colors after a theme change")
	}
}

func TestStreamingBashOutputItemRepaintsOnThemeChange(t *testing.T) {
	withTheme(t, "kitt")

	item := NewStreamingBashOutputItem("id", "echo hi")
	item.AppendStdout("hi")
	item.MarkComplete()

	before := item.Render(80)
	if before == "" {
		t.Fatal("bash output item rendered empty")
	}

	withTheme(t, "catppuccin")

	if after := item.Render(80); after == before {
		t.Fatal("bash output item kept its old colors after a theme change")
	}
}

// --------------------------------------------------------------------------
// End-to-end: /theme repaints existing scrollback
// --------------------------------------------------------------------------

// ansiCodes extracts the SGR color sequences from a rendered string, which is
// what actually has to change when the palette does.
func ansiCodes(s string) []string {
	var codes []string
	for {
		i := strings.Index(s, "\x1b[")
		if i < 0 {
			return codes
		}
		s = s[i+2:]
		j := strings.IndexByte(s, 'm')
		if j < 0 {
			return codes
		}
		codes = append(codes, s[:j])
		s = s[j+1:]
	}
}

func TestThemeCommandRepaintsExistingScrollback(t *testing.T) {
	withTheme(t, "kitt")

	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)

	m.printUserMessage("hello there")
	m.printSystemMessage("a system note")

	if len(m.messages) < 2 {
		t.Fatalf("expected messages in the scrollback, got %d", len(m.messages))
	}

	before := make([]string, len(m.messages))
	for i, item := range m.messages {
		before[i] = item.Render(m.width)
	}

	m.handleThemeCommand("catppuccin")
	t.Cleanup(func() { _ = style.ApplyThemeWithoutSave("kitt") })

	for i := range before {
		after := m.messages[i].Render(m.width)
		if after == before[i] {
			t.Errorf("message %d kept its old rendering after /theme", i)
			continue
		}
		if strings.Join(ansiCodes(before[i]), ",") == strings.Join(ansiCodes(after), ",") {
			t.Errorf("message %d re-rendered but its colors did not change", i)
		}
	}
}

// ScrollList caches item heights measured from the previous rendering. After a
// repaint those entries have to agree with what the item now draws, or the
// scroll math and mouse hit-testing drift against the visible output.
func TestThemeCommandLeavesHeightCacheConsistent(t *testing.T) {
	withTheme(t, "kitt")

	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.printUserMessage("hello there")
	m.printSystemMessage("a system note")

	// Populate the height cache against the old theme.
	_ = m.scrollList.View()

	m.handleThemeCommand("catppuccin")
	t.Cleanup(func() { _ = style.ApplyThemeWithoutSave("kitt") })
	_ = m.scrollList.View()

	for i := range m.scrollList.items {
		id := m.scrollList.items[i].ID()
		cached, ok := m.scrollList.heightCache[id]
		if !ok {
			continue // never measured; nothing to disagree with
		}
		if want := m.scrollList.renderedHeight(i); cached != want {
			t.Errorf("item %d: cached height %d, actual rendered height %d", i, cached, want)
		}
	}
}

// InvalidateHeights is the hook handleThemeCommand relies on; pin it directly
// so a refactor of the theme path cannot quietly remove the only caller and
// leave the method looking unused.
func TestScrollListInvalidateHeights(t *testing.T) {
	sl := NewScrollList(80, 20)
	sl.SetItems([]MessageItem{
		NewStyledMessageItem("a", "user", "raw", "line one\nline two"),
		NewStyledMessageItem("b", "user", "raw", "only one line"),
	})

	_ = sl.View()
	if len(sl.heightCache) == 0 {
		t.Fatal("expected View to populate the height cache")
	}

	sl.InvalidateHeights()
	if len(sl.heightCache) != 0 {
		t.Fatalf("height cache still holds %d entries after InvalidateHeights", len(sl.heightCache))
	}
}

// An extension calling ctx.SetTheme changes the theme without going through
// /theme, so the TUI learns about it via ThemeChangedEvent. That path has to
// repaint the same things the slash command does — in particular the
// MessageRenderer's cached typography, which otherwise keeps the old palette
// for every message rendered from then on.
func TestThemeChangedEventRepaintsRenderer(t *testing.T) {
	withTheme(t, "kitt")

	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.printSystemMessage("a system note")
	before := m.messages[0].Render(m.width)

	// Simulate ctx.SetTheme: the global theme is swapped outside the TUI,
	// then the app notifies it.
	withTheme(t, "catppuccin")
	m = sendMsg(m, app.ThemeChangedEvent{})

	if after := m.messages[0].Render(m.width); after == before {
		t.Error("scrollback did not repaint after ThemeChangedEvent")
	}

	// The renderer must also be repainted, or messages rendered after the
	// switch come out in the old palette.
	m.printSystemMessage("another note")
	fresh := m.messages[len(m.messages)-1].Render(m.width)
	if strings.Join(ansiCodes(fresh), ",") == strings.Join(ansiCodes(before), ",") {
		t.Error("renderer still produces the old palette after ThemeChangedEvent")
	}
}
