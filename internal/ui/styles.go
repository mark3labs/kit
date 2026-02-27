package ui

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/mark3labs/kit/internal/config"
	"github.com/spf13/viper"
)

// uintPtr returns a pointer to u. Used by ansi.StyleConfig fields.
//
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

// colorScheme holds resolved color values for markdown rendering.
type colorScheme struct {
	text    string
	muted   string
	heading string
	emph    string
	strong  string
	link    string
	code    string
	err     string
	keyword string
	str     string
	number  string
	comment string
}

// resolveColorScheme determines the color palette based on user config and background.
func resolveColorScheme() colorScheme {
	var mdTheme config.MarkdownTheme
	err := config.FilepathOr("markdown-theme", &mdTheme)
	fromConfig := err == nil && viper.InConfig("markdown-theme")

	if fromConfig && IsDarkBackground() {
		return colorScheme{
			text: mdTheme.Text.Light, muted: mdTheme.Muted.Light,
			heading: mdTheme.Heading.Light, emph: mdTheme.Emph.Light,
			strong: mdTheme.Strong.Light, link: mdTheme.Link.Light,
			code: mdTheme.Code.Light, err: mdTheme.Error.Light,
			keyword: mdTheme.Keyword.Light, str: mdTheme.String.Light,
			number: mdTheme.Number.Light, comment: mdTheme.Comment.Light,
		}
	}
	if fromConfig {
		return colorScheme{
			text: mdTheme.Text.Dark, muted: mdTheme.Muted.Dark,
			heading: mdTheme.Heading.Dark, emph: mdTheme.Emph.Dark,
			strong: mdTheme.Strong.Dark, link: mdTheme.Link.Dark,
			code: mdTheme.Code.Dark, err: mdTheme.Error.Dark,
			keyword: mdTheme.Keyword.Dark, str: mdTheme.String.Dark,
			number: mdTheme.Number.Dark, comment: mdTheme.Comment.Dark,
		}
	}
	if IsDarkBackground() {
		return colorScheme{
			text: "#F9FAFB", muted: "#9CA3AF",
			heading: "#22D3EE", emph: "#FDE047",
			strong: "#F9FAFB", link: "#60A5FA",
			code: "#D1D5DB", err: "#F87171",
			keyword: "#C084FC", str: "#34D399",
			number: "#FBBF24", comment: "#9CA3AF",
		}
	}
	return colorScheme{
		text: "#1F2937", muted: "#6B7280",
		heading: "#0891B2", emph: "#D97706",
		strong: "#1F2937", link: "#2563EB",
		code: "#374151", err: "#DC2626",
		keyword: "#7C3AED", str: "#059669",
		number: "#D97706", comment: "#6B7280",
	}
}

// generateMarkdownStyleConfig creates an ansi.StyleConfig for markdown rendering.
func generateMarkdownStyleConfig() ansi.StyleConfig {
	cs := resolveColorScheme()

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "",
				BlockSuffix: "",
				Color:       &cs.text,
			},
			Margin: uintPtr(0), // Remove margin to prevent spacing
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  &cs.muted,
				Italic: new(true),
				Prefix: "┃ ",
			},
			Indent: uintPtr(1),
		},
		List: ansi.StyleList{
			LevelIndent: 0, // Remove list indentation
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: &cs.text,
				},
			},
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       &cs.heading,
				Bold:        new(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "# ",
				Color:  &cs.heading,
				Bold:   new(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
				Color:  &cs.heading,
				Bold:   new(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Color:  &cs.heading,
				Bold:   new(true),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
				Color:  &cs.heading,
				Bold:   new(true),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
				Color:  &cs.heading,
				Bold:   new(true),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  &cs.heading,
				Bold:   new(true),
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: new(true),
			Color:      &cs.muted,
		},
		Emph: ansi.StylePrimitive{
			Color:  &cs.emph,
			Italic: new(true),
		},
		Strong: ansi.StylePrimitive{
			Bold:  new(true),
			Color: &cs.strong,
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  &cs.muted,
			Format: "\n─────────────────────────────────────────\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       &cs.text,
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       &cs.text,
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{},
			Ticked:         "[✓] ",
			Unticked:       "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     &cs.link,
			Underline: new(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: &cs.link,
			Bold:  new(true),
		},
		Image: ansi.StylePrimitive{
			Color:     &cs.link,
			Underline: new(true),
			Format:    "🖼 {{.text}}",
		},
		ImageText: ansi.StylePrimitive{
			Color:  &cs.link,
			Format: "{{.text}}",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  &cs.code,
				Prefix: "",
				Suffix: "",
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Prefix: "",
					Color:  &cs.code,
				},
				Margin: uintPtr(0), // Remove margin
			},
			Chroma: &ansi.Chroma{
				Text:           ansi.StylePrimitive{Color: &cs.text},
				Error:          ansi.StylePrimitive{Color: &cs.err},
				Comment:        ansi.StylePrimitive{Color: &cs.comment},
				CommentPreproc: ansi.StylePrimitive{Color: &cs.keyword},
				Keyword:        ansi.StylePrimitive{Color: &cs.keyword},
				KeywordReserved: ansi.StylePrimitive{
					Color: &cs.keyword,
				},
				KeywordNamespace: ansi.StylePrimitive{
					Color: &cs.keyword,
				},
				KeywordType:   ansi.StylePrimitive{Color: &cs.keyword},
				Operator:      ansi.StylePrimitive{Color: &cs.text},
				Punctuation:   ansi.StylePrimitive{Color: &cs.text},
				Name:          ansi.StylePrimitive{Color: &cs.text},
				NameBuiltin:   ansi.StylePrimitive{Color: &cs.text},
				NameTag:       ansi.StylePrimitive{Color: &cs.keyword},
				NameAttribute: ansi.StylePrimitive{Color: &cs.text},
				NameClass:     ansi.StylePrimitive{Color: &cs.keyword},
				NameConstant:  ansi.StylePrimitive{Color: &cs.text},
				NameDecorator: ansi.StylePrimitive{Color: &cs.text},
				NameFunction:  ansi.StylePrimitive{Color: &cs.text},
				LiteralNumber: ansi.StylePrimitive{Color: &cs.number},
				LiteralString: ansi.StylePrimitive{Color: &cs.str},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: &cs.keyword,
				},
				GenericDeleted: ansi.StylePrimitive{Color: &cs.err},
				GenericEmph: ansi.StylePrimitive{
					Color:  &cs.emph,
					Italic: new(true),
				},
				GenericInserted: ansi.StylePrimitive{Color: &cs.str},
				GenericStrong: ansi.StylePrimitive{
					Color: &cs.strong,
					Bold:  new(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: &cs.heading,
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
			Color:       &cs.link,
		},
		Text: ansi.StylePrimitive{
			Color: &cs.text,
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: &cs.text,
			},
		},
	}
}

// toMarkdown renders markdown content using glamour.
func toMarkdown(content string, width int) string {
	r := GetMarkdownRenderer(width)
	rendered, _ := r.Render(content)
	return rendered
}
