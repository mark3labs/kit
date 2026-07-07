package kit

import (
	"fmt"
	"strings"

	"github.com/mark3labs/kit/internal/agents"
	"github.com/mark3labs/kit/internal/core"
)

// ==== Named Agent Types ====

// AgentDefinition is a named, reusable subagent preset discovered from
// markdown definition files or built into Kit. The definition's system
// prompt, model, tool allowlist, temperature, and timeout act as defaults
// when a subagent is spawned with [SubagentConfig].Agent set to its name.
//
// Definition files are markdown with YAML frontmatter. The filename (minus
// the .md extension) is the agent name, and the body is the system prompt:
//
//	---
//	description: Reviews code for quality and best practices   # required
//	model: anthropic/claude-sonnet-4                           # optional
//	tools: [read, grep, find, ls]                              # optional allowlist
//	temperature: 0.1                                           # optional
//	timeout: 300                                               # optional, seconds
//	hidden: false                                              # optional
//	disabled: false                                            # optional
//	---
//	You are in code review mode. Focus on...
type AgentDefinition = agents.Agent

// ==== Named Agent Functions ====

// LoadAgentDefinitions discovers named agents from the standard locations,
// applies precedence, and filters out disabled agents:
//
//  1. <cwd>/.agents/agents/ (project, cross-client convention)
//  2. <cwd>/.kit/agents/ (project, Kit-specific)
//  3. $XDG_CONFIG_HOME/kit/agents/ (user, default ~/.config/kit/agents/)
//  4. Built-in agents ("general", "explore")
//
// Higher entries take precedence on name collisions. cwd is the working
// directory for project-local discovery; if empty, the current working
// directory is used. Per-file parse failures do not abort discovery — the
// aggregated error is returned alongside the successfully loaded agents.
func LoadAgentDefinitions(cwd string) ([]*AgentDefinition, error) {
	return agents.LoadAgents(cwd)
}

// GetAgents returns the named agent definitions discovered at construction
// time (including built-ins, excluding disabled ones). The returned slice is
// a snapshot — mutating it does not affect Kit state. Returns nil when agent
// discovery was disabled via [Options].NoAgents.
func (m *Kit) GetAgents() []*AgentDefinition {
	if len(m.namedAgents) == 0 {
		return nil
	}
	out := make([]*AgentDefinition, len(m.namedAgents))
	copy(out, m.namedAgents)
	return out
}

// GetAgent returns the named agent definition with the given name, or
// (nil, false) when no such agent was discovered.
func (m *Kit) GetAgent(name string) (*AgentDefinition, bool) {
	for _, a := range m.namedAgents {
		if a.Name == name {
			return a, true
		}
	}
	return nil, false
}

// ==== Internal helpers ====

// namedAgentSpecs converts discovered agent definitions into the summaries
// advertised in the subagent tool description. Hidden agents are omitted —
// they stay resolvable by name but are not advertised to the LLM.
func namedAgentSpecs(defs []*AgentDefinition) []core.NamedAgentSpec {
	var specs []core.NamedAgentSpec
	for _, a := range defs {
		if a.Hidden {
			continue
		}
		specs = append(specs, core.NamedAgentSpec{
			Name:        a.Name,
			Description: a.Description,
			Tools:       a.Tools,
		})
	}
	return specs
}

// resolveAgentDefinition looks up a named agent and merges its presets into
// cfg. Explicitly set cfg fields win over the definition's values. It
// reports whether the definition restricted the tool set (callers must then
// prevent the child from re-loading MCP servers, which would bypass the
// allowlist).
func (m *Kit) resolveAgentDefinition(cfg *SubagentConfig) (restricted bool, err error) {
	def, ok := m.GetAgent(cfg.Agent)
	if !ok {
		return false, fmt.Errorf("unknown agent %q (available: %s)",
			cfg.Agent, strings.Join(m.agentNames(), ", "))
	}
	applyAgentDefinition(cfg, def)
	if len(def.Tools) > 0 {
		base := cfg.Tools
		if base == nil {
			base = SubagentTools()
		}
		cfg.Tools = filterToolsByName(base, def.Tools)
		restricted = true
	}
	return restricted, nil
}

// agentNames returns the names of all non-hidden discovered agents.
func (m *Kit) agentNames() []string {
	var names []string
	for _, a := range m.namedAgents {
		if a.Hidden {
			continue
		}
		names = append(names, a.Name)
	}
	return names
}

// applyAgentDefinition merges a named agent definition's presets into cfg.
// Fields the caller already set (model, system prompt, timeout, temperature)
// are left untouched so explicit arguments override the definition.
func applyAgentDefinition(cfg *SubagentConfig, def *AgentDefinition) {
	if cfg.Model == "" {
		cfg.Model = def.Model
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = def.SystemPrompt
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = def.Timeout
	}
	if cfg.Temperature == nil {
		cfg.Temperature = def.Temperature
	}
}

// filterToolsByName returns the tools whose names appear in the allowlist,
// preserving order. Allowlisted names with no matching tool are ignored.
// The result is never nil so an empty allowlist match still counts as an
// explicit (empty) tool set rather than "use defaults".
func filterToolsByName(tools []Tool, names []string) []Tool {
	allowed := make(map[string]struct{}, len(names))
	for _, n := range names {
		allowed[n] = struct{}{}
	}
	filtered := make([]Tool, 0, len(names))
	for _, t := range tools {
		if _, ok := allowed[t.Info().Name]; ok {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
