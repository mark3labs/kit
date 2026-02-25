package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"charm.land/fantasy"
	"github.com/mark3labs/mcp-go/mcp"
)

// mcpFantasyTool adapts an MCP tool to the fantasy.AgentTool interface.
// It bridges the MCP tool protocol with fantasy's agent tool system, handling
// name prefixing, schema conversion, connection pooling, and result marshaling.
type mcpFantasyTool struct {
	toolInfo        fantasy.ToolInfo
	mapping         *toolMapping
	providerOptions fantasy.ProviderOptions
}

// Info returns the fantasy tool info including name, description, and parameter schema.
func (t *mcpFantasyTool) Info() fantasy.ToolInfo {
	return t.toolInfo
}

// Run executes the MCP tool by routing through the connection pool.
// It maps the prefixed tool name back to the original name, retrieves a healthy
// connection, invokes the tool, and converts the MCP result to a fantasy ToolResponse.
func (t *mcpFantasyTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	// Parse and validate JSON arguments
	var arguments any
	input := call.Input
	if input == "" || input == "{}" {
		arguments = nil
	} else {
		var temp any
		if err := json.Unmarshal([]byte(input), &temp); err != nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid JSON arguments: %v", err)), nil
		}
		arguments = json.RawMessage(input)
	}

	// Get connection from pool with health check
	conn, err := t.mapping.manager.connectionPool.GetConnectionWithHealthCheck(
		ctx, t.mapping.serverName, t.mapping.serverConfig,
	)
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("failed to get healthy connection from pool: %w", err)
	}

	// Call the MCP tool using the original (unprefixed) name
	result, err := conn.client.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: mcp.CallToolParams{
			Name:      t.mapping.originalName,
			Arguments: arguments,
		},
	})
	if err != nil {
		// Mark connection as unhealthy for automatic recovery
		t.mapping.manager.connectionPool.HandleConnectionError(t.mapping.serverName, err)
		return fantasy.ToolResponse{}, fmt.Errorf("failed to call mcp tool: %w", err)
	}

	// Marshal the MCP result to JSON string
	marshaledResult, err := json.Marshal(result)
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("failed to marshal mcp tool result: %w", err)
	}

	// Return as text response, preserving error status from MCP
	if result.IsError {
		return fantasy.NewTextErrorResponse(string(marshaledResult)), nil
	}
	return fantasy.NewTextResponse(string(marshaledResult)), nil
}

// ProviderOptions returns provider-specific options for this tool.
func (t *mcpFantasyTool) ProviderOptions() fantasy.ProviderOptions {
	return t.providerOptions
}

// SetProviderOptions sets provider-specific options for this tool.
func (t *mcpFantasyTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.providerOptions = opts
}
