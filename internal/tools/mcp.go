package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPTool represents a tool discovered from an MCP server. It contains all
// the metadata needed to present the tool to an LLM (name, description, JSON
// schema) plus the server origin information needed to execute it.
type MCPTool struct {
	// Name is the prefixed tool name: "serverName__toolName".
	Name string
	// Description is the human-readable tool description.
	Description string
	// Parameters is the JSON Schema properties for the tool's input.
	Parameters map[string]any
	// Required lists the required parameter names.
	Required []string
	// ServerName is the MCP server this tool belongs to.
	ServerName string
	// OriginalName is the unprefixed tool name on the MCP server.
	OriginalName string
}

// MCPToolResult is the result of executing an MCP tool via ExecuteTool.
type MCPToolResult struct {
	// Content is the JSON-encoded result from the MCP server.
	Content string
	// IsError indicates the MCP server reported a tool-level error.
	IsError bool
}

// MCPPrompt represents a prompt discovered from an MCP server.
type MCPPrompt struct {
	// Name is the prompt name on the MCP server.
	Name string
	// Description is the human-readable prompt description.
	Description string
	// Arguments lists the prompt's expected arguments.
	Arguments []MCPPromptArgument
	// ServerName is the MCP server this prompt belongs to.
	ServerName string
}

// MCPPromptArgument describes an argument that a prompt template can accept.
type MCPPromptArgument struct {
	// Name is the argument name.
	Name string
	// Description is a human-readable description.
	Description string
	// Required indicates whether this argument must be provided.
	Required bool
}

// MCPPromptMessage is a single message returned by a prompt expansion.
type MCPPromptMessage struct {
	// Role is "user" or "assistant".
	Role string
	// Content is the text content of the message.
	Content string
}

// MCPPromptResult is the result of expanding an MCP prompt via GetPrompt.
type MCPPromptResult struct {
	// Description is an optional description returned by the server.
	Description string
	// Messages contains the expanded prompt messages.
	Messages []MCPPromptMessage
}

// MCPToolManager manages MCP (Model Context Protocol) tools and clients across multiple servers.
// It provides a unified interface for loading, managing, and executing tools from various MCP servers,
// including stdio, SSE, streamable HTTP, and built-in server types. The manager handles connection
// pooling, health checks, tool name prefixing to avoid conflicts, and OAuth re-authorization.
// Thread-safe for concurrent tool invocations.
type MCPToolManager struct {
	connectionPool    *MCPConnectionPool
	tools             []MCPTool
	toolMap           map[string]*toolMapping // maps prefixed tool names to their server and original name
	prompts           []MCPPrompt             // prompts discovered from all servers
	mu                sync.Mutex              // protects tools, toolMap, and prompts during parallel loading
	authHandler       MCPAuthHandler          // OAuth handler for remote servers (nil = no OAuth)
	tokenStoreFactory TokenStoreFactory       // factory for creating per-server token stores (nil = default FileTokenStore)
	config            *config.Config
	debug             bool
	debugLogger       DebugLogger

	// onServerLoaded, if non-nil, is called when each server finishes loading.
	// Called with server name, tool count, and error (nil on success).
	onServerLoaded func(serverName string, toolCount int, err error)

	// onToolsChanged, if non-nil, is called after AddServer or RemoveServer
	// mutates the tool list. The agent layer uses this to trigger a rebuild
	// so the LLM sees the updated tools.
	onToolsChanged func()
}

// toolMapping stores the mapping between prefixed tool names and their original details
type toolMapping struct {
	serverName   string
	originalName string
	serverConfig config.MCPServerConfig
}

// NewMCPToolManager creates a new MCP tool manager instance.
// Returns an initialized manager with empty tool collections ready to load tools from MCP servers.
// The manager must be configured with LoadTools before use.
func NewMCPToolManager() *MCPToolManager {
	return &MCPToolManager{
		tools:   make([]MCPTool, 0),
		toolMap: make(map[string]*toolMapping),
	}
}

