package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"
)

func writeFileOrFail(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}

// ---------------------------------------------------------------------------
// fuzzyMatch — the core bug fix
// ---------------------------------------------------------------------------

func TestFuzzyMatch_TrailingWhitespace(t *testing.T) {
	// The original bug: trailing whitespace on lines caused mapFuzzyIndex
	// to return wrong byte positions, corrupting the replacement splice.
	content := "line1   \nline2   \nline3   \nTAIL\n"
	search := "line2\nline3"

	idx, matchLen := fuzzyMatch(content, search)
	if idx < 0 {
		t.Fatal("expected fuzzy match, got none")
	}

	matched := content[idx : idx+matchLen]
	want := "line2   \nline3   "
	if matched != want {
		t.Errorf("matched=%q, want=%q", matched, want)
	}

	// Verify replacement is correct
	repl := content[:idx] + "REPLACED" + content[idx+matchLen:]
	wantRepl := "line1   \nREPLACED\nTAIL\n"
	if repl != wantRepl {
		t.Errorf("replacement=%q, want=%q", repl, wantRepl)
	}
}

func TestFuzzyMatch_TrailingWhitespace_FirstLine(t *testing.T) {
	content := "line1   \nline2   \nline3\n"
	search := "line1\nline2"

	idx, matchLen := fuzzyMatch(content, search)
	if idx < 0 {
		t.Fatal("expected fuzzy match")
	}

	matched := content[idx : idx+matchLen]
	want := "line1   \nline2   "
	if matched != want {
		t.Errorf("matched=%q, want=%q", matched, want)
	}
}

func TestFuzzyMatch_TrailingWhitespace_LastLine(t *testing.T) {
	content := "HEAD\nline1   \nline2   \n"
	search := "line1\nline2"

	idx, matchLen := fuzzyMatch(content, search)
	if idx < 0 {
		t.Fatal("expected fuzzy match")
	}

	matched := content[idx : idx+matchLen]
	want := "line1   \nline2   "
	if matched != want {
		t.Errorf("matched=%q, want=%q", matched, want)
	}
}

func TestFuzzyMatch_TrailingWhitespace_AtEOF(t *testing.T) {
	// Match extends to the very end of the content
	content := "HEAD\nline1   \nline2   "
	search := "line1\nline2"

	idx, matchLen := fuzzyMatch(content, search)
	if idx < 0 {
		t.Fatal("expected fuzzy match")
	}

	matched := content[idx : idx+matchLen]
	want := "line1   \nline2   "
	if matched != want {
		t.Errorf("matched=%q, want=%q", matched, want)
	}
}

func TestFuzzyMatch_UnicodeQuotes(t *testing.T) {
	content := "say \u201chello\u201d\n"
	search := "say \"hello\"\n"

	idx, matchLen := fuzzyMatch(content, search)
	if idx < 0 {
		t.Fatal("expected fuzzy match for unicode quotes")
	}

	matched := content[idx : idx+matchLen]
	if matched != content { // entire content should match
		t.Errorf("matched=%q, want=%q", matched, content)
	}
}

func TestFuzzyMatch_SmartSingleQuotes(t *testing.T) {
	content := "it\u2019s a test\n"
	search := "it's a test\n"

	idx, matchLen := fuzzyMatch(content, search)
	if idx < 0 {
		t.Fatal("expected fuzzy match for smart single quotes")
	}
	matched := content[idx : idx+matchLen]
	if matched != content {
		t.Errorf("matched=%q, want=%q", matched, content)
	}
}

func TestFuzzyMatch_EmDash(t *testing.T) {
	content := "foo \u2014 bar\n"
	search := "foo - bar\n"

	idx, matchLen := fuzzyMatch(content, search)
	if idx < 0 {
		t.Fatal("expected fuzzy match for em dash")
	}
	matched := content[idx : idx+matchLen]
	if matched != content {
		t.Errorf("matched=%q, want=%q", matched, content)
	}
}

func TestFuzzyMatch_NonBreakingSpace(t *testing.T) {
	content := "hello\u00a0world\n"
	search := "hello world\n"

	idx, matchLen := fuzzyMatch(content, search)
	if idx < 0 {
		t.Fatal("expected fuzzy match for non-breaking space")
	}
	matched := content[idx : idx+matchLen]
	if matched != content {
		t.Errorf("matched=%q, want=%q", matched, content)
	}
}

func TestFuzzyMatch_NoMatch(t *testing.T) {
	content := "hello world\n"
	search := "goodbye world\n"

	idx, _ := fuzzyMatch(content, search)
	if idx >= 0 {
		t.Error("expected no match")
	}
}

