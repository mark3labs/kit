package ui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/ui/style"
)

// The transcript is a single column of text with markers hanging in the left
// margin. That reads as structure only if every block agrees on where the
// column is, how wide it is, and how much air follows it — a block that
// invents its own geometry does not look like a variation, it looks like a
// bug. These tests pin the three properties that make the column hold, across
// every block type the transcript can contain.
//
// TestLeftEdgeAlignment (activity_test.go) covered four block types; the
// overflow bugs this file caught were all in the other thirteen.

var blockAnsiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripBlockAnsi removes ANSI escape sequences so geometry can be measured in
// terms of visible columns.
func stripBlockAnsi(s string) string {
	return blockAnsiPattern.ReplaceAllString(s, "")
}

// visibleLines returns the block's lines with styling removed.
func visibleLines(rendered string) []string {
	return strings.Split(stripBlockAnsi(rendered), "\n")
}

// widestColumn returns the number of columns occupied by the block's longest
// line, ignoring trailing whitespace (which lipgloss emits to pad a styled
// region and which costs nothing visually).
func widestColumn(rendered string) int {
	widest := 0
	for _, line := range visibleLines(rendered) {
		if w := len([]rune(strings.TrimRight(line, " "))); w > widest {
			widest = w
		}
	}
	return widest
}

// trailingBlankLines counts the whitespace-only lines at the end of a block.
// This is the gap the next block will sit above.
func trailingBlankLines(rendered string) int {
	lines := visibleLines(rendered)
	n := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			break
		}
		n++
	}
	return n
}

// textColumn returns the column at which a block's text begins, given whether
// the block carries a marker in column 0.
//
// Whether a marker is present cannot be inferred from the rendering: an
// unmarked block whose text happens to start at column 0 is indistinguishable
// from a marked one. (An earlier version of this helper guessed, and reported
// unindented reasoning text as "column 6" because it mistook the word "alpha"
// for a marker.) Each case therefore declares what it draws.
func textColumn(rendered string, hasMarker bool) int {
	var runes []rune
	for _, line := range visibleLines(rendered) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		runes = []rune(line)
		break
	}

	col := 0
	if hasMarker {
		// The marker occupies column 0. Skip it and the space after it.
		for col < len(runes) && runes[col] != ' ' {
			col++
		}
	}
	for col < len(runes) && runes[col] == ' ' {
		col++
	}
	return col
}

// blockCase is one renderer under test, described by what it produces rather
// than by how it is built, so the table survives the renderers being rewritten.
type blockCase struct {
	name string
	// render produces the block at the given terminal width.
	render func(r *MessageRenderer, width int) string
	// marker reports whether the block draws a glyph in column 0 (a gutter
	// bar, a tool bullet). Unmarked blocks indent their text to ContentOffset
	// instead. This cannot be detected from the output, so it is declared.
	marker bool
	// skipGap marks blocks that are composed into a larger block by their
	// caller and so do not carry their own trailing gap.
	skipGap bool
}

// longProse is wide enough to force wrapping at any terminal width under test.
var longProse = strings.Repeat("alpha bravo charlie delta echo foxtrot ", 30)

// longUnbrokenLine has no spaces, so a renderer that relies on word wrapping
// alone will overflow on it. This is what caught the Read tool.
var longUnbrokenLine = strings.Repeat("x", 300)

