// Package skills provides skill loading, parsing, and system prompt composition.
//
// Skills are markdown instruction files with optional YAML frontmatter that
// provide domain-specific context, instructions, and workflows to the agent.
// They follow a hierarchical discovery pattern similar to extensions:
//
//	~/.config/kit/skills/           global skills directory
//	.kit/skills/                    project-local skills directory
//
// Skills can be single .md/.txt files or subdirectories containing a SKILL.md file.
package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a markdown-based instruction file that provides
// domain-specific context and workflows to the agent.
type Skill struct {
	// Name is the human-readable identifier for this skill.
	Name string `yaml:"name" json:"name"`
	// Description summarises what this skill provides.
	Description string `yaml:"description" json:"description"`
	// Content is the full markdown body (after frontmatter).
	Content string `yaml:"-" json:"content"`
	// Path is the absolute filesystem path the skill was loaded from.
	Path string `yaml:"-" json:"path"`
	// Tags are optional labels for categorisation.
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	// When controls automatic inclusion: "always", "on-demand", or a
	// file-glob like "file:*.go".  Empty defaults to "on-demand".
	When string `yaml:"when,omitempty" json:"when,omitempty"`
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

	skill := &Skill{Path: abs}

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

			if err := yaml.Unmarshal([]byte(frontmatter), skill); err != nil {
				return nil, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
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
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		skill.Name = strings.TrimSuffix(base, ext)
		// Convert SKILL → directory name for SKILL.md files.
		if strings.EqualFold(skill.Name, "SKILL") || strings.EqualFold(skill.Name, "skill") {
			skill.Name = filepath.Base(filepath.Dir(path))
		}
	}

	return skill, nil
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
	var errs []string

	for _, entry := range entries {
		full := filepath.Join(dir, entry.Name())

		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".md" || ext == ".txt" {
				s, err := LoadSkill(full)
				if err != nil {
					errs = append(errs, err.Error())
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
					errs = append(errs, err.Error())
					continue
				}
				skills = append(skills, s)
				break // only one SKILL.md per subdirectory
			}
		}
	}

	if len(errs) > 0 {
		return skills, fmt.Errorf("some skills failed to load: %s", strings.Join(errs, "; "))
	}
	return skills, nil
}

// LoadSkills auto-discovers skills from standard directories:
//  1. Global: $XDG_CONFIG_HOME/kit/skills/ (default ~/.config/kit/skills/)
//  2. Project-local: <cwd>/.kit/skills/
//
// Skills from project-local directories take precedence (appended last).
// cwd is the working directory for project-local discovery; if empty the
// current working directory is used.
func LoadSkills(cwd string) ([]*Skill, error) {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	seen := make(map[string]bool)
	var all []*Skill

	addUnique := func(skills []*Skill) {
		for _, s := range skills {
			if !seen[s.Path] {
				seen[s.Path] = true
				all = append(all, s)
			}
		}
	}

	// Global skills.
	globalDir := globalSkillsDir()
	if globalDir != "" {
		global, _ := LoadSkillsFromDir(globalDir)
		addUnique(global)
	}

	// Project-local skills: .agents/skills/ (standardized cross-tool convention).
	agentsDir := filepath.Join(cwd, ".agents", "skills")
	agentsSkills, _ := LoadSkillsFromDir(agentsDir)
	addUnique(agentsSkills)

	// Project-local skills: .kit/skills/ (kit-specific).
	localDir := filepath.Join(cwd, ".kit", "skills")
	local, _ := LoadSkillsFromDir(localDir)
	addUnique(local)

	return all, nil
}

// FormatForPrompt formats skills as metadata-only XML for inclusion in a
// system prompt. Only the name, description, and file location are included;
// the agent reads the full skill file on demand using the read tool. This
// matches the Pi SDK's formatSkillsForPrompt convention.
func FormatForPrompt(skills []*Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("The following skills provide specialized instructions for specific tasks.\n")
	buf.WriteString("Use the read tool to load a skill's file when the task matches its description.\n")
	buf.WriteString("When a skill file references a relative path, resolve it against the skill directory (parent of SKILL.md) and use that absolute path in tool commands.\n")
	buf.WriteString("\n<available_skills>\n")

	for _, s := range skills {
		buf.WriteString("  <skill>\n")
		buf.WriteString(fmt.Sprintf("    <name>%s</name>\n", s.Name))
		if s.Description != "" {
			buf.WriteString(fmt.Sprintf("    <description>%s</description>\n", s.Description))
		}
		buf.WriteString(fmt.Sprintf("    <location>file://%s</location>\n", s.Path))
		buf.WriteString("  </skill>\n")
	}

	buf.WriteString("</available_skills>")
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
