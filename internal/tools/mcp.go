package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"charm.land/fantasy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/kit/internal/config"
)

// MCPToolManager manages MCP (Model Context Protocol) tools and clients across multiple servers.
// It provides a unified interface for loading, managing, and executing tools from various MCP servers,
// including stdio, SSE, streamable HTTP, and built-in server types. The manager handles connection
// pooling, health checks, tool name prefixing to avoid conflicts, and sampling support for LLM interactions.
// Thread-safe for concurrent tool invocations.
type MCPToolManager struct {
	connectionPool *MCPConnectionPool
	tools          []fantasy.AgentTool
	toolMap        map[string]*toolMapping // maps prefixed tool names to their server and original name
	model          fantasy.LanguageModel   // LLM model for sampling
	config         *config.Config
	debug          bool
	debugLogger    DebugLogger
}

// toolMapping stores the mapping between prefixed tool names and their original details
type toolMapping struct {
	serverName   string
	originalName string
	serverConfig config.MCPServerConfig
	manager      *MCPToolManager
}

// NewMCPToolManager creates a new MCP tool manager instance.
// Returns an initialized manager with empty tool collections ready to load tools from MCP servers.
// The manager must be configured with SetModel and LoadTools before use.
func NewMCPToolManager() *MCPToolManager {
	return &MCPToolManager{
		tools:   make([]fantasy.AgentTool, 0),
		toolMap: make(map[string]*toolMapping),
	}
}

// SetModel sets the LLM model for sampling support.
// The model is used when MCP servers request sampling operations, allowing them to
// leverage the host's LLM capabilities for text generation tasks.
// This method should be called before LoadTools if any MCP servers require sampling support.
func (m *MCPToolManager) SetModel(model fantasy.LanguageModel) {
	m.model = model
}

// SetDebugLogger sets the debug logger for the tool manager.
// The logger will be used to output detailed debugging information about MCP connections,
// tool loading, and execution. If a connection pool exists, it will also be configured
// to use the same logger for consistent debugging output.
func (m *MCPToolManager) SetDebugLogger(logger DebugLogger) {
	m.debugLogger = logger
	if m.connectionPool != nil {
		m.connectionPool.SetDebugLogger(logger)
	}
}

// LoadTools loads tools from all configured MCP servers based on the provided configuration.
// It initializes the connection pool, connects to each configured server, and loads their tools.
// Tools from different servers are prefixed with the server name to avoid naming conflicts.
// Returns an error only if all configured servers fail to load; partial failures are logged as warnings.
// This method is thread-safe and idempotent.
func (m *MCPToolManager) LoadTools(ctx context.Context, config *config.Config) error {
	// Initialize connection pool
	m.config = config
	m.debug = config.Debug
	if m.debugLogger == nil {
		m.debugLogger = NewSimpleDebugLogger(config.Debug)
	}
	m.connectionPool = NewMCPConnectionPool(DefaultConnectionPoolConfig(), m.model, config.Debug)
	m.connectionPool.SetDebugLogger(m.debugLogger)

	var loadErrors []string

	for serverName, serverConfig := range config.MCPServers {
		if err := m.loadServerTools(ctx, serverName, serverConfig); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("server %s: %v", serverName, err))
			fmt.Printf("Warning: Failed to load MCP server '%s': %v\n", serverName, err)
			continue
		}
	}

	// If all servers failed to load, return an error
	if len(loadErrors) == len(config.MCPServers) && len(config.MCPServers) > 0 {
		return fmt.Errorf("all MCP servers failed to load: %s", strings.Join(loadErrors, "; "))
	}

	return nil
}

