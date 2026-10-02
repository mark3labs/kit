package ui

import (
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/style"
)

func TestShellRunItemShowsHeaderImmediately(t *testing.T) {
	item := NewShellRunItem("id", "ls -la", false)

	if !item.Spinning() {
		t.Error("a new item must be pending")
	}
	out := xansi.Strip(item.Render(80))
	if !strings.Contains(out, "$ ls -la") {
		t.Errorf("header missing: %q", out)
	}
	if !strings.Contains(out, "running") {
		t.Errorf("pending block must show a running marker: %q", out)
	}
}

func TestShellRunItemExcludedMarker(t *testing.T) {
	item := NewShellRunItem("id", "secret", true)
	if out := xansi.Strip(item.Render(80)); !strings.Contains(out, "excluded from context") {
		t.Errorf("exclusion marker missing: %q", out)
	}
}

// While the command runs the window follows the end; once it is over the window
// snaps back to the start. This is the behaviour that makes a live block usable.
func TestShellRunItemWindowFlipsAtCompletion(t *testing.T) {
	var out strings.Builder
	for i := range 60 {
		out.WriteString("line-")
		out.WriteString(strings.Repeat("0", 0))
		out.WriteString(itoa(i + 1))
		out.WriteString("\n")
	}

	item := NewShellRunItem("id", "seq 1 60", false)
	item.AppendOutput(out.String())

	live := xansi.Strip(item.Render(80))
	if !strings.Contains(live, "line-60") {
		t.Errorf("live window must show the newest line: %q", live)
	}
	if strings.Contains(live, "line-1\n") {
		t.Errorf("live window must not show the oldest line")
	}
	if !strings.Contains(live, "earlier lines") {
		t.Errorf("live window must report the hidden lines above: %q", live)
	}

	item.MarkComplete(out.String(), 0)
	done := xansi.Strip(item.Render(80))
	if !strings.Contains(done, "line-1") {
		t.Errorf("finished window must show the oldest line: %q", done)
	}
	if !strings.Contains(done, "more lines") {
		t.Errorf("finished window must report the hidden lines below: %q", done)
	}
	if strings.Contains(done, "running") {
		t.Error("a finished block must not claim to be running")
	}
}

func TestShellRunItemExpansionLiftsTheCap(t *testing.T) {
	item := NewShellRunItem("id", "seq 1 60", false)
	item.MarkComplete(lines(1, 60), 0)

	collapsed := xansi.Strip(item.Render(80))
	if !strings.Contains(collapsed, "more lines") {
		t.Fatalf("expected a cap at 60 lines: %q", collapsed)
	}
	if strings.Contains(collapsed, "line-30") {
		t.Error("collapsed block must not show the middle of a long run")
	}

	if expanded := item.ToggleExpanded(); !expanded {
		t.Fatal("ToggleExpanded should report the expanded state")
	}
	full := xansi.Strip(item.Render(80))
	if strings.Contains(full, "more lines") {
		t.Errorf("expanded block must show everything: %q", full)
	}
	if !strings.Contains(full, "line-30") {
		t.Errorf("expanded block must show the middle of a long run: %q", full)
	}

	if item.ToggleExpanded() {
		t.Fatal("ToggleExpanded must toggle back")
	}
}

// The whole point of the feature: colour from the command reaches the frame.
func TestShellRunItemKeepsCommandColour(t *testing.T) {
	item := NewShellRunItem("id", "colour", false)
	item.AppendOutput("\x1b[31mRED\x1b[0m plain\n")
	item.MarkComplete("\x1b[31mRED\x1b[0m plain\n", 0)

	out := item.Render(80)
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("rendered block carries no escape sequences: %q", out)
	}
	if strings.Contains(out, "\x1b[31m") {
		t.Errorf("base colour was not remapped: %q", out)
	}
	if !strings.Contains(xansi.Strip(out), "RED plain") {
		t.Errorf("text lost: %q", xansi.Strip(out))
	}
}

