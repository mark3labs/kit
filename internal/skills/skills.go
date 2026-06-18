// Package skills provides skill loading, parsing, and system prompt composition.
//
// Skills are markdown instruction files with optional YAML frontmatter that
// provide domain-specific context, instructions, and workflows to the agent.
// They follow the cross-client agentskills.io discovery convention plus a
// Kit-native location:
//
//	~/.agents/skills/               user-level cross-client skills
//	~/.config/kit/skills/           user-level Kit skills ($XDG_CONFIG_HOME aware)
//	<project>/.agents/skills/       project-local cross-client skills
//	<project>/.kit/skills/          project-local Kit skills
//
// Skills can be single .md/.txt files or subdirectories containing a SKILL.md
// file. Project-level skills take precedence over user-level skills when two
// skills share the same name.
package skills

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	"gopkg.in/yaml.v3"
)

// Skill represents a markdown-based instruction file that provides
// domain-specific context and workflows to the agent.
//
// The Name and Description fields are required by the agentskills.io
// specification. License, Compatibility, Metadata, and AllowedTools are
// optional spec fields. Tags and When are Kit-specific extensions that other
// clients ignore.
type Skill struct {
	// Name is the human-readable identifier for this skill. Required.
	Name string `yaml:"name" json:"name"`
	// Description summarises what this skill provides and when to use it.
	// Required by the spec — it is the sole basis on which the model decides
	// whether a skill is relevant, so a skill without one is omitted from the
	// catalog.
	Description string `yaml:"description" json:"description"`
	// Content is the full markdown body (after frontmatter).
	Content string `yaml:"-" json:"content"`
	// Path is the absolute filesystem path the skill was loaded from.
	Path string `yaml:"-" json:"path"`

	// License is an optional SPDX license identifier (spec field).
	License string `yaml:"license,omitempty" json:"license,omitempty"`
	// Compatibility is an optional free-form note describing the environments
	// or clients the skill targets (spec field). The model can use it to adapt
	// execution.
	Compatibility string `yaml:"compatibility,omitempty" json:"compatibility,omitempty"`
	// Metadata is an optional bag of arbitrary string key/value pairs (spec
	// field) for client-specific annotations.
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	// AllowedTools optionally restricts which tools the skill may use. This is
	// an experimental spec field carried for portability; Kit does not yet
	// enforce it.
	AllowedTools string `yaml:"allowed-tools,omitempty" json:"allowed_tools,omitempty"`
	// DisableModelInvocation, when true, hides the skill from the
	// model-facing catalog (spec field). The skill can still be activated
	// explicitly via the /skill: slash command.
	DisableModelInvocation bool `yaml:"disable-model-invocation,omitempty" json:"disable_model_invocation,omitempty"`

	// Tags are optional labels for categorisation. Kit extension.
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	// When controls automatic inclusion: "always", "on-demand", or a
	// file-glob like "file:*.go". Empty defaults to "on-demand". Kit extension.
	When string `yaml:"when,omitempty" json:"when,omitempty"`

	// project records whether the skill was discovered in a project-local
	// scope. Used internally for name-collision precedence (project > user).
	project bool `yaml:"-" json:"-"`
}

// Diagnostic describes a validation problem with a skill. Severity is either
// "error" (the skill cannot be used) or "warning" (the skill is usable but
// non-compliant).
type Diagnostic struct {
	// Severity is "error" or "warning".
	Severity string `json:"severity"`
	// Field names the frontmatter field the diagnostic relates to, if any.
	Field string `json:"field,omitempty"`
	// Message is a human-readable description of the problem.
	Message string `json:"message"`
}

// Validate checks the skill against the agentskills.io specification and
// returns a list of diagnostics. An empty slice means the skill is fully
// compliant. A missing description is reported as an error because the spec
// makes it required for discovery.
func (s *Skill) Validate() []Diagnostic {
	var diags []Diagnostic
	if strings.TrimSpace(s.Name) == "" {
		diags = append(diags, Diagnostic{Severity: "error", Field: "name", Message: "name is required"})
	}
	if strings.TrimSpace(s.Description) == "" {
		diags = append(diags, Diagnostic{
			Severity: "error",
			Field:    "description",
			Message:  "description is required for skill discovery",
		})
	}
	return diags
}

