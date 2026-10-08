// Package slug converts human text into URL-safe slugs.
//
// It is self-contained: only the standard library is used, and it has no
// dependency on other kit packages.
package slug

import (
	"strings"
	"unicode"
)

// Make returns a slug for text with '-' as the separator. Runs of letters
// and digits are kept (lowercased); every other run becomes one separator.
// Leading and trailing separators are removed. An empty result is possible
// when text has no letters or digits.
func Make(text string) string {
	return MakeWith(text, '-')
}

// MakeWith returns a slug for text with sep as the separator. See Make for
// the rules. An empty result is possible when text has no letters or digits.
func MakeWith(text string, sep rune) string {
	var sb strings.Builder
	sb.Grow(len(text))

	lastWasWord := false
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(unicode.ToLower(r))
			lastWasWord = true
			continue
		}
		if lastWasWord {
			sb.WriteRune(sep)
			lastWasWord = false
		}
	}

	// Drop a separator left over at the end, if any.
	s := sb.String()
	return strings.Trim(s, string(sep))
}

// Is reports whether text is already a valid slug with sep as the separator.
// A valid slug is not empty, holds only lowercase letters, digits, and
// separators, has no separator at the start or end, and has no run of two
// or more separators.
func Is(text string, sep rune) bool {
	if text == "" {
		return false
	}

	prevWasSep := false
	for i, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if r != unicode.ToLower(r) {
				return false
			}
			prevWasSep = false
		case r == sep:
			if i == 0 || prevWasSep {
				return false
			}
			prevWasSep = true
		default:
			return false
		}
	}

	// The loop covers the head; this covers the tail.
	return !prevWasSep
}

// Truncate returns slug cut to at most max runes. It cuts on the last
// separator inside the first max runes, so full words stay whole and the
// result has no trailing separator. With no separator inside the first max
// runes, it returns the first max runes. A non-positive max gives "".
func Truncate(slug string, max int, sep rune) string {
	if max <= 0 {
		return ""
	}

	count := 0
	cut := -1     // byte index of the last separator inside the first max runes
	over := false // slug holds more than max runes
	for i, r := range slug {
		if count == max {
			over = true
			if cut < 0 {
				cut = i
			}
			break
		}
		count++
		if r == sep {
			cut = i
		}
	}
	if !over {
		// slug already fits within max runes.
		return slug
	}
	return slug[:cut]
}