// loadServerTools loads tools from a single MCP server
func (m *MCPToolManager) loadServerTools(ctx context.Context, serverName string, serverConfig config.MCPServerConfig) error {
	// Add debug logging
	m.debugLogConnectionInfo(serverName, serverConfig)

	// Get connection from pool
	conn, err := m.connectionPool.GetConnection(ctx, serverName, serverConfig)
	if err != nil {
		return fmt.Errorf("failed to get connection from pool: %v", err)
	}

	// Get tools from this server
	listResults, err := conn.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		// Handle connection error
		m.connectionPool.HandleConnectionError(serverName, err)
		return fmt.Errorf("failed to list tools: %v", err)
	}

	// Create name set for allowed tools
	var nameSet map[string]struct{}
	if len(serverConfig.AllowedTools) > 0 {
		nameSet = make(map[string]struct{})
		for _, name := range serverConfig.AllowedTools {
			nameSet[name] = struct{}{}
		}
	}

	// Convert MCP tools to fantasy AgentTools with prefixed names
	for _, mcpTool := range listResults.Tools {
		// Filter tools based on allowedTools/excludedTools
		if len(serverConfig.AllowedTools) > 0 {
			if _, ok := nameSet[mcpTool.Name]; !ok {
				continue
			}
		}

		// Check if tool should be excluded
		if m.shouldExcludeTool(mcpTool.Name, serverConfig) {
			continue
		}

		// Convert MCP InputSchema to map[string]any for fantasy ToolInfo
		marshaledSchema, err := json.Marshal(mcpTool.InputSchema)
		if err != nil {
			return fmt.Errorf("conv mcp tool input schema fail(marshal): %w, tool name: %s", err, mcpTool.Name)
		}

		// Fix for JSON Schema draft-07 vs draft-04 compatibility
		marshaledSchema = convertExclusiveBoundsToBoolean(marshaledSchema)

		// Parse into map[string]any for fantasy's parameters format
		var schemaMap map[string]any
		if err := json.Unmarshal(marshaledSchema, &schemaMap); err != nil {
			return fmt.Errorf("conv mcp tool input schema fail(unmarshal): %w, tool name: %s", err, mcpTool.Name)
		}

		// Extract properties and required from the schema
		parameters := make(map[string]any)
		required := []string{}

		if props, ok := schemaMap["properties"].(map[string]any); ok {
			parameters = props
		}

		// Fix for issue #89: Ensure object schemas have a properties field.
		// When schema type is "object" with no properties, we keep the
		// empty parameters map — fantasy handles this fine.

		if req, ok := schemaMap["required"].([]any); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					required = append(required, s)
				}
			}
		}

		// Create prefixed tool name
		prefixedName := fmt.Sprintf("%s__%s", serverName, mcpTool.Name)

		// Create tool mapping
		mapping := &toolMapping{
			serverName:   serverName,
			originalName: mcpTool.Name,
			serverConfig: serverConfig,
			manager:      m,
		}
		m.toolMap[prefixedName] = mapping

		// Create fantasy AgentTool
		fantasyTool := &mcpFantasyTool{
			toolInfo: fantasy.ToolInfo{
				Name:        prefixedName,
				Description: mcpTool.Description,
				Parameters:  parameters,
				Required:    required,
			},
			mapping: mapping,
		}

		m.tools = append(m.tools, fantasyTool)
	}

	return nil
}

// GetTools returns all loaded tools as fantasy AgentTools from all configured MCP servers.
// Tools are returned with their prefixed names (serverName__toolName) to ensure uniqueness.
func (m *MCPToolManager) GetTools() []fantasy.AgentTool {
	return m.tools
}

// GetLoadedServerNames returns the names of all successfully loaded MCP servers.
// This includes servers that are currently connected and have had their tools loaded,
// regardless of their current health status. Useful for debugging and status reporting.
func (m *MCPToolManager) GetLoadedServerNames() []string {
	var names []string
	for serverName := range m.connectionPool.GetClients() {
		names = append(names, serverName)
	}
	return names
}

// Close closes all MCP client connections and cleans up resources.
// This method should be called when the tool manager is no longer needed to ensure
// proper cleanup of stdio processes, network connections, and other resources.
// It is safe to call Close multiple times.
func (m *MCPToolManager) Close() error {
	return m.connectionPool.Close()
}

