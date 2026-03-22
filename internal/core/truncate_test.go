package core

import (
	"strings"
	"testing"
)

func TestTruncateTail_LongLines(t *testing.T) {
	// A single line of 5000 chars should be truncated to defaultMaxLineLen.
	longLine := strings.Repeat("x", 5000)
	tr := TruncateTail(longLine, 2000, 50*1024)

	if len(tr.Content) > defaultMaxLineLen+100 { // +100 for the "[...N chars truncated]" suffix
		t.Errorf("single long line not truncated: got %d chars, want <= %d", len(tr.Content), defaultMaxLineLen+100)
	}
	if !strings.Contains(tr.Content, "chars truncated]") {
		t.Error("truncated line should contain truncation marker")
	}
}

func TestTruncateTail_NormalLines(t *testing.T) {
	// Lines within the limit should pass through unchanged.
	content := "line1\nline2\nline3"
	tr := TruncateTail(content, 2000, 50*1024)
	if tr.Content != content {
		t.Errorf("got %q, want %q", tr.Content, content)
	}
	if tr.Truncated {
		t.Error("should not be marked as truncated")
	}
}

func TestTruncateTail_LineCount(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line"
	}
	content := strings.Join(lines, "\n")
	tr := TruncateTail(content, 10, 50*1024)

	if !tr.Truncated {
		t.Error("should be marked as truncated")
	}
	if tr.Total != 100 {
		t.Errorf("total = %d, want 100", tr.Total)
	}
	if tr.Kept != 10 {
		t.Errorf("kept = %d, want 10", tr.Kept)
	}
}

func TestTruncateHead_LongLines(t *testing.T) {
	longLine := strings.Repeat("y", 5000)
	tr := truncateHead(longLine, 2000, 50*1024)

	if len(tr.Content) > defaultMaxLineLen+100 {
		t.Errorf("single long line not truncated: got %d chars, want <= %d", len(tr.Content), defaultMaxLineLen+100)
	}
	if !strings.Contains(tr.Content, "chars truncated]") {
		t.Error("truncated line should contain truncation marker")
	}
}

func TestTruncateHead_NormalLines(t *testing.T) {
	content := "line1\nline2\nline3"
	tr := truncateHead(content, 2000, 50*1024)
	if tr.Content != content {
		t.Errorf("got %q, want %q", tr.Content, content)
	}
	if tr.Truncated {
		t.Error("should not be marked as truncated")
	}
}

func TestTruncateHead_LineCount(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line"
	}
	content := strings.Join(lines, "\n")
	tr := truncateHead(content, 10, 50*1024)

	if !tr.Truncated {
		t.Error("should be marked as truncated")
	}
	if tr.Total != 100 {
		t.Errorf("total = %d, want 100", tr.Total)
	}
	if tr.Kept != 10 {
		t.Errorf("kept = %d, want 10", tr.Kept)
	}
}

func TestTruncateLongLines(t *testing.T) {
	lines := []string{
		"short",
		strings.Repeat("a", 3000),
		"also short",
	}
	result := truncateLongLines(lines, 100)

	if result[0] != "short" {
		t.Error("short line should be unchanged")
	}
	if len(result[1]) > 200 { // 100 chars + marker
		t.Errorf("long line not truncated: len=%d", len(result[1]))
	}
	if !strings.Contains(result[1], "chars truncated]") {
		t.Error("should contain truncation marker")
	}
	if result[2] != "also short" {
		t.Error("short line should be unchanged")
	}
}

func TestTruncateTail_MixedLongAndManyLines(t *testing.T) {
	// 50 lines, each 3000 chars — tests both per-line and total truncation.
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = strings.Repeat("z", 3000)
	}
	content := strings.Join(lines, "\n")

	tr := TruncateTail(content, 10, 50*1024)

	// Should keep 10 lines.
	if tr.Kept != 10 {
		t.Errorf("kept = %d, want 10", tr.Kept)
	}
	// Each line should be capped at ~defaultMaxLineLen.
	resultLines := strings.Split(tr.Content, "\n")
	for i, line := range resultLines {
		if len(line) > defaultMaxLineLen+100 {
			t.Errorf("line %d too long: %d chars", i, len(line))
		}
	}
}

func TestTruncateLine(t *testing.T) {
	short := "hello"
	if truncateLine(short, 10) != short {
		t.Error("short line should be unchanged")
	}

	long := strings.Repeat("x", 100)
	result := truncateLine(long, 10)
	if len(result) != 13 { // 10 + "..."
		t.Errorf("got len %d, want 13", len(result))
	}

	// Default max for 0 — input shorter than default, so unchanged
	result2 := truncateLine(long, 0)
	if result2 != long {
		t.Errorf("100-char line should be unchanged when maxChars defaults to %d", grepMaxLineLen)
	}

	// Longer input with default
	veryLong := strings.Repeat("x", 1000)
	result3 := truncateLine(veryLong, 0)
	if len(result3) != grepMaxLineLen+3 {
		t.Errorf("got len %d, want %d", len(result3), grepMaxLineLen+3)
	}
}