func TestFuzzyMatch_AmbiguousReturnsNoMatch(t *testing.T) {
	// Two identical blocks — fuzzy match should refuse to pick one
	content := "block\nblock\n"
	search := "block"

	idx, _ := fuzzyMatch(content, search)
	if idx >= 0 {
		t.Error("expected no match for ambiguous fuzzy hit")
	}
}

func TestFuzzyMatch_EmptySearch(t *testing.T) {
	idx, _ := fuzzyMatch("content", "")
	if idx >= 0 {
		t.Error("expected no match for empty search")
	}
}

func TestFuzzyMatch_MultiLineWithMixedWhitespace(t *testing.T) {
	content := "func foo() {\t  \n\treturn 1  \n}\t \n"
	search := "func foo() {\n\treturn 1\n}"

	idx, matchLen := fuzzyMatch(content, search)
	if idx < 0 {
		t.Fatal("expected fuzzy match")
	}

	// Replacement should preserve surrounding content
	repl := content[:idx] + "func bar() {\n\treturn 2\n}" + content[idx+matchLen:]
	if !strings.HasPrefix(repl, "func bar()") {
		t.Errorf("unexpected replacement start: %q", repl[:20])
	}
	if !strings.HasSuffix(repl, "\n") {
		t.Errorf("replacement should end with newline: %q", repl)
	}
}

// ---------------------------------------------------------------------------
// normalizeWithMap — position mapping correctness
// ---------------------------------------------------------------------------

func TestNormalizeWithMap_NoTrailingWhitespace(t *testing.T) {
	s := "abc\ndef"
	norm, mapping := normalizeWithMap(s)
	if norm != s {
		t.Errorf("norm=%q, want=%q", norm, s)
	}
	// Each byte should map to itself
	for i, orig := range mapping {
		if orig != i {
			t.Errorf("mapping[%d]=%d, want=%d", i, orig, i)
		}
	}
}

func TestNormalizeWithMap_TrailingWhitespace(t *testing.T) {
	s := "ab   \ncd"
	norm, mapping := normalizeWithMap(s)
	wantNorm := "ab\ncd"
	if norm != wantNorm {
		t.Errorf("norm=%q, want=%q", norm, wantNorm)
	}
	// 'a'→0, 'b'→1, '\n'→5, 'c'→6, 'd'→7
	wantMapping := []int{0, 1, 5, 6, 7}
	if len(mapping) != len(wantMapping) {
		t.Fatalf("mapping len=%d, want=%d", len(mapping), len(wantMapping))
	}
	for i, want := range wantMapping {
		if mapping[i] != want {
			t.Errorf("mapping[%d]=%d, want=%d", i, mapping[i], want)
		}
	}
}

func TestNormalizeWithMap_UnicodeReplacement(t *testing.T) {
	// \u201c is 3 bytes in UTF-8, replaced with " which is 1 byte
	s := "\u201chello\u201d"
	norm, mapping := normalizeWithMap(s)
	wantNorm := "\"hello\""
	if norm != wantNorm {
		t.Errorf("norm=%q, want=%q", norm, wantNorm)
	}
	// " maps to byte 0 (start of \u201c), h maps to 3, e→4, l→5, l→6, o→7, " maps to 8 (start of \u201d)
	wantMapping := []int{0, 3, 4, 5, 6, 7, 8}
	if len(mapping) != len(wantMapping) {
		t.Fatalf("mapping len=%d, want=%d", len(mapping), len(wantMapping))
	}
	for i, want := range wantMapping {
		if mapping[i] != want {
			t.Errorf("mapping[%d]=%d, want=%d", i, mapping[i], want)
		}
	}
}

func TestNormalizeWithMap_EmptyString(t *testing.T) {
	norm, mapping := normalizeWithMap("")
	if norm != "" {
		t.Errorf("norm=%q, want empty", norm)
	}
	if len(mapping) != 0 {
		t.Errorf("mapping len=%d, want 0", len(mapping))
	}
}

func TestNormalizeWithMap_OnlyWhitespace(t *testing.T) {
	norm, _ := normalizeWithMap("   \n   ")
	if norm != "\n" {
		t.Errorf("norm=%q, want %q", norm, "\n")
	}
}

// ---------------------------------------------------------------------------
// normalizeForFuzzy — consistency with normalizeWithMap
// ---------------------------------------------------------------------------