// An embedded reset must not punch a hole in the panel fill. The padding after
// the content has to carry the background again.
func TestShellRunItemFillSurvivesInnerReset(t *testing.T) {
	item := NewShellRunItem("id", "colour", false)
	item.MarkComplete("\x1b[31mRED\x1b[0m\n", 0)

	out := item.Render(80)
	row := ""
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(xansi.Strip(line), "RED") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatalf("output row not found in %q", xansi.Strip(out))
	}
	if !strings.Contains(row, "\x1b[48;") {
		t.Errorf("panel fill missing from the output row: %q", row)
	}
	// The fill has to be re-armed after the content, not only present at the
	// start of the row: the program's own reset would otherwise leave the
	// trailing padding unstyled.
	text := strings.Index(row, "RED")
	if text < 0 {
		t.Fatalf("output text missing from the row: %q", row)
	}
	if !strings.Contains(row[text:], "\x1b[48;") {
		t.Errorf("fill was not re-armed after the content: %q", row)
	}
}

// Cursor control must never reach the frame.
func TestShellRunItemStripsCursorControl(t *testing.T) {
	raw := "\x1b[2J\x1b[H\x1b[10;10H\x1b[KTEXT\x1b[1;1Hmore\n"
	item := NewShellRunItem("id", "erase", false)
	item.MarkComplete(raw, 0)

	out := item.Render(80)
	for _, seq := range []string{"\x1b[2J", "\x1b[H", "\x1b[10;10H", "\x1b[K", "\x1b[1;1H"} {
		if strings.Contains(out, seq) {
			t.Errorf("cursor control %q survived into the render", seq)
		}
	}
	if plain := xansi.Strip(out); !strings.Contains(plain, "TEXT") {
		t.Errorf("text lost: %q", plain)
	}
}

// A chunk boundary can land inside an escape sequence. The half must not be
// printed as text.
func TestShellRunItemSplitEscapeIsNotPrinted(t *testing.T) {
	item := NewShellRunItem("id", "colour", false)
	item.AppendOutput("\x1b[31mRED\x1b[0m plain\n\x1b[3")
	out := item.Render(80)

	if strings.Contains(xansi.Strip(out), "[3") {
		t.Errorf("partial escape sequence printed as text: %q", xansi.Strip(out))
	}
	if !strings.Contains(xansi.Strip(out), "plain") {
		t.Errorf("earlier text lost: %q", xansi.Strip(out))
	}
}

func TestShellRunItemExitCode(t *testing.T) {
	item := NewShellRunItem("id", "fail", false)
	item.MarkComplete("boom\n", 3)
	if out := xansi.Strip(item.Render(80)); !strings.Contains(out, "exit 3") {
		t.Errorf("exit code not shown: %q", out)
	}
}

func TestShellRunItemTimeout(t *testing.T) {
	item := NewShellRunItem("id", "sleep 99", false)
	item.MarkTimedOut("partial\n")
	out := xansi.Strip(item.Render(80))
	if !strings.Contains(out, "timed out") {
		t.Errorf("timeout not shown: %q", out)
	}
	if !strings.Contains(out, "partial") {
		t.Errorf("output before the timeout was lost: %q", out)
	}
	if item.Spinning() {
		t.Error("a timed-out block is not pending")
	}
}

// Output arriving after the block was settled must not reopen it.
func TestShellRunItemIgnoresChunksAfterCompletion(t *testing.T) {
	item := NewShellRunItem("id", "cmd", false)
	item.MarkComplete("final\n", 0)
	item.AppendOutput("late\n")

	if out := xansi.Strip(item.Render(80)); strings.Contains(out, "late") {
		t.Errorf("late chunk was accepted: %q", out)
	}
}

func TestShellRunItemHorizontalScroll(t *testing.T) {
	line := "HEAD-" + strings.Repeat("X", 400) + "-TAIL"
	item := NewShellRunItem("id", "wide", false)
	item.MarkComplete(line+"\n", 0)

	start := xansi.Strip(item.Render(80))
	if !strings.Contains(start, "HEAD") {
		t.Fatalf("expected the line start at offset 0: %q", start)
	}
	if strings.Contains(start, "TAIL") {
		t.Fatal("the line end should not fit at offset 0")
	}

	if !item.ScrollHorizontalBy(1) {
		t.Fatal("scrolling right should move the window")
	}
	if !item.ScrollHorizontalBy(1) {
		t.Fatal("scrolling right should keep moving the window")
	}
	mid := xansi.Strip(item.Render(80))
	if strings.Contains(mid, "HEAD") {
		t.Errorf("the line start should have scrolled out: %q", mid)
	}

	// Scrolling past the end clamps and then reports no movement.
	for range 200 {
		item.ScrollHorizontalBy(1)
	}
	end := xansi.Strip(item.Render(80))
	if !strings.Contains(end, "TAIL") {
		t.Errorf("scrolling to the end must reveal the line end: %q", end)
	}
	if item.ScrollHorizontalBy(1) {
		t.Error("scrolling past the end must report no movement")
	}

	if item.ScrollHorizontalBy(-1) != true {
		t.Error("scrolling back must move the window")
	}
	if !item.ScrollHorizontalBy(-1) {
		t.Error("scrolling back must keep moving")
	}
	for range 200 {
		item.ScrollHorizontalBy(-1)
	}
	if item.ScrollHorizontalBy(-1) {
		t.Error("scrolling before the start must report no movement")
	}
	if start2 := xansi.Strip(item.Render(80)); !strings.Contains(start2, "HEAD") {
		t.Errorf("scrolling back to zero must restore the line start: %q", start2)
	}
}

