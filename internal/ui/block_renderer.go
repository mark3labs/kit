package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/ui/style"
)

// blockRenderer handles rendering of content blocks with configurable options
type blockRenderer struct {
	align         *lipgloss.Position
	borderColor   *color.Color
	background    *color.Color
	foreground    *color.Color
	fullWidth     bool
	noBorder      bool
	paddingTop    int
	paddingBottom int
	paddingLeft   int
	paddingRight  int
	marginTop     int
	marginBottom  int
	width         int
}

// renderingOption configures block rendering
type renderingOption func(*blockRenderer)

// WithNoBorder returns a renderingOption that disables all borders on the
// block, rendering content with only padding.
func WithNoBorder() renderingOption {
	return func(c *blockRenderer) {
		c.noBorder = true
	}
}

// WithAlign returns a renderingOption that sets the horizontal alignment
// of the block content within its container. The align parameter accepts
// lipgloss.Left, lipgloss.Center, or lipgloss.Right positions.
func WithAlign(align lipgloss.Position) renderingOption {
	return func(c *blockRenderer) {
		c.align = &align
	}
}

// WithBorderColor returns a renderingOption that sets the border color
// for the block. The color parameter uses lipgloss.AdaptiveColor to support
// both light and dark terminal themes automatically.
func WithBorderColor(c color.Color) renderingOption {
	return func(br *blockRenderer) {
		br.borderColor = &c
	}
}

// WithBackground returns a renderingOption that sets a background color for
// the entire block, including the padded region.
func WithBackground(c color.Color) renderingOption {
	return func(r *blockRenderer) {
		r.background = &c
	}
}

// WithMarginBottom returns a renderingOption that sets the bottom margin
// for the block. The margin is specified in number of lines and adds
// vertical space below the block.
func WithMarginBottom(margin int) renderingOption {
	return func(c *blockRenderer) {
		c.marginBottom = margin
	}
}

// WithPaddingTop returns a renderingOption that sets the top padding
// for the block content. The padding is specified in number of lines
// and adds vertical space between the top border and the content.
func WithPaddingTop(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingTop = padding
	}
}

// WithPaddingBottom returns a renderingOption that sets the bottom padding
// for the block content. The padding is specified in number of lines
// and adds vertical space between the content and the bottom border.
func WithPaddingBottom(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingBottom = padding
	}
}

// renderContentBlock renders content with configurable styling options
func renderContentBlock(content string, containerWidth int, options ...renderingOption) string {
	renderer := &blockRenderer{
		fullWidth: true,
		// Blocks carry no interior vertical padding. Vertical rhythm in the
		// transcript comes from the single trailing gap every block appends
		// (style.BlockGap); padding inside the gutter as well opens a hole
		// above and below the text and makes the stripe look detached from
		// what it marks.
		paddingTop:    0,
		paddingBottom: 0,
		// The border glyph occupies column 0, so the padding that follows it
		// is one less than the shared content offset.
		paddingLeft:  style.ContentOffset - 1,
		paddingRight: style.RightMargin,
		width:        containerWidth,
	}

	for _, option := range options {
		option(renderer)
	}

	// Trailing blank lines inside the block would be drawn with the gutter
	// glyph beside them, which reads as an unfinished block rather than as
	// space. Command output usually ends with a newline, so this is the
	// common case, not the exceptional one. Vertical separation is the
	// margin's job.
	content = strings.TrimRight(content, "\n")

	// Resolve border configuration.
	hasBorder := !renderer.noBorder
	borderChars := 0

	borderAlign := lipgloss.Left
	if renderer.align != nil {
		borderAlign = *renderer.align
	}

	var borderColor color.Color = lipgloss.NoColor{}
	if renderer.borderColor != nil {
		borderColor = *renderer.borderColor
	}

	if hasBorder {
		borderChars = 1
	}

	theme := style.GetTheme()

	// Resolve foreground color: caller override or theme default.
	fgColor := theme.Text
	if renderer.foreground != nil {
		fgColor = *renderer.foreground
	}

	// Single-pass render: padding, border, and foreground in one style.
	blockStyle := lipgloss.NewStyle().
		PaddingLeft(renderer.paddingLeft).
		PaddingRight(renderer.paddingRight).
		PaddingTop(renderer.paddingTop).
		PaddingBottom(renderer.paddingBottom).
		Foreground(fgColor)

	if hasBorder {
		// One gutter glyph is used for every attributed block in the UI, so
		// the left edge reads as a single vocabulary rather than as several
		// competing border weights.
		blockStyle = blockStyle.BorderStyle(style.GutterBorder())

		switch borderAlign {
		case lipgloss.Right:
			blockStyle = blockStyle.
				BorderRight(true).
				BorderRightForeground(borderColor)
		default:
			blockStyle = blockStyle.
				BorderLeft(true).
				BorderLeftForeground(borderColor)
		}
	}

	if renderer.background != nil {
		blockStyle = blockStyle.Background(*renderer.background)
	}

	if renderer.fullWidth {
		blockStyle = blockStyle.Width(renderer.width - borderChars)
	}

	content = blockStyle.Render(content)

	// Add margins
	if renderer.marginTop > 0 {
		for range renderer.marginTop {
			content = "\n" + content
		}
	}
	if renderer.marginBottom > 0 {
		for range renderer.marginBottom {
			content = content + "\n"
		}
	}

	return content
}