func TestNormalizeForFuzzy_ConsistentWithMap(t *testing.T) {
	inputs := []string{
		"hello   \nworld   ",
		"\u201chello\u201d\u2014world",
		"a\u00a0b\u2013c\n  trailing  \n",
		"no changes here",
		"",
	}
	for _, s := range inputs {
		norm := normalizeForFuzzy(s)
		normMap, _ := normalizeWithMap(s)
		if norm != normMap {
			t.Errorf("normalizeForFuzzy(%q) = %q, normalizeWithMap = %q", s, norm, normMap)
		}
	}
}

// ---------------------------------------------------------------------------
// generateDiff — correct unified diff output
// ---------------------------------------------------------------------------

func TestGenerateDiff_SingleLineChange(t *testing.T) {
	old := "line1\nline2\nline3\nline4\nline5\nline6\nline7\n"
	new := "line1\nline2\nline3\nLINE4\nline5\nline6\nline7\n"

	diff := generateDiff("test.go", old, new)

	// Should contain standard unified diff markers
	if !strings.Contains(diff, "--- test.go") {
		t.Error("diff should contain --- header")
	}
	if !strings.Contains(diff, "+++ test.go") {
		t.Error("diff should contain +++ header")
	}
	if !strings.Contains(diff, "@@") {
		t.Error("diff should contain @@ hunk header")
	}

	// Should show the actual change
	if !strings.Contains(diff, "-line4") {
		t.Error("diff should show removed line")
	}
	if !strings.Contains(diff, "+LINE4") {
		t.Error("diff should show added line")
	}

	// Should NOT mark all remaining lines as changed (the old bug)
	deletedCount := strings.Count(diff, "\n-")
	if deletedCount > 2 { // at most 1 deleted line + some tolerance
		t.Errorf("diff shows %d deletions, expected ~1 (old bug: marked rest of file as deleted)", deletedCount)
	}
}

func TestGenerateDiff_MultiLineChange(t *testing.T) {
	old := "aaa\nbbb\nccc\nddd\n"
	new := "aaa\nBBB\nCCC\nddd\n"

	diff := generateDiff("x.go", old, new)
	if !strings.Contains(diff, "-bbb") {
		t.Error("diff should show bbb removed")
	}
	if !strings.Contains(diff, "-ccc") {
		t.Error("diff should show ccc removed")
	}
	if !strings.Contains(diff, "+BBB") {
		t.Error("diff should show BBB added")
	}
	if !strings.Contains(diff, "+CCC") {
		t.Error("diff should show CCC added")
	}
}

func TestGenerateDiff_NoChange(t *testing.T) {
	content := "hello\nworld\n"
	diff := generateDiff("x.go", content, content)
	if diff != "" {
		t.Errorf("expected empty diff for identical content, got %q", diff)
	}
}

func TestGenerateDiff_Addition(t *testing.T) {
	old := "line1\nline2\n"
	new := "line1\nnew line\nline2\n"

	diff := generateDiff("x.go", old, new)
	if !strings.Contains(diff, "+new line") {
		t.Error("diff should show added line")
	}
}

func TestGenerateDiff_Deletion(t *testing.T) {
	old := "line1\nremove me\nline2\n"
	new := "line1\nline2\n"

	diff := generateDiff("x.go", old, new)
	if !strings.Contains(diff, "-remove me") {
		t.Error("diff should show deleted line")
	}
}

// ---------------------------------------------------------------------------
// End-to-end: executeEdit via tool call
// ---------------------------------------------------------------------------

func TestExecuteEdit_ExactMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	original := "func main() {\n\tfmt.Println(\"hello\")\n}\n"
	writeFileOrFail(t, path, original)

	input, _ := json.Marshal(editArgs{
		Path:    path,
		OldText: "fmt.Println(\"hello\")",
		NewText: "fmt.Println(\"world\")",
	})

	resp, err := executeEdit(t.Context(), fantasy.ToolCall{Input: string(input)}, dir)
	if err != nil {
		t.Fatalf("executeEdit error: %v", err)
	}
	if resp.IsError {
		t.Fatalf("tool returned error: %s", resp.Content)
	}

	got, _ := os.ReadFile(path)
	want := "func main() {\n\tfmt.Println(\"world\")\n}\n"
	if string(got) != want {
		t.Errorf("file content=%q, want=%q", string(got), want)
	}
}

