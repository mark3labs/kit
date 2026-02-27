package skills

import (
	"bytes"
	"fmt"
)

// section is a named block of text that will be included in the final
// system prompt.
type section struct {
	name    string
	content string
}

// PromptBuilder composes a system prompt from a base prompt, skills, and
// arbitrary named sections.
type PromptBuilder struct {
	basePrompt string
	sections   []section
}

// NewPromptBuilder creates a PromptBuilder with the given base system prompt.
// The base prompt is always emitted first.
func NewPromptBuilder(basePrompt string) *PromptBuilder {
	return &PromptBuilder{basePrompt: basePrompt}
}

// WithSkills appends a formatted skills section.  If skills is empty, no
// section is added.  Returns the builder for chaining.
func (pb *PromptBuilder) WithSkills(skills []*Skill) *PromptBuilder {
	formatted := FormatForPrompt(skills)
	if formatted != "" {
		pb.sections = append(pb.sections, section{
			name:    "Skills",
			content: formatted,
		})
	}
	return pb
}

// WithSection appends a named section.  Duplicate names are allowed (both
// will appear).  Returns the builder for chaining.
func (pb *PromptBuilder) WithSection(name, content string) *PromptBuilder {
	if content != "" {
		pb.sections = append(pb.sections, section{
			name:    name,
			content: content,
		})
	}
	return pb
}

// Build assembles the final system prompt. The base prompt comes first,
// followed by each section separated by double newlines.
func (pb *PromptBuilder) Build() string {
	var buf bytes.Buffer

	if pb.basePrompt != "" {
		buf.WriteString(pb.basePrompt)
	}

	for _, s := range pb.sections {
		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		if s.name != "" {
			buf.WriteString(fmt.Sprintf("# %s\n\n", s.name))
		}
		buf.WriteString(s.content)
	}

	return buf.String()
}
