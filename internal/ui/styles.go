package ui

import (
	"charm.land/lipgloss/v2"
	"github.com/indaco/herald"
	heraldmd "github.com/indaco/herald-md"
)

// BaseStyle returns a new, empty lipgloss style that can be customized with
// additional styling methods. This serves as the foundation for building more
// complex styled components.
func BaseStyle() lipgloss.Style {
	return lipgloss.NewStyle()
}

// markdownTypographyCache holds the last-created Typography instance for
// herald-md rendering. It is cached to avoid re-initialization on every
// streaming flush tick. The cache is invalidated by SetTheme when the
// active theme changes.
// This is only accessed from BubbleTea's single-threaded Update/View cycle,
// so no mutex is required.
var markdownTypographyCache *herald.Typography

// GetMarkdownTypography returns a herald.Typography configured with our
// active theme colors. The typography is cached and only rebuilt when
// the theme changes via SetTheme.
func GetMarkdownTypography() *herald.Typography {
	if markdownTypographyCache != nil {
		return markdownTypographyCache
	}

	theme := GetTheme()
	md := theme.Markdown

	// Build herald theme from our theme colors
	hty := herald.Theme{
		// Headings - use heading color
		H1: lipgloss.NewStyle().Foreground(md.Heading).Bold(true),
		H2: lipgloss.NewStyle().Foreground(md.Heading).Bold(true),
		H3: lipgloss.NewStyle().Foreground(md.Heading).Bold(true),
		H4: lipgloss.NewStyle().Foreground(md.Heading).Bold(true),
		H5: lipgloss.NewStyle().Foreground(md.Heading).Bold(true),
		H6: lipgloss.NewStyle().Foreground(md.Muted).Bold(true),

		// Text blocks
		Paragraph:  lipgloss.NewStyle().Foreground(md.Text),
		Blockquote: lipgloss.NewStyle().Foreground(md.Muted).Italic(true),
		CodeInline: lipgloss.NewStyle().Foreground(md.Code),
		CodeBlock:  lipgloss.NewStyle().Foreground(md.Code),
		HR:         lipgloss.NewStyle().Foreground(md.Muted),

		// Lists
		ListBullet: lipgloss.NewStyle().Foreground(md.Text),
		ListItem:   lipgloss.NewStyle().Foreground(md.Text),

		// Inline styles
		Bold:          lipgloss.NewStyle().Foreground(md.Strong).Bold(true),
		Italic:        lipgloss.NewStyle().Foreground(md.Emph).Italic(true),
		Strikethrough: lipgloss.NewStyle().Foreground(md.Muted).Strikethrough(true),
		Link:          lipgloss.NewStyle().Foreground(md.Link).Underline(true),

		// Definition lists
		DT: lipgloss.NewStyle().Foreground(md.Text).Bold(true),
		DD: lipgloss.NewStyle().Foreground(md.Muted),

		// Key-value
		KVKey:   lipgloss.NewStyle().Foreground(md.Text).Bold(true),
		KVValue: lipgloss.NewStyle().Foreground(md.Text),

		// Badges/Tags - use semantic colors
		Badge:        lipgloss.NewStyle().Foreground(md.Text).Bold(true),
		SuccessBadge: lipgloss.NewStyle().Foreground(theme.Success).Bold(true),
		WarningBadge: lipgloss.NewStyle().Foreground(theme.Warning).Bold(true),
		ErrorBadge:   lipgloss.NewStyle().Foreground(theme.Error).Bold(true),
		InfoBadge:    lipgloss.NewStyle().Foreground(theme.Info).Bold(true),

		// Heading decorations
		H1UnderlineChar: "═",
		H2UnderlineChar: "─",
		H3UnderlineChar: "·",
	}

	ty := herald.New(herald.WithTheme(hty))
	markdownTypographyCache = ty
	return ty
}

// toMarkdown renders markdown content using herald-md.
// The width parameter is currently unused as herald handles wrapping
// based on terminal width internally.
func toMarkdown(content string, width int) string {
	ty := GetMarkdownTypography()
	rendered := heraldmd.Render(ty, []byte(content))
	return rendered
}