// SetAuthHandler sets the OAuth handler for remote MCP server authentication.
// When set, remote transports (streamable HTTP, SSE) are configured with OAuth
// support, enabling automatic authorization flows when servers require authentication.
// This method should be called before LoadTools.
func (m *MCPToolManager) SetAuthHandler(handler MCPAuthHandler) {
	m.authHandler = handler
}

// GetAuthHandler returns the OAuth handler for remote MCP server authentication.
// Returns nil if no handler is configured.
func (m *MCPToolManager) GetAuthHandler() MCPAuthHandler {
	return m.authHandler
}

// SetTokenStoreFactory sets a custom factory for creating per-server OAuth token
// stores. When set, the factory is called for each remote MCP server instead of
// using the default file-based token store. This method should be called before
// LoadTools.
func (m *MCPToolManager) SetTokenStoreFactory(factory TokenStoreFactory) {
	m.tokenStoreFactory = factory
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

// SetOnServerLoaded sets the callback that's invoked when each MCP server finishes
// loading. The callback receives the server name, tool count, and any error.
// Call this before LoadTools to receive per-server notifications.
func (m *MCPToolManager) SetOnServerLoaded(cb func(serverName string, toolCount int, err error)) {
	m.onServerLoaded = cb
}

// SetOnToolsChanged sets the callback that's invoked after AddServer or
// RemoveServer mutates the tool list. The agent layer uses this to trigger
// a rebuild so the LLM sees the updated tool set.
func (m *MCPToolManager) SetOnToolsChanged(cb func()) {
	m.onToolsChanged = cb
}

// AddServer connects to a new MCP server at runtime and loads its tools.
// The server's tools are immediately available to the agent after this call.
// Returns the number of tools loaded from the server.
//
// If the connection pool has not been initialised yet (i.e. LoadTools was never
// called), AddServer creates one automatically using the manager's current
// configuration.
//
// Returns an error if a server with the same name is already loaded, or if
// the connection or tool loading fails.
func (m *MCPToolManager) AddServer(ctx context.Context, name string, cfg config.MCPServerConfig) (int, error) {
	m.mu.Lock()
	// Check for duplicate.
	if _, exists := m.toolMap[name+"__"]; exists {
		m.mu.Unlock()
		return 0, fmt.Errorf("MCP server %q is already loaded", name)
	}
	// More thorough duplicate check: scan toolMap for any key with the server prefix.
	prefix := name + "__"
	for k := range m.toolMap {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			m.mu.Unlock()
			return 0, fmt.Errorf("MCP server %q is already loaded", name)
		}
	}
	m.mu.Unlock()

	// Lazily create the connection pool if LoadTools was never called.
	m.ensureConnectionPool()

	count, err := m.loadServerTools(ctx, name, cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to add MCP server %q: %w", name, err)
	}

	// Notify listeners.
	if m.onServerLoaded != nil {
		m.onServerLoaded(name, count, nil)
	}
	if m.onToolsChanged != nil {
		m.onToolsChanged()
	}

	return count, nil
}

// RemoveServer disconnects an MCP server and removes all its tools and prompts.
// After this call the agent will no longer see or be able to call tools from
// the named server. Returns an error if the server is not loaded.
func (m *MCPToolManager) RemoveServer(name string) error {
	prefix := name + "__"

	m.mu.Lock()

	// Check the server actually has tools or prompts loaded.
	found := false
	for k := range m.toolMap {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			found = true
			break
		}
	}
	if !found {
		// Also check prompts — a server might expose only prompts.
		for _, p := range m.prompts {
			if p.ServerName == name {
				found = true
				break
			}
		}
	}
	if !found {
		m.mu.Unlock()
		return fmt.Errorf("MCP server %q is not loaded", name)
	}

	// Remove tools belonging to this server.
	newTools := make([]MCPTool, 0, len(m.tools))
	for _, t := range m.tools {
		if len(t.Name) < len(prefix) || t.Name[:len(prefix)] != prefix {
			newTools = append(newTools, t)
		}
	}
	m.tools = newTools

	// Remove tool mappings.
	for k := range m.toolMap {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(m.toolMap, k)
		}
	}

	// Remove prompts belonging to this server.
	newPrompts := make([]MCPPrompt, 0, len(m.prompts))
	for _, p := range m.prompts {
		if p.ServerName != name {
			newPrompts = append(newPrompts, p)
		}
	}
	m.prompts = newPrompts

	m.mu.Unlock()

	// Close the connection in the pool (best-effort).
	if m.connectionPool != nil {
		_ = m.connectionPool.RemoveConnection(name)
	}

	if m.onToolsChanged != nil {
		m.onToolsChanged()
	}

	return nil
}

