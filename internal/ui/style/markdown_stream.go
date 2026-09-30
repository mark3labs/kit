package style

import (
	"strings"
)

// StreamingMarkdown renders a markdown document that only ever grows at the
// end (a streaming LLM response) without re-rendering the whole document on
// every update.
//
// ToMarkdown is linear in the document length, and a streaming message is
// re-rendered on every flush, so over a long response the total work is
// quadratic: ~50ms per render at 60KB. StreamingMarkdown splits the document
// at stable block boundaries (see stableMarkdownBoundary), keeps the rendered
// output of everything before the last boundary, and renders only the tail.
//
// The split relies on herald joining top-level blocks with a blank line, so
// Render(content) equals ToMarkdown(content) for the boundaries chosen here.
// The boundary rule is conservative, but markdown has corner cases (e.g. a
// link reference definition in the tail that a link in the prefix uses), so
// callers should do one full ToMarkdown when the document is complete.
//
// The zero value is ready to use. Not safe for concurrent use.
type StreamingMarkdown struct {
	// prefix is the source text rendered into rendered; it always ends at
	// a stable boundary. Empty when nothing is cached.
	prefix   string
	rendered string

	width    int
	themeGen uint64
}

// Render returns the rendered markdown for content at width, reusing the
// cached prefix when content extends the previously rendered content.
func (s *StreamingMarkdown) Render(content string, width int) string {
	if s.width != width || s.themeGen != ThemeGeneration() || !strings.HasPrefix(content, s.prefix) {
		s.Reset()
		s.width = width
		s.themeGen = ThemeGeneration()
	}

	// Advance the cached prefix to the last stable boundary. Scanning resumes
	// at the old boundary: a boundary is always outside a code fence, so the
	// scan state there is known.
	if cut := stableMarkdownBoundary(content, len(s.prefix)); cut > len(s.prefix) {
		segment := ToMarkdown(content[len(s.prefix):cut], width)
		if s.rendered == "" {
			s.rendered = segment
		} else if segment != "" {
			s.rendered += "\n\n" + segment
		}
		s.prefix = content[:cut]
	}

	tail := content[len(s.prefix):]
	if strings.TrimSpace(tail) == "" {
		return s.rendered
	}
	tailRendered := ToMarkdown(tail, width)
	switch {
	case s.rendered == "":
		return tailRendered
	case tailRendered == "":
		return s.rendered
	default:
		return s.rendered + "\n\n" + tailRendered
	}
}

// Reset drops the cached prefix.
func (s *StreamingMarkdown) Reset() {
	*s = StreamingMarkdown{}
}

// stableMarkdownBoundary returns the byte offset of the last stable block
// boundary in content at or after from, or from if there is none. from must
// be 0 or a value previously returned by this function for a prefix of
// content.
//
// A stable boundary is the start of a line that:
//   - follows one or more blank lines,
//   - is outside a fenced code block,
//   - is not indented (so it cannot continue a list item or an indented
//     code block), and
//   - does not start a list item (so a loose list is never split in two).
//
// Everything before such a line is a sequence of complete top-level blocks
// that later text cannot change, and the text from it on renders as blocks
// of its own. The boundary line itself must also be complete (followed by a
// newline) so a marker that is still streaming in is not misread.
func stableMarkdownBoundary(content string, from int) int {
	best := from
	var fenceChar byte // '`' or '~' while inside a fence
	var fenceLen int
	prevBlank := false

	for pos := from; pos < len(content); {
		end := strings.IndexByte(content[pos:], '\n')
		if end < 0 {
			break // incomplete last line: never a boundary, never a fence
		}
		line := content[pos : pos+end]
		lineStart := pos
		pos += end + 1

		if fenceChar != 0 {
			if isFenceClose(line, fenceChar, fenceLen) {
				fenceChar = 0
			}
			prevBlank = false
			continue
		}
		if strings.TrimSpace(line) == "" {
			prevBlank = true
			continue
		}
		if prevBlank && lineStart > 0 && line[0] != ' ' && line[0] != '\t' && !isListItemStart(line) {
			best = lineStart
		}
		prevBlank = false
		if c, n := fenceOpen(line); c != 0 {
			fenceChar, fenceLen = c, n
		}
	}
	return best
}

// fenceOpen reports the fence character and length if line opens a fenced
// code block (up to three spaces of indent, then 3+ backticks or tildes).
func fenceOpen(line string) (byte, int) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 {
		return 0, 0
	}
	c := trimmed[0]
	if c != '`' && c != '~' {
		return 0, 0
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0
	}
	// A backtick fence's info string may not contain backticks.
	if c == '`' && strings.IndexByte(trimmed[n:], '`') >= 0 {
		return 0, 0
	}
	return c, n
}

// isFenceClose reports whether line closes a fence opened with n chars c.
func isFenceClose(line string, c byte, n int) bool {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return false
	}
	m := 0
	for m < len(trimmed) && trimmed[m] == c {
		m++
	}
	return m >= n && strings.TrimSpace(trimmed[m:]) == ""
}

// isListItemStart reports whether line starts with a bullet (-, *, +) or an
// ordered list marker (digits followed by . or )), followed by a space or
// the end of the line.
func isListItemStart(line string) bool {
	if line == "" {
		return false
	}
	markerEnd := 0
	switch line[0] {
	case '-', '*', '+':
		markerEnd = 1
	default:
		for markerEnd < len(line) && markerEnd < 9 && line[markerEnd] >= '0' && line[markerEnd] <= '9' {
			markerEnd++
		}
		if markerEnd == 0 || markerEnd >= len(line) || (line[markerEnd] != '.' && line[markerEnd] != ')') {
			return false
		}
		markerEnd++
	}
	return markerEnd == len(line) || line[markerEnd] == ' ' || line[markerEnd] == '\t'
}
