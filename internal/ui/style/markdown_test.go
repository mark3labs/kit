package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// --------------------------------------------------------------------------
// Markdown typography
// --------------------------------------------------------------------------

// TestMarkdownKeepsHeraldsGlyphTokens is the regression guard for a whole
// class of silently-missing markup.
//
// The typography used to be built from a herald.Theme literal, and WithTheme
// *replaces* the theme rather than merging into it. Every field the literal did
// not mention was left at its zero value — and many of herald's fields are
// glyphs and widths, not colors. The result was a markdown renderer with no
// list bullet, no nested-list indent, no blockquote bar, no horizontal rule and
// no table borders: `- item` rendered as a bare indented line and `---`
// rendered as nothing at all.
//
// Each case below is one token that a theme literal drops.
func TestMarkdownKeepsHeraldsGlyphTokens(t *testing.T) {
	SetTheme(DefaultTheme())

	tests := []struct {
		name     string
		markdown string
		want     string
	}{
		{"list bullet", "- item one\n", "•"},
		{"nested bullet", "- outer\n  - inner\n", "◦"},
		{"blockquote bar", "> quoted\n", "│"},
		{"horizontal rule", "before\n\n---\n\nafter\n", "───"},
		{"table border", "| a | b |\n|---|---|\n| 1 | 2 |\n", "┌"},
		{"table junction", "| a | b |\n|---|---|\n| 1 | 2 |\n", "┼"},
	}

	for _, tt := range tests {
		got := xansi.Strip(ToMarkdown(tt.markdown, 60))
		if !strings.Contains(got, tt.want) {
			t.Errorf("%s: rendered output has no %q\n%s", tt.name, tt.want, got)
		}
	}
}

// TestMarkdownNestedListsIndent verifies the nesting indent survives, which is
// the difference between a hierarchy and a flat list.
func TestMarkdownNestedListsIndent(t *testing.T) {
	SetTheme(DefaultTheme())

	out := xansi.Strip(ToMarkdown("- outer\n  - inner\n", 60))
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected two list rows, got %d:\n%s", len(lines), out)
	}

	outerIndent := len(lines[0]) - len(strings.TrimLeft(lines[0], " "))
	innerIndent := len(lines[1]) - len(strings.TrimLeft(lines[1], " "))
	if innerIndent <= outerIndent {
		t.Errorf("nested item indent %d is not deeper than its parent's %d\n%s",
			innerIndent, outerIndent, out)
	}
}

// TestMarkdownHighlightsFencedCode verifies fenced blocks are syntax
// highlighted. The hook herald offers for this (CodeFormatter) was never wired
// up, so code blocks rendered in one flat colour throughout the UI.
func TestMarkdownHighlightsFencedCode(t *testing.T) {
	SetTheme(DefaultTheme())

	const code = "```go\nfunc main() { return }\n```\n"

	out := ToMarkdown(code, 60)
	plain := xansi.Strip(out)

	if !strings.Contains(plain, "func main()") {
		t.Fatalf("code block missing from output:\n%s", plain)
	}

	// A highlighted block sets a foreground per token, so the keyword and the
	// identifier land in different colours. One colour for the whole block
	// means the formatter never ran.
	colors := map[string]bool{}
	for seq := range strings.SplitSeq(out, "\x1b[") {
		if i := strings.IndexByte(seq, 'm'); i > 0 && strings.HasPrefix(seq, "38;2;") {
			colors[seq[:i]] = true
		}
	}
	if len(colors) < 2 {
		t.Errorf("fenced code block carries %d foreground colours, want several (not highlighted)", len(colors))
	}
}

// TestMarkdownLeavesUnlabelledFencesAlone verifies a fence with no language is
// passed through rather than guessed at. Content-sniffing a block of prose or
// log output would colour it as whatever language it least resembles.
func TestMarkdownLeavesUnlabelledFencesAlone(t *testing.T) {
	SetTheme(DefaultTheme())

	out := ToMarkdown("```\nplain text, no language\n```\n", 60)
	if !strings.Contains(xansi.Strip(out), "plain text, no language") {
		t.Error("unlabelled fence content missing from output")
	}
}

