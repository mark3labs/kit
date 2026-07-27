package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/app"
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