func TestShellRunItemRawContentKeepsEverything(t *testing.T) {
	item := NewShellRunItem("id", "cmd", false)
	item.MarkComplete(lines(1, 101), 0)

	raw := item.RawContent()
	for _, want := range []string{"$ cmd", "line-1\n", "line-100\n"} {
		if !strings.Contains(raw, want) {
			t.Errorf("RawContent missing %q", want)
		}
	}
}

// The block must not grow past the terminal width, at any width.
func TestShellRunItemFitsWidth(t *testing.T) {
	for _, width := range []int{20, 40, 60, 80, 120, 200} {
		item := NewShellRunItem("id", strings.Repeat("c", 300), false)
		item.MarkComplete(strings.Repeat("W", 500)+"\n", 0)
		rendered := item.Render(width)
		for line := range strings.SplitSeq(rendered, "\n") {
			if w := xansi.StringWidth(line); w > width {
				t.Errorf("width %d: line is %d columns: %q", width, w, line)
			}
		}
	}
}

// The render cache must not survive a theme switch: it holds colour codes.
func TestShellRunItemCacheFollowsTheme(t *testing.T) {
	item := NewShellRunItem("id", "cmd", false)
	item.MarkComplete("\x1b[31mRED\x1b[0m\n", 0)

	if before := item.Render(80); before == "" {
		t.Fatal("first render produced nothing")
	}
	if item.pending {
		t.Fatal("the item should be settled")
	}
	if item.cachedRender == "" {
		t.Fatal("a settled block must cache its render")
	}

	item.gen = 0 // as if the theme had changed

	after := item.Render(80)
	if item.gen == 0 {
		t.Error("the theme generation was not re-stamped")
	}
	if item.cachedRender == "" {
		t.Error("the settled block did not re-cache its render after the theme change")
	}
	// The remap reads the palette, so a fresh render may differ; what matters is
	// that it happened and stayed valid output.
	if !strings.Contains(xansi.Strip(after), "RED") {
		t.Errorf("text lost after theme change: %q", xansi.Strip(after))
	}
}

func TestShellRunItemRole(t *testing.T) {
	item := NewShellRunItem("id", "cmd", false)
	if item.Role() != "shell" {
		t.Errorf("Role = %q, want shell", item.Role())
	}
	if item.ID() != "id" {
		t.Errorf("ID = %q, want id", item.ID())
	}
}

// The block must satisfy the transcript's own interfaces, or navigation and the
// inspector skip it.
func TestShellRunItemImplementsTranscriptInterfaces(t *testing.T) {
	item := NewShellRunItem("id", "cmd", false)
	var _ MessageItem = item
	var _ InspectableItem = item
	if itemRole(item) != "shell" {
		t.Error("itemRole did not pick up the shell role")
	}
}

// Normalizing the same bytes one chunk at a time must land on the same rendering
// as normalizing them at once. This is the property that makes chunked streaming
// safe.
func TestShellRunItemIsSplitInsensitive(t *testing.T) {
	full := "\x1b[1;31mfirst\x1b[0m\n\x1b[32msecond\x1b[0m\n\x1b[38;5;208mthird\x1b[0m\n"

	whole := NewShellRunItem("a", "c", false)
	whole.MarkComplete(full, 0)

	split := NewShellRunItem("b", "c", false)
	for i := range len(full) {
		split.AppendOutput(string(full[i]))
	}
	split.MarkComplete(full, 0)

	if got, want := xansi.Strip(split.Render(80)), xansi.Strip(whole.Render(80)); got != want {
		t.Errorf("split rendering differs:\n got %q\nwant %q", got, want)
	}
}

