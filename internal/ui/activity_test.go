package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/ui/style"
)

func TestActivityVerb(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		args     string
		expected string
	}{
		{"bash", "bash", `{"command":"go test ./..."}`, "Running go test ./..."},
		{"read", "read", `{"path":"internal/ui/model.go"}`, "Reading internal/ui/model.go"},
		{"write", "write", `{"path":"a.go"}`, "Writing a.go"},
		{"edit", "edit", `{"path":"a.go"}`, "Editing a.go"},
		{"grep", "grep", `{"pattern":"func main"}`, "Searching func main"},
		{"find", "find", `{"pattern":"*.go"}`, "Matching *.go"},
		{"ls", "ls", `{"path":"/tmp"}`, "Listing /tmp"},
		{"subagent with agent", "subagent", `{"agent":"explore"}`, "Delegating to explore"},
		{"todo", "todo", `{}`, "Updating todos"},
		// Multi-line commands must collapse so the row stays one line.
		{"multiline bash", "bash", `{"command":"echo a\necho b"}`, "Running echo a echo b"},
		// Malformed and empty arguments degrade to the bare tool name rather
		// than erroring: the activity row must always render something.
		{"bad json", "read", `{not json`, "Read"},
		{"no args", "mytool", ``, "Mytool"},
		// Unknown tools still get a target when one is available.
		{"unknown with arg", "deploy", `{"query":"prod"}`, "Deploy prod"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := activityVerb(tt.tool, tt.args); got != tt.expected {
				t.Errorf("activityVerb(%q, %q) = %q, want %q", tt.tool, tt.args, got, tt.expected)
			}
		})
	}
}

