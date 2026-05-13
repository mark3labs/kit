package agent

import (
	"context"
	"fmt"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/tools"
)

// mcpExecutor is the subset of *tools.MCPToolManager that the adapter
// actually uses. Extracted as an interface so the adapter is unit-testable
// without constructing a full manager + connection pool.
type mcpExecutor interface {
	ExecuteTool(ctx context.Context, prefixedName, inputJSON string) (*tools.MCPToolResult, error)
}

// mcpAgentTool adapts an tools.MCPTool to the fantasy.AgentTool interface.
// This keeps the fantasy dependency confined to the agent layer — the tools
// package is a pure MCP client library with no LLM framework dependency.
type mcpAgentTool struct {
	tool            tools.MCPTool
	exec            mcpExecutor
	providerOptions fantasy.ProviderOptions
}

// Info returns the fantasy tool info including name, description, and parameter schema.
func (t *mcpAgentTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{
		Name:        t.tool.Name,
		Description: t.tool.Description,
		Parameters:  t.tool.Parameters,
		Required:    t.tool.Required,
	}
}

// Run executes the MCP tool by delegating to the MCPToolManager.
//
// MCP-side failures (JSON-RPC protocol errors, transport failures, schema
// validation rejections from the server) are surfaced to the model as soft
// tool errors rather than escalated to a critical agent error. This matches
// the contract that native Kit tools follow via kit.ErrorResult(...) and
// lets the model self-correct (e.g. retry with a fixed argument shape) or
// give up gracefully rather than aborting the turn mid-run.
//
// Context cancellation is the one exception: if the caller cancelled the
// context the turn was aborted intentionally, so we propagate the ctx error
// to let the agent loop unwind cleanly.
func (t *mcpAgentTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	result, err := t.exec.ExecuteTool(ctx, t.tool.Name, call.Input)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fantasy.ToolResponse{}, ctxErr
		}
		return fantasy.NewTextErrorResponse(
			fmt.Sprintf("MCP tool %q failed: %s", t.tool.Name, err.Error()),
		), nil
	}

	if result.IsError {
		return fantasy.NewTextErrorResponse(result.Content), nil
	}
	return fantasy.NewTextResponse(result.Content), nil
}

// ProviderOptions returns provider-specific options for this tool.
func (t *mcpAgentTool) ProviderOptions() fantasy.ProviderOptions {
	return t.providerOptions
}

// SetProviderOptions sets provider-specific options for this tool.
func (t *mcpAgentTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.providerOptions = opts
}

// mcpToolsToAgentTools converts a slice of MCPTool to fantasy.AgentTool
// implementations that route execution through the MCPToolManager.
func mcpToolsToAgentTools(mcpTools []tools.MCPTool, manager *tools.MCPToolManager) []fantasy.AgentTool {
	agentTools := make([]fantasy.AgentTool, len(mcpTools))
	for i, t := range mcpTools {
		agentTools[i] = &mcpAgentTool{
			tool: t,
			exec: manager,
		}
	}
	return agentTools
}