// hasError reports whether diags contains a diagnostic with "error" severity.
func hasError(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// BaseDir returns the directory the skill was loaded from. Relative resources
// referenced by a skill (scripts/, references/, assets/) resolve against this
// directory.
func (s *Skill) BaseDir() string {
	if s.Path == "" {
		return ""
	}
	return filepath.Dir(s.Path)
}

// resourceDirs are the conventional subdirectories a skill may bundle.
var resourceDirs = []string{"scripts", "references", "assets"}

// maxResources caps how many bundled resources are enumerated to avoid
// flooding the prompt for skills with large asset trees.
const maxResources = 50

// Resources walks one level into the skill's scripts/, references/, and
// assets/ subdirectories and returns the relative paths of any files found
// (slash-separated, relative to BaseDir). The result is capped at 50 entries.
// It returns nil when the skill has no bundled resources or its Path is not a
// real on-disk file.
func (s *Skill) Resources() []string {
	base := s.BaseDir()
	if base == "" {
		return nil
	}
	var out []string
	for _, sub := range resourceDirs {
		dir := filepath.Join(base, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			out = append(out, sub+"/"+e.Name())
			if len(out) >= maxResources {
				sort.Strings(out)
				return out
			}
		}
	}
	sort.Strings(out)
	return out
}

// frontmatterSep is the YAML frontmatter delimiter.
const frontmatterSep = "---"

// LoadSkill reads a single skill file (markdown with optional YAML
// frontmatter).  If no frontmatter is present the skill name is derived
// from the filename.
func LoadSkill(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading skill %s: %w", path, err)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	return parseSkill(data, path, abs)
}

// parseSkill parses skill bytes that originated from srcPath (used for error
// messages and name derivation) and records storePath as the skill's Path.
// It is shared by the os-backed and fs.FS-backed loaders.
func parseSkill(data []byte, srcPath, storePath string) (*Skill, error) {
	skill := &Skill{Path: storePath}

	content := string(data)

	// Try to parse YAML frontmatter (--- ... ---).
	if strings.HasPrefix(strings.TrimSpace(content), frontmatterSep) {
		trimmed := strings.TrimSpace(content)
		// Find the closing separator (skip the opening one).
		rest := trimmed[len(frontmatterSep):]
		frontmatter, body, found := strings.Cut(rest, "\n"+frontmatterSep)
		if found {
			// Strip an optional trailing newline right after the closing ---.
			body = strings.TrimPrefix(body, "\n")

			if err := unmarshalFrontmatter([]byte(frontmatter), skill); err != nil {
				return nil, fmt.Errorf("parsing frontmatter in %s: %w", srcPath, err)
			}
			skill.Content = strings.TrimSpace(body)
		} else {
			// Opening --- but no closing --- — treat entire file as content.
			skill.Content = strings.TrimSpace(content)
		}
	} else {
		skill.Content = strings.TrimSpace(content)
	}

	// Fallback: derive name from filename if frontmatter didn't set one.
	if skill.Name == "" {
		base := filepath.Base(srcPath)
		ext := filepath.Ext(base)
		skill.Name = strings.TrimSuffix(base, ext)
		// Convert SKILL → directory name for SKILL.md files.
		if strings.EqualFold(skill.Name, "SKILL") || strings.EqualFold(skill.Name, "skill") {
			skill.Name = filepath.Base(filepath.Dir(srcPath))
		}
	}

	return skill, nil
}

// unquotedColonRe matches a YAML scalar line whose value contains an unquoted
// colon, e.g. `description: Use when: extracting tables`. This is the most
// common frontmatter authoring mistake in cross-client skills and breaks
// strict YAML parsing.
var unquotedColonRe = regexp.MustCompile(`^(\s*[A-Za-z0-9_-]+):[ \t]+([^'"\n].*:.*)$`)

// unmarshalFrontmatter unmarshals YAML frontmatter into skill, tolerating the
// common "unquoted colon in a scalar value" mistake (e.g.
// `description: Use when: …`). On a parse failure it quotes offending scalar
// values and retries once before giving up.
func unmarshalFrontmatter(frontmatter []byte, skill *Skill) error {
	err := yaml.Unmarshal(frontmatter, skill)
	if err == nil {
		return nil
	}

	// Attempt a single recovery pass: quote scalar values that contain an
	// unquoted colon, which is the dominant cross-client failure mode.
	repaired, changed := repairUnquotedColons(string(frontmatter))
	if !changed {
		return err
	}
	if retryErr := yaml.Unmarshal([]byte(repaired), skill); retryErr != nil {
		// The original error is more useful to the author.
		return err
	}
	return nil
}

// repairUnquotedColons quotes scalar values containing an unquoted colon and
// reports whether any line was changed.
func repairUnquotedColons(frontmatter string) (string, bool) {
	lines := strings.Split(frontmatter, "\n")
	changed := false
	for i, line := range lines {
		m := unquotedColonRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key, value := m[1], strings.TrimRight(m[2], " \t")
		// Escape embedded double quotes before wrapping.
		value = strings.ReplaceAll(value, `"`, `\"`)
		lines[i] = fmt.Sprintf(`%s: "%s"`, key, value)
		changed = true
	}
	if !changed {
		return frontmatter, false
	}
	return strings.Join(lines, "\n"), true
}

// LoadSkillsFromDir loads all skills from a single directory. It looks for:
//   - *.md and *.txt files directly in dir
//   - SKILL.md (case-insensitive) in immediate subdirectories
//
// Files that fail to parse are skipped with a warning logged via the
// returned error list.
func LoadSkillsFromDir(dir string) ([]*Skill, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, nil // directory doesn't exist — not an error
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading skills directory %s: %w", dir, err)
	}

	var skills []*Skill
	var errs []error

	for _, entry := range entries {
		full := filepath.Join(dir, entry.Name())

		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".md" || ext == ".txt" {
				s, err := LoadSkill(full)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				skills = append(skills, s)
			}
			continue
		}

		// Subdirectory: look for SKILL.md (case-insensitive).
		subEntries, err := os.ReadDir(full)
		if err != nil {
			continue
		}
		for _, se := range subEntries {
			if !se.IsDir() && strings.EqualFold(se.Name(), "SKILL.md") {
				s, err := LoadSkill(filepath.Join(full, se.Name()))
				if err != nil {
					errs = append(errs, err)
					continue
				}
				skills = append(skills, s)
				break // only one SKILL.md per subdirectory
			}
		}
	}

	if len(errs) > 0 {
		return skills, fmt.Errorf("some skills failed to load: %w", errors.Join(errs...))
	}
	return skills, nil
}