func TestExecuteEdit_ExactMatch_DoesNotCorruptRest(t *testing.T) {
	// This is the key regression test for the screenshot bug: editing a
	// small section must NOT delete/corrupt the rest of the file.
	dir := t.TempDir()
	path := filepath.Join(dir, "big.go")

	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, fmt.Sprintf("line_%03d_%s", i, strings.Repeat("x", 40)))
	}
	original := strings.Join(lines, "\n") + "\n"
	writeFileOrFail(t, path, original)

	// Replace just line 50
	target := lines[49]
	replacement := "REPLACED_LINE_50"
	input, _ := json.Marshal(editArgs{
		Path:    path,
		OldText: target,
		NewText: replacement,
	})

	resp, err := executeEdit(t.Context(), fantasy.ToolCall{Input: string(input)}, dir)
	if err != nil {
		t.Fatalf("executeEdit error: %v", err)
	}
	if resp.IsError {
		t.Fatalf("tool returned error: %s", resp.Content)
	}

	got, _ := os.ReadFile(path)
	gotLines := strings.Split(string(got), "\n")

	// File should still have 101 elements (100 lines + trailing empty)
	if len(gotLines) != 101 {
		t.Fatalf("file has %d lines, want 101 (content was corrupted)", len(gotLines))
	}

	// Line 50 should be replaced
	if gotLines[49] != replacement {
		t.Errorf("line 50=%q, want=%q", gotLines[49], replacement)
	}

	// Lines before and after should be untouched
	if gotLines[0] != lines[0] {
		t.Errorf("line 1 corrupted: %q", gotLines[0])
	}
	if gotLines[98] != lines[98] {
		t.Errorf("line 99 corrupted: %q", gotLines[98])
	}
}

func TestExecuteEdit_FuzzyMatch_TrailingWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ws.go")
	// File has trailing whitespace on some lines
	original := "func foo() {   \n\treturn 1   \n}\nfunc bar() {\n}\n"
	writeFileOrFail(t, path, original)

	// Search without trailing whitespace (common LLM behavior)
	input, _ := json.Marshal(editArgs{
		Path:    path,
		OldText: "func foo() {\n\treturn 1\n}",
		NewText: "func foo() {\n\treturn 2\n}",
	})

	resp, err := executeEdit(t.Context(), fantasy.ToolCall{Input: string(input)}, dir)
	if err != nil {
		t.Fatalf("executeEdit error: %v", err)
	}
	if resp.IsError {
		t.Fatalf("tool returned error: %s", resp.Content)
	}

	got, _ := os.ReadFile(path)
	gotStr := string(got)

	// The fuzzy match replaces the matched region (which includes trailing
	// whitespace) with the new_text. The key invariant is that the rest of
	// the file (func bar) must be preserved.
	if !strings.Contains(gotStr, "return 2") {
		t.Error("edit was not applied: missing 'return 2'")
	}
	if !strings.Contains(gotStr, "func bar()") {
		t.Errorf("file was corrupted: missing func bar(). got=%q", gotStr)
	}

	// Verify response mentions fuzzy match
	if !strings.Contains(resp.Content, "fuzzy match") {
		t.Error("response should mention fuzzy match")
	}
}

func TestExecuteEdit_FuzzyMatch_DoesNotCorruptRest(t *testing.T) {
	// Regression test: fuzzy match must not corrupt content after the match.
	dir := t.TempDir()
	path := filepath.Join(dir, "fuzzy.txt")

	// 20 lines, each with trailing whitespace
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, strings.Repeat("x", 10)+"   ") // trailing spaces
	}
	original := strings.Join(lines, "\n") + "\nEND\n"
	writeFileOrFail(t, path, original)

	// Search for lines 10-11 without trailing whitespace
	search := strings.Repeat("x", 10) + "\n" + strings.Repeat("x", 10)
	// But this matches lines 1-2, 2-3, etc. — should fail due to ambiguity.
	input, _ := json.Marshal(editArgs{
		Path:    path,
		OldText: search,
		NewText: "REPLACED",
	})

	resp, err := executeEdit(t.Context(), fantasy.ToolCall{Input: string(input)}, dir)
	if err != nil {
		t.Fatalf("executeEdit error: %v", err)
	}

	// This should either fail (ambiguous) or produce correct output.
	// With identical lines, fuzzy match should refuse (ambiguous).
	got, _ := os.ReadFile(path)
	if !resp.IsError {
		// If it didn't error, verify the file is not corrupted
		if !strings.HasSuffix(string(got), "END\n") {
			t.Error("file was corrupted: missing END marker")
		}
	}
}

