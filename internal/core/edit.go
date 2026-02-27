package core

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode"

	"charm.land/fantasy"
)

type editArgs struct {
	Path    string `json:"path"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

// NewEditTool creates the edit core tool.
func NewEditTool(opts ...ToolOption) fantasy.AgentTool {
	cfg := ApplyOptions(opts)
	return &coreTool{
		info: fantasy.ToolInfo{
			Name:        "edit",
			Description: "Edit a file by replacing exact text. The old_text must match exactly (including whitespace). Use this for precise, surgical edits. Fails if old_text is not found or matches multiple locations.",
			Parameters: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to edit (relative or absolute)",
				},
				"old_text": map[string]any{
					"type":        "string",
					"description": "Exact text to find and replace (must match exactly)",
				},
				"new_text": map[string]any{
					"type":        "string",
					"description": "New text to replace the old text with",
				},
			},
			Required: []string{"path", "old_text", "new_text"},
		},
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return executeEdit(ctx, call, cfg.WorkDir)
		},
	}
}

func executeEdit(ctx context.Context, call fantasy.ToolCall, workDir string) (fantasy.ToolResponse, error) {
	var args editArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse("path, old_text, and new_text parameters are required"), nil
	}
	if args.Path == "" {
		return fantasy.NewTextErrorResponse("path parameter is required"), nil
	}

	absPath, err := resolvePathWithWorkDir(args.Path, workDir)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid path: %v", err)), nil
	}

	contentBytes, err := os.ReadFile(absPath)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to read file: %v", err)), nil
	}

	content := string(contentBytes)

	// Normalize line endings for matching
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalizedOld := strings.ReplaceAll(args.OldText, "\r\n", "\n")

	// Try exact match first
	count := strings.Count(normalized, normalizedOld)

	// If no exact match, try fuzzy matching
	if count == 0 {
		if idx, matchLen := fuzzyMatch(normalized, normalizedOld); idx >= 0 {
			// Apply fuzzy match
			newContent := normalized[:idx] + args.NewText + normalized[idx+matchLen:]
			if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to write file: %v", err)), nil
			}
			diff := generateDiff(absPath, normalized, newContent, idx)
			return fantasy.NewTextResponse(fmt.Sprintf("Applied edit (fuzzy match) to %s\n%s", args.Path, diff)), nil
		}
		return fantasy.NewTextErrorResponse(fmt.Sprintf("old_text not found in %s", args.Path)), nil
	}

	if count > 1 {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("found %d matches for old_text in %s. Provide more context to identify the correct match.", count, args.Path)), nil
	}

	// Apply the edit
	newContent := strings.Replace(normalized, normalizedOld, args.NewText, 1)

	if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to write file: %v", err)), nil
	}

	idx := strings.Index(normalized, normalizedOld)
	diff := generateDiff(absPath, normalized, newContent, idx)
	return fantasy.NewTextResponse(fmt.Sprintf("Applied edit to %s\n%s", args.Path, diff)), nil
}

// fuzzyMatch tries to find old_text with relaxed matching:
// - Strips trailing whitespace per line
// - Normalizes unicode quotes to ASCII
// - Normalizes unicode dashes/spaces
// Returns (index, matchLength) or (-1, 0) if not found.
func fuzzyMatch(content, search string) (int, int) {
	normalizedContent := normalizeForFuzzy(content)
	normalizedSearch := normalizeForFuzzy(search)

	idx := strings.Index(normalizedContent, normalizedSearch)
	if idx < 0 {
		return -1, 0
	}

	// Map back to original content position
	// Since normalization can change lengths, we need to find the
	// corresponding region in the original content
	origIdx := mapFuzzyIndex(content, normalizedContent, idx)
	origEnd := mapFuzzyIndex(content, normalizedContent, idx+len(normalizedSearch))

	return origIdx, origEnd - origIdx
}

func normalizeForFuzzy(s string) string {
	// Strip trailing whitespace per line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRightFunc(line, unicode.IsSpace)
	}
	result := strings.Join(lines, "\n")

	// Normalize smart quotes
	replacer := strings.NewReplacer(
		"\u201c", "\"", // left double quote
		"\u201d", "\"", // right double quote
		"\u2018", "'", // left single quote
		"\u2019", "'", // right single quote
		"\u2013", "-", // en dash
		"\u2014", "-", // em dash
		"\u00a0", " ", // non-breaking space
	)
	return replacer.Replace(result)
}

func mapFuzzyIndex(original, normalized string, normIdx int) int {
	// Simple approach: count runes up to normIdx in normalized,
	// then advance that many runes in original.
	// This works because our normalization only replaces runes 1:1.
	origRunes := []rune(original)
	normRunes := []rune(normalized)

	if normIdx >= len(normRunes) {
		return len(original)
	}

	// Count bytes for the first normIdx runes in original
	byteCount := 0
	for i := 0; i < normIdx && i < len(origRunes); i++ {
		byteCount += len(string(origRunes[i]))
	}
	return byteCount
}

// generateDiff creates a simple unified diff showing the change.
func generateDiff(path, old, new string, changeIdx int) string {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	// Find the line number where the change starts
	lineNum := strings.Count(old[:changeIdx], "\n") + 1

	// Show context around the change
	contextLines := 3
	start := max(lineNum-contextLines-1, 0)

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- %s\n+++ %s\n", path, path))

	// Find changed region
	endOld := min(lineNum+contextLines+countNewlines(old[changeIdx:])+1, len(oldLines))
	endNew := min(lineNum+contextLines+countNewlines(new[changeIdx:])+1, len(newLines))

	diff.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", start+1, endOld-start, start+1, endNew-start))

	// Very simplified diff: show old lines as removed, new lines as added
	// around the change region
	for i := start; i < endOld && i < len(oldLines); i++ {
		prefix := " "
		if i >= lineNum-1 && i < lineNum-1+countNewlines(old[changeIdx:])+1 {
			prefix = "-"
		}
		diff.WriteString(fmt.Sprintf("%s %s\n", prefix, oldLines[i]))
	}

	return diff.String()
}

func countNewlines(s string) int {
	return strings.Count(s, "\n")
}
