package style

import (
	"fmt"
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
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

	// Markdown rendering and syntax highlighting colors
	Markdown MarkdownThemeColors
}

// DefaultTheme creates and returns the default KIT theme inspired by the
// Knight Rider KITT aesthetic — scanner reds, amber dashboard glows, and a
// dark cockpit. No blues or bright greens; everything stays in the warm
// red/amber/gray family of KITT's instrument panel.
func DefaultTheme() Theme {
	return Theme{
		Primary:     AdaptiveColor("#CC1100", "#FF2200"), // KITT scanner red
		Secondary:   AdaptiveColor("#CC6600", "#FF8800"), // Amber dashboard glow
		Success:     AdaptiveColor("#998800", "#CCAA00"), // Warm gold — system OK
		Warning:     AdaptiveColor("#CC8800", "#FFB800"), // Amber caution light
		Error:       AdaptiveColor("#CC0000", "#FF3333"), // Alert red
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

		// Markdown & syntax highlighting — all warm tones
		Markdown: MarkdownThemeColors{
			Text:    AdaptiveColor("#1A1A1A", "#E0E0E0"), // Console text
			Muted:   AdaptiveColor("#707070", "#808080"), // Dimmed readout
			Heading: AdaptiveColor("#CC1100", "#FF4444"), // Scanner red accent
			Emph:    AdaptiveColor("#CC8800", "#FFB800"), // Amber emphasis
			Strong:  AdaptiveColor("#1A1A1A", "#E0E0E0"), // Bright text
			Link:    AdaptiveColor("#CC4400", "#FF7744"), // Warm orange link
			Code:    AdaptiveColor("#333333", "#CCCCCC"), // Inline code
			Error:   AdaptiveColor("#CC0000", "#FF3333"), // Alert red
			Keyword: AdaptiveColor("#CC3300", "#FF6644"), // Orange-red keyword
			String:  AdaptiveColor("#BB7700", "#DDAA33"), // Amber string
			Number:  AdaptiveColor("#CC8800", "#FFB800"), // Amber number
			Comment: AdaptiveColor("#909090", "#606060"), // Dark gray comment
		},
	}
}

// StyleCard creates a lipgloss style for card-like containers with rounded borders,
// padding, and appropriate width. Used for grouping related content in a visually
// distinct box.
func StyleCard(width int, theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(1, 2).
		MarginBottom(1)
}

// IsDarkBackground returns the cached terminal background detection result.
func IsDarkBackground() bool {
	return isDarkBg
}

// StyleHeader creates a lipgloss style for primary headers using the theme's
// primary color with bold text for emphasis and hierarchy.
func StyleHeader(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)
}

// StyleSubheader creates a lipgloss style for secondary headers using the theme's
// secondary color with bold text, providing visual hierarchy below primary headers.
func StyleSubheader(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Secondary).
		Bold(true)
}

// StyleMuted creates a lipgloss style for de-emphasized text using muted colors
// and italic formatting, suitable for supplementary or less important information.
func StyleMuted(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Muted).
		Italic(true)
}

// StyleSuccess creates a lipgloss style for success messages using green colors
// with bold text to indicate successful operations or positive outcomes.
func StyleSuccess(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Success).
		Bold(true)
}

// StyleError creates a lipgloss style for error messages using red colors
// with bold text to ensure visibility of problems or failures.
func StyleError(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Error).
		Bold(true)
}

// StyleWarning creates a lipgloss style for warning messages using yellow/amber
// colors with bold text to draw attention to potential issues or cautions.
func StyleWarning(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Warning).
		Bold(true)
}

// StyleInfo creates a lipgloss style for informational messages using blue colors
// with bold text for general notifications and status updates.
func StyleInfo(theme Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Info).
		Bold(true)
}

// CreateSeparator generates a horizontal separator line with the specified width,
// character, and color. Useful for visually dividing sections of content in the UI.
func CreateSeparator(width int, char string, c color.Color) string {
	return lipgloss.NewStyle().
		Foreground(c).
		Width(width).
		Render(lipgloss.PlaceHorizontal(width, lipgloss.Center, char))
}

// CreateProgressBar generates a visual progress bar with filled and empty segments
// based on the percentage complete. The bar uses Unicode block characters for smooth
// appearance and theme colors to indicate progress.
func CreateProgressBar(width int, percentage float64, theme Theme) string {
	filled := int(float64(width) * percentage / 100)
	empty := width - filled

	filledBar := lipgloss.NewStyle().
		Foreground(theme.Success).
		Render(lipgloss.PlaceHorizontal(filled, lipgloss.Left, "█"))

	emptyBar := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Render(lipgloss.PlaceHorizontal(empty, lipgloss.Left, "░"))

	return filledBar + emptyBar
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

// KitBanner returns the KIT ASCII art title with KITT scanner lights,
// rendered with a KITT red gradient.
func KitBanner() string {
	kittDark := lipgloss.Color("#8B0000")
	kittBright := lipgloss.Color("#FF2200")
	lines := []string{
		"            ██╗  ██╗ ██╗ ████████╗",
		"            ██║ ██╔╝ ██║ ╚══██╔══╝",
		"            █████╔╝  ██║    ██║",
		"            ██╔═██╗  ██║    ██║",
		"            ██║  ██╗ ██║    ██║",
		"            ╚═╝  ╚═╝ ╚═╝    ╚═╝",
		" ░░░░░░▒▒▒▒▒▓▓▓▓███████████████▓▓▓▓▒▒▒▒▒░░░░░░",
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