// LoadSkillsFromFS is the fs.FS-typed counterpart of LoadSkillsFromDir. It
// walks fsys starting at root (which may be "." or a subdirectory), finds
// *.md and *.txt files plus SKILL.md files in subdirectories, parses YAML
// frontmatter + markdown body, and returns the loaded skills.
//
// Because fs.FS has no notion of an absolute on-disk path, each loaded skill's
// Path is set to its slash-separated path within fsys. Files that fail to
// parse are skipped and reported via the returned error.
func LoadSkillsFromFS(fsys fs.FS, root string) ([]*Skill, error) {
	if fsys == nil {
		return nil, nil
	}
	if root == "" {
		root = "."
	}

	var skills []*Skill
	var errs []error

	walkErr := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries rather than aborting the walk
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		ext := strings.ToLower(path.Ext(name))
		if ext != ".md" && ext != ".txt" {
			return nil
		}
		// Top-level .md/.txt files, or SKILL.md anywhere.
		isTopLevel := path.Dir(p) == root
		if !isTopLevel && !strings.EqualFold(name, "SKILL.md") {
			return nil
		}
		data, readErr := fs.ReadFile(fsys, p)
		if readErr != nil {
			errs = append(errs, fmt.Errorf("reading skill %s: %w", p, readErr))
			return nil
		}
		s, parseErr := parseSkill(data, p, p)
		if parseErr != nil {
			errs = append(errs, parseErr)
			return nil
		}
		skills = append(skills, s)
		return nil
	})
	if walkErr != nil {
		return skills, fmt.Errorf("walking skills fs at %s: %w", root, walkErr)
	}
	if len(errs) > 0 {
		return skills, fmt.Errorf("some skills failed to load: %w", errors.Join(errs...))
	}
	return skills, nil
}

