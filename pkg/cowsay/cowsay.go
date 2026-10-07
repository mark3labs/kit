// Package cowsay renders a short message inside an ASCII speech bubble,
// spoken by a cow. It is a tiny, dependency-free nod to the classic
// cowsay(6) program. Kit does not need a cow, but every good terminal
// toolbox deserves one.
package cowsay

import (
	"strings"
	"unicode/utf8"
)

// maxLineWidth caps the text width inside the speech bubble, in runes.
const maxLineWidth = 36

// cow is the animal that speaks the message.
const cow = `        \   ^__^
         \  (oo)\_______
            (__)\       )\/\
                ||----w |
                ||     ||`

// Say renders message inside an ASCII speech bubble, spoken by a cow.
// Text wraps at spaces so no line inside the bubble exceeds
// maxLineWidth runes; a single long word breaks across lines. An empty
// message is fine: the cow then says nothing, silently.
func Say(message string) string {
	lines := wrap(message, maxLineWidth)
	width := 0
	for _, line := range lines {
		if n := utf8.RuneCountInString(line); n > width {
			width = n
		}
	}

	var b strings.Builder
	writeBubble(&b, lines, width)
	b.WriteString(cow)
	b.WriteByte('\n')
	return b.String()
}

// writeBubble writes the speech bubble for lines of text, padded to
// width runes. The top border uses underscores, the bottom border uses
// hyphens, and the side marks follow cowsay rules; see bubbleSides.
func writeBubble(b *strings.Builder, lines []string, width int) {
	writeBorder(b, '_', width)
	for i, line := range lines {
		open, close := bubbleSides(i, len(lines))
		b.WriteString(open)
		b.WriteByte(' ')
		b.WriteString(line)
		b.WriteString(strings.Repeat(" ", width-utf8.RuneCountInString(line)))
		b.WriteByte(' ')
		b.WriteString(close)
		b.WriteByte('\n')
	}
	writeBorder(b, '-', width)
}

// bubbleSides returns the open and close marks for line i of n. A
// single line is "< ... >"; otherwise the first line is "/ ... \", the
// last is "\ ... /", and middle lines are "| ... |".
func bubbleSides(i, n int) (open, close string) {
	switch {
	case n == 1:
		return "<", ">"
	case i == 0:
		return "/", "\\"
	case i == n-1:
		return "\\", "/"
	default:
		return "|", "|"
	}
}

// writeBorder writes one border line: one leading space, then ruler
// repeated (width+2) times, then a newline. The leading space keeps the
// border one column right of the bubble's side marks, as in cowsay.
func writeBorder(b *strings.Builder, ruler byte, width int) {
	b.WriteByte(' ')
	b.WriteString(strings.Repeat(string(ruler), width+2))
	b.WriteByte('\n')
}

// wrap splits message into bubble lines of at most width runes.
// Explicit newlines in message start a new bubble line. Within a
// segment, text breaks at spaces when it can, and a word that is longer
// than width breaks across lines. An empty message yields one empty
// line.
func wrap(message string, width int) []string {
	if message == "" {
		return []string{""}
	}

	var lines []string
	for segment := range strings.SplitSeq(message, "\n") {
		lines = append(lines, wrapLine(segment, width)...)
	}
	return lines
}

// wrapLine wraps one segment of text, without newlines, into bubble
// lines of at most width runes.
func wrapLine(segment string, width int) []string {
	var lines []string
	current := ""
	for word := range strings.FieldsSeq(segment) {
		rest := word
		if current != "" {
			if utf8.RuneCountInString(current)+1+utf8.RuneCountInString(rest) <= width {
				current += " " + rest
				continue
			}
			lines = append(lines, current)
		}
		// Hard-break words that cannot fit on a line of their own.
		for utf8.RuneCountInString(rest) > width {
			chunk := firstRunes(rest, width)
			lines = append(lines, chunk)
			rest = rest[len(chunk):]
		}
		current = rest
	}
	return append(lines, current)
}

// firstRunes returns the first n runes of s.
func firstRunes(s string, n int) string {
	i := 0
	for range n {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return s[:i]
}
