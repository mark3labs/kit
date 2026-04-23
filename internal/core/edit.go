package core

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/fantasy"

	udiff "github.com/aymanbagabas/go-udiff"
)

// Edit represents a single replacement in a multi-edit operation.
type Edit struct {
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

// editArgs holds the arguments for the edit tool.
type editArgs struct {
	Path  string `json:"path"`
	Edits []Edit `json:"edits"`
}

// replacement represents a normalized edit ready for processing.
type replacement struct {
	oldText     string // normalized old text for matching
	newText     string // normalized new text
	originalOld string // original old text for metadata
	originalNew string // original new text for metadata
	index       int    // index in the original edits array (for error messages)
}

// matchedReplacement represents a replacement with its match location.
type matchedReplacement struct {
	replacement
	start          int  // start index in normalized content
	end            int  // end index in normalized content
	usedFuzzyMatch bool // true if fuzzy matching was used
}

// NewEditTool creates the edit core tool.
func NewEditTool(opts ...ToolOption) fantasy.AgentTool {
	cfg := ApplyOptions(opts)
	return &coreTool{
		info: fantasy.ToolInfo{
			Name:        "edit",
			Description: "Edit a file by replacing exact text. All edits in the array are matched against the original file content (non-incremental) and must be non-overlapping.",
			Parameters: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to edit (relative or absolute)",
				},
				"edits": map[string]any{
					"type":        "array",
					"description": "Array of edits for multi-region replacement. Each edit must have unique, non-overlapping old_text. All matches are against the original file content.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"old_text": map[string]any{
								"type":        "string",
								"description": "Exact text to find and replace for this edit",
							},
							"new_text": map[string]any{
								"type":        "string",
								"description": "New text for this edit",
							},
						},
						"required": []string{"old_text", "new_text"},
					},
				},
			},
			Required: []string{"path", "edits"},
		},
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return executeEdit(ctx, call, cfg.WorkDir)
		},
	}
}

func executeEdit(ctx context.Context, call fantasy.ToolCall, workDir string) (fantasy.ToolResponse, error) {
	var args editArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse("failed to parse arguments: " + err.Error()), nil
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

	// Normalize and validate input
	replacements, err := normalizeEditInput(args)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	// Apply all edits
	newContent, applied, err := applyEdits(content, replacements)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	// Write the file
	if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to write file: %v", err)), nil
	}

	// Generate diff
	normalizedContent := strings.ReplaceAll(content, "\r\n", "\n")
	diff := generateDiff(absPath, normalizedContent, newContent)

	// Build response with fuzzy match indication
	fuzzyCount := 0
	for _, m := range applied {
		if m.usedFuzzyMatch {
			fuzzyCount++
		}
	}

	var msg string
	if len(applied) == 1 {
		if fuzzyCount > 0 {
			msg = fmt.Sprintf("Applied edit (fuzzy match) to %s\n%s", args.Path, diff)
		} else {
			msg = fmt.Sprintf("Applied edit to %s\n%s", args.Path, diff)
		}
	} else {
		if fuzzyCount > 0 {
			msg = fmt.Sprintf("Applied %d edits (%d fuzzy) to %s\n%s", len(applied), fuzzyCount, args.Path, diff)
		} else {
			msg = fmt.Sprintf("Applied %d edits to %s\n%s", len(applied), args.Path, diff)
		}
	}

	resp := fantasy.NewTextResponse(msg)
	return fantasy.WithResponseMetadata(resp, editDiffMeta(absPath, applied)), nil
}

// normalizeEditInput validates and normalizes the edit input.
func normalizeEditInput(args editArgs) ([]replacement, error) {
	if len(args.Edits) == 0 {
		return nil, fmt.Errorf("edits array is required and must not be empty")
	}

	var reps []replacement
	for i, edit := range args.Edits {
		if edit.OldText == "" {
			return nil, fmt.Errorf("edits[%d].old_text is required", i)
		}
		reps = append(reps, replacement{
			oldText:     strings.ReplaceAll(edit.OldText, "\r\n", "\n"),
			newText:     strings.ReplaceAll(edit.NewText, "\r\n", "\n"),
			originalOld: edit.OldText,
			originalNew: edit.NewText,
			index:       i,
		})
	}
	return reps, nil
}

// applyEdits applies multiple replacements to the content.
// All matches are against the original content (non-incremental).
// Returns the new content, the applied matches, and any error.
func applyEdits(content string, edits []replacement) (string, []matchedReplacement, error) {
	normalizedContent := strings.ReplaceAll(content, "\r\n", "\n")

	// Find all matches
	var matched []matchedReplacement
	for _, edit := range edits {
		m, err := findMatch(normalizedContent, edit)
		if err != nil {
			return "", nil, err
		}
		matched = append(matched, *m)
	}

	// Sort by position
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].start < matched[j].start
	})

	// Check for overlaps
	for i := 1; i < len(matched); i++ {
		if matched[i-1].end > matched[i].start {
			return "", nil, fmt.Errorf("edits[%d] and edits[%d] overlap; merge them into a single edit",
				matched[i-1].index, matched[i].index)
		}
	}

	// Apply edits in reverse order (end to start) to maintain stable offsets
	result := normalizedContent
	for i := len(matched) - 1; i >= 0; i-- {
		m := matched[i]
		result = result[:m.start] + m.newText + result[m.end:]
	}

	return result, matched, nil
}

// findMatch finds a unique match for the edit in the content.
// Returns error if not found or ambiguous.
func findMatch(content string, edit replacement) (*matchedReplacement, error) {
	// Try exact match first
	count := strings.Count(content, edit.oldText)

	if count == 0 {
		// Try fuzzy match
		idx, matchLen := fuzzyMatch(content, edit.oldText)
		if idx < 0 {
			return nil, fmt.Errorf("edits[%d]: could not find old_text in file. The text must match exactly (including whitespace)", edit.index)
		}
		// Use the matched text from content for the replacement
		matchedText := content[idx : idx+matchLen]
		return &matchedReplacement{
			replacement: replacement{
				oldText:     matchedText,
				newText:     edit.newText,
				originalOld: edit.originalOld,
				originalNew: edit.originalNew,
				index:       edit.index,
			},
			start:          idx,
			end:            idx + matchLen,
			usedFuzzyMatch: true,
		}, nil
	}

	if count > 1 {
		return nil, fmt.Errorf("found %d matches for edits[%d].old_text; each old_text must be unique, provide more context to identify the correct match", count, edit.index)
	}

	// Single exact match
	idx := strings.Index(content, edit.oldText)
	return &matchedReplacement{
		replacement: edit,
		start:       idx,
		end:         idx + len(edit.oldText),
	}, nil
}

// editDiffMeta builds the structured metadata attached to edit tool responses.
func editDiffMeta(path string, applied []matchedReplacement) map[string]any {
	var diffBlocks []map[string]any
	totalAdditions, totalDeletions := 0, 0

	for _, m := range applied {
		diffBlocks = append(diffBlocks, map[string]any{
			"old_text": m.originalOld,
			"new_text": m.originalNew,
		})
		totalAdditions += strings.Count(m.originalNew, "\n") + 1
		totalDeletions += strings.Count(m.originalOld, "\n") + 1
	}

	return map[string]any{
		"file_diffs": []map[string]any{{
			"path":        path,
			"additions":   totalAdditions,
			"deletions":   totalDeletions,
			"diff_blocks": diffBlocks,
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
