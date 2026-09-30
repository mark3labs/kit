package style

import (
	"strings"
	"testing"
)

// streamingMarkdownDocs are documents built to stress the block-boundary
// rule: blank lines inside fences, loose lists, list continuations,
// indented code, thematic breaks that look like list items, and CRLF.
var streamingMarkdownDocs = map[string]string{
	"paragraphs and headings":         "# Title\n\nFirst **bold** paragraph.\n\nSecond paragraph\nover two lines.\n\n## Sub\n\nEnd.\n",
	"backtick fence with blank lines": "Before.\n\n```go\nfunc main() {\n\n\tx := 1\n\n}\n```\n\nAfter.\n",
	"tilde fence and longer fence":    "A.\n\n~~~\n```\n\nnot a close\n~~~\n\nB.\n\n````md\n```\n\ninner\n```\n````\n\nC.\n",
	"unclosed fence while streaming":  "Intro.\n\n```python\nprint(1)\n\nprint(2)\n",
	"loose list":                      "Items:\n\n- one\n\n- two\n\n- three\n\nDone.\n",
	"ordered loose list":              "Steps:\n\n1. first\n\n2. second\n\n10) tenth\n\nDone.\n",
	"list continuation":               "- item\n\n  continued paragraph\n\n      indented code in item\n\nOutside.\n",
	"indented code block":             "Text.\n\n    code line 1\n\n    code line 2\n\nText again.\n",
	"thematic breaks":                 "Top.\n\n* * *\n\nMiddle.\n\n---\n\n___\n\nBottom.\n",
	"table and quote":                 "| a | b |\n|---|---|\n| 1 | 2 |\n\n> quoted\n> text\n\n> second quote\n\nTail.\n",
	"leading and repeated blanks":     "\n\n\nStart.\n\n\n\nMiddle.\n\n\nEnd.\n",
	"crlf":                            "Line one.\r\n\r\nLine two.\r\n\r\n- a\r\n- b\r\n\r\nEnd.\r\n",
}

// TestStreamingMarkdown_MatchesFullRenderAtEveryStep feeds each document one
// byte at a time and checks every intermediate render against ToMarkdown.
func TestStreamingMarkdown_MatchesFullRenderAtEveryStep(t *testing.T) {
	for name, doc := range streamingMarkdownDocs {
		t.Run(name, func(t *testing.T) {
			var sm StreamingMarkdown
			for k := 1; k <= len(doc); k++ {
				got := sm.Render(doc[:k], 60)
				if want := ToMarkdown(doc[:k], 60); got != want {
					t.Fatalf("mismatch after %d bytes (cached prefix %q)\n got: %q\nwant: %q",
						k, sm.prefix, got, want)
				}
			}
		})
	}
}

func TestStreamingMarkdown_UsesCachedPrefix(t *testing.T) {
	var sm StreamingMarkdown
	// The boundary line ("Three...") must be complete before it counts, as a
	// partial line like "-" may still turn into a list item.
	doc := "One.\n\nTwo.\n\nThree is done.\nFour is still stream"
	_ = sm.Render(doc, 60)
	if want := "One.\n\nTwo.\n\n"; sm.prefix != want {
		t.Fatalf("prefix = %q, want %q", sm.prefix, want)
	}
}

func TestStreamingMarkdown_ResetsWhenInputsChange(t *testing.T) {
	var sm StreamingMarkdown
	_ = sm.Render("One.\n\nTwo.\n\nThree", 60)

	// Width change: must not reuse output wrapped at the old width.
	long := strings.Repeat("word ", 30) + "\n\nnext\n\ntail"
	if got, want := sm.Render(long, 20), ToMarkdown(long, 20); got != want {
		t.Errorf("after width change:\n got %q\nwant %q", got, want)
	}

	// Content that does not extend the cached prefix (e.g. a reset buffer).
	other := "Different.\n\nText.\n\nhere"
	if got, want := sm.Render(other, 20), ToMarkdown(other, 20); got != want {
		t.Errorf("after content replaced:\n got %q\nwant %q", got, want)
	}
}

func TestStableMarkdownBoundary(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string // the text before the boundary
	}{
		{"no blank line", "one\ntwo\n", ""},
		{"paragraph boundary", "one\n\ntwo\n", "one\n\n"},
		{"boundary line must be complete", "one\n\ntw", ""},
		{"inside a fence", "a\n\n```\nx\n\ny\n", "a\n\n"},
		{"after a closed fence", "```\nx\n\n```\n\nafter\n", "```\nx\n\n```\n\n"},
		{"list item is not a boundary", "a\n\n- b\n", ""},
		{"ordered item is not a boundary", "a\n\n12. b\n", ""},
		{"indented line is not a boundary", "a\n\n    code\n", ""},
		{"bold is not a list item", "a\n\n**b**\n", "a\n\n"},
		{"rule is a boundary", "a\n\n---\n", "a\n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.content[:stableMarkdownBoundary(tt.content, 0)]; got != tt.want {
				t.Errorf("prefix = %q, want %q", got, tt.want)
			}
		})
	}
}
