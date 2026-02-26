package ui

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

// colorHex returns the hex string representation of a color.Color by
// converting its RGBA values.
func colorHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

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
func SetTheme(theme Theme) {
	currentTheme = theme
}

// Theme defines a comprehensive color scheme for the application's UI, supporting
// both light and dark terminal modes through adaptive colors. It includes semantic
// colors for different message types and UI elements, based on the Catppuccin color palette.
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
}

// DefaultTheme creates and returns the default KIT theme based on the Catppuccin
// Mocha (dark) and Latte (light) color palettes. This theme provides a cohesive,
// pleasant visual experience with carefully selected colors for different UI elements.
func DefaultTheme() Theme {
	return Theme{
		Primary:     AdaptiveColor("#8839ef", "#cba6f7"), // Latte/Mocha Mauve
		Secondary:   AdaptiveColor("#04a5e5", "#89dceb"), // Latte/Mocha Sky
		Success:     AdaptiveColor("#40a02b", "#a6e3a1"), // Latte/Mocha Green
		Warning:     AdaptiveColor("#df8e1d", "#f9e2af"), // Latte/Mocha Yellow
		Error:       AdaptiveColor("#d20f39", "#f38ba8"), // Latte/Mocha Red
		Info:        AdaptiveColor("#1e66f5", "#89b4fa"), // Latte/Mocha Blue
		Text:        AdaptiveColor("#4c4f69", "#cdd6f4"), // Latte/Mocha Text
		Muted:       AdaptiveColor("#6c6f85", "#a6adc8"), // Latte/Mocha Subtext 0
		VeryMuted:   AdaptiveColor("#9ca0b0", "#6c7086"), // Latte/Mocha Overlay 0
		Background:  AdaptiveColor("#eff1f5", "#1e1e2e"), // Latte/Mocha Base
		Border:      AdaptiveColor("#acb0be", "#585b70"), // Latte/Mocha Surface 2
		MutedBorder: AdaptiveColor("#ccd0da", "#313244"), // Latte/Mocha Surface 0
		System:      AdaptiveColor("#179299", "#94e2d5"), // Latte/Mocha Teal
		Tool:        AdaptiveColor("#fe640b", "#fab387"), // Latte/Mocha Peach
		Accent:      AdaptiveColor("#ea76cb", "#f5c2e7"), // Latte/Mocha Pink
		Highlight:   AdaptiveColor("#e6e9ef", "#181825"), // Latte Mantle / Mocha Mantle
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
	segmentSize := len(runes) / maxStops
	if segmentSize < 1 {
		segmentSize = 1
	}

	var result strings.Builder
	for i := 0; i < len(runes); i += segmentSize {
		end := i + segmentSize
		if end > len(runes) {
			end = len(runes)
		}

		pos := float64(i) / float64(len(runes))
		c := interpolateColor(colorA, colorB, pos)
		style := lipgloss.NewStyle().Foreground(c)
		result.WriteString(style.Render(string(runes[i:end])))
	}

	return result.String()
}

// CreateGradientText creates styled text with a gradient effect between two colors.
func CreateGradientText(text string, startColor, endColor color.Color) string {
	return ApplyGradient(text, startColor, endColor)
}

// Compact styling utilities

// StyleCompactSymbol creates a lipgloss style for message type indicators in
// compact mode, using bold colored text to distinguish different message categories.
func StyleCompactSymbol(symbol string, c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(c).
		Bold(true)
}

// StyleCompactLabel creates a lipgloss style for message labels in compact mode
// with fixed width for alignment and bold colored text for readability.
func StyleCompactLabel(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(c).
		Bold(true).
		Width(8)
}

// StyleCompactContent creates a simple lipgloss style for message content in
// compact mode, applying only color without additional formatting.
func StyleCompactContent(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(c)
}

// FormatCompactLine assembles a complete compact mode message line with consistent
// spacing and styling. Combines a symbol, fixed-width label, and content with their
// respective colors to create a uniform appearance across all message types.
func FormatCompactLine(symbol, label, content string, symbolColor, labelColor, contentColor color.Color) string {
	styledSymbol := StyleCompactSymbol(symbol, symbolColor).Render(symbol)
	styledLabel := StyleCompactLabel(labelColor).Render(label)
	styledContent := StyleCompactContent(contentColor).Render(content)

	return fmt.Sprintf("%s  %-8s %s", styledSymbol, styledLabel, styledContent)
}
