package fences

import (
	"testing"
)

func TestRanges(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    [][2]int
	}{
		{
			name:    "no fences",
			content: "hello world\nno code here",
			want:    nil,
		},
		{
			name:    "single backtick fence",
			content: "before\n```\ncode\n```\nafter",
			want:    [][2]int{{7, 20}},
		},
		{
			name:    "single tilde fence",
			content: "before\n~~~\ncode\n~~~\nafter",
			want:    [][2]int{{7, 20}},
		},
		{
			name:    "fence with info string",
			content: "before\n```go\ncode\n```\nafter",
			want:    [][2]int{{7, 22}},
		},
		{
			name:    "multiple fences",
			content: "a\n```\nx\n```\nb\n~~~\ny\n~~~\nc",
			want:    [][2]int{{2, 12}, {14, 24}},
		},
		{
			name:    "unclosed fence",
			content: "before\n```\ncode\nmore code",
			want:    [][2]int{{7, 25}},
		},
		{
			name:    "longer closing fence",
			content: "before\n```\ncode\n`````\nafter",
			want:    [][2]int{{7, 22}},
		},
		{
			name:    "shorter closing fence ignored",
			content: "before\n`````\ncode\n```\nmore\n`````\nafter",
			want:    [][2]int{{7, 33}},
		},
		{
			name:    "indented fence up to 3 spaces",
			content: "before\n   ```\ncode\n   ```\nafter",
			want:    [][2]int{{7, 26}},
		},
		{
			name:    "4 space indent is not a fence",
			content: "before\n    ```\ncode\n    ```\nafter",
			want:    nil,
		},
		{
			name: "backtick in info string rejects open",
			// The ```foo`bar line is not a valid opener (backtick in info).
			// The standalone ``` becomes an opener with no close.
			content: "before\n```foo`bar\ncode\n```\nafter",
			want:    [][2]int{{23, 32}},
		},
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
		{
			name:    "fence only",
			content: "```\ncode\n```",
			want:    [][2]int{{0, 12}},
		},
		{
			name:    "fence at end without trailing newline",
			content: "```\ncode\n```",
			want:    [][2]int{{0, 12}},
		},
		{
			name:    "tilde fence does not close with backticks",
			content: "~~~\ncode\n```\nmore\n~~~\nafter",
			want:    [][2]int{{0, 22}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Ranges(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("Ranges() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Ranges()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestReplaceOutside(t *testing.T) {
	upper := func(s string) string {
		b := []byte(s)
		for i, c := range b {
			if c >= 'a' && c <= 'z' {
				b[i] = c - 32
			}
		}
		return string(b)
	}

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "no fences",
			content: "hello world",
			want:    "HELLO WORLD",
		},
		{
			name:    "text around fence",
			content: "before\n```\ncode\n```\nafter",
			want:    "BEFORE\n```\ncode\n```\nAFTER",
		},
		{
			name:    "multiple fences",
			content: "aaa\n```\nxxx\n```\nbbb\n~~~\nyyy\n~~~\nccc",
			want:    "AAA\n```\nxxx\n```\nBBB\n~~~\nyyy\n~~~\nCCC",
		},
		{
			name:    "unclosed fence preserves code",
			content: "before\n```\ncode",
			want:    "BEFORE\n```\ncode",
		},
		{
			name:    "only fenced content",
			content: "```\ncode\n```",
			want:    "```\ncode\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplaceOutside(tt.content, upper)
			if got != tt.want {
				t.Errorf("ReplaceOutside() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestStripFenced(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "no fences",
			content: "hello $1 world",
			want:    "hello $1 world",
		},
		{
			name:    "strips fenced code",
			content: "before $1\n```\n$2 inside\n```\nafter $3",
			want:    "before $1\nafter $3",
		},
		{
			name:    "multiple fences",
			content: "a\n```\nx\n```\nb\n~~~\ny\n~~~\nc",
			want:    "a\nb\nc",
		},
		{
			name:    "unclosed fence",
			content: "before\n```\n$1 inside",
			want:    "before\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripFenced(tt.content)
			if got != tt.want {
				t.Errorf("StripFenced() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInlineCodeRanges(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want [][2]int
	}{
		{"no backticks", "hello world", nil},
		{"single backtick span", "use `$1` here", [][2]int{{4, 8}}},
		{"double backtick span", "use ``$1`` here", [][2]int{{4, 10}}},
		{"multiple spans", "`$1` and `$2`", [][2]int{{0, 4}, {9, 13}}},
		{"unmatched backtick", "use `$1 here", nil},
		{"mismatched backtick counts", "use ``$1` here", nil},
		{"empty inline content", "use `` `` here", [][2]int{{4, 9}}},
		{"backticks inside double", "use ``foo`bar`` here", [][2]int{{4, 15}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inlineCodeRanges(tt.s)
			if len(got) != len(tt.want) {
				t.Fatalf("inlineCodeRanges() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("inlineCodeRanges()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestReplaceOutside_InlineCode(t *testing.T) {
	upper := func(s string) string {
		b := []byte(s)
		for i, c := range b {
			if c >= 'a' && c <= 'z' {
				b[i] = c - 32
			}
		}
		return string(b)
	}

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "inline code preserved",
			content: "use `code` here",
			want:    "USE `code` HERE",
		},
		{
			name:    "double backtick inline code",
			content: "use ``co`de`` here",
			want:    "USE ``co`de`` HERE",
		},
		{
			name:    "mixed fenced and inline",
			content: "before `x` mid\n```\nfenced\n```\nafter `y` end",
			want:    "BEFORE `x` MID\n```\nfenced\n```\nAFTER `y` END",
		},
		{
			name:    "only inline code",
			content: "`code`",
			want:    "`code`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplaceOutside(tt.content, upper)
			if got != tt.want {
				t.Errorf("ReplaceOutside() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestStripCode(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "no code",
			content: "hello $1 world",
			want:    "hello $1 world",
		},
		{
			name:    "strips inline code",
			content: "use `$1` and `$2` for positional args",
			want:    "use  and  for positional args",
		},
		{
			name:    "strips fenced and inline",
			content: "before `$1`\n```\n$2 inside\n```\nafter",
			want:    "before \nafter",
		},
		{
			name:    "real world prompt template",
			content: "Use $@ for all args.\n`$1`, `$2` for positional.\n```bash\necho $1\n```\n",
			want:    "Use $@ for all args.\n,  for positional.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripCode(tt.content)
			if got != tt.want {
				t.Errorf("StripCode() = %q, want %q", got, tt.want)
			}
		})
	}
}
