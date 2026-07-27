package style

import (
	"fmt"
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

// Enhanced styling utilities and theme definitions

// isDarkBg caches the terminal background detection result at package init.
var isDarkBg = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)

// AdaptiveColor picks between a light-mode and dark-mode hex color string
// based on the detected terminal background. This replaces the old
// lipgloss.AdaptiveColor{Light: ..., Dark: ...} pattern from v1.
func AdaptiveColor(light, dark string) color.Color {
	if isDarkBg {
		return lipgloss.Color(dark)
	}
	return lipgloss.Color(light)
}

// Global theme instance
var currentTheme = DefaultTheme()

// GetTheme returns the currently active UI theme. The theme controls all color
// and styling decisions throughout the application's interface.
func GetTheme() Theme {
	return currentTheme
}

// SetTheme updates the global UI theme, affecting all subsequent rendering
// operations. This allows runtime theme switching for different visual preferences.
// It also invalidates the markdownTypographyCache so the next call to
// GetMarkdownTypography picks up the new theme.
func SetTheme(theme Theme) {
	currentTheme = theme
	markdownTypographyCache = nil // invalidate cached renderer; colors may have changed
	uiTypographyCache = nil       // invalidate cached block typography; colors may have changed
	styleCache = nil              // invalidate cached styles; colors may have changed
	syntaxStyleCache = nil        // invalidate cached chroma style; colors may have changed
}

// CachedStyles holds pre-built lipgloss styles that are reused across
// render frames. Invalidated by SetTheme, lazily rebuilt on next access.
// Only accessed from BubbleTea's single-threaded Update/View cycle.
type CachedStyles struct {
	// render/blocks.go
	FileTokenAccent lipgloss.Style // Foreground(Accent).Bold(true)
	Muted           lipgloss.Style // Foreground(Muted)
	VeryMuted       lipgloss.Style // Foreground(VeryMuted)
	Accent          lipgloss.Style // Foreground(Accent)
	MarginBottom1   lipgloss.Style // MarginBottom(1)

	// stream.go - spinner phases
	SpinnerBright lipgloss.Style // Foreground(Primary)
	SpinnerMed    lipgloss.Style // Foreground(Muted)
	SpinnerDim    lipgloss.Style // Foreground(VeryMuted)
	SpinnerOff    lipgloss.Style // Foreground(MutedBorder)

	// message_items.go - bash output
	BashHeader lipgloss.Style // Foreground(Muted).Italic(true)
	BashStderr lipgloss.Style // Foreground(Error)

	// render/blocks.go - tool block
	ToolSuccess lipgloss.Style // Foreground(Success)
	ToolError   lipgloss.Style // Foreground(Error)
	ToolInfo    lipgloss.Style // Foreground(Info).Bold(true)
	ToolMuted   lipgloss.Style // Foreground(Muted)

	// common
	ErrorFg  lipgloss.Style // Foreground(Error)
	TextBold lipgloss.Style // Foreground(Text).Bold(true)
}

var styleCache *CachedStyles

// GetCachedStyles returns the pre-built style cache, creating it lazily
// from the current theme. Invalidated by SetTheme.
func GetCachedStyles() *CachedStyles {
	if styleCache != nil {
		return styleCache
	}
	theme := GetTheme()
	styleCache = &CachedStyles{
		FileTokenAccent: lipgloss.NewStyle().Foreground(theme.Accent).Bold(true),
		Muted:           lipgloss.NewStyle().Foreground(theme.Muted),
		VeryMuted:       lipgloss.NewStyle().Foreground(theme.VeryMuted),
		Accent:          lipgloss.NewStyle().Foreground(theme.Accent),
		MarginBottom1:   lipgloss.NewStyle().MarginBottom(1),
		SpinnerBright:   lipgloss.NewStyle().Foreground(theme.Primary),
		SpinnerMed:      lipgloss.NewStyle().Foreground(theme.Muted),
		SpinnerDim:      lipgloss.NewStyle().Foreground(theme.VeryMuted),
		SpinnerOff:      lipgloss.NewStyle().Foreground(theme.MutedBorder),
		BashHeader:      lipgloss.NewStyle().Foreground(theme.Muted).Italic(true),
		BashStderr:      lipgloss.NewStyle().Foreground(theme.Error),
		ToolSuccess:     lipgloss.NewStyle().Foreground(theme.Success),
		ToolError:       lipgloss.NewStyle().Foreground(theme.Error),
		ToolInfo:        lipgloss.NewStyle().Foreground(theme.Info).Bold(true),
		ToolMuted:       lipgloss.NewStyle().Foreground(theme.Muted),
		ErrorFg:         lipgloss.NewStyle().Foreground(theme.Error),
		TextBold:        lipgloss.NewStyle().Foreground(theme.Text).Bold(true),
	}
	return styleCache
}