// ensureConnectionPool lazily creates a connection pool if one does not exist.
// This allows AddServer to work even if LoadTools was never called.
func (m *MCPToolManager) ensureConnectionPool() {
	if m.connectionPool != nil {
		return
	}
	debug := false
	if m.config != nil {
		debug = m.config.Debug
	}
	if m.debugLogger == nil {
		m.debugLogger = NewSimpleDebugLogger(debug)
	}
	m.connectionPool = NewMCPConnectionPool(DefaultConnectionPoolConfig(), debug, m.authHandler, m.tokenStoreFactory)
	m.connectionPool.SetDebugLogger(m.debugLogger)
}

// LoadTools loads tools from all configured MCP servers based on the provided configuration.
// It initializes the connection pool, connects to each configured server, and loads their tools.
// Tools from different servers are prefixed with the server name to avoid naming conflicts.
// Returns an error only if all configured servers fail to load; partial failures are logged as warnings.
// This method is thread-safe and idempotent.
func (m *MCPToolManager) LoadTools(ctx context.Context, cfg *config.Config) error {
	// Initialize connection pool
	m.config = cfg
	m.debug = cfg.Debug
	if m.debugLogger == nil {
		m.debugLogger = NewSimpleDebugLogger(cfg.Debug)
	}
	m.connectionPool = NewMCPConnectionPool(DefaultConnectionPoolConfig(), cfg.Debug, m.authHandler, m.tokenStoreFactory)
	m.connectionPool.SetDebugLogger(m.debugLogger)

	// Load all servers in parallel. Each server connection (subprocess
	// spawn, MCP initialize handshake, ListTools) is independent and
	// typically dominated by process startup latency. Running them
	// concurrently reduces total wall-clock time from O(n * avg) to
	// O(max).
	type serverResult struct {
		name string
		err  error
	}

	results := make(chan serverResult, len(cfg.MCPServers))
	var wg sync.WaitGroup

	for serverName, serverConfig := range cfg.MCPServers {
		wg.Add(1)
		go func(name string, sc config.MCPServerConfig) {
			defer wg.Done()
			count, err := m.loadServerTools(ctx, name, sc)
			results <- serverResult{name: name, err: err}
			// Notify callback if set (for real-time UI updates).
			if m.onServerLoaded != nil {
				m.onServerLoaded(name, count, err)
			}
		}(serverName, serverConfig)
	}

	// Close results channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	var loadErrors []string
	for r := range results {
		if r.err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("server %s: %v", r.name, r.err))
			fmt.Printf("Warning: Failed to load MCP server '%s': %v\n", r.name, r.err)
		}
	}

	// If all servers failed to load, return an error
	if len(loadErrors) == len(cfg.MCPServers) && len(cfg.MCPServers) > 0 {
		return fmt.Errorf("all MCP servers failed to load: %s", strings.Join(loadErrors, "; "))
	}

	return nil
}