// ---------------------------------------------------------------------------

func lines(from, to int) string {
	var b strings.Builder
	for i := range to - from {
		b.WriteString("line-")
		b.WriteString(itoa(from + i))
		b.WriteByte('\n')
	}
	return b.String()
}

func TestLastWindowBytes(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		count   int
		maxByte int
		want    string
	}{
		{"fewer lines than the cap", "a\nb\n", 5, 1 << 20, "a\nb\n"},
		{"last n lines", "1\n2\n3\n4\n5\n", 2, 1 << 20, "4\n5\n"},
		{"byte cap not reached", "1\n2\n3\n", 5, 1 << 20, "1\n2\n3\n"},
		// The line boundary is preferred over the exact byte count, so the
		// window may come out shorter than maxBytes. A cut inside a line is the
		// one that can land inside an escape sequence.
		{"byte cap advances to a line boundary", "1\n2\n3\n4\n5\n6\n", 10, 4, "6\n"},
		{"byte cap lands on a newline", "1\n2\n3\n4\n", 10, 3, "4\n"},
		{"one enormous line", strings.Repeat("X", 100), 10, 10, strings.Repeat("X", 10)},
		{"empty", "", 10, 1 << 20, ""},
		{"no trailing newline", "a\nb\nc", 2, 1 << 20, "b\nc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lastWindowBytes(tt.in, tt.count, tt.maxByte); got != tt.want {
				t.Errorf("lastWindowBytes(%q, %d, %d) = %q, want %q", tt.in, tt.count, tt.maxByte, got, tt.want)
			}
		})
	}
}

// A byte cut must not split a multi-byte character.
func TestLastWindowBytesKeepsRunesWhole(t *testing.T) {
	in := strings.Repeat("日", 100) // three bytes each
	got := lastWindowBytes(in, 10, 10)
	if !utf8.ValidString(got) {
		t.Errorf("cut split a rune: %q", got)
	}
}

// The window cut must not print a fragment of an escape sequence. This is the
// one place a line boundary is unavailable.
func TestShellRunItemHugeLineDoesNotPrintEscapeFragment(t *testing.T) {
	line := "\x1b[32m" + strings.Repeat("X", 200_000) + "\x1b[0m"
	item := NewShellRunItem("id", "huge", false)
	item.AppendOutput(line)

	out := xansi.Strip(item.Render(80))
	if strings.Contains(out, ";32m") || strings.Contains(out, "[32m") {
		t.Errorf("an escape fragment reached the frame: %.120q", out)
	}
	if !strings.Contains(out, "X") {
		t.Errorf("content lost: %.120q", out)
	}
}

// The window is rebuilt when the output changes and reused when it does not, so
// an idle frame costs nothing.
func TestShellRunItemCachesAcrossIdleFrames(t *testing.T) {
	item := NewShellRunItem("id", "cmd", false)
	item.AppendOutput("one\ntwo\n")

	first := item.Render(80)
	window := item.window
	if !item.windowValid {
		t.Fatal("the window was not built on first render")
	}

	second := item.Render(80)
	if second != first {
		t.Errorf("an idle frame changed the render:\n%q\n%q", first, second)
	}
	if item.window != window {
		t.Error("an idle frame rebuilt the window")
	}

	item.AppendOutput("three\n")
	third := item.Render(80)
	if third == first {
		t.Error("new output did not change the render")
	}
	if item.window == window {
		t.Error("new output did not rebuild the window")
	}
	if !strings.Contains(xansi.Strip(third), "three") {
		t.Errorf("new output missing: %q", xansi.Strip(third))
	}
}

// A resize must rebuild the render even though the output is unchanged.
func TestShellRunItemCacheFollowsWidth(t *testing.T) {
	item := NewShellRunItem("id", "cmd", false)
	item.MarkComplete(strings.Repeat("W", 400)+"\n", 0)

	narrow := item.Render(40)
	wide := item.Render(120)
	if narrow == wide {
		t.Error("two widths produced the same render")
	}
	for line := range strings.SplitSeq(wide, "\n") {
		if w := xansi.StringWidth(line); w > 120 {
			t.Errorf("wide render line is %d columns: %q", w, line)
		}
	}
}

