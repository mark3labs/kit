package tools

import (
	"context"
	"fmt"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// newTestPromptServer creates an in-process MCP server with prompt capabilities
// and the specified prompts + handlers. Returns an initialized MCPClient.
func newTestPromptServer(t *testing.T, prompts ...server.ServerPrompt) mcpclient.MCPClient {
	t.Helper()

	mcpServer := server.NewMCPServer(
		"test-prompt-server", "1.0.0",
		server.WithPromptCapabilities(true),
		server.WithToolCapabilities(true),
	)

	if len(prompts) > 0 {
		mcpServer.AddPrompts(prompts...)
	}

	// Add a dummy tool so loadServerTools has something to list.
	mcpServer.AddTool(
		mcp.NewTool("noop", mcp.WithDescription("no-op tool")),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText("ok"), nil
		},
	)

	client, err := mcpclient.NewInProcessClient(mcpServer)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1.0"}
	if _, err := client.Initialize(ctx, initReq); err != nil {
		t.Fatalf("client.Initialize: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })
	return client
}

// injectClientIntoManager sets up an MCPToolManager with a pre-connected
// in-process client, bypassing the normal connection pool flow.
func injectClientIntoManager(t *testing.T, serverName string, client mcpclient.MCPClient) *MCPToolManager {
	t.Helper()

	m := NewMCPToolManager()

	// Create a minimal connection pool and inject our client.
	pool := NewMCPConnectionPool(DefaultConnectionPoolConfig(), false, nil, nil)
	pool.mu.Lock()
	pool.connections[serverName] = &MCPConnection{
		client:     client,
		serverName: serverName,
		isHealthy:  true,
	}
	pool.mu.Unlock()
	m.connectionPool = pool

	return m
}

func TestLoadServerPrompts_Basic(t *testing.T) {
	ctx := context.Background()

	client := newTestPromptServer(t,
		server.ServerPrompt{
			Prompt: mcp.NewPrompt("review-pr",
				mcp.WithPromptDescription("Review a pull request"),
				mcp.WithArgument("pr_number",
					mcp.ArgumentDescription("The PR number to review"),
					mcp.RequiredArgument(),
				),
				mcp.WithArgument("focus",
					mcp.ArgumentDescription("Area to focus on"),
				),
			),
			Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				prNum := req.Params.Arguments["pr_number"]
				return &mcp.GetPromptResult{
					Description: "PR review prompt",
					Messages: []mcp.PromptMessage{
						{
							Role: mcp.RoleUser,
							Content: mcp.TextContent{
								Type: "text",
								Text: fmt.Sprintf("Please review PR #%s", prNum),
							},
						},
					},
				}, nil
			},
		},
		server.ServerPrompt{
			Prompt: mcp.NewPrompt("explain-code",
				mcp.WithPromptDescription("Explain a piece of code"),
			),
			Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return &mcp.GetPromptResult{
					Messages: []mcp.PromptMessage{
						{
							Role: mcp.RoleUser,
							Content: mcp.TextContent{
								Type: "text",
								Text: "Please explain the following code.",
							},
						},
					},
				}, nil
			},
		},
	)

	m := injectClientIntoManager(t, "github", client)

	conn := &MCPConnection{
		client:     client,
		serverName: "github",
		isHealthy:  true,
	}
	m.loadServerPrompts(ctx, "github", conn)

	prompts := m.GetPrompts()
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(prompts))
	}

	// Find review-pr prompt.
	var reviewPR *MCPPrompt
	for i := range prompts {
		if prompts[i].Name == "review-pr" {
			reviewPR = &prompts[i]
			break
		}
	}
	if reviewPR == nil {
		t.Fatal("review-pr prompt not found")
	}
	if reviewPR.Description != "Review a pull request" {
		t.Errorf("unexpected description: %q", reviewPR.Description)
	}
	if reviewPR.ServerName != "github" {
		t.Errorf("unexpected server name: %q", reviewPR.ServerName)
	}
	if len(reviewPR.Arguments) != 2 {
		t.Fatalf("expected 2 arguments, got %d", len(reviewPR.Arguments))
	}

	// Verify argument metadata.
	arg0 := reviewPR.Arguments[0]
	if arg0.Name != "pr_number" {
		t.Errorf("expected first arg name 'pr_number', got %q", arg0.Name)
	}
	if !arg0.Required {
		t.Error("expected first arg to be required")
	}
	arg1 := reviewPR.Arguments[1]
	if arg1.Name != "focus" {
		t.Errorf("expected second arg name 'focus', got %q", arg1.Name)
	}
	if arg1.Required {
		t.Error("expected second arg to be optional")
	}
}

func TestGetPrompt_ExpandsWithArgs(t *testing.T) {
	ctx := context.Background()

	client := newTestPromptServer(t,
		server.ServerPrompt{
			Prompt: mcp.NewPrompt("greet",
				mcp.WithPromptDescription("Greet someone"),
				mcp.WithArgument("name", mcp.RequiredArgument()),
			),
			Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				name := req.Params.Arguments["name"]
				return &mcp.GetPromptResult{
					Description: "Greeting",
					Messages: []mcp.PromptMessage{
						{
							Role: mcp.RoleUser,
							Content: mcp.TextContent{
								Type: "text",
								Text: fmt.Sprintf("Hello, %s!", name),
							},
						},
					},
				}, nil
			},
		},
	)

	m := injectClientIntoManager(t, "myserver", client)

	result, err := m.GetPrompt(ctx, "myserver", "greet", map[string]string{"name": "World"})
	if err != nil {
		t.Fatalf("GetPrompt error: %v", err)
	}
	if result.Description != "Greeting" {
		t.Errorf("unexpected description: %q", result.Description)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("unexpected role: %q", result.Messages[0].Role)
	}
	if result.Messages[0].Content != "Hello, World!" {
		t.Errorf("unexpected content: %q", result.Messages[0].Content)
	}
}

