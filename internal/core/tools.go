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
func CodingTools() []fantasy.AgentTool {
	return []fantasy.AgentTool{
		NewBashTool(),
		NewReadTool(),
		NewWriteTool(),
		NewEditTool(),
	}
}

// ReadOnlyTools returns tools for read-only exploration:
// read, grep, find, ls. This matches pi's readOnlyTools collection.
func ReadOnlyTools() []fantasy.AgentTool {
	return []fantasy.AgentTool{
		NewReadTool(),
		NewGrepTool(),
		NewFindTool(),
		NewLsTool(),
	}
}

// AllTools returns all available core tools.
func AllTools() []fantasy.AgentTool {
	return []fantasy.AgentTool{
		NewBashTool(),
		NewReadTool(),
		NewWriteTool(),
		NewEditTool(),
		NewGrepTool(),
		NewFindTool(),
		NewLsTool(),
	}
}
