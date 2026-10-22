package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/emoji"
)

const (
	// maxEmojiSuggestions caps the :shortcode popup.
	maxEmojiSuggestions = 8

	// minEmojiPrefix is the number of characters after ":" needed before the
	// popup opens. Two stops it on emoticons such as ":)" and ":D".
	minEmojiPrefix = 2

	// emojiGlyphCols is the column width given to the glyph in each popup row
	// so the shortcodes line up.
	emojiGlyphCols = 3
)

// ExtractColonPrefix detects a :shortcode being typed at the end of line.
//
// Returns:
//   - ok: true when the last word starts with ":" and the rest is a valid
//     shortcode prefix of at least minEmojiPrefix characters
//   - prefix: lowercase text after the ":"
//   - startIdx: byte offset of the ":" in the line
//
// The ":" must be at the start of the line or after whitespace, so URLs
// ("http://"), times ("12:30") and "key:value" text do not trigger. A closed
// shortcode (":smile:") does not trigger either.
func ExtractColonPrefix(line string) (ok bool, prefix string, startIdx int) {
	idx := strings.LastIndexAny(line, " \t:")
	if idx < 0 || line[idx] != ':' {
		return false, "", 0
	}
	if idx > 0 && line[idx-1] != ' ' && line[idx-1] != '\t' {
		return false, "", 0
	}

	raw := line[idx+1:]
	if len(raw) < minEmojiPrefix {
		return false, "", 0
	}
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		valid := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '_' || c == '-' || c == '+'
		if !valid {
			return false, "", 0
		}
	}
	return true, strings.ToLower(raw), idx
}

// GetEmojiSuggestions returns the best emoji matches for a shortcode prefix.
// The prefix may use "-" in place of "_".
func GetEmojiSuggestions(prefix string) []emoji.Match {
	return emoji.Search(strings.ReplaceAll(prefix, "-", "_"), maxEmojiSuggestions)
}

// emojiPopupLabel builds the popup row text: the glyph padded to a fixed
// column width, then the :shortcode:. Emoji display width is not the rune
// count (flags are two runes, ZWJ sequences are many), so pad by cell width.
func emojiPopupLabel(m emoji.Match) string {
	pad := max(emojiGlyphCols-ansi.StringWidth(m.Glyph), 1)
	return m.Glyph + strings.Repeat(" ", pad) + ":" + m.Name + ":"
}