func blockCases() []blockCase {
	now := time.Now()
	return []blockCase{
		{
			name:   "user",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderUserMessage(longProse, now).Content
			},
		},
		{
			name:   "user/unbroken",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderUserMessage(longUnbrokenLine, now).Content
			},
		},
		{
			name: "assistant",
			render: func(r *MessageRenderer, w int) string {
				return r.RenderAssistantMessage(longProse, now, "model").Content
			},
		},
		{
			name: "assistant/code",
			render: func(r *MessageRenderer, w int) string {
				md := "text before\n\n```go\n" + longUnbrokenLine + "\n```\n\ntext after"
				return r.RenderAssistantMessage(md, now, "model").Content
			},
		},
		{
			name: "reasoning",
			render: func(r *MessageRenderer, w int) string {
				return r.RenderReasoningBlock(longProse, now).Content
			},
		},
		{
			name:   "system",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderSystemMessage(longProse, now).Content
			},
		},
		{
			name:   "system/unbroken",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderSystemMessage(longUnbrokenLine, now).Content
			},
		},
		{
			name:   "error",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderErrorMessage(longProse, now).Content
			},
		},
		{
			name:   "custom",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderCustomMessage(longProse, "Help", now).Content
			},
		},
		{
			name:   "tool/read",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				body := "1: package main\n2: " + longUnbrokenLine + "\n3: func main() {}\n"
				return r.RenderToolMessage("read", `{"path":"main.go"}`, body, false).Content
			},
		},
		{
			name:   "tool/bash",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				out := "<stdout>\n" + longUnbrokenLine + "\n" + longProse + "\n</stdout>"
				return r.RenderToolMessage("bash", `{"command":"ls -la"}`, out, false).Content
			},
		},
		{
			name:   "tool/grep",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderToolMessage("grep", `{"pattern":"func"}`, longUnbrokenLine+"\n"+longProse, false).Content
			},
		},
		{
			name:   "tool/ls",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderToolMessage("ls", `{"path":"/tmp"}`, longUnbrokenLine, false).Content
			},
		},
		{
			name:   "tool/write",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				args := `{"path":"a.go","content":"package main\n` + strings.Repeat("z", 200) + `"}`
				return r.RenderToolMessage("write", args, "written", false).Content
			},
		},
		{
			// The diff renderer was the one tool body this table originally
			// missed, and it overflowed the terminal below ~50 columns.
			name:   "tool/edit",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				args := `{"path":"a.go","edits":[{"old_text":"a somewhat longer original line here",` +
					`"new_text":"a somewhat longer replacement line here"}]}`
				return r.RenderToolMessage("edit", args, "edited", false).Content
			},
		},
		{
			name:   "tool/edit-multiline",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				old := strings.Repeat("an original line of some length\n", 12)
				new := strings.Repeat("a replacement line of some length\n", 12)
				args := `{"path":"a.go","edits":[{"old_text":"` +
					strings.ReplaceAll(old, "\n", "\\n") + `","new_text":"` +
					strings.ReplaceAll(new, "\n", "\\n") + `"}]}`
				return r.RenderToolMessage("edit", args, "edited", false).Content
			},
		},
		{
			name:   "tool/error",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderToolMessage("bash", `{"command":"ls"}`, longProse, true).Content
			},
		},
		{
			name:   "tool/error-unbroken",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderToolMessage("bash", `{"command":"ls"}`, longUnbrokenLine, true).Content
			},
		},
		{
			name: "splash",
			render: func(r *MessageRenderer, w int) string {
				return style.SplashBlock([]string{"KIT", "anthropic · claude"})
			},
		},
		{
			name:   "extension",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderExtensionBlock(longProse, "#00FF00", "a subtitle").Content
			},
		},
		{
			name:   "extension/unbroken",
			marker: true,
			render: func(r *MessageRenderer, w int) string {
				return r.RenderExtensionBlock(longUnbrokenLine, "", "").Content
			},
		},
		{
			name: "streaming/assistant",
			render: func(r *MessageRenderer, w int) string {
				item := NewStreamingMessageItem("s1", "assistant", "model")
				item.AppendChunk(longProse)
				return item.Render(w)
			},
		},
		{
			name: "streaming/reasoning",
			render: func(r *MessageRenderer, w int) string {
				item := NewStreamingMessageItem("s2", "reasoning", "model")
				item.AppendChunk(longProse)
				item.MarkComplete()
				return item.Render(w)
			},
		},
		{
			name:   "streaming/bash",
			marker: true,
			// Live output is superseded by the finished tool block, so it does
			// not append a gap of its own mid-turn.
			skipGap: true,
			render: func(r *MessageRenderer, w int) string {
				item := NewStreamingBashOutputItem("s3", "ls -la")
				item.AppendStdout(longUnbrokenLine)
				item.AppendStderr(longProse)
				return item.Render(w)
			},
		},
		{
			name:   "fallback/user",
			marker: true,
			// The fallback renders bare content; the gap belongs to the block
			// renderer that would normally have produced it.
			skipGap: true,
			render: func(r *MessageRenderer, w int) string {
				return NewStyledMessageItem("f1", "user", longUnbrokenLine, "").Render(w)
			},
		},
		{
			name:    "fallback/assistant",
			skipGap: true,
			render: func(r *MessageRenderer, w int) string {
				return NewStyledMessageItem("f2", "assistant", longProse, "").Render(w)
			},
		},
	}
}

