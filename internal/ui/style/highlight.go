package style

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
)

// ---------------------------------------------------------------------------
// Syntax highlighting
// ---------------------------------------------------------------------------
//
// Highlighting lives here rather than in the ui package because the theme-
// derived chroma palette (SyntaxStyle) already does, and because the markdown
// typography needs a code formatter — style cannot import ui, so the shared
// implementation has to sit on this side of the boundary.

// fgOnlyReset replaces chroma's full SGR reset. A full reset clears the
// background as well as the foreground, which punches holes in the panel fill
// that the containing lipgloss style painted. Resetting only foreground,
// bold, italic and underline leaves the background owned by the caller.
const fgOnlyReset = "\x1b[39;22;23;24m"

// langLexerCache memoizes language-name lookups. lexers.Get falls back to
// scanning every registered lexer's filename patterns when a name is not an
// exact match, which is far too slow to repeat for every fenced code block on
// every streaming flush.
//
// A nil entry records "no lexer for this language" so the miss is cached too.
// Only accessed from BubbleTea's single-threaded Update/View cycle.
var langLexerCache = map[string]chroma.Lexer{}

// HighlightFile applies syntax highlighting to source, choosing a lexer from
// fileName and falling back to content analysis. Returns the source unchanged
// when no lexer matches or highlighting fails.
func HighlightFile(source, fileName string) string {
	if source == "" {
		return source
	}

	lexer := lexers.Match(fileName)
	if lexer == nil {
		lexer = lexers.Analyse(source)
	}
	return highlightWith(source, lexer)
}

// HighlightLang applies syntax highlighting to source using the lexer for the
// given language name or alias — the info string of a markdown fence, or a
// file extension. Returns the source unchanged for an unknown language.
func HighlightLang(source, lang string) string {
	if source == "" || lang == "" {
		return source
	}

	lexer, ok := langLexerCache[lang]
	if !ok {
		lexer = lexers.Get(lang)
		langLexerCache[lang] = lexer
	}
	return highlightWith(source, lexer)
}

// highlightWith formats source with lexer using the active theme's palette.
// A nil lexer, or any failure along the way, yields the source unchanged —
// unhighlighted code is a cosmetic loss, mangled code is not.
func highlightWith(source string, lexer chroma.Lexer) string {
	if lexer == nil {
		return source
	}

	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		formatter = formatters.Get("terminal256")
	}
	if formatter == nil {
		return source
	}

	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return source
	}

	// Token backgrounds are unset in SyntaxStyle, so the containing lipgloss
	// style owns the fill and code sits in the same color family as the UI.
	var buf bytes.Buffer
	if err := formatter.Format(&buf, SyntaxStyle(), iterator); err != nil {
		return source
	}

	out := strings.ReplaceAll(buf.String(), "\x1b[0m", fgOnlyReset)
	return strings.TrimRight(out, "\n")
}
