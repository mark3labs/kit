package kit

import (
	"fmt"
	"os"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/skills"
)

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

// ---------------------------------------------------------------------------
// Skill Bridge for Extensions (Phase 2)
// ---------------------------------------------------------------------------

// DiscoverSkillsForExtension finds skills in standard locations for extensions.
// Returns skills in the extension-facing format. Results are cached per-Kit
// instance to avoid reloading on every call.
func (m *Kit) DiscoverSkillsForExtension() []extensions.Skill {
	cwd, _ := os.Getwd()

	m.skillCache.mu.Lock()
	defer m.skillCache.mu.Unlock()
	if len(m.skillCache.skills) == 0 {
		m.skillCache.skills, _ = skills.LoadSkills(cwd)
	}
	return m.convertSkills(m.skillCache.skills)
}

// LoadSkillForExtension loads a single skill file for extensions.
func (m *Kit) LoadSkillForExtension(path string) (*extensions.Skill, string) {
	s, err := skills.LoadSkill(path)
	if err != nil {
		return nil, err.Error()
	}
	return m.convertSkill(s), ""
}

// LoadSkillsFromDirForExtension loads all skills from a directory for extensions.
func (m *Kit) LoadSkillsFromDirForExtension(dir string) extensions.SkillLoadResult {
	skillList, err := skills.LoadSkillsFromDir(dir)
	if err != nil {
		return extensions.SkillLoadResult{Error: err.Error()}
	}
	return extensions.SkillLoadResult{Skills: m.convertSkills(skillList)}
}

// convertSkill converts internal skill to extension-facing format.
func (m *Kit) convertSkill(s *skills.Skill) *extensions.Skill {
	return &extensions.Skill{
		Name:        s.Name,
		Description: s.Description,
		Content:     s.Content,
		Path:        s.Path,
		Tags:        s.Tags,
		When:        s.When,
	}
}

// convertSkills converts a slice of skills.
func (m *Kit) convertSkills(skillList []*skills.Skill) []extensions.Skill {
	result := make([]extensions.Skill, 0, len(skillList))
	for _, s := range skillList {
		result = append(result, *m.convertSkill(s))
	}
	return result
}

// ClearSkillCache clears the skill cache for this Kit instance.
func (m *Kit) ClearSkillCache() {
	m.skillCache.mu.Lock()
	defer m.skillCache.mu.Unlock()
	m.skillCache.skills = nil
}

// ReloadSkills re-discovers skills from disk, replacing the current set.
// This is called by file watchers when skill files change.
func (m *Kit) ReloadSkills() error {
	newSkills, err := loadSkills(m.opts)
	if err != nil {
		return fmt.Errorf("reloading skills: %w", err)
	}
	m.skills = newSkills
	m.ClearSkillCache()
	return nil
}
