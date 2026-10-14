package tools

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Embed the interface so unexpected discovery or unsubscribe calls fail the test.
type lifecycleClient struct {
	client.MCPClient
	prompts     []mcp.Prompt
	resources   []mcp.Resource
	tools       []mcp.Tool
	listErr     error
	closeErr    error
	closes      int
	cancelList  bool
	optionalErr error
}

func (c *lifecycleClient) ListTools(ctx context.Context, _ mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	if c.cancelList {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &mcp.ListToolsResult{Tools: c.tools}, c.listErr
}
func (c *lifecycleClient) ListPrompts(context.Context, mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
	return &mcp.ListPromptsResult{Prompts: c.prompts}, c.optionalErr
}
func (c *lifecycleClient) ListResources(context.Context, mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	return &mcp.ListResourcesResult{Resources: c.resources}, c.optionalErr
}
func (c *lifecycleClient) Close() error { c.closes++; return c.closeErr }

func lifecycleManager(t *testing.T) *MCPToolManager {
	t.Helper()
	m := NewMCPToolManager()
	m.ensureConnectionPool()
	t.Cleanup(func() { _ = m.Close() })
	return m
}
func seedLifecycleClient(m *MCPToolManager, name string, c *lifecycleClient) {
	m.connectionPool.connections[name] = &MCPConnection{client: c, serverName: name, isHealthy: true, lastUsed: time.Now()}
}

func TestMandatoryDiscoveryRollback(t *testing.T) {
	discoveryErr := errors.New("discovery failed")
	for _, mode := range []string{"list", "close", "conversion", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			m := lifecycleManager(t)
			c := &lifecycleClient{listErr: discoveryErr}
			switch mode {
			case "close":
				c.closeErr = errors.New("close failed")
			case "conversion":
				c.listErr = nil
				bad := mcp.NewTool("bad")
				bad.InputSchema.Properties = map[string]any{"bad": make(chan int)}
				c.tools = []mcp.Tool{mcp.NewTool("staged"), bad}
			case "cancel":
				c.listErr = nil
				c.cancelList = true
			}
			seedLifecycleClient(m, "failed", c)
			healthy := &lifecycleClient{}
			seedLifecycleClient(m, "healthy", healthy)
			m.tools = []MCPTool{{Name: "healthy__tool", ServerName: "healthy"}}
			m.toolMap["healthy__tool"] = &toolMapping{serverName: "healthy"}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if mode == "cancel" {
				cancel()
			}
			_, err := m.loadServerTools(ctx, "failed", config.MCPServerConfig{})
			if err == nil {
				t.Fatal("expected discovery error")
			}
			if (mode == "list" || mode == "close") && !errors.Is(err, discoveryErr) {
				t.Fatalf("lost discovery error: %v", err)
			}
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation: %v", err)
			}
			if c.closes != 1 || m.connectionPool.HasConnection("failed") {
				t.Fatalf("rollback: closes=%d clients=%v", c.closes, m.connectionPool.GetClients())
			}
			if len(m.tools) != 1 || len(m.toolMap) != 1 || len(m.prompts) != 0 || len(m.resources) != 0 || healthy.closes != 0 {
				t.Fatal("rollback changed catalogs or healthy connection")
			}
			if err := m.RemoveServer("failed"); err == nil {
				t.Fatal("rolled back server should be absent")
			}
		})
	}
}