// MarkdownThemeColors defines colors for markdown rendering and syntax highlighting.
type MarkdownThemeColors struct {
	Text    color.Color
	Muted   color.Color
	Heading color.Color
	Emph    color.Color
	Strong  color.Color
	Link    color.Color
	Code    color.Color
	Error   color.Color
	Keyword color.Color
	String  color.Color
	Number  color.Color
	Comment color.Color
}

// Theme defines a comprehensive color scheme for the application's UI, supporting
// both light and dark terminal modes through adaptive colors. Inspired by the
// Knight Rider KITT aesthetic — scanner reds, amber dashboard glows, and dark
// cockpit tones.
type Theme struct {
	Primary     color.Color
	Secondary   color.Color
	Success     color.Color
	Warning     color.Color
	Error       color.Color
	Info        color.Color
	Text        color.Color
	Muted       color.Color
	VeryMuted   color.Color
	Background  color.Color
	Border      color.Color
	MutedBorder color.Color
	System      color.Color
	Tool        color.Color
	Accent      color.Color
	Highlight   color.Color

	// Diff block backgrounds
	DiffInsertBg  color.Color // Green-tinted bg for added lines
	DiffDeleteBg  color.Color // Red-tinted bg for removed lines
	DiffEqualBg   color.Color // Neutral bg for context lines
	DiffMissingBg color.Color // Empty-cell bg when sides are uneven

	// Code/output block backgrounds
	CodeBg   color.Color // Background for code blocks (Read tool)
	GutterBg color.Color // Line-number gutter background
	WriteBg  color.Color // Green-tinted bg for Write tool content

	// InputBg fills the composer bar. It is a shade off the terminal
	// background — enough to read as a distinct surface, not enough to
	// compete with message content for attention.
	InputBg color.Color

	// Markdown rendering and syntax highlighting colors
	Markdown MarkdownThemeColors
}

