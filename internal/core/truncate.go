package core

import (
	"fmt"
	"strings"
)

const (
	defaultMaxLines = 2000
	defaultMaxBytes = 50 * 1024 // 50KB
	grepMaxLineLen  = 500
)

// TruncationResult describes how output was truncated.
type TruncationResult struct {
	Content   string
	Truncated bool
	TruncBy   string // "lines", "bytes", or ""
	Total     int    // total lines before truncation
	Kept      int    // lines kept after truncation
}

// truncateTail keeps the last maxLines lines and at most maxBytes bytes.
// Used for bash output where the tail is most relevant.
func truncateTail(content string, maxLines, maxBytes int) TruncationResult {
	if maxLines <= 0 {
		maxLines = defaultMaxLines
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	lines := strings.Split(content, "\n")
	total := len(lines)

	if len(content) <= maxBytes && total <= maxLines {
		return TruncationResult{Content: content, Total: total, Kept: total}
	}

	// Truncate by lines first (keep tail)
	truncBy := ""
	if total > maxLines {
		lines = lines[total-maxLines:]
		truncBy = "lines"
	}

	result := strings.Join(lines, "\n")

	// Then truncate by bytes if still too large
	if len(result) > maxBytes {
		// Find a line boundary near the byte limit
		result = result[len(result)-maxBytes:]
		// Discard partial first line
		if idx := strings.Index(result, "\n"); idx >= 0 {
			result = result[idx+1:]
		}
		truncBy = "bytes"
	}

	kept := strings.Count(result, "\n") + 1
	if truncBy != "" {
		header := fmt.Sprintf("[truncated %d/%d lines, showing last %d lines]\n", total-kept, total, kept)
		result = header + result
	}

	return TruncationResult{
		Content:   result,
		Truncated: truncBy != "",
		TruncBy:   truncBy,
		Total:     total,
		Kept:      kept,
	}
}

// truncateHead keeps the first maxLines lines and at most maxBytes bytes.
// Used for read, grep, find, ls output where the head is most relevant.
func truncateHead(content string, maxLines, maxBytes int) TruncationResult {
	if maxLines <= 0 {
		maxLines = defaultMaxLines
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	lines := strings.Split(content, "\n")
	total := len(lines)

	if len(content) <= maxBytes && total <= maxLines {
		return TruncationResult{Content: content, Total: total, Kept: total}
	}

	truncBy := ""
	if total > maxLines {
		lines = lines[:maxLines]
		truncBy = "lines"
	}

	result := strings.Join(lines, "\n")

	if len(result) > maxBytes {
		result = result[:maxBytes]
		// Discard partial last line
		if idx := strings.LastIndex(result, "\n"); idx >= 0 {
			result = result[:idx]
		}
		truncBy = "bytes"
	}

	kept := strings.Count(result, "\n") + 1
	if truncBy != "" {
		result += fmt.Sprintf("\n[truncated %d/%d lines, showing first %d lines]", total-kept, total, kept)
	}

	return TruncationResult{
		Content:   result,
		Truncated: truncBy != "",
		TruncBy:   truncBy,
		Total:     total,
		Kept:      kept,
	}
}

// truncateLine truncates a single line to maxChars, appending "..." if cut.
func truncateLine(line string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = grepMaxLineLen
	}
	if len(line) <= maxChars {
		return line
	}
	return line[:maxChars] + "..."
}
