// Package emoji provides the shortcode table and search used by the
// :shortcode autocomplete popup in the input editor.
package emoji

//go:generate go run gen/main.go

import (
	"sort"
	"strings"
)

// Emoji is one entry in the shortcode table.
type Emoji struct {
	Glyph string // rendered emoji, for example "🇹🇩"
	Name  string // shortcode without colons, for example "flag_chad"
	Group string // emoji-api.com group, for example "flags"
}

// All returns every known emoji. The caller must not modify the slice.
func All() []Emoji { return all }

// Match is a search result with its rank score.
type Match struct {
	Emoji
	Score int
}

// Search returns up to limit emojis whose shortcode matches query, best
// matches first. Order of rank: exact name, name prefix, word prefix (after
// "_"), substring, then characters in order. Ties go to the shorter name.
func Search(query string, limit int) []Match {
	query = strings.ToLower(query)
	if query == "" || limit <= 0 {
		return nil
	}

	var matches []Match
	for _, e := range all {
		if score := score(query, e.Name); score > 0 {
			matches = append(matches, Match{Emoji: e, Score: score})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Name < matches[j].Name
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

// score ranks name against query. Zero means no match. Inside each tier a
// shorter name gets a higher score.
func score(query, name string) int {
	lengthPenalty := min(len(name), 99)
	switch {
	case name == query:
		return 1000
	case strings.HasPrefix(name, query):
		return 800 - lengthPenalty
	case strings.Contains(name, "_"+query):
		return 600 - lengthPenalty
	case strings.Contains(name, query):
		return 400 - lengthPenalty
	case inOrder(query, name):
		return 200 - lengthPenalty
	}
	return 0
}

// inOrder reports whether all bytes of query appear in name in order.
func inOrder(query, name string) bool {
	qi := 0
	for i := 0; i < len(name) && qi < len(query); i++ {
		if name[i] == query[qi] {
			qi++
		}
	}
	return qi == len(query)
}
