package style

import (
	"charm.land/lipgloss/v2"
	"github.com/indaco/herald"
	heraldmd "github.com/indaco/herald-md"
)

// markdownTypographyCache holds the last-created Typography instance for
// herald-md rendering. It is cached to avoid re-initialization on every
// streaming flush tick. The cache is invalidated by SetTheme when the
// active theme changes.
// This is only accessed from BubbleTea's single-threaded Update/View cycle,
// so no mutex is required.
var markdownTypographyCache *herald.Typography

// uiTypographyCache holds the last-created Typography instance for message
// block rendering (reasoning blocks, system notes, etc.). Constructing a
// herald.Typography is expensive (dozens of lipgloss styles), so it must
// never happen inside a per-frame render path. Invalidated by SetTheme.
// Only accessed from BubbleTea's single-threaded Update/View cycle.
var uiTypographyCache *herald.Typography

// uiTypographyOptions returns the herald options that define KIT's block
// vocabulary for the given theme.
//
// This exists so there is exactly one description of what a KIT alert looks
// like. CustomBlock used to build its own Typography in order to override a
// single alert label, and in copying the palette it silently dropped
// WithAlertBar — so `/help` drew a `│` bar while every other alert drew `▌`.
// Any caller needing a variant starts from these options and appends.
func uiTypographyOptions(theme Theme) []herald.Option {
	return []herald.Option{
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
		herald.WithAlertPalette(herald.AlertPalette{
			// theme.System colors system notices, so a theme can distinguish
			// the agent talking about itself from ordinary informational text.
			Note:      theme.System,
			Tip:       theme.Success,
			Important: theme.Accent,
			Warning:   theme.Warning,
			Caution:   theme.Error,
		}),
		herald.WithCodeLineNumbers(true),
		// Alerts use the same gutter glyph as every other attributed block, so
		// the UI speaks one visual language instead of mixing bar weights.
		herald.WithAlertBar(GutterGlyph),
		// Customize alert labels
		herald.WithAlertLabel(herald.AlertNote, "Info"),
		herald.WithAlertLabel(herald.AlertTip, ""),
		herald.WithAlertIcon(herald.AlertTip, ""),
		herald.WithAlertLabel(herald.AlertWarning, "Working"),
		herald.WithAlertLabel(herald.AlertCaution, "Error"),
	}
}

// GetUITypography returns the shared herald.Typography used for message
// block rendering, configured from the active theme. The instance is cached
// and only rebuilt after a theme change (SetTheme invalidates it).
func GetUITypography() *herald.Typography {
	if uiTypographyCache != nil {
		return uiTypographyCache
	}

	ty := herald.New(uiTypographyOptions(GetTheme())...)
	uiTypographyCache = ty
	return ty
}

// NewNoteTypography returns a Typography identical to the shared UI instance
// but with the Note alert relabelled — for one-off blocks that want a title
// like "Help" or "Warning" without changing the default "Info".
//
// The result is not cached: callers are rare (slash-command output), and
// caching per label would keep a Typography alive for every label ever used.
func NewNoteTypography(label string) *herald.Typography {
	opts := append(uiTypographyOptions(GetTheme()), herald.WithAlertLabel(herald.AlertNote, label))
	return herald.New(opts...)
}