// LoadUserSkills discovers skills from the user-level scopes only:
//
//  1. ~/.agents/skills/ (cross-client convention)
//  2. $XDG_CONFIG_HOME/kit/skills/ (default ~/.config/kit/skills/)
//
// The returned skills are not yet validated or deduplicated; pass them through
// Combine together with project skills to produce the final catalog set.
func LoadUserSkills() []*Skill {
	var loaded []*Skill
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dir := filepath.Join(home, ".agents", "skills")
		ss, loadErr := LoadSkillsFromDir(dir)
		if loadErr != nil {
			// Missing directories are already swallowed by LoadSkillsFromDir,
			// so a non-nil error here is genuine (permission denied, read
			// failure, or a malformed skill file) and would otherwise yield a
			// silently partial catalog.
			log.Warn("failed to load some user skills", "dir", dir, "err", loadErr)
		}
		loaded = append(loaded, ss...)
	}
	if g := globalSkillsDir(); g != "" {
		ss, loadErr := LoadSkillsFromDir(g)
		if loadErr != nil {
			log.Warn("failed to load some user skills", "dir", g, "err", loadErr)
		}
		loaded = append(loaded, ss...)
	}
	for _, s := range loaded {
		s.project = false
	}
	return loaded
}

// LoadProjectSkills discovers skills from the project-local scopes only:
//
//  1. <cwd>/.agents/skills/ (cross-client convention)
//  2. <cwd>/.kit/skills/ (Kit-specific)
//
// Because project-local skills are injected into the system prompt, callers
// may wish to gate this on a trust check before including the result. The
// returned skills are not yet validated or deduplicated; pass them through
// Combine.
func LoadProjectSkills(cwd string) []*Skill {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	var loaded []*Skill
	for _, dir := range []string{
		filepath.Join(cwd, ".agents", "skills"),
		filepath.Join(cwd, ".kit", "skills"),
	} {
		ss, loadErr := LoadSkillsFromDir(dir)
		if loadErr != nil {
			log.Warn("failed to load some project skills", "dir", dir, "err", loadErr)
		}
		loaded = append(loaded, ss...)
	}
	for _, s := range loaded {
		s.project = true
	}
	return loaded
}

// Combine validates and deduplicates the union of user-level and project-level
// skills. Skills missing a required description are skipped with a logged
// warning; when two skills share a Name the project-level one wins (also
// logged). User skills are considered before project skills so first-seen
// ordering is stable.
func Combine(user, project []*Skill) []*Skill {
	combined := make([]*Skill, 0, len(user)+len(project))
	combined = append(combined, user...)
	combined = append(combined, project...)
	return finalizeSkills(combined)
}

// LoadSkills auto-discovers skills from the standard agentskills.io scopes:
//
//  1. User-level: ~/.agents/skills/ (cross-client convention)
//  2. User-level: $XDG_CONFIG_HOME/kit/skills/ (default ~/.config/kit/skills/)
//  3. Project-local: <cwd>/.agents/skills/ (cross-client convention)
//  4. Project-local: <cwd>/.kit/skills/ (Kit-specific)
//
// When two skills share the same Name, the project-level skill takes
// precedence over a user-level one and a warning is logged. cwd is the working
// directory for project-local discovery; if empty the current working
// directory is used.
func LoadSkills(cwd string) ([]*Skill, error) {
	return Combine(LoadUserSkills(), LoadProjectSkills(cwd)), nil
}

