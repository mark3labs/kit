package kit

import "github.com/mark3labs/kit/internal/skills"

// ==== Skills Types ====

// Skill represents a markdown-based instruction file with optional YAML
// frontmatter that provides domain-specific context and workflows.
type Skill = skills.Skill

// PromptTemplate is a named text template with {{variable}} placeholders.
type PromptTemplate = skills.PromptTemplate

// PromptBuilder composes a system prompt from a base prompt, skills, and
// arbitrary named sections.
type PromptBuilder = skills.PromptBuilder

// ==== Skills Functions ====

// LoadSkill reads a single skill file (markdown with optional YAML frontmatter).
// If no frontmatter is present the skill name is derived from the filename.
func LoadSkill(path string) (*Skill, error) {
	return skills.LoadSkill(path)
}

// LoadSkillsFromDir loads all skills from a single directory. It finds *.md
// and *.txt files directly in the directory, and SKILL.md files in immediate
// subdirectories.
func LoadSkillsFromDir(dir string) ([]*Skill, error) {
	return skills.LoadSkillsFromDir(dir)
}

// LoadSkills auto-discovers skills from standard directories:
//   - Global: $XDG_CONFIG_HOME/kit/skills/ (default ~/.config/kit/skills/)
//   - Project-local: <cwd>/.kit/skills/
//
// cwd is the working directory for project-local discovery; if empty the
// current working directory is used.
func LoadSkills(cwd string) ([]*Skill, error) {
	return skills.LoadSkills(cwd)
}

// FormatSkillsForPrompt formats skills for inclusion in a system prompt.
// Each skill is rendered as a named section with its content.
func FormatSkillsForPrompt(s []*Skill) string {
	return skills.FormatForPrompt(s)
}

// ==== Prompt Template Functions ====

// NewPromptTemplate creates a PromptTemplate, automatically extracting
// variable names from {{...}} placeholders in content.
func NewPromptTemplate(name, content string) *PromptTemplate {
	return skills.NewPromptTemplate(name, content)
}

// LoadPromptTemplate reads a template from a file. The template name is
// derived from the filename (without extension).
func LoadPromptTemplate(path string) (*PromptTemplate, error) {
	return skills.LoadPromptTemplate(path)
}

// ==== Prompt Builder Functions ====

// NewPromptBuilder creates a PromptBuilder with the given base system prompt.
// The base prompt is always emitted first.
func NewPromptBuilder(basePrompt string) *PromptBuilder {
	return skills.NewPromptBuilder(basePrompt)
}
