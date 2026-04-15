package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// newTestInProcessServer creates a simple MCP server with one tool for testing.
func newTestInProcessServer() *server.MCPServer {
	srv := server.NewMCPServer("test-server", "1.0.0",
		server.WithToolCapabilities(true),
	)
	srv.AddTool(
		mcp.NewTool("greet",
			mcp.WithDescription("Say hello"),
			mcp.WithString("name", mcp.Required(), mcp.Description("Name to greet")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.GetArguments()["name"].(string)
			return mcp.NewToolResultText("Hello, " + name + "!"), nil
		},
	)
	return srv
}

func TestInProcessTransportType(t *testing.T) {
	cfg := config.MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: newTestInProcessServer(),
	}
	if got := cfg.GetTransportType(); got != "inprocess" {
		t.Errorf("GetTransportType() = %q, want %q", got, "inprocess")
	}
}

func TestInProcessTransportTypeInferred(t *testing.T) {
	// When Type is empty but InProcessServer is set, infer "inprocess".
	cfg := config.MCPServerConfig{
		InProcessServer: newTestInProcessServer(),
	}
	if got := cfg.GetTransportType(); got != "inprocess" {
		t.Errorf("GetTransportType() = %q, want %q", got, "inprocess")
	}
}

func TestInProcessValidation(t *testing.T) {
	// Valid: InProcessServer is set.
	validCfg := &config.Config{
		MCPServers: map[string]config.MCPServerConfig{
			"test": {
				Type:            "inprocess",
				InProcessServer: newTestInProcessServer(),
			},
		},
	}
	if err := validCfg.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}

	// Invalid: type is inprocess but InProcessServer is nil.
	invalidCfg := &config.Config{
		MCPServers: map[string]config.MCPServerConfig{
			"test": {
				Type: "inprocess",
			},
		},
	}
	if err := invalidCfg.Validate(); err == nil {
		t.Error("expected validation error for nil InProcessServer, got nil")
	}
}

func TestConnectionPoolInProcessClient(t *testing.T) {
	pool := NewMCPConnectionPool(DefaultConnectionPoolConfig(), false, nil, nil)
	defer func() { _ = pool.Close() }()

	ctx := context.Background()
	srv := newTestInProcessServer()

	cfg := config.MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: srv,
	}

	conn, err := pool.GetConnection(ctx, "test-inproc", cfg)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}

	// Verify the connection is healthy and functional.
	if !conn.isHealthy {
		t.Error("expected connection to be healthy")
	}

	// List tools to verify the connection works end-to-end.
	toolsResp, err := conn.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(toolsResp.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(toolsResp.Tools))
	}
	if toolsResp.Tools[0].Name != "greet" {
		t.Errorf("expected tool name 'greet', got %q", toolsResp.Tools[0].Name)
	}
}

func TestConnectionPoolInProcessToolExecution(t *testing.T) {
	pool := NewMCPConnectionPool(DefaultConnectionPoolConfig(), false, nil, nil)
	defer func() { _ = pool.Close() }()

	ctx := context.Background()
	srv := newTestInProcessServer()

	cfg := config.MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: srv,
	}

	conn, err := pool.GetConnection(ctx, "test-inproc", cfg)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}

	// Call the tool.
	result, err := conn.client.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name:      "greet",
			Arguments: map[string]any{"name": "World"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if result.IsError {
		t.Error("expected non-error result")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if text.Text != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", text.Text)
	}
}

func TestMCPToolManagerInProcess(t *testing.T) {
	ctx := context.Background()
	srv := newTestInProcessServer()

	mgr := NewMCPToolManager()

	cfg := config.MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: srv,
	}

	count, err := mgr.AddServer(ctx, "myserver", cfg)
	if err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 tool, got %d", count)
	}

	tools := mgr.GetTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "myserver__greet" {
		t.Errorf("expected tool name 'myserver__greet', got %q", tools[0].Name)
	}

	// Execute the tool.
	input, _ := json.Marshal(map[string]any{"name": "SDK"})
	result, err := mgr.ExecuteTool(ctx, "myserver__greet", string(input))
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if result.IsError {
		t.Error("expected non-error result")
	}
	if result.Content == "" {
		t.Error("expected non-empty result content")
	}

	// Verify result contains our greeting.
	if !strings.Contains(result.Content, "Hello, SDK!") {
		t.Errorf("expected 'Hello, SDK!' in result, got %q", result.Content)
	}
}

func TestConnectionPoolInProcessInvalidServer(t *testing.T) {
	pool := NewMCPConnectionPool(DefaultConnectionPoolConfig(), false, nil, nil)
	defer func() { _ = pool.Close() }()

	ctx := context.Background()

	// Pass a non-*server.MCPServer value.
	cfg := config.MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: "not a server",
	}

	_, err := pool.GetConnection(ctx, "bad", cfg)
	if err == nil {
		t.Fatal("expected error for invalid InProcessServer type")
	}
}

func TestConnectionPoolInProcessReuse(t *testing.T) {
	pool := NewMCPConnectionPool(DefaultConnectionPoolConfig(), false, nil, nil)
	defer func() { _ = pool.Close() }()

	ctx := context.Background()
	srv := newTestInProcessServer()
	cfg := config.MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: srv,
	}

	// Get connection twice — should reuse.
	conn1, err := pool.GetConnection(ctx, "reuse-test", cfg)
	if err != nil {
		t.Fatalf("first GetConnection failed: %v", err)
	}
	conn2, err := pool.GetConnection(ctx, "reuse-test", cfg)
	if err != nil {
		t.Fatalf("second GetConnection failed: %v", err)
	}
	if conn1 != conn2 {
		t.Error("expected same connection object on reuse")
	}
}