// DefaultTheme creates and returns the default KIT theme inspired by the
// Knight Rider KITT aesthetic — scanner reds, amber dashboard glows, and a
// dark cockpit. Everything stays in the warm red/amber/gray family of KITT's
// instrument panel, with two deliberate exceptions.
//
// Success and Error must never be mistaken for Warning or for the brand red,
// because they carry opposite meanings and they appear in the same places (the
// turn receipt, the activity row, tool markers). Success therefore leans
// olive-green and Error leans crimson: both still warm, both unmistakably not
// amber and not scanner red.
func DefaultTheme() Theme {
	return Theme{
		Primary:     AdaptiveColor("#CC1100", "#FF2200"), // KITT scanner red
		Secondary:   AdaptiveColor("#CC6600", "#FF8800"), // Amber dashboard glow
		Success:     AdaptiveColor("#5F7A1F", "#A3BE4C"), // Olive green — distinct from amber
		Warning:     AdaptiveColor("#CC8800", "#FFB800"), // Amber caution light
		Error:       AdaptiveColor("#C21038", "#FF4466"), // Crimson — distinct from scanner red
		Info:        AdaptiveColor("#BB6600", "#DD8833"), // Warm amber readout
		Text:        AdaptiveColor("#1A1A1A", "#E0E0E0"), // Console text
		Muted:       AdaptiveColor("#707070", "#808080"), // Dimmed readout
		VeryMuted:   AdaptiveColor("#A0A0A0", "#505050"), // Inactive element
		Background:  AdaptiveColor("#F0F0F0", "#0D0D0D"), // Cockpit interior
		Border:      AdaptiveColor("#B0B0B0", "#3A3A3A"), // Panel edge
		MutedBorder: AdaptiveColor("#D0D0D0", "#222222"), // Subtle divider
		System:      AdaptiveColor("#CC6600", "#FF8800"), // Amber system status
		Tool:        AdaptiveColor("#CC6600", "#FF8800"), // Amber instrument
		Accent:      AdaptiveColor("#DD2222", "#FF4444"), // Secondary scanner glow
		Highlight:   AdaptiveColor("#FFF0F0", "#1A1010"), // Red-tinted mantle

		// Diff backgrounds
		DiffInsertBg:  AdaptiveColor("#F0E8D0", "#2A2410"), // Warm amber tint (added)
		DiffDeleteBg:  AdaptiveColor("#F5D5D5", "#2E1A1A"), // Red tint (removed)
		DiffEqualBg:   AdaptiveColor("#E8E8E8", "#161616"), // Neutral
		DiffMissingBg: AdaptiveColor("#E0E0E0", "#111111"), // Darker neutral

		// Code & output backgrounds
		CodeBg:   AdaptiveColor("#E8E8E8", "#161616"), // Matches DiffEqualBg
		GutterBg: AdaptiveColor("#E0E0E0", "#111111"), // Slightly darker
		WriteBg:  AdaptiveColor("#F0E8D0", "#2A2410"), // Warm amber tint

		// Composer surface — one step off the terminal background.
		InputBg: AdaptiveColor("#E6E6E6", "#141414"),

		// Markdown & syntax highlighting — all warm tones
		Markdown: MarkdownThemeColors{
			Text:    AdaptiveColor("#1A1A1A", "#E0E0E0"), // Console text
			Muted:   AdaptiveColor("#707070", "#808080"), // Dimmed readout
			Heading: AdaptiveColor("#CC1100", "#FF2200"), // Scanner red, matching Primary
			Emph:    AdaptiveColor("#CC8800", "#FFB800"), // Amber emphasis
			Strong:  AdaptiveColor("#1A1A1A", "#E0E0E0"), // Bright text
			Link:    AdaptiveColor("#CC4400", "#FF7744"), // Warm orange link
			Code:    AdaptiveColor("#333333", "#CCCCCC"), // Inline code
			Error:   AdaptiveColor("#C21038", "#FF4466"), // Crimson, matching Theme.Error
			Keyword: AdaptiveColor("#CC3300", "#FF6644"), // Orange-red keyword
			String:  AdaptiveColor("#BB7700", "#DDAA33"), // Amber string
			Number:  AdaptiveColor("#CC8800", "#FFB800"), // Amber number
			Comment: AdaptiveColor("#909090", "#606060"), // Dark gray comment
		},
	}
}

// IsDarkBackground returns the cached terminal background detection result.
func IsDarkBackground() bool {
	return isDarkBg
}

// CreateBadge generates a styled badge or label with inverted colors (text on
// colored background) for highlighting important tags, statuses, or categories.
func CreateBadge(text string, c color.Color) string {
	return lipgloss.NewStyle().
		Foreground(AdaptiveColor("#FFFFFF", "#000000")).
		Background(c).
		Padding(0, 1).
		Bold(true).
		Render(text)
}

// interpolateColor blends between two colors based on position (0.0 to 1.0)
// using linear RGB channel interpolation.
func interpolateColor(a, b color.Color, pos float64) color.Color {
	r1, g1, b1, _ := a.RGBA()
	r2, g2, b2, _ := b.RGBA()

	r := uint8(float64(r1>>8)*(1-pos) + float64(r2>>8)*pos)
	g := uint8(float64(g1>>8)*(1-pos) + float64(g2>>8)*pos)
	bl := uint8(float64(b1>>8)*(1-pos) + float64(b2>>8)*pos)

	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, bl))
}

// ApplyGradient applies a color gradient from colorA to colorB across the text.
// Uses ~8 color stops for performance rather than per-character coloring.
func ApplyGradient(text string, colorA, colorB color.Color) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}

	const maxStops = 8
	segmentSize := max(len(runes)/maxStops, 1)

	var result strings.Builder
	for i := 0; i < len(runes); i += segmentSize {
		end := min(i+segmentSize, len(runes))

		pos := float64(i) / float64(len(runes))
		c := interpolateColor(colorA, colorB, pos)
		style := lipgloss.NewStyle().Foreground(c)
		result.WriteString(style.Render(string(runes[i:end])))
	}

	return result.String()
}

