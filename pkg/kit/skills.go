package kit

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/skills"
	"github.com/mark3labs/kit/internal/trust"
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

// LoadSkillsFromFS is the [fs.FS]-typed counterpart of [LoadSkillsFromDir].
// It walks fsys starting at root (which may be "." or a subdirectory), finds
// *.md/*.txt files and SKILL.md files in subdirectories, parses YAML
// frontmatter + markdown body, and returns the loaded skills. Use it when
// skill discovery is wrapped in an fs.FS abstraction (embed.FS distribution,
// fstest.MapFS tests, or per-tenant virtual filesystems).
//
// Each loaded skill's Path is its slash-separated path within fsys, since
// fs.FS has no notion of an absolute on-disk path.
func LoadSkillsFromFS(fsys fs.FS, root string) ([]*Skill, error) {
	return skills.LoadSkillsFromFS(fsys, root)
}

// LoadSkills auto-discovers skills from standard directories:
//   - User-level: ~/.agents/skills/ (cross-client convention)
//   - User-level: $XDG_CONFIG_HOME/kit/skills/ (default ~/.config/kit/skills/)
//   - Project-local: <cwd>/.agents/skills/ (cross-client convention)
//   - Project-local: <cwd>/.kit/skills/ (Kit-specific)
//
// Project-level skills take precedence over user-level skills with the same
// name. cwd is the working directory for project-local discovery; if empty
// the current working directory is used.
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
		Name:                   s.Name,
		Description:            s.Description,
		Content:                s.Content,
		Path:                   s.Path,
		License:                s.License,
		Compatibility:          s.Compatibility,
		Metadata:               s.Metadata,
		AllowedTools:           s.AllowedTools,
		DisableModelInvocation: s.DisableModelInvocation,
		Tags:                   s.Tags,
		When:                   s.When,
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
// This is called by file watchers when skill files change. The system prompt
// is recomposed and applied to the running agent so subsequent turns see the
// new skill set.
func (m *Kit) ReloadSkills() error {
	newSkills, err := loadSkills(m.opts)
	if err != nil {
		return fmt.Errorf("reloading skills: %w", err)
	}
	m.runtimeMu.Lock()
	m.skills = newSkills
	m.runtimeMu.Unlock()
	m.ClearSkillCache()
	m.applyComposedSystemPrompt()
	return nil
}

// ---------------------------------------------------------------------------
// Runtime skill management (Issue #36)
// ---------------------------------------------------------------------------
//
// The methods below let SDK consumers (chatbot hosts, multi-tenant agents)
// mutate the active skill set after Kit construction. Each mutation recomposes
// the system prompt and applies it to the underlying agent so the LLM sees
// the new skill metadata on its next turn.

// AddSkill registers a single skill on this Kit instance. The skill object
// can be built programmatically (no file on disk required) — only Name and
// Content are mandatory. If a skill with the same Name is already loaded the
// new skill replaces it. Returns an error when skill is nil or has an empty
// name.
//
// After mutation the system prompt is recomposed and applied to the running
// agent so the next turn sees the updated skill metadata. AddSkill is safe to
// call from any goroutine.
func (m *Kit) AddSkill(skill *Skill) error {
	if skill == nil {
		return fmt.Errorf("AddSkill: skill is nil")
	}
	if skill.Name == "" {
		return fmt.Errorf("AddSkill: skill name is required")
	}

	m.runtimeMu.Lock()
	replaced := false
	for i, s := range m.skills {
		if s.Name == skill.Name {
			m.skills[i] = skill
			replaced = true
			break
		}
	}
	if !replaced {
		m.skills = append(m.skills, skill)
	}
	m.runtimeMu.Unlock()

	m.ClearSkillCache()
	m.applyComposedSystemPrompt()
	return nil
}

// LoadAndAddSkill loads a skill from a filesystem path (single .md/.txt file)
// and adds it via [Kit.AddSkill]. Returns the loaded skill on success.
func (m *Kit) LoadAndAddSkill(path string) (*Skill, error) {
	s, err := skills.LoadSkill(path)
	if err != nil {
		return nil, fmt.Errorf("LoadAndAddSkill: %w", err)
	}
	if err := m.AddSkill(s); err != nil {
		return nil, err
	}
	return s, nil
}

// RemoveSkill removes the named skill from this Kit instance and recomposes
// the system prompt. Returns true when a skill with that name was found and
// removed, false otherwise.
func (m *Kit) RemoveSkill(name string) bool {
	m.runtimeMu.Lock()
	found := false
	for i, s := range m.skills {
		if s.Name == name {
			m.skills = append(m.skills[:i], m.skills[i+1:]...)
			found = true
			break
		}
	}
	m.runtimeMu.Unlock()

	if !found {
		return false
	}
	m.ClearSkillCache()
	m.applyComposedSystemPrompt()
	return true
}

