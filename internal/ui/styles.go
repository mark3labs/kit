package ui

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/spf13/viper"
)

const defaultMargin = 1

// Helper functions for style pointers
//
//go:fix inline
func boolPtr(b bool) *bool { return new(b) }

//go:fix inline
func stringPtr(s string) *string { return new(s) }

//go:fix inline
func uintPtr(u uint) *uint { return new(u) }

// BaseStyle returns a new, empty lipgloss style that can be customized with
// additional styling methods. This serves as the foundation for building more
// complex styled components.
func BaseStyle() lipgloss.Style {
	return lipgloss.NewStyle()
}

// GetMarkdownRenderer creates and returns a configured glamour.TermRenderer for
// rendering markdown content with syntax highlighting and proper formatting. The
// renderer is customized with our theme colors and adapted to the specified width.
func GetMarkdownRenderer(width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(generateMarkdownStyleConfig()),
		glamour.WithWordWrap(width),
	)
	return r
}

// generateMarkdownStyleConfig creates an ansi.StyleConfig for markdown rendering
func generateMarkdownStyleConfig() ansi.StyleConfig {

	var textColor, mutedColor string
	var headingColor, emphColor, strongColor, linkColor, codeColor, errorColor, keywordColor, stringColor, numberColor, commentColor string
	var mdTheme config.MarkdownTheme

	err := config.FilepathOr("markdown-theme", &mdTheme)
	fromConfig := err == nil && viper.InConfig("markdown-theme")
	if fromConfig && IsDarkBackground() {
		textColor = mdTheme.Text.Light
		mutedColor = mdTheme.Muted.Light
		headingColor = mdTheme.Heading.Light
		emphColor = mdTheme.Emph.Light
		strongColor = mdTheme.Strong.Light
		linkColor = mdTheme.Link.Light
		codeColor = mdTheme.Code.Light
		errorColor = mdTheme.Error.Light
		keywordColor = mdTheme.Keyword.Light
		stringColor = mdTheme.String.Light
		numberColor = mdTheme.Number.Light
		commentColor = mdTheme.Comment.Light
	} else if fromConfig {
		textColor = mdTheme.Text.Dark
		mutedColor = mdTheme.Muted.Dark
		headingColor = mdTheme.Heading.Dark
		emphColor = mdTheme.Emph.Dark
		strongColor = mdTheme.Strong.Dark
		linkColor = mdTheme.Link.Dark
		codeColor = mdTheme.Code.Dark
		errorColor = mdTheme.Error.Dark
		keywordColor = mdTheme.Keyword.Dark
		stringColor = mdTheme.String.Dark
		numberColor = mdTheme.Number.Dark
		commentColor = mdTheme.Comment.Dark
	} else if IsDarkBackground() {
		textColor = "#F9FAFB"  // Light text for dark backgrounds
		mutedColor = "#9CA3AF" // Light muted for dark backgrounds
		// Dark background colors
		headingColor = "#22D3EE" // Cyan
		emphColor = "#FDE047"    // Yellow
		strongColor = "#F9FAFB"  // Light gray
		linkColor = "#60A5FA"    // Blue
		codeColor = "#D1D5DB"    // Light gray
		errorColor = "#F87171"   // Red
		keywordColor = "#C084FC" // Purple
		stringColor = "#34D399"  // Green
		numberColor = "#FBBF24"  // Orange
		commentColor = "#9CA3AF" // Muted gray
	} else {
		textColor = "#1F2937"  // Dark text for light backgrounds
		mutedColor = "#6B7280" // Dark muted for light backgrounds
		// Light background colors
		headingColor = "#0891B2" // Dark cyan
		emphColor = "#D97706"    // Orange
		strongColor = "#1F2937"  // Dark gray
		linkColor = "#2563EB"    // Blue
		codeColor = "#374151"    // Dark gray
		errorColor = "#DC2626"   // Red
		keywordColor = "#7C3AED" // Purple
		stringColor = "#059669"  // Green
		numberColor = "#D97706"  // Orange
		commentColor = "#6B7280" // Muted gray
	}

	// Don't apply background in markdown - let the block renderer handle it
	bgColor := ""

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "",
				BlockSuffix: "",
				Color:       new(textColor),
			},
			Margin: uintPtr(0), // Remove margin to prevent spacing
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  new(mutedColor),
				Italic: new(true),
				Prefix: "┃ ",
			},
			Indent:      uintPtr(1),
			IndentToken: new(lipgloss.NewStyle().Background(lipgloss.Color(bgColor)).Render(" ")),
		},
		List: ansi.StyleList{
			LevelIndent: 0, // Remove list indentation
			StyleBlock: ansi.StyleBlock{
				IndentToken: new(lipgloss.NewStyle().Background(lipgloss.Color(bgColor)).Render(" ")),
				StylePrimitive: ansi.StylePrimitive{
					Color: new(textColor),
				},
			},
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       new(headingColor),
				Bold:        new(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "# ",
				Color:  new(headingColor),
				Bold:   new(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
				Color:  new(headingColor),
				Bold:   new(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Color:  new(headingColor),
				Bold:   new(true),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
				Color:  new(headingColor),
				Bold:   new(true),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
				Color:  new(headingColor),
				Bold:   new(true),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  new(headingColor),
				Bold:   new(true),
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: new(true),
			Color:      new(mutedColor),
		},
		Emph: ansi.StylePrimitive{
			Color: new(emphColor),

			Italic: new(true),
		},
		Strong: ansi.StylePrimitive{
			Bold:  new(true),
			Color: new(strongColor),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  new(mutedColor),
			Format: "\n─────────────────────────────────────────\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       new(textColor),
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       new(textColor),
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{},
			Ticked:         "[✓] ",
			Unticked:       "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color: new(linkColor),

			Underline: new(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: new(linkColor),

			Bold: new(true),
		},
		Image: ansi.StylePrimitive{
			Color: new(linkColor),

			Underline: new(true),
			Format:    "🖼 {{.text}}",
		},
		ImageText: ansi.StylePrimitive{
			Color: new(linkColor),

			Format: "{{.text}}",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: new(codeColor),

				Prefix: "",
				Suffix: "",
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Prefix: "",
					Color:  new(codeColor),
				},
				Margin: uintPtr(0), // Remove margin
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: new(textColor),
				},
				Error: ansi.StylePrimitive{
					Color: new(errorColor),
				},
				Comment: ansi.StylePrimitive{
					Color: new(commentColor),
				},
				CommentPreproc: ansi.StylePrimitive{
					Color: new(keywordColor),
				},
				Keyword: ansi.StylePrimitive{
					Color: new(keywordColor),
				},
				KeywordReserved: ansi.StylePrimitive{
					Color: new(keywordColor),
				},
				KeywordNamespace: ansi.StylePrimitive{
					Color: new(keywordColor),
				},
				KeywordType: ansi.StylePrimitive{
					Color: new(keywordColor),
				},
				Operator: ansi.StylePrimitive{
					Color: new(textColor),
				},
				Punctuation: ansi.StylePrimitive{
					Color: new(textColor),
				},
				Name: ansi.StylePrimitive{
					Color: new(textColor),
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: new(textColor),
				},
				NameTag: ansi.StylePrimitive{
					Color: new(keywordColor),
				},
				NameAttribute: ansi.StylePrimitive{
					Color: new(textColor),
				},
				NameClass: ansi.StylePrimitive{
					Color: new(keywordColor),
				},
				NameConstant: ansi.StylePrimitive{
					Color: new(textColor),
				},
				NameDecorator: ansi.StylePrimitive{
					Color: new(textColor),
				},
				NameFunction: ansi.StylePrimitive{
					Color: new(textColor),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: new(numberColor),
				},
				LiteralString: ansi.StylePrimitive{
					Color: new(stringColor),
				},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: new(keywordColor),
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: new(errorColor),
				},
				GenericEmph: ansi.StylePrimitive{
					Color: new(emphColor),

					Italic: new(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: new(stringColor),
				},
				GenericStrong: ansi.StylePrimitive{
					Color: new(strongColor),

					Bold: new(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: new(headingColor),
				},
			},
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					BlockPrefix: "\n",
					BlockSuffix: "\n",
				},
			},
			CenterSeparator: new("┼"),
			ColumnSeparator: new("│"),
			RowSeparator:    new("─"),
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n ❯ ",
			Color:       new(linkColor),
		},
		Text: ansi.StylePrimitive{
			Color: new(textColor),
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: new(textColor),
			},
		},
	}
}

// toMarkdown renders markdown content using glamour
func toMarkdown(content string, width int) string {
	r := GetMarkdownRenderer(width)
	rendered, _ := r.Render(content)
	return rendered
}