// shouldExcludeTool determines if a tool should be excluded based on excludedTools
func (m *MCPToolManager) shouldExcludeTool(toolName string, serverConfig config.MCPServerConfig) bool {
	if len(serverConfig.ExcludedTools) > 0 {
		if slices.Contains(serverConfig.ExcludedTools, toolName) {
			return true
		}
	}
	return false
}

// debugLogConnectionInfo logs detailed connection information for debugging
func (m *MCPToolManager) debugLogConnectionInfo(serverName string, serverConfig config.MCPServerConfig) {
	if m.debugLogger == nil || !m.debugLogger.IsDebugEnabled() {
		return
	}

	m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] Connecting to MCP server: %s", serverName))
	m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] Transport type: %s", serverConfig.GetTransportType()))

	switch serverConfig.GetTransportType() {
	case "stdio":
		if len(serverConfig.Command) > 0 {
			m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] Command: %s %v", serverConfig.Command[0], serverConfig.Command[1:]))
		}
		if len(serverConfig.Environment) > 0 {
			m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] Environment variables: %d", len(serverConfig.Environment)))
		}
	case "sse", "streamable":
		m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] URL: %s", serverConfig.URL))
		if len(serverConfig.Headers) > 0 {
			m.debugLogger.LogDebug(fmt.Sprintf("[DEBUG] Headers: %v", serverConfig.Headers))
		}
	}
}

// convertExclusiveBoundsToBoolean converts JSON Schema draft-07 style exclusive bounds
// (where exclusiveMinimum/exclusiveMaximum are numbers) to draft-04 style
// (where they are booleans that modify minimum/maximum).
func convertExclusiveBoundsToBoolean(schemaJSON []byte) []byte {
	var data map[string]any
	if err := json.Unmarshal(schemaJSON, &data); err != nil {
		return schemaJSON
	}

	convertSchemaRecursive(data)

	result, err := json.Marshal(data)
	if err != nil {
		return schemaJSON
	}
	return result
}

// convertSchemaRecursive recursively processes a schema map to:
//   - Convert numeric exclusiveMinimum/exclusiveMaximum to boolean format (draft-07 → draft-04)
//   - Remove null "required" fields that cause OpenAI API validation errors
func convertSchemaRecursive(schema map[string]any) {
	if exMin, ok := schema["exclusiveMinimum"]; ok {
		if num, isNum := exMin.(float64); isNum {
			schema["minimum"] = num
			schema["exclusiveMinimum"] = true
		}
	}

	if exMax, ok := schema["exclusiveMaximum"]; ok {
		if num, isNum := exMax.(float64); isNum {
			schema["maximum"] = num
			schema["exclusiveMaximum"] = true
		}
	}

	// Fix null "required" fields — OpenAI rejects "required": null,
	// it must be an array or absent entirely.
	if req, exists := schema["required"]; exists {
		if req == nil {
			delete(schema, "required")
		} else if _, isArr := req.([]any); !isArr {
			// Not an array — remove invalid value
			delete(schema, "required")
		}
	}

	if props, ok := schema["properties"].(map[string]any); ok {
		for _, prop := range props {
			if propSchema, ok := prop.(map[string]any); ok {
				convertSchemaRecursive(propSchema)
			}
		}
	}

	if items, ok := schema["items"].(map[string]any); ok {
		convertSchemaRecursive(items)
	}

	if addProps, ok := schema["additionalProperties"].(map[string]any); ok {
		convertSchemaRecursive(addProps)
	}

	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		if arr, ok := schema[key].([]any); ok {
			for _, item := range arr {
				if itemSchema, ok := item.(map[string]any); ok {
					convertSchemaRecursive(itemSchema)
				}
			}
		}
	}

	if not, ok := schema["not"].(map[string]any); ok {
		convertSchemaRecursive(not)
	}
}