// loadServerTools loads tools from a single MCP server.
// Thread-safe: may be called concurrently for different servers.
// Returns the number of tools loaded from this server, or -1 on error.
func (m *MCPToolManager) loadServerTools(ctx context.Context, serverName string, serverConfig config.MCPServerConfig) (int, error) {
	// Add debug logging
	m.debugLogConnectionInfo(serverName, serverConfig)

	// Get connection from pool
	conn, err := m.connectionPool.GetConnection(ctx, serverName, serverConfig)
	if err != nil {
		return -1, fmt.Errorf("failed to get connection from pool: %v", err)
	}

	// Get tools from this server
	listResults, err := conn.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		// Handle connection error
		m.connectionPool.HandleConnectionError(serverName, err)
		return -1, fmt.Errorf("failed to list tools: %v", err)
	}

	// Create name set for allowed tools
	var nameSet map[string]struct{}
	if len(serverConfig.AllowedTools) > 0 {
		nameSet = make(map[string]struct{})
		for _, name := range serverConfig.AllowedTools {
			nameSet[name] = struct{}{}
		}
	}

	// Build tools locally before acquiring the lock.
	var localTools []MCPTool
	localMap := make(map[string]*toolMapping)

	// Convert MCP tools to MCPTool structs with prefixed names
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

		// Convert MCP InputSchema to map[string]any
		marshaledSchema, err := json.Marshal(mcpTool.InputSchema)
		if err != nil {
			return -1, fmt.Errorf("conv mcp tool input schema fail(marshal): %w, tool name: %s", err, mcpTool.Name)
		}

		// Fix for JSON Schema draft-07 vs draft-04 compatibility
		marshaledSchema = convertExclusiveBoundsToBoolean(marshaledSchema)

		// Parse into map[string]any
		var schemaMap map[string]any
		if err := json.Unmarshal(marshaledSchema, &schemaMap); err != nil {
			return -1, fmt.Errorf("conv mcp tool input schema fail(unmarshal): %w, tool name: %s", err, mcpTool.Name)
		}

		// Extract properties and required from the schema
		parameters := make(map[string]any)
		required := []string{}

		if props, ok := schemaMap["properties"].(map[string]any); ok {
			parameters = props
		}

		// Fix for issue #89: Ensure object schemas have a properties field.
		// When schema type is "object" with no properties, we keep the
		// empty parameters map.

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
		}
		localMap[prefixedName] = mapping

		// Create MCPTool
		localTools = append(localTools, MCPTool{
			Name:         prefixedName,
			Description:  mcpTool.Description,
			Parameters:   parameters,
			Required:     required,
			ServerName:   serverName,
			OriginalName: mcpTool.Name,
		})
	}

	// Merge into the manager under the lock.
	m.mu.Lock()
	maps.Copy(m.toolMap, localMap)
	m.tools = append(m.tools, localTools...)
	m.mu.Unlock()

	// Also load prompts from this server (best-effort, non-blocking).
	m.loadServerPrompts(ctx, serverName, conn)

	return len(localTools), nil
}

// ExecuteTool calls an MCP tool through the connection pool, handling health
// checks, OAuth re-authorization, and connection error tracking.
// The inputJSON parameter is the raw JSON arguments from the LLM.
// Returns the result content, error flag, and any execution error.
func (m *MCPToolManager) ExecuteTool(ctx context.Context, prefixedName, inputJSON string) (*MCPToolResult, error) {
	m.mu.Lock()
	mapping, ok := m.toolMap[prefixedName]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("tool %q not found", prefixedName)
	}

	// Parse and validate JSON arguments
	var arguments any
	if inputJSON == "" || inputJSON == "{}" {
		arguments = nil
	} else {
		var temp any
		if err := json.Unmarshal([]byte(inputJSON), &temp); err != nil {
			return &MCPToolResult{
				Content: fmt.Sprintf("invalid JSON arguments: %v", err),
				IsError: true,
			}, nil
		}
		arguments = json.RawMessage(inputJSON)
	}

	// Get connection from pool with health check
	conn, err := m.connectionPool.GetConnectionWithHealthCheck(
		ctx, mapping.serverName, mapping.serverConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get healthy connection from pool: %w", err)
	}

	callRequest := mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: mcp.CallToolParams{
			Name:      mapping.originalName,
			Arguments: arguments,
		},
	}

	// Call the MCP tool using the original (unprefixed) name
	result, err := conn.client.CallTool(ctx, callRequest)
	if err != nil {
		// Handle OAuth re-authorization: token may have expired mid-session.
		if m.connectionPool.oauthFlow != nil && IsOAuthError(err) {
			if flowErr := m.connectionPool.oauthFlow.RunAuthFlow(ctx, mapping.serverName, err); flowErr != nil {
				return nil, fmt.Errorf("OAuth re-authorization failed for tool %s: %w", mapping.originalName, flowErr)
			}
			// Retry the tool call after successful re-auth.
			result, err = conn.client.CallTool(ctx, callRequest)
			if err != nil {
				m.connectionPool.HandleConnectionError(mapping.serverName, err)
				return nil, fmt.Errorf("failed to call mcp tool after re-auth: %w", err)
			}
		} else {
			// Mark connection as unhealthy for automatic recovery
			m.connectionPool.HandleConnectionError(mapping.serverName, err)
			return nil, fmt.Errorf("failed to call mcp tool: %w", err)
		}
	}

	// Marshal the MCP result to JSON string
	marshaledResult, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal mcp tool result: %w", err)
	}

	return &MCPToolResult{
		Content: string(marshaledResult),
		IsError: result.IsError,
	}, nil
}

