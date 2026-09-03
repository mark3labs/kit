// Package core provides the built-in core tools for KIT's coding agent.
// These tools are direct fantasy.AgentTool implementations — no MCP layer,
// no JSON-RPC, no serialization overhead. Core tool set: shell, read, write,
// edit, grep, find, ls.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"charm.land/fantasy"
)

// ToolOption configures tool behavior.
type ToolOption func(*ToolConfig)

// ToolConfig holds configuration for tool construction.
type ToolConfig struct {
	WorkDir string
	// NamedAgents lists discovered named agent definitions advertised in
	// the subagent tool description. Only the subagent tool consumes this.
	NamedAgents []NamedAgentSpec
	// ShellTimeout overrides the default per-call timeout for the shell tool.
	// Zero uses the built-in default (120s). Only the shell tool consumes this.
	ShellTimeout time.Duration
	// ShellMaxTimeout overrides the ceiling a shell tool call may request via
	// its timeout argument. Zero uses the built-in default (600s). Only the
	// shell tool consumes this.
	ShellMaxTimeout time.Duration
	// Shell is the shell the shell tool runs a command string through, plus
	// its own leading arguments, e.g. ["bash"] or ["busybox", "ash"]. Nil or
	// empty uses the built-in default ["bash"]. This exists so KIT can run
	// on images that do not ship bash (Alpine, distroless and similar). Only
	// the shell tool consumes this.
	Shell []string
}

// WithWorkDir sets the working directory for file-based tools.
// If empty, os.Getwd() is used at execution time.
func WithWorkDir(dir string) ToolOption {
	return func(c *ToolConfig) {
		c.WorkDir = dir
	}
}

// WithNamedAgents advertises named agent definitions in the subagent tool
// description so the LLM can delegate tasks to them by name.
func WithNamedAgents(agents ...NamedAgentSpec) ToolOption {
	return func(c *ToolConfig) {
		c.NamedAgents = append(c.NamedAgents, agents...)
	}
}

// WithShellTimeout sets the default per-call timeout for the shell tool.
// A non-positive duration leaves the built-in default (120s) in place.
func WithShellTimeout(d time.Duration) ToolOption {
	return func(c *ToolConfig) {
		if d > 0 {
			c.ShellTimeout = d
		}
	}
}

// WithShellMaxTimeout sets the maximum timeout a shell tool call may request.
// A non-positive duration leaves the built-in default (600s) in place.
func WithShellMaxTimeout(d time.Duration) ToolOption {
	return func(c *ToolConfig) {
		if d > 0 {
			c.ShellMaxTimeout = d
		}
	}
}

// WithShell sets the shell the shell tool runs command strings through: the
// shell plus its own leading arguments, e.g. ["bash"] or ["busybox", "ash"].
// A nil or empty vector leaves the built-in default ["bash"] in place, so an
// installation that sets nothing executes exactly what it did before.
func WithShell(argv []string) ToolOption {
	return func(c *ToolConfig) {
		if len(argv) > 0 {
			c.Shell = argv
		}
	}
}

// ApplyOptions applies the given ToolOptions to a ToolConfig and returns it.
func ApplyOptions(opts []ToolOption) ToolConfig {
	var cfg ToolConfig
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

// coreTool is the base implementation for all core tools. It implements
// the fantasy.AgentTool interface with typed parameters and direct execution.
type coreTool struct {
	info            fantasy.ToolInfo
	handler         func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error)
	providerOptions fantasy.ProviderOptions
}

func (t *coreTool) Info() fantasy.ToolInfo                          { return t.info }
func (t *coreTool) ProviderOptions() fantasy.ProviderOptions        { return t.providerOptions }
func (t *coreTool) SetProviderOptions(opts fantasy.ProviderOptions) { t.providerOptions = opts }

func (t *coreTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return t.handler(ctx, call)
}

// parseArgs unmarshals the JSON input from a tool call into the target struct.
func parseArgs(input string, target any) error {
	if input == "" || input == "{}" {
		return nil
	}
	if err := json.Unmarshal([]byte(input), target); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

type initTool func(...ToolOption) fantasy.AgentTool

// The command-execution tool is a shell tool, and ShellToolName is its name:
// in the registry, in the tool definition sent to the model, and in a session
// transcript. It does not vary with the configured shell.
//
// LegacyShellToolName is the name the tool had before the shell became
// configurable. It appears in existing user configuration and in existing
// sessions, so it is accepted wherever a user names the tool; see
// NormalizeCoreToolName.
const (
	ShellToolName       = "shell"
	LegacyShellToolName = "bash"
)

// NormalizeCoreToolName maps a user-supplied core tool name onto its registry
// key, so that the earlier name for the shell tool keeps selecting it. An
// unrecognised name in include-core-tools or exclude-core-tools produces a
// warning and no effect, which is what this avoids.
func NormalizeCoreToolName(name string) string {
	if name == LegacyShellToolName {
		return ShellToolName
	}
	return name
}

var coreTools = map[string]initTool{
	ShellToolName: NewShellTool,
	"read":        NewReadTool,
	"write":       NewWriteTool,
	"edit":        NewEditTool,
	"grep":        NewGrepTool,
	"find":        NewFindTool,
	"ls":          NewLsTool,
	"subagent":    NewSubagentTool,
}

// ListAllCoreToolNames always returns the full list of available core
// tools. It can be used to validate a user provided tool list.
func ListAllCoreToolNames() []string {
	return slices.Collect(maps.Keys(coreTools))
}

// CodingTools returns the default set of core tools for a coding agent:
// shell, read, write, edit.
func CodingTools(opts ...ToolOption) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		NewShellTool(opts...),
		NewReadTool(opts...),
		NewWriteTool(opts...),
		NewEditTool(opts...),
	}
}

// ReadOnlyTools returns tools for read-only exploration:
// read, grep, find, ls.
func ReadOnlyTools(opts ...ToolOption) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		NewReadTool(opts...),
		NewGrepTool(opts...),
		NewFindTool(opts...),
		NewLsTool(opts...),
	}
}

// SubagentTools returns all core tools except subagent. This prevents
// infinite recursion when a subagent is itself a Kit instance.
func SubagentTools(opts ...ToolOption) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		NewShellTool(opts...),
		NewReadTool(opts...),
		NewWriteTool(opts...),
		NewEditTool(opts...),
		NewGrepTool(opts...),
		NewFindTool(opts...),
		NewLsTool(opts...),
	}
}

// ListedTools builds the named core tools. Names are normalized first, so a
// list carrying the shell tool's earlier name still selects it, and a name
// that matches no core tool is skipped rather than dereferenced.
func ListedTools(toolList []string, opts ...ToolOption) []fantasy.AgentTool {
	var result = []fantasy.AgentTool{}
	for _, t := range toolList {
		init, ok := coreTools[NormalizeCoreToolName(t)]
		if !ok {
			continue
		}
		result = append(result, init(opts...))
	}
	return result
}

// AllTools returns all available core tools.
func AllTools(opts ...ToolOption) []fantasy.AgentTool {
	return append(SubagentTools(opts...), NewSubagentTool(opts...))
}
