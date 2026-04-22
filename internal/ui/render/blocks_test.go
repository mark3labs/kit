package render

import (
	"strings"
	"testing"

	"github.com/indaco/herald"

	"github.com/mark3labs/kit/internal/ui/style"
)

// testTypography creates a herald Typography for tests.
func testTypography(theme style.Theme) *herald.Typography {
	return herald.New(
		herald.WithPalette(herald.ColorPalette{
			Primary:   theme.Primary,
			Secondary: theme.Secondary,
			Tertiary:  theme.Info,
			Accent:    theme.Accent,
			Highlight: theme.Highlight,
			Muted:     theme.Muted,
			Text:      theme.Text,
			Surface:   theme.Background,
			Base:      theme.CodeBg,
		}),
		herald.WithAlertLabel(herald.AlertTip, ""),
		herald.WithAlertIcon(herald.AlertTip, ""),
	)
}

func TestHighlightFileTokens(t *testing.T) {
	theme := style.DefaultTheme()

	tests := []struct {
		name     string
		input    string
		wantHas  []string // substrings that must be present in the output
		wantNone []string // substrings that must NOT be present as plain text
	}{
		{
			name:    "no tokens",
			input:   "hello world",
			wantHas: []string{"hello world"},
		},
		{
			name:    "single unquoted token",
			input:   "refactor @main.go please",
			wantHas: []string{"@main.go", "refactor", "please"},
		},
		{
			name:    "quoted token with spaces",
			input:   `check @"path with spaces/file.txt" out`,
			wantHas: []string{`@"path with spaces/file.txt"`, "check", "out"},
		},
		{
			name:    "multiple tokens",
			input:   "@main.go @utils.go refactor these",
			wantHas: []string{"@main.go", "@utils.go", "refactor these"},
		},
		{
			name:    "path with directory",
			input:   "look at @internal/ui/render/blocks.go",
			wantHas: []string{"@internal/ui/render/blocks.go", "look at"},
		},
		{
			name:    "empty string",
			input:   "",
			wantHas: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HighlightFileTokens(tt.input, theme)

			for _, want := range tt.wantHas {
				if !strings.Contains(result, want) {
					t.Errorf("HighlightFileTokens(%q) = %q, want substring %q", tt.input, result, want)
				}
			}

			// If there were @tokens, the result should contain ANSI escape
			// sequences (from lipgloss styling).
			if fileTokenPattern.MatchString(tt.input) && !strings.Contains(result, "\x1b[") {
				t.Errorf("HighlightFileTokens(%q) should contain ANSI escapes for @tokens but got %q", tt.input, result)
			}
		})
	}
}

func TestUserBlockHighlightsFileTokens(t *testing.T) {
	theme := style.DefaultTheme()
	ty := testTypography(theme)

	// A user message with @file tokens should contain ANSI escapes around the token.
	content := "refactor @main.go and @utils.go"
	result := UserBlock(content, 80, ty, theme)

	// The rendered output should contain both file references.
	if !strings.Contains(result, "@main.go") {
		t.Errorf("UserBlock output should contain @main.go, got:\n%s", result)
	}
	if !strings.Contains(result, "@utils.go") {
		t.Errorf("UserBlock output should contain @utils.go, got:\n%s", result)
	}

	// Verify ANSI codes are present (the tokens are styled).
	if !strings.Contains(result, "\x1b[") {
		t.Errorf("UserBlock output should contain ANSI escape codes for styled @file tokens")
	}
}