func TestFirstWindowBytes(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		maxByte int
		want    string
	}{
		{"under budget", "a\nb\n", 100, "a\nb\n"},
		{"exactly at budget", "abcd", 4, "abcd"},
		{"over budget, cut at a newline", "1\n2\n3\n4\n", 4, "1\n2\n"},
		{"one enormous line", strings.Repeat("X", 100), 10, strings.Repeat("X", 10)},
		{"empty", "", 10, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstWindowBytes(tt.in, tt.maxByte); got != tt.want {
				t.Errorf("firstWindowBytes(%q, %d) = %q, want %q", tt.in, tt.maxByte, got, tt.want)
			}
		})
	}
}

// Expanding a very large output must not hang the frame. Both budgets apply
// there too, and the notice has to admit it.
func TestShellRunItemExpandedViewIsBounded(t *testing.T) {
	big := strings.Repeat("0123456789abcdef\n", 60_000) // about 1 MB

	item := NewShellRunItem("id", "huge", false)
	item.MarkComplete(big, 0)

	if !item.ToggleExpanded() {
		t.Fatal("ToggleExpanded should report the expanded state")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		item.Render(80)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("expanding a very large output hung the render")
	}

	out := xansi.Strip(item.Render(80))
	if !strings.Contains(out, "not shown") {
		t.Errorf("the expanded cap is not disclosed: %.200q", out)
	}
	// The rendered block must stay inside its budget rather than drawing every
	// line the command produced.
	if rows := strings.Count(out, "\n"); rows > shellExpandedMaxLines+8 {
		t.Errorf("expanded block rendered %d rows, want at most %d", rows, shellExpandedMaxLines+8)
	}
	if !strings.Contains(out, "0") {
		t.Errorf("nothing rendered: %.120q", out)
	}
}

// The hidden count must describe lines the block actually dropped. Most
// commands end their output with a newline, and trimming it used to leave the
// notice one line too high on nearly every capped block.
func TestShellRunItemHiddenCountIsAccurate(t *testing.T) {
	const total = 60

	// With a trailing newline: 60 lines, 12 shown, 48 hidden.
	trimmed := NewShellRunItem("id", "seq 1 60", false)
	trimmed.MarkComplete(lines(1, total+1), 0)
	if got := noticeLineOf(t, trimmed); !strings.Contains(got, "48 more lines") {
		t.Errorf("with a trailing newline: %q, want 48 more lines", got)
	}

	// Without one: the same 60 lines, the same 48 hidden.
	untrimmed := NewShellRunItem("id", "seq 1 60", false)
	untrimmed.MarkComplete(strings.TrimSuffix(lines(1, total+1), "\n"), 0)
	if got := noticeLineOf(t, untrimmed); !strings.Contains(got, "48 more lines") {
		t.Errorf("without a trailing newline: %q, want 48 more lines", got)
	}
}

func TestShellRunItemHiddenCountWhilePending(t *testing.T) {
	item := NewShellRunItem("id", "seq 1 60", false)
	for i := 1; i <= 60; i++ {
		item.AppendOutput(itoa(i) + "\n")
	}
	if got := noticeLineOf(t, item); !strings.Contains(got, "48 earlier lines") {
		t.Errorf("%q, want 48 earlier lines", got)
	}
}

