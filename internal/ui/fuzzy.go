package ui

import (
	"strings"

	"github.com/mark3labs/kit/internal/ui/commands"
)

// FuzzyMatch represents the result of a fuzzy string matching operation,
// containing the matched command and its relevance score. Higher scores
// indicate better matches.
type FuzzyMatch struct {
	Command *commands.SlashCommand
	Score   int
}

// FuzzyMatchCommands performs fuzzy string matching on the provided slash commands
// based on the query string. Returns a slice of matches sorted by relevance score
// in descending order. An empty query returns all commands with zero scores.
func FuzzyMatchCommands(query string, commands []commands.SlashCommand) []FuzzyMatch {
	if query == "" || query == "/" {
		// Return all commands when query is empty or just "/"
		matches := make([]FuzzyMatch, len(commands))
		for i := range commands {
			matches[i] = FuzzyMatch{
				Command: &commands[i],
				Score:   0,
			}
		}
		return matches
	}

	// Normalize query
	query = strings.ToLower(strings.TrimPrefix(query, "/"))

	var matches []FuzzyMatch

	for i := range commands {
		cmd := &commands[i]
		score := fuzzyScore(query, cmd)
		if score > 0 {
			matches = append(matches, FuzzyMatch{
				Command: cmd,
				Score:   score,
			})
		}
	}

	// Sort by score (highest first)
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches
}

// fuzzyScore calculates the fuzzy match score for a command
func fuzzyScore(query string, cmd *commands.SlashCommand) int {
	// Check exact match first
	cmdName := strings.ToLower(strings.TrimPrefix(cmd.Name, "/"))
	if cmdName == query {
		return 1000
	}

	// Check aliases for exact match
	for _, alias := range cmd.Aliases {
		aliasName := strings.ToLower(strings.TrimPrefix(alias, "/"))
		if aliasName == query {
			return 900
		}
	}

	// Check if command starts with query
	if strings.HasPrefix(cmdName, query) {
		return 800 - len(cmdName) + len(query)
	}

	// Check if any alias starts with query
	for _, alias := range cmd.Aliases {
		aliasName := strings.ToLower(strings.TrimPrefix(alias, "/"))
		if strings.HasPrefix(aliasName, query) {
			return 700 - len(aliasName) + len(query)
		}
	}

	// Check if command contains query
	if strings.Contains(cmdName, query) {
		return 500
	}

	// Check if description contains query
	if strings.Contains(strings.ToLower(cmd.Description), query) {
		return 300
	}

	// Fuzzy character matching
	score := fuzzyCharacterMatch(query, cmdName)
	if score > 0 {
		return score
	}

	// Try fuzzy matching on aliases
	for _, alias := range cmd.Aliases {
		aliasName := strings.ToLower(strings.TrimPrefix(alias, "/"))
		score = fuzzyCharacterMatch(query, aliasName)
		if score > 0 {
			return score - 50 // Slightly lower score for alias matches
		}
	}

	return 0
}

// fuzzyCharacterMatch performs character-by-character fuzzy matching using
// rune-safe iteration so multi-byte Unicode characters are handled correctly.
// Returns a positive score if all query runes appear in order within target.
func fuzzyCharacterMatch(query, target string) int {
	qRunes := []rune(query)
	tRunes := []rune(target)
	if len(qRunes) > len(tRunes) {
		return 0
	}

	qi := 0
	score := 100
	consecutiveMatches := 0

	for ti := 0; ti < len(tRunes) && qi < len(qRunes); ti++ {
		if tRunes[ti] == qRunes[qi] {
			qi++
			consecutiveMatches++
			score += consecutiveMatches * 10
		} else {
			consecutiveMatches = 0
			score -= 5
		}
	}

	// Must match all characters in query
	if qi < len(qRunes) {
		return 0
	}

	return score
}