// GetTools returns all loaded MCP tools from all configured MCP servers.
// Tools are returned with their prefixed names (serverName__toolName) to ensure uniqueness.
func (m *MCPToolManager) GetTools() []MCPTool {
	return m.tools
}

// GetPrompts returns all prompts discovered from connected MCP servers.
func (m *MCPToolManager) GetPrompts() []MCPPrompt {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]MCPPrompt, len(m.prompts))
	copy(result, m.prompts)
	return result
}

// GetPrompt retrieves and expands a specific prompt from an MCP server.
// The serverName identifies which server to query, promptName is the prompt's
// name on that server, and args are the template arguments to substitute.
// This call is lazy — it contacts the MCP server on each invocation.
func (m *MCPToolManager) GetPrompt(ctx context.Context, serverName, promptName string, args map[string]string) (*MCPPromptResult, error) {
	if m.connectionPool == nil {
		return nil, fmt.Errorf("no connection pool available")
	}

	clients := m.connectionPool.GetClients()
	mcpClient, ok := clients[serverName]
	if !ok {
		return nil, fmt.Errorf("MCP server %q not found", serverName)
	}

	req := mcp.GetPromptRequest{}
	req.Params.Name = promptName
	if len(args) > 0 {
		req.Params.Arguments = args
	}

	result, err := mcpClient.GetPrompt(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt %q from server %q: %w", promptName, serverName, err)
	}

	// Convert MCP messages to our types, extracting text content.
	var messages []MCPPromptMessage
	for _, msg := range result.Messages {
		text := extractContentText(msg.Content)
		if text != "" {
			messages = append(messages, MCPPromptMessage{
				Role:    string(msg.Role),
				Content: text,
			})
		}
	}

	return &MCPPromptResult{
		Description: result.Description,
		Messages:    messages,
	}, nil
}

// extractContentText extracts text from an MCP Content value.
// Content can be TextContent, ImageContent, AudioContent, or EmbeddedResource.
// We only extract text content; other types are skipped.
func extractContentText(content mcp.Content) string {
	if tc, ok := content.(mcp.TextContent); ok {
		return tc.Text
	}
	// Try pointer form as well.
	if tc, ok := content.(*mcp.TextContent); ok && tc != nil {
		return tc.Text
	}
	return ""
}

// loadServerPrompts loads prompts from a single MCP server connection.
// Called inside loadServerTools after a successful connection is established.
// Thread-safe: acquires m.mu to merge results.
func (m *MCPToolManager) loadServerPrompts(ctx context.Context, serverName string, conn *MCPConnection) {
	listResult, err := conn.client.ListPrompts(ctx, mcp.ListPromptsRequest{})
	if err != nil {
		// Prompts are optional — servers may not support them.
		// Silently skip.
		return
	}

	if len(listResult.Prompts) == 0 {
		return
	}

	var localPrompts []MCPPrompt
	for _, p := range listResult.Prompts {
		var args []MCPPromptArgument
		for _, a := range p.Arguments {
			args = append(args, MCPPromptArgument{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			})
		}
		localPrompts = append(localPrompts, MCPPrompt{
			Name:        p.Name,
			Description: p.Description,
			Arguments:   args,
			ServerName:  serverName,
		})
	}

	m.mu.Lock()
	m.prompts = append(m.prompts, localPrompts...)
	m.mu.Unlock()
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
	if m.connectionPool == nil {
		return nil
	}
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