// kitLogoArt is the KIT wordmark in block letters. Every line is exactly
// kitLogoWidth columns wide, so the scanner bar beneath it lines up exactly
// and the whole block can be placed at any left offset without re-centering.
var kitLogoArt = []string{
	"██╗  ██╗ ██╗ ████████╗",
	"██║ ██╔╝ ██║ ╚══██╔══╝",
	"█████╔╝  ██║    ██║",
	"██╔═██╗  ██║    ██║",
	"██║  ██╗ ██║    ██║",
	"╚═╝  ╚═╝ ╚═╝    ╚═╝",
	// KITT's scanner bar, sized to match the wordmark above it.
	"░░ ▒▒ ▓▓ ████ ▓▓ ▒▒ ░░",
}

// kitLogoWidth is the column width of every line in kitLogoArt.
const kitLogoWidth = 22

// KitLogoLines returns the KIT wordmark as gradient-colored lines, sized for a
// block that will be rendered at the given content width.
//
// Fixed-width block art cannot wrap, so below the width it needs the wordmark
// degrades to a plain bold "KIT" rather than spilling across the terminal.
func KitLogoLines(contentWidth int) []string {
	theme := GetTheme()
	if contentWidth < kitLogoWidth {
		return []string{lipgloss.NewStyle().Bold(true).Foreground(theme.Primary).Render("KIT")}
	}

	out := make([]string, 0, len(kitLogoArt))
	for _, line := range kitLogoArt {
		out = append(out, ApplyGradient(line, theme.Primary, theme.Accent))
	}
	return out
}

// KitBanner returns the KIT ASCII art title with KITT scanner lights,
// rendered with a KITT red gradient.
//
// This is the banner used in `--help` output, where the art stands alone and
// is centered by its own leading padding. The TUI splash uses KitLogoLines
// instead, which is unpadded so it can sit inside a gutter-marked block.
func KitBanner() string {
	kittDark := lipgloss.Color("#8B0000")
	kittBright := lipgloss.Color("#FF2200")
	lines := []string{
		"             ██╗  ██╗ ██╗ ████████╗",
		"             ██║ ██╔╝ ██║ ╚══██╔══╝",
		"             █████╔╝  ██║    ██║",
		"             ██╔═██╗  ██║    ██║",
		"             ██║  ██╗ ██║    ██║",
		"             ╚═╝  ╚═╝ ╚═╝    ╚═╝",
		"░░ ░░ ░░ ▒▒ ▒▒ ▓▓ ▓▓ ████ ▓▓ ▓▓ ▒▒ ▒▒ ░░ ░░ ░░",
	}

	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(ApplyGradient(line, kittDark, kittBright))
	}
	return result.String()
}

// --------------------------------------------------------------------------
// Gutter
// --------------------------------------------------------------------------

// GutterGlyph is the single character used to mark a block's left edge
// throughout the UI: user messages, alerts, permission prompts and any other
// element that needs to be visually attributed.
//
// One glyph, colored by role, replaces what used to be three competing
// conventions (a thick ┃ for user messages, a light │ for alerts, and bare
// indentation for tool bodies). A half-block reads as a deliberate stripe at
// any font weight, where box-drawing characters vary between terminals and
// invite the eye to look for a matching corner that never comes.
const GutterGlyph = "▌"

// ContentOffset is the column at which every block's text begins.
//
// The UI keeps one left-edge contract: a marker (gutter glyph, tool bullet,
// receipt check) occupies column 0, a single space follows, and text starts at
// ContentOffset. Blocks with no marker — assistant prose — are indented to the
// same column, so the transcript reads as one aligned text column with markers
// hanging in the left margin. Anything that draws a left edge must honour this,
// or the eye sees a ragged margin and reads it as misalignment rather than as
// structure.
const ContentOffset = 2

// GutterBorder returns a lipgloss border consisting only of a left edge drawn
// with GutterGlyph. Callers enable just the left side; the remaining fields
// exist because lipgloss requires a complete Border value.
func GutterBorder() lipgloss.Border {
	return lipgloss.Border{
		Left:        GutterGlyph,
		Right:       "",
		Top:         "",
		Bottom:      "",
		TopLeft:     "",
		TopRight:    "",
		BottomLeft:  "",
		BottomRight: "",
	}
}