// SetSkills replaces the active skill set with the provided slice. Pass nil
// or an empty slice to remove all skills. The system prompt is recomposed and
// applied. Skills with empty names are rejected and no mutation is performed.
func (m *Kit) SetSkills(skillList []*Skill) error {
	// Validate first so a bad input doesn't partially mutate state.
	for i, s := range skillList {
		if s == nil {
			return fmt.Errorf("SetSkills: skill at index %d is nil", i)
		}
		if s.Name == "" {
			return fmt.Errorf("SetSkills: skill at index %d has empty name", i)
		}
	}

	copied := make([]*Skill, len(skillList))
	copy(copied, skillList)

	m.runtimeMu.Lock()
	m.skills = copied
	m.runtimeMu.Unlock()

	m.ClearSkillCache()
	m.applyComposedSystemPrompt()
	return nil
}

// applyComposedSystemPrompt recomposes the system prompt from the captured
// base prompt + current contextFiles + current skills + date/cwd, and pushes
// the result onto the underlying agent. No-op when the agent is unset (i.e.
// during construction).
func (m *Kit) applyComposedSystemPrompt() {
	if m.agent == nil {
		return
	}
	m.runtimeMu.RLock()
	base := m.basePrompt
	m.runtimeMu.RUnlock()
	composed := m.composeSystemPrompt(base)
	m.agent.SetSystemPrompt(composed)
}

// RefreshSystemPrompt manually recomposes the system prompt from the current
// skills and context files and applies it to the agent. Call this after a
// batch of low-level mutations or to force a re-render of the date/cwd
// section. Most callers don't need to invoke this directly because
// AddSkill, RemoveSkill, SetSkills, AddContextFile, RemoveContextFile, and
// SetContextFiles all refresh automatically.
func (m *Kit) RefreshSystemPrompt() {
	m.applyComposedSystemPrompt()
}

// ---------------------------------------------------------------------------
// Per-skill disable (Issue #65, gap #10)
// ---------------------------------------------------------------------------

// applySkillDisableList sets DisableModelInvocation on every skill whose Name
// appears in names. Disabled skills remain loaded (so explicit /skill:
// activation still works) but are hidden from the model-facing catalog.
func applySkillDisableList(skillList []*skills.Skill, names []string) {
	if len(names) == 0 {
		return
	}
	disabled := make(map[string]bool, len(names))
	for _, n := range names {
		disabled[n] = true
	}
	for _, s := range skillList {
		if disabled[s.Name] {
			s.DisableModelInvocation = true
		}
	}
}

// DisableSkill hides the named skill from the model-facing catalog while
// keeping it loaded (so it can still be activated explicitly via /skill:).
// The system prompt is recomposed and applied. Returns true when a skill with
// that name was found.
func (m *Kit) DisableSkill(name string) bool {
	return m.setSkillModelInvocation(name, true)
}

// EnableSkill re-exposes a previously disabled skill in the model-facing
// catalog. The system prompt is recomposed and applied. Returns true when a
// skill with that name was found.
func (m *Kit) EnableSkill(name string) bool {
	return m.setSkillModelInvocation(name, false)
}

// setSkillModelInvocation toggles DisableModelInvocation on the named skill
// and refreshes the system prompt. Returns true when the skill was found.
func (m *Kit) setSkillModelInvocation(name string, disabled bool) bool {
	m.runtimeMu.Lock()
	found := false
	for _, s := range m.skills {
		if s.Name == name {
			s.DisableModelInvocation = disabled
			found = true
			break
		}
	}
	m.runtimeMu.Unlock()

	if !found {
		return false
	}
	m.ClearSkillCache()
	m.applyComposedSystemPrompt()
	return true
}

// ---------------------------------------------------------------------------
// Project-skill trust gate (Issue #65, gap #8)
// ---------------------------------------------------------------------------

// TrustDecision is the outcome of a project-skill trust prompt.
type TrustDecision = trust.Decision

// Trust-prompt outcomes. They mirror the trust package decisions.
const (
	// SkipProjectSkills declines to load project skills this session.
	SkipProjectSkills = trust.Skip
	// TrustProject loads project skills and persists the directory as trusted.
	TrustProject = trust.Trust
	// TrustProjectOnce loads project skills this session without persisting.
	TrustProjectOnce = trust.TrustOnce
)

// projectSkillsTrusted decides whether project-local skills discovered in dir
// should be loaded. When no SkillTrustPrompt is configured the directory is
// trusted by default (preserving historical behaviour). Otherwise a persisted
// allowlist is consulted first, then the prompt is invoked for an unknown
// directory and the decision is persisted when the user chooses TrustProject.
func projectSkillsTrusted(opts *Options, dir string, count int) bool {
	if opts.SkillTrustPrompt == nil {
		return true
	}

	store, err := trust.Load("")
	if err == nil && store.IsTrusted(dir) {
		return true
	}

	switch opts.SkillTrustPrompt(dir, count) {
	case TrustProject:
		if store != nil {
			_ = store.Trust(dir)
		}
		return true
	case TrustProjectOnce:
		return true
	default:
		return false
	}
}