// TestMarkdownParagraphsAreNotDoubleSpaced pins the one token deliberately
// overridden away from herald's default: a bottom margin on every paragraph
// doubles the height of ordinary prose in a scrollback.
func TestMarkdownParagraphsAreNotDoubleSpaced(t *testing.T) {
	SetTheme(DefaultTheme())

	out := xansi.Strip(ToMarkdown("one\n", 60))
	if got := strings.Count(strings.TrimRight(out, "\n"), "\n"); got != 0 {
		t.Errorf("a one-line paragraph rendered %d extra rows:\n%q", got, out)
	}
}

// TestMarkdownFollowsThemeChanges verifies the typography cache is rebuilt when
// the theme changes, rather than serving colours from the previous one.
func TestMarkdownFollowsThemeChanges(t *testing.T) {
	SetTheme(DefaultTheme())
	first := ToMarkdown("# Heading\n", 40)

	// Any theme with a different heading colour will do.
	alt := DefaultTheme()
	alt.Markdown.Heading = lipgloss.Color("#00ff00")
	SetTheme(alt)
	second := ToMarkdown("# Heading\n", 40)

	if first == second {
		t.Error("markdown output did not change with the theme")
	}

	SetTheme(DefaultTheme())
}

// --------------------------------------------------------------------------
// Syntax highlighting
// --------------------------------------------------------------------------

// TestHighlightLangResolvesAliases verifies languages are looked up by name and
// alias, so a fence tagged `sh` or `golang` highlights as well as `bash` or
// `go`.
func TestHighlightLangResolvesAliases(t *testing.T) {
	SetTheme(DefaultTheme())

	for _, lang := range []string{"go", "golang", "python", "py", "bash", "sh", "json"} {
		if got := HighlightLang("x = 1", lang); got == "x = 1" {
			t.Errorf("language %q produced no highlighting", lang)
		}
	}
}

// TestHighlightLangUnknownIsPassthrough verifies an unrecognised language
// leaves the source untouched. Unhighlighted code is a cosmetic loss; mangled
// code is not.
func TestHighlightLangUnknownIsPassthrough(t *testing.T) {
	SetTheme(DefaultTheme())

	const src = "some arbitrary content"
	for _, lang := range []string{"", "not-a-language-at-all", "zzzz"} {
		if got := HighlightLang(src, lang); got != src {
			t.Errorf("language %q altered the source: %q", lang, got)
		}
	}
}

// TestHighlightUsesForegroundOnlyResets verifies the emitted escapes never
// clear the background.
//
// chroma ends each token with a full SGR reset, which wipes the background the
// containing lipgloss style painted — punching holes through the panel fills
// that tool output is rendered on.
func TestHighlightUsesForegroundOnlyResets(t *testing.T) {
	SetTheme(DefaultTheme())

	for _, got := range []string{
		HighlightLang("func main() { return }", "go"),
		HighlightFile("func main() { return }", "main.go"),
	} {
		if strings.Contains(got, "\x1b[0m") {
			t.Error("highlighted output contains a full SGR reset; it would clear the panel background")
		}
		if !strings.Contains(got, "\x1b[") {
			t.Fatal("nothing was highlighted")
		}
	}
}

// TestHighlightFileFallsBackToContent verifies a nameless buffer is still
// highlighted by analysing what is in it.
func TestHighlightFileFallsBackToContent(t *testing.T) {
	SetTheme(DefaultTheme())

	const src = "#!/bin/bash\necho hello\n"
	if got := HighlightFile(src, ""); got == src {
		t.Error("content-based lexer detection produced no highlighting")
	}
}

// TestHighlightEmptySourceIsPassthrough guards the trivial input.
func TestHighlightEmptySourceIsPassthrough(t *testing.T) {
	if got := HighlightLang("", "go"); got != "" {
		t.Errorf("empty source produced %q", got)
	}
	if got := HighlightFile("", "main.go"); got != "" {
		t.Errorf("empty source produced %q", got)
	}
}

// TestHighlightFollowsThemeChanges verifies highlighting recolours with the
// theme rather than serving the palette it was first built with.
func TestHighlightFollowsThemeChanges(t *testing.T) {
	SetTheme(DefaultTheme())
	first := HighlightLang("func main() {}", "go")

	alt := DefaultTheme()
	alt.Markdown.Keyword = lipgloss.Color("#00ff00")
	SetTheme(alt)
	second := HighlightLang("func main() {}", "go")

	if first == second {
		t.Error("highlighting did not follow the theme change")
	}

	SetTheme(DefaultTheme())
}
