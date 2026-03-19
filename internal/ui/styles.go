package ui

import (
	"fmt"
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
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

// colorHex converts a color.Color to a hex string suitable for ansi.StyleConfig.
func colorHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// colorHexPtr returns a pointer to the hex string of a color.Color.
func colorHexPtr(c color.Color) *string {
	s := colorHex(c)
	return &s
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

// generateMarkdownStyleConfig creates an ansi.StyleConfig from the active theme.
func generateMarkdownStyleConfig() ansi.StyleConfig {
	md := GetTheme().Markdown
	text := colorHexPtr(md.Text)
	muted := colorHexPtr(md.Muted)
	heading := colorHexPtr(md.Heading)
	emph := colorHexPtr(md.Emph)
	strong := colorHexPtr(md.Strong)
	link := colorHexPtr(md.Link)
	code := colorHexPtr(md.Code)
	errClr := colorHexPtr(md.Error)
	keyword := colorHexPtr(md.Keyword)
	str := colorHexPtr(md.String)
	number := colorHexPtr(md.Number)
	comment := colorHexPtr(md.Comment)

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "",
				BlockSuffix: "",
				Color:       text,
			},
			Margin: uintPtr(0),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  muted,
				Italic: new(true),
				Prefix: "┃ ",
			},
			Indent: uintPtr(1),
		},
		List: ansi.StyleList{
			LevelIndent: 0,
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: text,
				},
			},
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       heading,
				Bold:        new(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "# ",
				Color:  heading,
				Bold:   new(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
				Color:  heading,
				Bold:   new(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Color:  heading,
				Bold:   new(true),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
				Color:  heading,
				Bold:   new(true),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
				Color:  heading,
				Bold:   new(true),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  heading,
				Bold:   new(true),
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: new(true),
			Color:      muted,
		},
		Emph: ansi.StylePrimitive{
			Color:  emph,
			Italic: new(true),
		},
		Strong: ansi.StylePrimitive{
			Bold:  new(true),
			Color: strong,
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  muted,
			Format: "\n─────────────────────────────────────────\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       text,
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       text,
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{},
			Ticked:         "[✓] ",
			Unticked:       "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     link,
			Underline: new(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: link,
			Bold:  new(true),
		},
		Image: ansi.StylePrimitive{
			Color:     link,
			Underline: new(true),
			Format:    "🖼 {{.text}}",
		},
		ImageText: ansi.StylePrimitive{
			Color:  link,
			Format: "{{.text}}",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  code,
				Prefix: "",
				Suffix: "",
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Prefix: "",
					Color:  code,
				},
				Margin: uintPtr(0),
			},
			Chroma: &ansi.Chroma{
				Text:             ansi.StylePrimitive{Color: text},
				Error:            ansi.StylePrimitive{Color: errClr},
				Comment:          ansi.StylePrimitive{Color: comment},
				CommentPreproc:   ansi.StylePrimitive{Color: keyword},
				Keyword:          ansi.StylePrimitive{Color: keyword},
				KeywordReserved:  ansi.StylePrimitive{Color: keyword},
				KeywordNamespace: ansi.StylePrimitive{Color: keyword},
				KeywordType:      ansi.StylePrimitive{Color: keyword},
				Operator:         ansi.StylePrimitive{Color: text},
				Punctuation:      ansi.StylePrimitive{Color: text},
				Name:             ansi.StylePrimitive{Color: text},
				NameBuiltin:      ansi.StylePrimitive{Color: text},
				NameTag:          ansi.StylePrimitive{Color: keyword},
				NameAttribute:    ansi.StylePrimitive{Color: text},
				NameClass:        ansi.StylePrimitive{Color: keyword},
				NameConstant:     ansi.StylePrimitive{Color: text},
				NameDecorator:    ansi.StylePrimitive{Color: text},
				NameFunction:     ansi.StylePrimitive{Color: text},
				LiteralNumber:    ansi.StylePrimitive{Color: number},
				LiteralString:    ansi.StylePrimitive{Color: str},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: keyword,
				},
				GenericDeleted: ansi.StylePrimitive{Color: errClr},
				GenericEmph: ansi.StylePrimitive{
					Color:  emph,
					Italic: new(true),
				},
				GenericInserted: ansi.StylePrimitive{Color: str},
				GenericStrong: ansi.StylePrimitive{
					Color: strong,
					Bold:  new(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: heading,
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
			Color:       link,
		},
		Text: ansi.StylePrimitive{
			Color: text,
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: text,
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
