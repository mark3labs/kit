// Package agents provides discovery and parsing of named agent definitions.
//
// Named agents are reusable subagent presets defined in markdown files with
// YAML frontmatter. The filename (minus the .md extension) becomes the agent
// name, the frontmatter configures the agent, and the markdown body is used
// as the agent's system prompt.
//
// Discovery follows Kit's existing skills/prompts conventions:
//
//	$XDG_CONFIG_HOME/kit/agents/    user-level agents (default ~/.config/kit/agents/)
//	<project>/.agents/agents/       project-local cross-client agents
//	<project>/.kit/agents/          project-local Kit agents
//
// Project-level agents take precedence over user-level agents, which take
// precedence over the built-in agents shipped with Kit. An agent with
// `disabled: true` removes it (and anything it shadows) from the final set.
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Source values identifying where an agent definition was discovered.
const (
	SourceBuiltin = "builtin"
	SourceUser    = "user"
	SourceProject = "project"
)

// Agent is a named, reusable subagent preset.
type Agent struct {
	// Name identifies the agent. Derived from the definition filename
	// (minus the .md extension) for file-based agents.
	Name string

	// Description summarises what the agent does. Required — it is
	// advertised to the LLM in the subagent tool description.
	Description string

	// Model optionally overrides the spawning agent's model
	// (e.g. "anthropic/claude-sonnet-4"). Empty inherits the parent model.
	Model string

	// Tools is an optional allowlist of tool names available to the agent.
	// Empty means the default subagent tool set (all tools except
	// "subagent").
	Tools []string

	// Temperature optionally overrides the sampling temperature.
	Temperature *float32

	// Timeout optionally bounds the agent's execution time. Zero uses the
	// subagent default.
	Timeout time.Duration

	// Hidden excludes the agent from the subagent tool description while
	// keeping it resolvable by name.
	Hidden bool

	// Disabled removes the agent from the discovered set entirely.
	Disabled bool

	// SystemPrompt is the agent's system prompt (the markdown body).
	SystemPrompt string

	// FilePath is the on-disk path of the definition file. Empty for
	// built-in agents.
	FilePath string

	// Source records where the agent was discovered: "builtin", "user",
	// or "project".
	Source string
}

// frontmatter mirrors the YAML frontmatter schema of an agent definition.
type frontmatter struct {
	Description string   `yaml:"description"`
	Model       string   `yaml:"model"`
	Tools       []string `yaml:"tools"`
	Temperature *float32 `yaml:"temperature"`
	Timeout     int      `yaml:"timeout"` // seconds
	Hidden      bool     `yaml:"hidden"`
	Disabled    bool     `yaml:"disabled"`
}

// Load parses a single agent definition file. The agent name is derived from
// the filename (minus the .md extension). A missing or empty description in
// the frontmatter is an error.
func Load(path string) (*Agent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agent file %s: %w", path, err)
	}

	name := strings.TrimSuffix(filepath.Base(path), ".md")
	fmRaw, body := splitFrontmatter(string(data))

	var fm frontmatter
	if fmRaw != "" {
		if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
			return nil, fmt.Errorf("agent %s: parsing frontmatter: %w", name, err)
		}
	}
	if strings.TrimSpace(fm.Description) == "" {
		return nil, fmt.Errorf("agent %s: description is required in frontmatter", name)
	}

	var timeout time.Duration
	if fm.Timeout > 0 {
		timeout = time.Duration(fm.Timeout) * time.Second
	}

	return &Agent{
		Name:         name,
		Description:  strings.TrimSpace(fm.Description),
		Model:        strings.TrimSpace(fm.Model),
		Tools:        fm.Tools,
		Temperature:  fm.Temperature,
		Timeout:      timeout,
		Hidden:       fm.Hidden,
		Disabled:     fm.Disabled,
		SystemPrompt: strings.TrimSpace(body),
		FilePath:     path,
	}, nil
}

// LoadFromDir loads all agent definitions (*.md files) directly inside dir.
// A missing directory is not an error. Files that fail to parse are skipped;
// their errors are aggregated into the returned error alongside the
// successfully parsed agents.
func LoadFromDir(dir string) ([]*Agent, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, nil // directory doesn't exist — not an error
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading agents directory %s: %w", dir, err)
	}

	var loaded []*Agent
	var errs []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		a, err := Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		loaded = append(loaded, a)
	}

	if len(errs) > 0 {
		return loaded, fmt.Errorf("some agents failed to load: %s", strings.Join(errs, "; "))
	}
	return loaded, nil
}

