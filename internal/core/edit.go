package core

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/fantasy"

	udiff "github.com/aymanbagabas/go-udiff"
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
			// Apply fuzzy match — the matched text is the original content slice
			matchedText := normalized[idx : idx+matchLen]
			newContent := normalized[:idx] + args.NewText + normalized[idx+matchLen:]
			if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to write file: %v", err)), nil
			}
			diff := generateDiff(absPath, normalized, newContent)
			resp := fantasy.NewTextResponse(fmt.Sprintf("Applied edit (fuzzy match) to %s\n%s", args.Path, diff))
			return fantasy.WithResponseMetadata(resp, editDiffMeta(absPath, matchedText, args.NewText)), nil
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

	diff := generateDiff(absPath, normalized, newContent)
	resp := fantasy.NewTextResponse(fmt.Sprintf("Applied edit to %s\n%s", args.Path, diff))
	return fantasy.WithResponseMetadata(resp, editDiffMeta(absPath, normalizedOld, args.NewText)), nil
}

// editDiffMeta builds the structured metadata attached to edit tool responses.
func editDiffMeta(path, oldText, newText string) map[string]any {
	return map[string]any{
		"file_diffs": []map[string]any{{
			"path":      path,
			"additions": strings.Count(newText, "\n") + 1,
			"deletions": strings.Count(oldText, "\n") + 1,
			"diff_blocks": []map[string]any{{
				"old_text": oldText,
				"new_text": newText,
			}},
		}},
	}
}

// fuzzyMatch tries to find old_text with relaxed matching:
//   - Strips trailing whitespace per line
//   - Normalizes unicode quotes to ASCII
//   - Normalizes unicode dashes/spaces
//
// Returns (index, matchLength) in the original content, or (-1, 0) if not
// found or ambiguous (multiple matches).
func fuzzyMatch(content, search string) (int, int) {
	normContent, contentMap := normalizeWithMap(content)
	normSearch := normalizeForFuzzy(search)

	if normSearch == "" {
		return -1, 0
	}

	idx := strings.Index(normContent, normSearch)
	if idx < 0 {
		return -1, 0
	}

	// Reject ambiguous matches — if there are multiple fuzzy matches
	// we can't safely pick one.
	if strings.Count(normContent, normSearch) > 1 {
		return -1, 0
	}

	// Map normalized byte positions back to original byte positions.
	origStart := contentMap[idx]
	endNorm := idx + len(normSearch)
	var origEnd int
	if endNorm >= len(normContent) {
		origEnd = len(content)
	} else {
		origEnd = contentMap[endNorm]
	}

	return origStart, origEnd - origStart
}

// normalizeWithMap normalizes s for fuzzy matching and returns both the
// normalized string and a byte-position mapping where mapping[i] is the
// original byte position corresponding to normalized byte position i.
//
// Normalization: trim trailing whitespace per line, replace unicode
// quotes/dashes/spaces with their ASCII equivalents.
func normalizeWithMap(s string) (string, []int) {
	var result []byte
	var mapping []int // mapping[i] = original byte position for result byte i

	lines := strings.Split(s, "\n")
	origPos := 0
	for li, line := range lines {
		if li > 0 {
			result = append(result, '\n')
			mapping = append(mapping, origPos)
			origPos++ // skip \n in original
		}

		trimmed := strings.TrimRightFunc(line, unicode.IsSpace)

		for j := 0; j < len(trimmed); {
			r, size := utf8.DecodeRuneInString(trimmed[j:])
			repl := normalizeRune(r)
			for k := 0; k < len(repl); k++ {
				mapping = append(mapping, origPos+j)
			}
			result = append(result, repl...)
			j += size
		}

		origPos += len(line) // advance past full original line including trailing ws
	}

	return string(result), mapping
}

// normalizeRune maps unicode quotes, dashes, and non-breaking spaces to
// their ASCII equivalents. Returns the original rune as a string for all
// other characters.
func normalizeRune(r rune) string {
	switch r {
	case '\u201c', '\u201d': // left/right double quote
		return "\""
	case '\u2018', '\u2019': // left/right single quote
		return "'"
	case '\u2013', '\u2014': // en dash, em dash
		return "-"
	case '\u00a0': // non-breaking space
		return " "
	default:
		return string(r)
	}
}

// normalizeForFuzzy normalizes s for fuzzy matching (without position mapping).
// Used for the search string where position mapping is not needed.
func normalizeForFuzzy(s string) string {
	norm, _ := normalizeWithMap(s)
	return norm
}

// generateDiff creates a unified diff showing the change between old and new
// file contents. Uses the go-udiff library for correct diff computation.
func generateDiff(path, old, new string) string {
	return udiff.Unified(path, path, old, new)
}
