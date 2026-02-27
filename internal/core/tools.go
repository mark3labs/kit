// Package core provides the built-in core tools for KIT's coding agent.
// These tools are direct fantasy.AgentTool implementations — no MCP layer,
// no JSON-RPC, no serialization overhead. They match the pi coding agent's
// core tool set: bash, read, write, edit, grep, find, ls.
package core

import (
	"context"
	"encoding/json"
	"fmt"

	"charm.land/fantasy"
)

// ToolOption configures tool behavior.
type ToolOption func(*ToolConfig)

// ToolConfig holds configuration for tool construction.
type ToolConfig struct {
	WorkDir string
}

// WithWorkDir sets the working directory for file-based tools.
// If empty, os.Getwd() is used at execution time.
func WithWorkDir(dir string) ToolOption {
	return func(c *ToolConfig) {
		c.WorkDir = dir
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

// CodingTools returns the default set of core tools for a coding agent:
// bash, read, write, edit. This matches pi's codingTools collection.
func CodingTools(opts ...ToolOption) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		NewBashTool(opts...),
		NewReadTool(opts...),
		NewWriteTool(opts...),
		NewEditTool(opts...),
	}
}

// ReadOnlyTools returns tools for read-only exploration:
// read, grep, find, ls. This matches pi's readOnlyTools collection.
func ReadOnlyTools(opts ...ToolOption) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		NewReadTool(opts...),
		NewGrepTool(opts...),
		NewFindTool(opts...),
		NewLsTool(opts...),
	}
}

// AllTools returns all available core tools.
func AllTools(opts ...ToolOption) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		NewBashTool(opts...),
		NewReadTool(opts...),
		NewWriteTool(opts...),
		NewEditTool(opts...),
		NewGrepTool(opts...),
		NewFindTool(opts...),
		NewLsTool(opts...),
	}
}