// Gutter prefixes every line of s with a colored gutter glyph and a single
// space. Unlike a lipgloss border it does not re-wrap or re-measure the
// content, so it is safe to apply to text that is already laid out.
func Gutter(s string, c color.Color) string {
	bar := lipgloss.NewStyle().Foreground(c).Render(GutterGlyph)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = bar + " " + line
	}
	return strings.Join(lines, "\n")
}

// --------------------------------------------------------------------------
// Splash
// --------------------------------------------------------------------------

// SplashBar renders a gradient stripe down the left of a block of content
// lines, in the manner of a magazine pull-quote:
//
//	█   KIT
//	█   anthropic · claude-opus-5
//	█
//	█   context   ~/project/AGENTS.md
//	█   skills    btca-cli, kit-extensions
//
// The stripe scales to the number of content lines, so the banner costs
// exactly as many rows as it has something to say — unlike block-letter ASCII
// art, whose height is fixed no matter how little information accompanies it.
// It also adapts to narrow terminals, where wide ASCII art simply wraps.
// SplashGutterWidth is the number of columns SplashBar consumes to the left of
// its content. It matches ContentOffset so the splash stripe lines up with
// every other gutter in the UI.
const SplashGutterWidth = ContentOffset

func SplashBar(lines []string, from, to color.Color) string {
	if len(lines) == 0 {
		return ""
	}

	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		pos := 0.0
		if len(lines) > 1 {
			pos = float64(i) / float64(len(lines)-1)
		}
		bar := lipgloss.NewStyle().
			Foreground(interpolateColor(from, to, pos)).
			Render("█")
		b.WriteString(bar + strings.Repeat(" ", ContentOffset-1) + line)
	}
	return b.String()
}

// --------------------------------------------------------------------------
// Syntax highlighting
// --------------------------------------------------------------------------

// syntaxStyleCache holds the chroma style derived from the active theme.
// Building a chroma style allocates a map of token entries, so it must not
// happen inside a per-frame render path. Invalidated by SetTheme.
var syntaxStyleCache *chroma.Style

// hexOf renders a color as a chroma-compatible "#rrggbb" string.
func hexOf(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// SyntaxStyle returns a chroma style built from the active theme's markdown
// colors.
//
// Syntax highlighting used to be pinned to an external palette, which meant a
// warm red/amber theme rendered code in blues, purples and greens — a second,
// unrelated color scheme living inside the first. Deriving the style from the
// theme keeps code blocks in the same family as everything around them, and
// makes every user theme (and every custom theme file) apply to code as well.
//
// Token backgrounds are left unset so the containing lipgloss style controls
// the fill; setting both produces visible seams at each token boundary.
func SyntaxStyle() *chroma.Style {
	if syntaxStyleCache != nil {
		return syntaxStyleCache
	}

	md := GetTheme().Markdown
	keyword := hexOf(md.Keyword)
	str := hexOf(md.String)
	number := hexOf(md.Number)
	comment := hexOf(md.Comment)
	text := hexOf(md.Text)
	name := hexOf(md.Link)

	builder := chroma.NewStyleBuilder("kit-theme")
	builder.Add(chroma.Text, text)
	builder.Add(chroma.Keyword, keyword+" bold")
	builder.Add(chroma.KeywordType, keyword)
	builder.Add(chroma.NameBuiltin, keyword)
	builder.Add(chroma.NameClass, name+" bold")
	builder.Add(chroma.NameFunction, name)
	builder.Add(chroma.LiteralString, str)
	builder.Add(chroma.LiteralNumber, number)
	builder.Add(chroma.Comment, comment+" italic")
	builder.Add(chroma.CommentPreproc, keyword)
	builder.Add(chroma.Operator, text)
	builder.Add(chroma.Punctuation, text)
	builder.Add(chroma.GenericError, hexOf(GetTheme().Error))

	built, err := builder.Build()
	if err != nil {
		// A malformed entry should degrade to no highlighting rather than
		// take down a render.
		built = styles.Fallback
	}
	syntaxStyleCache = built
	return built
}