func TestZeroToolLifecycle(t *testing.T) {
	for _, kind := range []string{"empty", "filtered", "prompt", "resource", "optional-failure"} {
		t.Run(kind, func(t *testing.T) {
			m := lifecycleManager(t)
			c := &lifecycleClient{}
			cfg := config.MCPServerConfig{}
			if kind == "optional-failure" {
				c.optionalErr = errors.New("optional discovery failed")
			}
			if kind == "filtered" {
				c.tools = []mcp.Tool{mcp.NewTool("hidden")}
				cfg.AllowedTools = []string{"other"}
			}
			if kind == "prompt" {
				c.prompts = []mcp.Prompt{{Name: "p"}}
			}
			if kind == "resource" {
				c.resources = []mcp.Resource{{URI: "shared", Name: "r"}}
			}
			seedLifecycleClient(m, "remote", c)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			n, err := m.loadServerTools(ctx, "remote", cfg)
			if err != nil || n != 0 {
				t.Fatalf("load=%d, %v", n, err)
			}
			m.subscriptions["owned"] = "remote"
			m.subscriptions["shared"] = "remote__child"
			child := &lifecycleClient{}
			seedLifecycleClient(m, "remote__child", child)
			m.tools = []MCPTool{{Name: "remote__child__tool", ServerName: "remote__child"}}
			m.toolMap["remote__child__tool"] = &toolMapping{serverName: "remote__child"}
			if _, err := m.AddServer(ctx, "remote", cfg); err == nil || !strings.Contains(err.Error(), "already loaded") {
				t.Fatalf("duplicate: %v", err)
			}
			if c.closes != 0 {
				t.Fatal("duplicate closed existing client")
			}
			changes := 0
			m.SetOnToolsChanged(func() { changes++; _ = m.GetTools() })
			if err := m.RemoveServer("remote"); err != nil {
				t.Fatal(err)
			}
			if c.closes != 1 || m.connectionPool.HasConnection("remote") || len(m.prompts) != 0 || len(m.resources) != 0 || len(m.subscriptions) != 1 || m.subscriptions["shared"] != "remote__child" {
				t.Fatal("incomplete removal")
			}
			if changes != 1 || child.closes != 0 || len(m.tools) != 1 || len(m.toolMap) != 1 {
				t.Fatal("removal changed another server or callback")
			}
			if err := m.RemoveServer("remote"); err == nil {
				t.Fatal("repeated removal must fail")
			}
		})
	}
}

func TestLegacyConnectionAndCatalogRemoval(t *testing.T) {
	for _, kind := range []string{"connection", "unhealthy", "tool", "mapping", "prompt", "resource", "subscription"} {
		t.Run(kind, func(t *testing.T) {
			m := lifecycleManager(t)
			c := &lifecycleClient{}
			switch kind {
			case "connection", "unhealthy":
				seedLifecycleClient(m, "legacy", c)
				m.connectionPool.connections["legacy"].isHealthy = kind != "unhealthy"
			case "tool":
				m.tools = []MCPTool{{ServerName: "legacy"}}
			case "mapping":
				m.toolMap["legacy__tool"] = &toolMapping{serverName: "legacy"}
			case "prompt":
				m.prompts = []MCPPrompt{{ServerName: "legacy"}}
			case "resource":
				m.resources = []MCPResource{{ServerName: "legacy"}}
			case "subscription":
				m.subscriptions["uri"] = "legacy"
			}
			if err := m.RemoveServer("legacy"); err != nil {
				t.Fatal(err)
			}
			if m.hasServerLocked("legacy") {
				t.Fatal("server remains after removal")
			}
			if (kind == "connection" || kind == "unhealthy") && c.closes != 1 {
				t.Fatal("connection not closed")
			}
		})
	}
}