// GetMarkdownTypography returns a herald.Typography configured with our
// active theme colors. The typography is cached and only rebuilt when
// the theme changes via SetTheme.
//
// The theme is assembled from a palette plus per-element overrides rather than
// from a herald.Theme literal. WithTheme *replaces* the theme wholesale, so a
// literal leaves every field it does not mention at its zero value — and many
// of herald's fields are glyphs and widths, not colors. A literal therefore
// silently erased the list bullet char, the nested-list bullets and indent, the
// blockquote bar, the heading bar, the horizontal-rule char and width, and the
// entire table border set: `- item` rendered as a bare indented line, `---`
// rendered as nothing at all, and a table collapsed to `a│b`. Seeding from the
// palette keeps herald's defaults for every token and spends the overrides
// only where the color has to be ours.
func GetMarkdownTypography() *herald.Typography {
	if markdownTypographyCache != nil {
		return markdownTypographyCache
	}

	theme := GetTheme()
	md := theme.Markdown

	ty := herald.New(
		// Palette first: it supplies a complete theme, including the glyph and
		// width tokens none of the overrides below touch.
		herald.WithPalette(herald.ColorPalette{
			Primary:   md.Heading,
			Secondary: md.Heading,
			Tertiary:  md.Link,
			Accent:    theme.Accent,
			Highlight: theme.Highlight,
			Muted:     md.Muted,
			Text:      md.Text,
			Surface:   theme.Background,
			Base:      theme.CodeBg,
		}),

		// Headings all take the heading color; the palette would otherwise
		// spread them across five different hues.
		herald.WithH1Style(lipgloss.NewStyle().Foreground(md.Heading).Bold(true)),
		herald.WithH2Style(lipgloss.NewStyle().Foreground(md.Heading).Bold(true)),
		herald.WithH3Style(lipgloss.NewStyle().Foreground(md.Heading).Bold(true)),
		herald.WithH4Style(lipgloss.NewStyle().Foreground(md.Heading).Bold(true)),
		herald.WithH5Style(lipgloss.NewStyle().Foreground(md.Heading).Bold(true)),
		herald.WithH6Style(lipgloss.NewStyle().Foreground(md.Muted).Bold(true)),

		// Body text carries no explicit foreground so it inherits whatever the
		// user's terminal uses for normal text. Pinning it to a theme color
		// overrides a deliberate color scheme and can land a near-invisible
		// gray on an unusual background. Color is spent only where it carries
		// meaning: headings, links, emphasis and semantic badges.
		//
		// The bottom margin the palette puts under every paragraph goes too: in
		// a scrollback, a blank line after each paragraph doubles the height of
		// ordinary prose.
		herald.WithParagraphStyle(lipgloss.NewStyle()),
		herald.WithBlockquoteStyle(lipgloss.NewStyle().Foreground(md.Muted).Italic(true)),
		herald.WithCodeInlineStyle(lipgloss.NewStyle().Foreground(md.Code)),
		herald.WithHRStyle(lipgloss.NewStyle().Foreground(md.Muted)),

		// Code blocks stay background-free. The transcript already uses filled
		// panels for tool output, and giving assistant prose a second kind of
		// surface makes the two compete; highlighting carries the distinction
		// instead. The foreground is the fallback for an unlabelled fence,
		// where there is no language to pick a lexer from.
		herald.WithCodeBlockStyle(lipgloss.NewStyle().Foreground(md.Code)),

		herald.WithListBulletStyle(lipgloss.NewStyle().Foreground(md.Muted)),
		herald.WithListItemStyle(lipgloss.NewStyle()),

		// Inline styles
		herald.WithBoldStyle(lipgloss.NewStyle().Bold(true)),
		herald.WithItalicStyle(lipgloss.NewStyle().Foreground(md.Emph).Italic(true)),
		herald.WithStrikethroughStyle(lipgloss.NewStyle().Foreground(md.Muted).Strikethrough(true)),
		herald.WithLinkStyle(lipgloss.NewStyle().Foreground(md.Link).Underline(true)),

		// Definition lists
		herald.WithDTStyle(lipgloss.NewStyle().Bold(true)),
		herald.WithDDStyle(lipgloss.NewStyle().Foreground(md.Muted)),

		// Key-value
		herald.WithKVKeyStyle(lipgloss.NewStyle().Bold(true)),
		herald.WithKVValueStyle(lipgloss.NewStyle().Foreground(md.Muted)),

		// Badges/Tags - use semantic colors
		herald.WithBadgeStyle(lipgloss.NewStyle().Bold(true)),
		herald.WithSuccessBadgeStyle(lipgloss.NewStyle().Foreground(theme.Success).Bold(true)),
		herald.WithWarningBadgeStyle(lipgloss.NewStyle().Foreground(theme.Warning).Bold(true)),
		herald.WithErrorBadgeStyle(lipgloss.NewStyle().Foreground(theme.Error).Bold(true)),
		herald.WithInfoBadgeStyle(lipgloss.NewStyle().Foreground(theme.Info).Bold(true)),

		// Tables borrow the muted frame color used by every other box in the
		// UI rather than the palette's heading-colored one.
		herald.WithTableHeaderStyle(lipgloss.NewStyle().Foreground(md.Heading).Bold(true)),
		herald.WithTableBorderStyle(lipgloss.NewStyle().Foreground(theme.Border)),

		// Heading decorations
		herald.WithH1UnderlineChar("═"),
		herald.WithH2UnderlineChar("─"),
		herald.WithH3UnderlineChar("·"),

		// Fenced code blocks get real syntax highlighting. herald calls this
		// only when the fence carries an info string, so an unlabelled block
		// falls through to the CodeBlock foreground above.
		herald.WithCodeFormatter(func(code, language string) string {
			return HighlightLang(code, language)
		}),
	)

	markdownTypographyCache = ty
	return ty
}

// ToMarkdown renders markdown content using herald-md and wraps the result
// to the given width so that long lines do not overflow the terminal.
func ToMarkdown(content string, width int) string {
	ty := GetMarkdownTypography()
	rendered := heraldmd.Render(ty, []byte(content))
	if width > 0 {
		rendered = lipgloss.Wrap(rendered, width, "")
	}
	return rendered
}
