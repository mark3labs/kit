package agent

import (
	"context"
	"fmt"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/tools"
)

// mcpAgentTool adapts an tools.MCPTool to the fantasy.AgentTool interface.
// This keeps the fantasy dependency confined to the agent layer — the tools
// package is a pure MCP client library with no LLM framework dependency.
type mcpAgentTool struct {
	tool            tools.MCPTool
	manager         *tools.MCPToolManager
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
func (t *mcpAgentTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	result, err := t.manager.ExecuteTool(ctx, t.tool.Name, call.Input)
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("mcp tool execution failed: %w", err)
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
			tool:    t,
			manager: manager,
		}
	}
	return agentTools
}