func TestGetPrompt_MultipleMessages(t *testing.T) {
	ctx := context.Background()

	client := newTestPromptServer(t,
		server.ServerPrompt{
			Prompt: mcp.NewPrompt("chat-starter"),
			Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return &mcp.GetPromptResult{
					Messages: []mcp.PromptMessage{
						{
							Role:    mcp.RoleUser,
							Content: mcp.TextContent{Type: "text", Text: "What is Go?"},
						},
						{
							Role:    mcp.RoleAssistant,
							Content: mcp.TextContent{Type: "text", Text: "Go is a programming language."},
						},
						{
							Role:    mcp.RoleUser,
							Content: mcp.TextContent{Type: "text", Text: "Tell me more."},
						},
					},
				}, nil
			},
		},
	)

	m := injectClientIntoManager(t, "server", client)

	result, err := m.GetPrompt(ctx, "server", "chat-starter", nil)
	if err != nil {
		t.Fatalf("GetPrompt error: %v", err)
	}
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("msg[0] role: got %q, want 'user'", result.Messages[0].Role)
	}
	if result.Messages[1].Role != "assistant" {
		t.Errorf("msg[1] role: got %q, want 'assistant'", result.Messages[1].Role)
	}
	if result.Messages[2].Content != "Tell me more." {
		t.Errorf("msg[2] content: got %q, want 'Tell me more.'", result.Messages[2].Content)
	}
}

func TestGetPrompt_ServerNotFound(t *testing.T) {
	m := NewMCPToolManager()
	pool := NewMCPConnectionPool(DefaultConnectionPoolConfig(), false, nil, nil)
	m.connectionPool = pool

	_, err := m.GetPrompt(context.Background(), "nonexistent", "foo", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent server")
	}
}

func TestGetPrompt_NoPool(t *testing.T) {
	m := NewMCPToolManager()

	_, err := m.GetPrompt(context.Background(), "any", "foo", nil)
	if err == nil {
		t.Fatal("expected error with no pool")
	}
}

func TestRemoveServer_RemovesPrompts(t *testing.T) {
	ctx := context.Background()

	client := newTestPromptServer(t,
		server.ServerPrompt{
			Prompt: mcp.NewPrompt("my-prompt",
				mcp.WithPromptDescription("A test prompt"),
			),
			Handler: func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return &mcp.GetPromptResult{
					Messages: []mcp.PromptMessage{
						{Role: mcp.RoleUser, Content: mcp.TextContent{Type: "text", Text: "hi"}},
					},
				}, nil
			},
		},
	)

	m := injectClientIntoManager(t, "testsvr", client)

	// Manually populate tools and prompts as loadServerTools would.
	conn := m.connectionPool.connections["testsvr"]
	m.loadServerPrompts(ctx, "testsvr", conn)

	// Also add a fake tool mapping so RemoveServer finds the server.
	m.toolMap["testsvr__noop"] = &toolMapping{
		serverName:   "testsvr",
		originalName: "noop",
	}
	m.tools = append(m.tools, MCPTool{
		Name:       "testsvr__noop",
		ServerName: "testsvr",
	})

	// Verify prompts exist before removal.
	if got := len(m.GetPrompts()); got != 1 {
		t.Fatalf("expected 1 prompt before removal, got %d", got)
	}

	// Remove the server.
	err := m.RemoveServer("testsvr")
	if err != nil {
		t.Fatalf("RemoveServer error: %v", err)
	}

	// Verify prompts are gone.
	if got := len(m.GetPrompts()); got != 0 {
		t.Fatalf("expected 0 prompts after removal, got %d", got)
	}
}

func TestLoadServerPrompts_NoPromptCapability(t *testing.T) {
	// Server without prompt capabilities — ListPrompts should fail gracefully.
	mcpServer := server.NewMCPServer("no-prompts", "1.0.0",
		server.WithToolCapabilities(true),
		// No WithPromptCapabilities
	)
	mcpServer.AddTool(
		mcp.NewTool("noop"),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText("ok"), nil
		},
	)

	client, err := mcpclient.NewInProcessClient(mcpServer)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	ctx := context.Background()
	_ = client.Start(ctx)
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1.0"}
	_, _ = client.Initialize(ctx, initReq)
	t.Cleanup(func() { _ = client.Close() })

	m := NewMCPToolManager()
	conn := &MCPConnection{
		client:     client,
		serverName: "no-prompts",
		isHealthy:  true,
	}

	// Should not panic or error — just silently skip.
	m.loadServerPrompts(ctx, "no-prompts", conn)

	if got := len(m.GetPrompts()); got != 0 {
		t.Fatalf("expected 0 prompts from server without prompt capability, got %d", got)
	}
}

func TestExtractContentText(t *testing.T) {
	tests := []struct {
		name    string
		content mcp.Content
		want    string
	}{
		{
			name:    "TextContent",
			content: mcp.TextContent{Type: "text", Text: "hello world"},
			want:    "hello world",
		},
		{
			name:    "ImageContent",
			content: mcp.ImageContent{Type: "image", Data: "base64data", MIMEType: "image/png"},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractContentText(tt.content)
			if got != tt.want {
				t.Errorf("extractContentText() = %q, want %q", got, tt.want)
			}
		})
	}
}