func TestStartupDiscoveryRollbackAndRetry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprint(mixed), func(t *testing.T) {
			m := lifecycleManager(t)
			// Without tools capability, this server initializes but rejects tools/list.
			broken := server.NewMCPServer("broken", "1")
			cfg := &config.Config{MCPServers: map[string]config.MCPServerConfig{"failed": {Type: "inprocess", InProcessServer: broken}}}
			if mixed {
				cfg.MCPServers["healthy"] = config.MCPServerConfig{Type: "inprocess", InProcessServer: newTestInProcessServer()}
			}
			var mu sync.Mutex
			loaded := map[string]error{}
			m.SetOnServerLoaded(func(name string, _ int, err error) { mu.Lock(); defer mu.Unlock(); loaded[name] = err })
			err := m.LoadTools(ctx, cfg)
			if mixed && err != nil {
				t.Fatal(err)
			}
			if !mixed && (err == nil || !strings.Contains(err.Error(), "all MCP servers failed")) {
				t.Fatalf("aggregate: %v", err)
			}
			if m.connectionPool.HasConnection("failed") {
				t.Fatal("startup retained failed connection")
			}
			mu.Lock()
			failedErr := loaded["failed"]
			mu.Unlock()
			if failedErr == nil {
				t.Fatal("startup failure callback missing")
			}
			changes, successes := 0, 0
			m.SetOnToolsChanged(func() { changes++ })
			m.SetOnServerLoaded(func(_ string, _ int, err error) {
				if err == nil {
					successes++
				}
			})
			if n, err := m.AddServer(ctx, "failed", cfg.MCPServers["failed"]); n != 0 || err == nil {
				t.Fatalf("runtime failure: %d %v", n, err)
			}
			if changes != 0 || successes != 0 {
				t.Fatal("failed add emitted success callback")
			}
			good := config.MCPServerConfig{Type: "inprocess", InProcessServer: newTestInProcessServer()}
			if n, err := m.AddServer(ctx, "failed", good); n != 1 || err != nil {
				t.Fatalf("retry: %d %v", n, err)
			}
			if changes != 1 || successes != 1 {
				t.Fatal("successful add callbacks missing")
			}
		})
	}
}

func TestExactOwnershipAdd(t *testing.T) {
	m := lifecycleManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg := config.MCPServerConfig{Type: "inprocess", InProcessServer: newTestInProcessServer()}
	for _, name := range []string{"remote__child", "remote"} {
		if n, err := m.AddServer(ctx, name, cfg); err != nil || n != 1 {
			t.Fatalf("add %s: %d %v", name, n, err)
		}
	}
	if err := m.RemoveServer("remote"); err != nil {
		t.Fatal(err)
	}
	if len(m.tools) != 1 || m.tools[0].ServerName != "remote__child" {
		t.Fatal("removed child tools")
	}
}

func TestToolKeyCollisionRollback(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			m := lifecycleManager(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			names := []string{"a", "a__b"}
			tools := []string{"b__x", "x"}
			if reverse {
				slices.Reverse(names)
				slices.Reverse(tools)
			}
			original := server.NewMCPServer("original", "1", server.WithToolCapabilities(true))
			original.AddTool(mcp.NewTool(tools[0]), func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return mcp.NewToolResultText("original"), nil
			})
			if n, err := m.AddServer(ctx, names[0], config.MCPServerConfig{Type: "inprocess", InProcessServer: original}); err != nil || n != 1 {
				t.Fatalf("original: %d %v", n, err)
			}
			// Stage another key first to prove a collision publishes nothing.
			conflicting := &lifecycleClient{tools: []mcp.Tool{mcp.NewTool("staged"), mcp.NewTool(tools[1])}}
			seedLifecycleClient(m, names[1], conflicting)
			_, err := m.loadServerTools(ctx, names[1], config.MCPServerConfig{})
			if err == nil || !strings.Contains(err.Error(), "conflicts") {
				t.Fatalf("collision: %v", err)
			}
			if conflicting.closes != 1 || m.connectionPool.HasConnection(names[1]) {
				t.Fatal("collision connection was not rolled back")
			}
			if len(m.tools) != 1 || len(m.toolMap) != 1 || m.toolMap["a__b__x"].serverName != names[0] {
				t.Fatal("collision changed original routing")
			}
			if err := m.RemoveServer(names[1]); err == nil {
				t.Fatal("rejected server must be absent")
			}
			result, err := m.ExecuteTool(ctx, "a__b__x", "{}")
			if err != nil || !strings.Contains(result.Content, "original") {
				t.Fatalf("original remains callable: %+v %v", result, err)
			}
		})
	}
}