func TestExecuteEdit_MultipleMatches_Fails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dup.txt")
	writeFileOrFail(t, path, "hello\nworld\nhello\n")

	input, _ := json.Marshal(editArgs{
		Path:    path,
		OldText: "hello",
		NewText: "goodbye",
	})

	resp, err := executeEdit(t.Context(), fantasy.ToolCall{Input: string(input)}, dir)
	if err != nil {
		t.Fatalf("executeEdit error: %v", err)
	}
	if !resp.IsError {
		t.Error("expected error for multiple matches")
	}
	if !strings.Contains(resp.Content, "2 matches") {
		t.Errorf("expected '2 matches' in error, got: %s", resp.Content)
	}

	// File should be untouched
	got, _ := os.ReadFile(path)
	if string(got) != "hello\nworld\nhello\n" {
		t.Error("file was modified despite error")
	}
}

func TestExecuteEdit_NoMatch_Fails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nomatch.txt")
	writeFileOrFail(t, path, "hello world\n")

	input, _ := json.Marshal(editArgs{
		Path:    path,
		OldText: "nonexistent text",
		NewText: "replacement",
	})

	resp, err := executeEdit(t.Context(), fantasy.ToolCall{Input: string(input)}, dir)
	if err != nil {
		t.Fatalf("executeEdit error: %v", err)
	}
	if !resp.IsError {
		t.Error("expected error for no match")
	}

	// File should be untouched
	got, _ := os.ReadFile(path)
	if string(got) != "hello world\n" {
		t.Error("file was modified despite error")
	}
}

func TestExecuteEdit_CRLFNormalization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crlf.txt")
	writeFileOrFail(t, path, "line1\r\nline2\r\nline3\r\n")

	input, _ := json.Marshal(editArgs{
		Path:    path,
		OldText: "line2",
		NewText: "LINE2",
	})

	resp, err := executeEdit(t.Context(), fantasy.ToolCall{Input: string(input)}, dir)
	if err != nil {
		t.Fatalf("executeEdit error: %v", err)
	}
	if resp.IsError {
		t.Fatalf("tool returned error: %s", resp.Content)
	}

	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "LINE2") {
		t.Error("edit was not applied")
	}
}

func TestExecuteEdit_MissingPath(t *testing.T) {
	input, _ := json.Marshal(editArgs{
		OldText: "x",
		NewText: "y",
	})
	resp, err := executeEdit(t.Context(), fantasy.ToolCall{Input: string(input)}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsError {
		t.Error("expected error for missing path")
	}
}

func TestExecuteEdit_NonexistentFile(t *testing.T) {
	input, _ := json.Marshal(editArgs{
		Path:    "/tmp/nonexistent_edit_test_file_12345.go",
		OldText: "x",
		NewText: "y",
	})
	resp, err := executeEdit(t.Context(), fantasy.ToolCall{Input: string(input)}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsError {
		t.Error("expected error for nonexistent file")
	}
}

func TestExecuteEdit_DiffContainsHunkHeader(t *testing.T) {
	// The UI's extractDiffStartLine parses @@ -N from the result.
	// Verify the diff output contains it.
	dir := t.TempDir()
	path := filepath.Join(dir, "hunk.go")
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("line_%02d_content", i))
	}
	writeFileOrFail(t, path, strings.Join(lines, "\n")+"\n")

	input, _ := json.Marshal(editArgs{
		Path:    path,
		OldText: "line_10_content",
		NewText: "REPLACED",
	})

	resp, err := executeEdit(t.Context(), fantasy.ToolCall{Input: string(input)}, dir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if resp.IsError {
		t.Fatalf("tool error: %s", resp.Content)
	}
	if !strings.Contains(resp.Content, "@@ ") {
		t.Error("diff output should contain @@ hunk header for UI parsing")
	}
}

func TestExecuteEdit_MetadataContainsFileDiffs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meta.go")
	writeFileOrFail(t, path, "old content\n")

	input, _ := json.Marshal(editArgs{
		Path:    path,
		OldText: "old content",
		NewText: "new content",
	})

	resp, err := executeEdit(t.Context(), fantasy.ToolCall{Input: string(input)}, dir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Check metadata is present
	metaJSON := resp.Metadata
	if metaJSON == "" {
		t.Fatal("expected metadata on response")
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		t.Fatalf("metadata is not valid JSON: %v", err)
	}

	diffs, ok := meta["file_diffs"]
	if !ok {
		t.Fatal("metadata missing file_diffs key")
	}

	diffList, ok := diffs.([]any)
	if !ok || len(diffList) == 0 {
		t.Fatal("file_diffs should be a non-empty array")
	}
}