// GlobalDir returns the XDG-aligned global agents directory, respecting
// $XDG_CONFIG_HOME. Defaults to ~/.config/kit/agents/. Returns an empty
// string if the user's home directory cannot be resolved.
func GlobalDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "kit", "agents")
}

// LoadAgents discovers named agents from the standard locations, applies
// precedence, and filters out disabled agents. cwd is the working directory
// for project-local discovery; if empty, the current working directory is
// used.
//
// Precedence (highest to lowest):
//
//  1. <cwd>/.agents/agents/  (project, cross-client convention)
//  2. <cwd>/.kit/agents/     (project, Kit-specific)
//  3. $XDG_CONFIG_HOME/kit/agents/ (user, default ~/.config/kit/agents/)
//  4. built-in agents
//
// When two definitions share a name, the higher-precedence one wins. An
// agent marked `disabled: true` is dropped after precedence resolution, so a
// project can disable a built-in or user-level agent by shadowing it.
//
// Per-file parse failures do not abort discovery: the aggregated error is
// returned alongside the successfully loaded agents.
func LoadAgents(cwd string) ([]*Agent, error) {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	type scope struct {
		dir    string
		source string
	}
	scopes := []scope{
		{filepath.Join(cwd, ".agents", "agents"), SourceProject},
		{filepath.Join(cwd, ".kit", "agents"), SourceProject},
	}
	if dir := GlobalDir(); dir != "" {
		scopes = append(scopes, scope{dir, SourceUser})
	}

	var sets [][]*Agent
	var errs []string
	for _, s := range scopes {
		loaded, err := LoadFromDir(s.dir)
		if err != nil {
			errs = append(errs, err.Error())
		}
		for _, a := range loaded {
			a.Source = s.source
		}
		sets = append(sets, loaded)
	}
	sets = append(sets, Builtins())

	result := Merge(sets...)

	if len(errs) > 0 {
		return result, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return result, nil
}

// Merge combines agent sets ordered from highest to lowest precedence. The
// first definition of a name wins; later definitions of the same name are
// dropped. Agents marked Disabled are removed after precedence resolution,
// so a high-precedence disabled definition also removes anything it shadows.
func Merge(sets ...[]*Agent) []*Agent {
	seen := make(map[string]struct{})
	var merged []*Agent
	for _, set := range sets {
		for _, a := range set {
			if a == nil || a.Name == "" {
				continue
			}
			if _, ok := seen[a.Name]; ok {
				continue
			}
			seen[a.Name] = struct{}{}
			merged = append(merged, a)
		}
	}

	result := merged[:0]
	for _, a := range merged {
		if a.Disabled {
			continue
		}
		result = append(result, a)
	}
	return result
}

// Builtins returns the built-in agents shipped with Kit. They can be
// overridden (or disabled) by user- or project-level definitions with the
// same name.
func Builtins() []*Agent {
	return []*Agent{
		{
			Name:         "general",
			Description:  "General-purpose agent for researching complex questions and executing multi-step tasks",
			Source:       SourceBuiltin,
			SystemPrompt: `You are a general-purpose agent handling a delegated task. Work autonomously: research, plan, and execute the task end to end without asking for clarification. When finished, report a single complete answer that includes concrete details (file paths, commands, findings) the delegating agent can act on directly.`,
		},
		{
			Name:         "explore",
			Description:  "Read-only agent specialized for exploring codebases: finds files, searches code, and answers questions about structure and behavior",
			Tools:        []string{"read", "grep", "find", "ls"},
			Source:       SourceBuiltin,
			SystemPrompt: `You are a read-only codebase exploration agent. Find files, search code, and answer questions about the project's structure and behavior. You cannot modify anything: never attempt to write, edit, or execute state-changing commands. Report concise findings with concrete file paths and, where useful, line references.`,
		},
	}
}

// splitFrontmatter separates YAML frontmatter from the markdown body.
// Frontmatter must start on the first line with "---" and end with a "---"
// line. When no frontmatter is present, the whole content is the body.
func splitFrontmatter(content string) (fm string, body string) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", content
	}
	rest := normalized[len("---\n"):]
	if before, after, ok := strings.Cut(rest, "\n---\n"); ok {
		return before, after
	}
	if trimmed, ok := strings.CutSuffix(rest, "\n---"); ok {
		return trimmed, ""
	}
	return "", content
}
