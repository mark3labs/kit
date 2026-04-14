// Package fences provides utilities for detecting markdown code regions
// (fenced code blocks and inline code spans) and applying transformations
// only to text outside those regions.
//
// This prevents special tokens like $1, $@, or @file from being interpreted
// when they appear inside ``` fences, ~~~ fences, or `inline` code spans.
package fences

import "strings"

// Ranges returns byte ranges [start, end) of fenced code blocks in content.
// Recognises both backtick (```) and tilde (~~~) fences, with optional
// leading indentation (up to 3 spaces) and optional info strings.
// An unclosed fence extends to the end of content.
func Ranges(content string) [][2]int {
	var result [][2]int
	var inFence bool
	var fenceChar byte
	var fenceCount int
	var fenceStart int

	pos := 0
	for pos < len(content) {
		// Find the end of the current line.
		lineEnd := strings.IndexByte(content[pos:], '\n')
		var line string
		var nextPos int
		if lineEnd < 0 {
			line = content[pos:]
			nextPos = len(content)
		} else {
			line = content[pos : pos+lineEnd]
			nextPos = pos + lineEnd + 1
		}

		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)

		if !inFence {
			if indent <= 3 {
				if ch, n := parseFenceOpen(trimmed); n > 0 {
					inFence = true
					fenceChar = ch
					fenceCount = n
					fenceStart = pos
				}
			}
		} else {
			if indent <= 3 && isFenceClose(trimmed, fenceChar, fenceCount) {
				result = append(result, [2]int{fenceStart, nextPos})
				inFence = false
			}
		}

		pos = nextPos
	}

	// Unclosed fence extends to end of content.
	if inFence {
		result = append(result, [2]int{fenceStart, len(content)})
	}

	return result
}

// ReplaceOutside applies fn to each text segment that is outside fenced code
// blocks and inline code spans, leaving code content unchanged. This is the
// primary entry point for callers that need to do regex replacement only on
// non-code text.
func ReplaceOutside(content string, fn func(string) string) string {
	ranges := Ranges(content)
	if len(ranges) == 0 {
		return replaceOutsideInline(content, fn)
	}

	var b strings.Builder
	b.Grow(len(content))
	pos := 0
	for _, r := range ranges {
		if pos < r[0] {
			// Within non-fenced segments, also skip inline code spans.
			b.WriteString(replaceOutsideInline(content[pos:r[0]], fn))
		}
		// Preserve fenced content verbatim.
		b.WriteString(content[r[0]:r[1]])
		pos = r[1]
	}
	if pos < len(content) {
		b.WriteString(replaceOutsideInline(content[pos:], fn))
	}
	return b.String()
}

// StripCode returns content with fenced code blocks and inline code spans
// removed. Useful for detection/matching where only non-code text matters.
func StripCode(content string) string {
	// First strip fenced blocks.
	stripped := StripFenced(content)
	// Then strip inline code spans from what remains.
	return stripInlineCode(stripped)
}

// StripFenced returns content with fenced code block regions removed.
// Useful for detection/matching where only non-fenced text matters.
// NOTE: this does NOT strip inline code spans; use StripCode for both.
func StripFenced(content string) string {
	ranges := Ranges(content)
	if len(ranges) == 0 {
		return content
	}

	var b strings.Builder
	b.Grow(len(content))
	pos := 0
	for _, r := range ranges {
		b.WriteString(content[pos:r[0]])
		pos = r[1]
	}
	b.WriteString(content[pos:])
	return b.String()
}

// parseFenceOpen checks whether trimmed (leading spaces already removed)
// starts a fenced code block. Returns the fence character and count, or
// (0, 0) if it is not a fence opener.
func parseFenceOpen(trimmed string) (byte, int) {
	if len(trimmed) == 0 {
		return 0, 0
	}
	ch := trimmed[0]
	if ch != '`' && ch != '~' {
		return 0, 0
	}
	count := 0
	for count < len(trimmed) && trimmed[count] == ch {
		count++
	}
	if count < 3 {
		return 0, 0
	}
	// Per CommonMark: backtick fences cannot have backticks in the info string.
	if ch == '`' && strings.ContainsRune(trimmed[count:], '`') {
		return 0, 0
	}
	return ch, count
}

// isFenceClose checks whether trimmed is a closing fence matching fenceChar
// with at least minCount characters. A closing fence line contains only the
// fence characters and optional trailing spaces.
func isFenceClose(trimmed string, fenceChar byte, minCount int) bool {
	if len(trimmed) == 0 || trimmed[0] != fenceChar {
		return false
	}
	count := 0
	for count < len(trimmed) && trimmed[count] == fenceChar {
		count++
	}
	if count < minCount {
		return false
	}
	// Closing fence must contain only fence chars (and optional trailing spaces).
	return strings.TrimRight(trimmed[count:], " ") == ""
}

// --------------------------------------------------------------------------
// Inline code span handling
// --------------------------------------------------------------------------

// inlineCodeRanges returns byte ranges [start, end) of inline code spans
// in segment. Per CommonMark, a code span opens with N backticks and closes
// with exactly N backticks.
func inlineCodeRanges(s string) [][2]int {
	var result [][2]int
	i := 0
	for i < len(s) {
		if s[i] != '`' {
			i++
			continue
		}
		// Count opening backticks.
		start := i
		n := 0
		for i < len(s) && s[i] == '`' {
			n++
			i++
		}
		// Scan for a closing run of exactly n backticks.
		for j := i; j < len(s); {
			if s[j] != '`' {
				j++
				continue
			}
			m := 0
			for j < len(s) && s[j] == '`' {
				m++
				j++
			}
			if m == n {
				result = append(result, [2]int{start, j})
				i = j
				break
			}
		}
		// If no closing run was found, i is already past the opening
		// backticks so the outer loop advances naturally.
	}
	return result
}

// replaceOutsideInline applies fn only to text outside inline code spans.
func replaceOutsideInline(segment string, fn func(string) string) string {
	ranges := inlineCodeRanges(segment)
	if len(ranges) == 0 {
		return fn(segment)
	}
	var b strings.Builder
	b.Grow(len(segment))
	pos := 0
	for _, r := range ranges {
		if pos < r[0] {
			b.WriteString(fn(segment[pos:r[0]]))
		}
		b.WriteString(segment[r[0]:r[1]])
		pos = r[1]
	}
	if pos < len(segment) {
		b.WriteString(fn(segment[pos:]))
	}
	return b.String()
}

// stripInlineCode removes inline code spans from s.
func stripInlineCode(s string) string {
	ranges := inlineCodeRanges(s)
	if len(ranges) == 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	pos := 0
	for _, r := range ranges {
		b.WriteString(s[pos:r[0]])
		pos = r[1]
	}
	b.WriteString(s[pos:])
	return b.String()
}