func TestActivityVerbTruncatesLongTargets(t *testing.T) {
	long := strings.Repeat("x", 200)
	got := activityVerb("bash", `{"command":"`+long+`"}`)

	if lipgloss.Width(got) > activityMaxTarget+len("Running ") {
		t.Errorf("expected long command to be truncated, got width %d: %q", lipgloss.Width(got), got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("expected an ellipsis to mark elision, got %q", got)
	}
}

func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, ""},
		{-time.Second, ""},
		{500 * time.Millisecond, "<1s"},
		{time.Second, "1s"},
		{45 * time.Second, "45s"},
		{time.Minute, "1m00s"},
		{90 * time.Second, "1m30s"},
		{2*time.Minute + 7*time.Second, "2m07s"},
	}

	for _, tt := range tests {
		if got := formatElapsed(tt.d); got != tt.want {
			t.Errorf("formatElapsed(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestShortenActivityPath(t *testing.T) {
	short := "internal/ui/model.go"
	if got := shortenActivityPath(short); got != short {
		t.Errorf("short path should pass through unchanged, got %q", got)
	}

	long := "/very/deeply/nested/directory/structure/that/keeps/going/and/going/pkg/file.go"
	got := shortenActivityPath(long)
	if !strings.HasPrefix(got, "…/") {
		t.Errorf("expected elided prefix, got %q", got)
	}
	if !strings.Contains(got, "file.go") {
		t.Errorf("expected the filename to survive elision, got %q", got)
	}
	if len([]rune(got)) > activityMaxTarget {
		t.Errorf("expected result within %d columns, got %d: %q", activityMaxTarget, len([]rune(got)), got)
	}
}

// TestRenderActivityRow_IdleIsEmpty verifies the row costs nothing at rest.
func TestRenderActivityRow_IdleIsEmpty(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m = sendMsg(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	m.state = stateInput
	if got := m.renderActivityRow(); got != "" {
		t.Errorf("expected empty activity row while idle, got %q", got)
	}
}

// TestRenderActivityRow_FitsWidth verifies the row never exceeds the terminal
// width, including at narrow sizes where the hint must be dropped.
func TestRenderActivityRow_FitsWidth(t *testing.T) {
	for _, width := range []int{20, 40, 60, 80, 120, 200} {
		ctrl := &stubAppController{}
		m, stream := newTestAppModelWithRealStream(ctrl)
		m = sendMsg(m, tea.WindowSizeMsg{Width: width, Height: 30})

		m, _ = sendMsgExec(m, app.SpinnerEvent{Show: true})
		m, _ = sendMsgExec(m, app.ToolExecutionEvent{
			ToolCallID: "c1",
			ToolName:   "bash",
			ToolArgs:   `{"command":"go test ./... -run TestSomethingWithAVeryLongName"}`,
			IsStarting: true,
		})
		_ = stream
		m.state = stateWorking
		m.turnStartedAt = time.Now().Add(-12 * time.Second)

		row := m.renderActivityRow()
		if row == "" {
			t.Errorf("width=%d: expected a non-empty activity row while working", width)
			continue
		}
		if got := lipgloss.Width(row); got > width {
			t.Errorf("width=%d: activity row overflows terminal, got width %d: %q", width, got, row)
		}
		if strings.Contains(row, "\n") {
			t.Errorf("width=%d: activity row must stay a single line, got %q", width, row)
		}
	}
}

// TestRenderActivityRow_DropsHintBeforePhrase verifies the interrupt hint is
// sacrificed before the activity text when space runs short.
func TestRenderActivityRow_DropsHintBeforePhrase(t *testing.T) {
	ctrl := &stubAppController{}
	m, _ := newTestAppModelWithRealStream(ctrl)
	m = sendMsg(m, tea.WindowSizeMsg{Width: 30, Height: 30})

	m, _ = sendMsgExec(m, app.SpinnerEvent{Show: true})
	m, _ = sendMsgExec(m, app.ToolExecutionEvent{
		ToolCallID: "c1", ToolName: "bash", ToolArgs: `{"command":"make build"}`, IsStarting: true,
	})
	m.state = stateWorking

	row := stripAnsi(m.renderActivityRow())
	if strings.Contains(row, "interrupt") {
		t.Errorf("expected the hint to be dropped at narrow width, got %q", row)
	}
	if !strings.Contains(row, "Running") {
		t.Errorf("expected the activity phrase to survive, got %q", row)
	}
}

// TestComposerFillsWidth verifies the composer bar paints a contiguous
// background across the full terminal width. A background set only on the
// outer container tears at every inner ANSI reset, so this guards the
// per-style fill in applyComposerStyles.
func TestComposerFillsWidth(t *testing.T) {
	const width = 60
	ic := NewInputComponent(width, nil)

	rendered := ic.View().Content
	firstLine := strings.SplitN(rendered, "\n", 2)[0]

	if got := lipgloss.Width(firstLine); got != width {
		t.Errorf("expected composer to span %d columns, got %d: %q", width, got, firstLine)
	}

	// Every reset of the background must be followed by it being set again
	// before any visible character, otherwise the bar shows gaps.
	if strings.Count(firstLine, "\x1b[49m") > 1 {
		t.Errorf("composer background is torn by inner resets: %q", firstLine)
	}
}

// TestComposerStartsSingleRow verifies the composer costs one text row when
// empty, rather than reserving a fixed multi-line block.
func TestComposerStartsSingleRow(t *testing.T) {
	ic := NewInputComponent(80, nil)

	if got := lipgloss.Height(ic.View().Content); got != 1 {
		t.Errorf("expected an empty composer to occupy 1 line, got %d", got)
	}
}

// TestGutterVocabularyIsConsistent verifies that every attributed block uses
// the same gutter glyph, rather than mixing border weights.
func TestGutterVocabularyIsConsistent(t *testing.T) {
	r := newMessageRenderer(80, false)

	user := r.RenderUserMessage("hello", time.Now())
	if !strings.Contains(user.Content, style.GutterGlyph) {
		t.Errorf("expected user message to use the shared gutter glyph %q, got %q",
			style.GutterGlyph, stripAnsi(user.Content))
	}

	// The retired glyphs must not reappear anywhere in an attributed block.
	for _, old := range []string{"┃", "│"} {
		if strings.Contains(stripAnsi(user.Content), old) {
			t.Errorf("user message still contains retired gutter glyph %q", old)
		}
	}
}

// TestToolHeaderReservesCheckmark verifies routine tool successes use a quiet
// marker, leaving ✓ to mean "the turn finished".
func TestToolHeaderReservesCheckmark(t *testing.T) {
	r := newMessageRenderer(80, false)

	ok := stripAnsi(r.RenderToolMessage("read", `{"path":"a.go"}`, "contents", false).Content)
	if strings.Contains(ok, "✓") {
		t.Errorf("routine tool success should not claim a checkmark, got %q", ok)
	}
	if !strings.Contains(ok, "·") {
		t.Errorf("expected a quiet marker on a routine tool success, got %q", ok)
	}

	failed := stripAnsi(r.RenderToolMessage("read", `{"path":"a.go"}`, "boom", true).Content)
	if !strings.Contains(failed, "×") {
		t.Errorf("expected a failure marker on tool error, got %q", failed)
	}
}

// TestTurnReceipt verifies the receipt appears for substantive turns, is
// suppressed for trivial ones, and reports the outcome.
func TestTurnReceipt(t *testing.T) {
	newModel := func() *AppModel {
		ctrl := &stubAppController{}
		m, _, _ := newTestAppModel(ctrl)
		return sendMsg(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	}

	t.Run("suppressed for trivial turns", func(t *testing.T) {
		m := newModel()
		before := len(m.messages)
		m.turnStartedAt = time.Now()
		m.turnToolCount = 0
		m.printTurnReceipt(turnDone)

		if len(m.messages) != before {
			t.Errorf("expected no receipt for a fast tool-free turn, got %q",
				stripAnsi(m.messages[len(m.messages)-1].Render(100)))
		}
	})

	t.Run("shown after tool use", func(t *testing.T) {
		m := newModel()
		m.turnStartedAt = time.Now().Add(-3 * time.Second)
		m.turnToolCount = 2
		m.printTurnReceipt(turnDone)

		got := stripAnsi(m.messages[len(m.messages)-1].Render(100))
		for _, want := range []string{"✓", "Done", "2 tools", "3s"} {
			if !strings.Contains(got, want) {
				t.Errorf("expected receipt to contain %q, got %q", want, got)
			}
		}
	})

	t.Run("singular tool", func(t *testing.T) {
		m := newModel()
		m.turnStartedAt = time.Now().Add(-time.Second)
		m.turnToolCount = 1
		m.printTurnReceipt(turnDone)

		got := stripAnsi(m.messages[len(m.messages)-1].Render(100))
		if !strings.Contains(got, "1 tool") || strings.Contains(got, "1 tools") {
			t.Errorf("expected singular %q, got %q", "1 tool", got)
		}
	})

	t.Run("interrupted", func(t *testing.T) {
		m := newModel()
		m.turnStartedAt = time.Now().Add(-time.Second)
		m.turnToolCount = 1
		m.printTurnReceipt(turnCancelled)

		got := stripAnsi(m.messages[len(m.messages)-1].Render(100))
		if !strings.Contains(got, "Interrupted") {
			t.Errorf("expected an interrupted receipt, got %q", got)
		}
	})
}

// TestSplashBlockScalesWithContent verifies the splash costs exactly as many
// rows as it has content, rather than a fixed block-art height.
func TestSplashBlockScalesWithContent(t *testing.T) {
	for _, n := range []int{1, 3, 8} {
		lines := make([]string, n)
		for i := range lines {
			lines[i] = "x"
		}
		got := style.SplashBlock(lines)
		if h := lipgloss.Height(got); h != n {
			t.Errorf("SplashBlock with %d lines rendered %d rows", n, h)
		}
	}

	if got := style.SplashBlock(nil); got != "" {
		t.Errorf("expected empty splash for no content, got %q", got)
	}
}

// TestSplashBlockHasNoGutter verifies the splash carries no left stripe. It is
// the application introducing itself, not a message attributed to anyone, so a
// gutter glyph there would imply an authorship the block does not have.
func TestSplashBlockHasNoGutter(t *testing.T) {
	got := stripAnsi(style.SplashBlock([]string{"KIT", "anthropic"}))

	if strings.Contains(got, style.GutterGlyph) {
		t.Errorf("splash must not carry a gutter glyph, got %q", got)
	}
	if strings.Contains(got, "█") {
		t.Errorf("splash must not carry a left stripe, got %q", got)
	}
}

// TestLeftEdgeAlignment verifies every block honours the shared left-edge
// contract: a marker in column 0, and text beginning at style.ContentOffset.
// A ragged left margin reads as a bug even when each block is individually
// well-formed, so this pins all the block types together.
func TestLeftEdgeAlignment(t *testing.T) {
	const width = 80
	r := newMessageRenderer(width, false)

	// firstVisibleLine returns the first non-blank line of a rendered block.
	firstVisibleLine := func(rendered string) []rune {
		for line := range strings.SplitSeq(stripAnsi(rendered), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			return []rune(line)
		}
		return nil
	}

	// contentColumn returns the column at which a block's text begins. For a
	// block carrying a marker the marker sits in column 0 and text follows the
	// separating space; for an unmarked block it is simply the indent.
	contentColumn := func(rendered string) int {
		runes := firstVisibleLine(rendered)
		col := 0
		for col < len(runes) && runes[col] == ' ' {
			col++
		}
		if col == 0 {
			// A marker occupies column 0. Skip it and the space after it.
			for col < len(runes) && runes[col] != ' ' {
				col++
			}
			for col < len(runes) && runes[col] == ' ' {
				col++
			}
		}
		return col
	}

	// startsAtColumnZero reports whether the block's marker sits in column 0.
	startsAtColumnZero := func(rendered string) bool {
		runes := firstVisibleLine(rendered)
		return len(runes) > 0 && runes[0] != ' '
	}

	t.Run("user message marker in column 0", func(t *testing.T) {
		got := r.RenderUserMessage("hello", time.Now()).Content
		if !startsAtColumnZero(got) {
			t.Errorf("user gutter must sit in column 0, got %q", stripAnsi(got))
		}
		if col := contentColumn(got); col != style.ContentOffset {
			t.Errorf("user text starts at column %d, want %d", col, style.ContentOffset)
		}
	})

	t.Run("tool marker in column 0", func(t *testing.T) {
		got := r.RenderToolMessage("read", `{"path":"a.go"}`, "x", false).Content
		if !startsAtColumnZero(got) {
			t.Errorf("tool marker must sit in column 0, got %q", stripAnsi(got))
		}
		if col := contentColumn(got); col != style.ContentOffset {
			t.Errorf("tool name starts at column %d, want %d", col, style.ContentOffset)
		}
	})

	t.Run("assistant prose indented to the content column", func(t *testing.T) {
		got := r.RenderAssistantMessage("plain prose", time.Now(), "m").Content
		if col := contentColumn(got); col != style.ContentOffset {
			t.Errorf("assistant text starts at column %d, want %d", col, style.ContentOffset)
		}
	})

	t.Run("splash indented to the content column", func(t *testing.T) {
		got := style.SplashBlock([]string{"KIT"})
		if col := contentColumn(got); col != style.ContentOffset {
			t.Errorf("splash text starts at column %d, want %d", col, style.ContentOffset)
		}
	})
}

// TestAssistantProseFitsWidth verifies assistant text uses the full width
// available after its indent, leaving a single column of right margin rather
// than the unexplained four it used to.
func TestAssistantProseFitsWidth(t *testing.T) {
	const width = 60
	r := newMessageRenderer(width, false)

	long := strings.Repeat("word ", 80)
	rendered := stripAnsi(r.RenderAssistantMessage(long, time.Now(), "m").Content)

	widest := 0
	for line := range strings.SplitSeq(rendered, "\n") {
		if w := len([]rune(strings.TrimRight(line, " "))); w > widest {
			widest = w
		}
	}
	if widest > width {
		t.Errorf("assistant prose overflows: widest line %d > width %d", widest, width)
	}
	if widest < width-style.ContentOffset-2 {
		t.Errorf("assistant prose wastes horizontal space: widest line %d, width %d", widest, width)
	}
}

// TestComposerGrowsAndShrinks verifies the composer tracks its content height,
// capped at inputMaxRows.
func TestComposerGrowsAndShrinks(t *testing.T) {
	ic := NewInputComponent(80, nil)
	height := func() int { return lipgloss.Height(ic.View().Content) }

	if got := height(); got != 1 {
		t.Fatalf("empty composer should be 1 line, got %d", got)
	}

	ic.textarea.SetValue("a\nb\nc")
	if got := height(); got != 3 {
		t.Errorf("3-line content should render 3 lines, got %d", got)
	}

	// Past the cap the textarea scrolls internally rather than growing.
	ic.textarea.SetValue(strings.Repeat("x\n", inputMaxRows+10))
	if got := height(); got != inputMaxRows {
		t.Errorf("composer should cap at %d lines, got %d", inputMaxRows, got)
	}

	ic.textarea.SetValue("")
	if got := height(); got != 1 {
		t.Errorf("cleared composer should return to 1 line, got %d", got)
	}
}

// TestComposerAcceptsLongInput guards against the composer's height cap being
// mistaken for a content limit. MaxHeight alone makes the textarea refuse
// newlines past that many logical lines, which would silently cap a prompt at
// inputMaxRows and lose the user's text.
func TestComposerAcceptsLongInput(t *testing.T) {
	ic := NewInputComponent(80, nil)

	// Newlines must arrive as real key presses: the content guard lives on the
	// InsertNewline key binding, so InsertString bypasses it entirely and
	// would let this test pass against the very bug it exists to catch.
	newline := tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}

	const lines = inputMaxRows * 4
	for i := range lines {
		ic.textarea.InsertString(fmt.Sprintf("line%d", i))
		updated, _ := ic.Update(newline)
		ic, _ = updated.(*InputComponent)
	}

	value := ic.textarea.Value()
	if got := strings.Count(value, "\n"); got != lines {
		t.Errorf("composer accepted %d newlines, want %d — input is being truncated", got, lines)
	}
	if !strings.Contains(value, fmt.Sprintf("line%d", lines-1)) {
		t.Errorf("composer dropped later input; value ends %q", value[max(0, len(value)-40):])
	}
}

// TestSyncInputHeightMarksLayoutDirty verifies that a composer size change
// schedules a layout recompute. Without this the transcript keeps a stale
// height and the joined view overflows the terminal, clipping the status bar.
func TestSyncInputHeightMarksLayoutDirty(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m = sendMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30})

	ic := NewInputComponent(80, nil)
	m.input = ic
	m.distributeHeight()
	m.layoutDirty = false

	// No change: layout stays clean.
	m.syncInputHeight()
	if m.layoutDirty {
		t.Error("unchanged composer should not dirty the layout")
	}

	// Growing the composer must schedule a recompute.
	ic.textarea.SetValue("a\nb\nc\nd")
	m.syncInputHeight()
	if !m.layoutDirty {
		t.Error("a taller composer must mark the layout dirty, or the transcript overflows")
	}
}