// noticeLineOf returns the truncation notice from a rendered block.
func noticeLineOf(t *testing.T, item *ShellRunItem) string {
	t.Helper()
	for line := range strings.SplitSeq(xansi.Strip(item.Render(80)), "\n") {
		if strings.Contains(line, "more lines") || strings.Contains(line, "earlier lines") {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("no truncation notice in %q", xansi.Strip(item.Render(80)))
	return ""
}

// A window cut inside an escape sequence must not print the fragment as text.
// This needs the cut to land inside the sequence rather than in the text run,
// which is what happens when one line is long enough to hit the byte budget.
func TestShellRunItemStraddlingEscapeIsNotPrinted(t *testing.T) {
	const seq = "\x1b[32m"

	// Place the sequence so it begins two bytes before the offset
	// lastWindowBytes cuts at, putting the cut inside the final byte.
	total := shellWindowMaxBytes + 10
	cut := total - shellWindowMaxBytes
	prefix := strings.Repeat("X", cut-2)
	line := prefix + seq + strings.Repeat("Y", total-len(prefix)-len(seq))
	if len(line) != total {
		t.Fatalf("test setup: len = %d, want %d", len(line), total)
	}

	item := NewShellRunItem("id", "huge", false)
	item.AppendOutput(line)

	out := xansi.Strip(item.Render(80))
	if strings.Contains(out, "32m") {
		t.Errorf("a fragment of the sequence reached the frame: %.80q", out)
	}
	if !strings.Contains(out, "Y") {
		t.Errorf("the window is empty: %.80q", out)
	}
}

// The inspector and the clipboard both read RawContent, and neither can use
// escape sequences: one draws it inside an overlay, the other pastes it
// somewhere that has no styling.
func TestShellRunItemRawContentIsPlain(t *testing.T) {
	item := NewShellRunItem("id", "colour", false)
	item.MarkComplete("\x1b[31mRED\x1b[0m text\n", 0)

	raw := item.RawContent()
	if strings.ContainsRune(raw, 0x1b) {
		t.Errorf("RawContent carries escape sequences: %q", raw)
	}
	for _, want := range []string{"$ colour", "RED text"} {
		if !strings.Contains(raw, want) {
			t.Errorf("RawContent missing %q: %q", want, raw)
		}
	}
}

func TestShellRunItemRawContentReportsFailure(t *testing.T) {
	item := NewShellRunItem("id", "fail", false)
	item.MarkTimedOut("partial output\n")

	raw := item.RawContent()
	if !strings.Contains(raw, "(timed out)") {
		t.Errorf("RawContent does not report the timeout: %q", raw)
	}
	if !strings.Contains(raw, "partial output") {
		t.Errorf("RawContent lost the output collected before the kill: %q", raw)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
//
// The render path runs the ANSI pipeline over the whole output buffer on every
// frame while a command is live. These measure what that costs, so a change that
// makes the block quadratic shows up as a number rather than as a stutter.
// ---------------------------------------------------------------------------

var benchShellRender string

func BenchmarkShellRunItemAppendOutput(b *testing.B) {
	for _, chunkSize := range []int{100, 1024} {
		b.Run(strconv.Itoa(chunkSize)+"B_chunks", func(b *testing.B) {
			chunk := strings.Repeat("x", chunkSize-1) + "\n"
			const outputSize = 1 << 20

			b.ReportAllocs()
			b.SetBytes(outputSize)
			// A fresh item per iteration: reusing one would grow the buffer
			// across the whole benchmark and report the growth, not the append.
			for b.Loop() {
				item := NewShellRunItem("bench", "benchmark", false)
				for written := 0; written < outputSize; written += chunkSize {
					remaining := outputSize - written
					if remaining < chunkSize {
						item.AppendOutput(chunk[:remaining])
					} else {
						item.AppendOutput(chunk)
					}
				}
				benchShellRender = item.RawContent()
			}
		})
	}
}

// BenchmarkShellRunItemLiveRender measures an idle frame: output is already in
// the buffer and nothing has changed since. This is the common case while a
// long-running command produces nothing.
func BenchmarkShellRunItemLiveRender(b *testing.B) {
	item := NewShellRunItem("bench", "benchmark", false)
	item.AppendOutput(strings.Repeat("0123456789abcdef\n", (1<<20)/17))

	b.ReportAllocs()
	for b.Loop() {
		benchShellRender = item.Render(120)
	}
}

// BenchmarkShellRunItemLiveRenderChunked measures a frame that arrives with new
// output: the window is rebuilt and the panel re-rendered. This is the worst
// case, and the number that has to stay well inside a frame budget.
func BenchmarkShellRunItemLiveRenderChunked(b *testing.B) {
	item := NewShellRunItem("bench", "benchmark", false)
	item.AppendOutput(strings.Repeat("0123456789abcdef\n", (1<<20)/17))

	// Pre-build every chunk so the benchmark measures the render, not the
	// allocation of the chunks themselves.
	var chunks []string
	for range 64 {
		chunks = append(chunks, strings.Repeat("more-output\n", 64))
	}

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		item.AppendOutput(chunks[i%len(chunks)])
		benchShellRender = item.Render(120)
	}
}

// The ANSI pipeline is the expensive part, so it gets its own measurement.
func BenchmarkNormalizeOutput(b *testing.B) {
	input := strings.Repeat("\x1b[32m0123456789abcdef\x1b[0m plain\n", (1<<20)/32)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for b.Loop() {
		benchShellRender = style.NormalizeOutput(input)
	}
}