// widths spans a comfortable terminal, a narrow one, and the awkward middle.
// Geometry bugs hide at the edges: a renderer that subtracts a constant from
// the width works fine at 120 and produces nonsense at 40.
var widths = []int{40, 60, 80, 120}

// TestBlockFitsWidth is the assertion that matters most. A block wider than
// the terminal wraps in the emulator instead of in lipgloss, which corrupts
// the scroll list's height accounting and makes the transcript scroll by the
// wrong amount.
func TestBlockFitsWidth(t *testing.T) {
	for _, w := range widths {
		for _, tc := range blockCases() {
			t.Run(tc.name+"/w"+itoa(w), func(t *testing.T) {
				r := newMessageRenderer(w, false)
				got := tc.render(r, w)
				if widest := widestColumn(got); widest > w {
					t.Errorf("block overflows terminal: widest line %d > width %d\nfirst offending line:\n%s",
						widest, w, firstOverflowingLine(got, w))
				}
			})
		}
	}
}

// TestBlockTextColumn pins the left edge: a marker in column 0, text at
// ContentOffset. A block that starts anywhere else makes the whole column
// look ragged even when it is internally consistent.
func TestBlockTextColumn(t *testing.T) {
	const w = 80
	for _, tc := range blockCases() {
		t.Run(tc.name, func(t *testing.T) {
			r := newMessageRenderer(w, false)
			got := tc.render(r, w)
			if got == "" {
				t.Skip("block renders empty")
			}
			if tc.marker && strings.HasPrefix(firstVisibleBlockLine(got), " ") {
				t.Errorf("marker must sit in column 0\nfirst line: %q", firstVisibleBlockLine(got))
			}
			if col := textColumn(got, tc.marker); col != style.ContentOffset {
				t.Errorf("text starts at column %d, want %d\nfirst line: %q",
					col, style.ContentOffset, firstVisibleBlockLine(got))
			}
		})
	}
}

// TestBlockTrailingGap pins vertical rhythm. The scroll list inserts nothing
// between items, so a block that forgets its gap collides with the next one
// and a block that adds two opens a hole.
func TestBlockTrailingGap(t *testing.T) {
	const w = 80
	for _, tc := range blockCases() {
		if tc.skipGap {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			r := newMessageRenderer(w, false)
			got := tc.render(r, w)
			if got == "" {
				t.Skip("block renders empty")
			}
			if gap := trailingBlankLines(got); gap != style.BlockGap {
				t.Errorf("block ends with %d blank lines, want %d", gap, style.BlockGap)
			}
		})
	}
}

// firstOverflowingLine returns the first line exceeding the given width, for
// failure messages.
func firstOverflowingLine(rendered string, width int) string {
	for _, line := range visibleLines(rendered) {
		trimmed := strings.TrimRight(line, " ")
		if len([]rune(trimmed)) > width {
			return trimmed
		}
	}
	return ""
}

func firstVisibleBlockLine(rendered string) string {
	for _, line := range visibleLines(rendered) {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

// itoa avoids pulling strconv in for test names alone.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
