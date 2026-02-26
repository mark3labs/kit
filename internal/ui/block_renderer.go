package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// blockRenderer handles rendering of content blocks with configurable options
type blockRenderer struct {
	align         *lipgloss.Position
	borderColor   *color.Color
	bgColor       *color.Color
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

// WithFullWidth returns a renderingOption that configures the block renderer
// to expand to the full available width of its container. When enabled, the
// block will fill the entire horizontal space rather than sizing to its content.
func WithFullWidth() renderingOption {
	return func(c *blockRenderer) {
		c.fullWidth = true
	}
}

// WithBackground returns a renderingOption that sets a background color
// for the entire block.
func WithBackground(c color.Color) renderingOption {
	return func(br *blockRenderer) {
		br.bgColor = &c
	}
}

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

// WithMarginTop returns a renderingOption that sets the top margin
// for the block. The margin is specified in number of lines and adds
// vertical space above the block.
func WithMarginTop(margin int) renderingOption {
	return func(c *blockRenderer) {
		c.marginTop = margin
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

// WithPaddingLeft returns a renderingOption that sets the left padding
// for the block content. The padding is specified in number of characters
// and adds horizontal space between the left border and the content.
func WithPaddingLeft(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingLeft = padding
	}
}

// WithPaddingRight returns a renderingOption that sets the right padding
// for the block content. The padding is specified in number of characters
// and adds horizontal space between the content and the right border.
func WithPaddingRight(padding int) renderingOption {
	return func(c *blockRenderer) {
		c.paddingRight = padding
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

// WithWidth returns a renderingOption that sets a specific width for the block
// in characters. This overrides the default container width and allows precise
// control over the block's horizontal dimensions.
func WithWidth(width int) renderingOption {
	return func(c *blockRenderer) {
		c.width = width
	}
}

// renderContentBlock renders content with configurable styling options
func renderContentBlock(content string, containerWidth int, options ...renderingOption) string {
	renderer := &blockRenderer{
		fullWidth:     true,
		paddingTop:    1,
		paddingBottom: 1,
		paddingLeft:   2,
		paddingRight:  0,
		width:         containerWidth,
	}

	for _, option := range options {
		option(renderer)
	}

	// Embed vertical padding as content newlines rather than style
	// PaddingTop/PaddingBottom — lipgloss adds those as raw newlines
	// that don't receive the background color, causing visible banding.
	for range renderer.paddingTop {
		content = "\n" + content
	}
	for range renderer.paddingBottom {
		content = content + "\n"
	}

	theme := GetTheme()
	style := lipgloss.NewStyle().
		PaddingLeft(renderer.paddingLeft).
		PaddingRight(renderer.paddingRight).
		Foreground(theme.Text)

	if renderer.bgColor != nil {
		style = style.Background(*renderer.bgColor)
	}

	// Border width used for full-width calculation.
	borderChars := 0

	if renderer.noBorder {
		// No borders — just padding.
	} else {
		style = style.BorderStyle(lipgloss.ThickBorder())

		align := lipgloss.Left
		if renderer.align != nil {
			align = *renderer.align
		}

		// Default to transparent/no border color
		var borderColor color.Color = lipgloss.NoColor{}
		if renderer.borderColor != nil {
			borderColor = *renderer.borderColor
		}

		// Only render the accent-side border to avoid background
		// banding from the opposite border character.
		switch align {
		case lipgloss.Right:
			style = style.
				BorderRight(true).
				BorderRightForeground(borderColor)
			borderChars = 1
		default: // Left (and fallback)
			style = style.
				BorderLeft(true).
				BorderLeftForeground(borderColor)
			borderChars = 1
		}
	}

	if renderer.fullWidth {
		// Subtract border characters so the total rendered width
		// equals containerWidth exactly.
		style = style.Width(renderer.width - borderChars)
	}

	content = style.Render(content)

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