// finalizeSkills applies validation (skipping skills missing a required
// description) and name-collision precedence (project overrides user). It
// preserves first-seen ordering for stable catalog output.
func finalizeSkills(loaded []*Skill) []*Skill {
	byName := make(map[string]int) // name → index in result
	var result []*Skill

	for _, s := range loaded {
		if diags := s.Validate(); hasError(diags) {
			for _, d := range diags {
				if d.Severity == "error" {
					log.Warn("skipping skill: validation failed", "path", s.Path, "field", d.Field, "reason", d.Message)
				}
			}
			continue
		}

		if idx, ok := byName[s.Name]; ok {
			existing := result[idx]
			// Project-level skills override user-level skills.
			if s.project && !existing.project {
				log.Warn("skill name collision: project skill overrides user skill",
					"name", s.Name, "project", s.Path, "user", existing.Path)
				result[idx] = s
			} else {
				log.Warn("skill name collision: keeping earlier skill, ignoring duplicate",
					"name", s.Name, "kept", existing.Path, "ignored", s.Path)
			}
			continue
		}

		byName[s.Name] = len(result)
		result = append(result, s)
	}

	return result
}

// FormatForPrompt formats skills as metadata-only XML for inclusion in a
// system prompt. Only the name, description, and file location are included;
// the agent reads the full skill file on demand using the read tool. Skill
// fields are XML-escaped so that descriptions containing <, >, & or quotes
// produce valid markup. Skills with DisableModelInvocation set are omitted
// from the catalog (they remain available via the /skill: slash command).
func FormatForPrompt(skills []*Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("The following skills provide specialized instructions for specific tasks.\n")
	buf.WriteString("Use the read tool to load a skill's file when the task matches its description.\n")
	buf.WriteString("When a skill file references a relative path, resolve it against the skill directory (parent of SKILL.md) and use that absolute path in tool commands.\n")
	buf.WriteString("\n<available_skills>\n")

	emitted := 0
	for _, s := range skills {
		if s.DisableModelInvocation {
			continue
		}
		buf.WriteString("  <skill>\n")
		fmt.Fprintf(&buf, "    <name>%s</name>\n", escapeXML(s.Name))
		if s.Description != "" {
			fmt.Fprintf(&buf, "    <description>%s</description>\n", escapeXML(s.Description))
		}
		if s.Compatibility != "" {
			fmt.Fprintf(&buf, "    <compatibility>%s</compatibility>\n", escapeXML(s.Compatibility))
		}
		fmt.Fprintf(&buf, "    <location>%s</location>\n", escapeXML(s.Path))
		buf.WriteString("  </skill>\n")
		emitted++
	}

	buf.WriteString("</available_skills>")
	if emitted == 0 {
		return ""
	}
	return buf.String()
}

// escapeXML escapes a string for safe inclusion as XML text content.
func escapeXML(s string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(s)); err != nil {
		return s
	}
	return buf.String()
}

// FormatResources renders a skill's bundled resources as a <skill_resources>
// block, or returns the empty string when the skill bundles no resources. It
// is used when a skill is explicitly activated so the model knows which files
// it can read without enumerating them itself.
func FormatResources(resources []string) string {
	if len(resources) == 0 {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString("<skill_resources>\n")
	limit := len(resources)
	truncated := false
	if limit > maxResources {
		limit = maxResources
		truncated = true
	}
	for _, r := range resources[:limit] {
		fmt.Fprintf(&buf, "  <file>%s</file>\n", escapeXML(r))
	}
	if truncated {
		buf.WriteString("  <!-- (truncated) -->\n")
	}
	buf.WriteString("</skill_resources>")
	return buf.String()
}

// globalSkillsDir returns the global skills directory, respecting
// $XDG_CONFIG_HOME.  Defaults to ~/.config/kit/skills.
func globalSkillsDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "kit", "skills")
}
