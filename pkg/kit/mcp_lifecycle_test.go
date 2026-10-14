package kit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func discoveryHTTPFixture(t *testing.T, fail bool) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if len(req.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		response := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "fixture", "version": "1"}}
		case "tools/list":
			if fail {
				response["error"] = map[string]any{"code": -32601, "message": "Method not found"}
			} else {
				result = map[string]any{"tools": []any{}}
			}
		case "prompts/list":
			result = map[string]any{"prompts": []any{}}
		case "resources/list":
			result = map[string]any{"resources": []any{}}
		case "ping":
			result = map[string]any{}
		default:
			response["error"] = map[string]any{"code": -32601, "message": "Method not found"}
		}
		if result != nil {
			response["result"] = result
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestMCPDiscoveryPublicLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	native := NewTool("native", "Native test tool", func(context.Context, struct{}) (ToolOutput, error) { return ToolOutput{Content: "native"}, nil })
	host, err := NewAgent(ctx, Isolated(), WithModel("offline/test"), WithExtraTools(native), func(o *Options) {
		o.Providers = map[string]ProviderFactory{"offline": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
			return &ProviderResult{Model: &echoModel{provider: "offline", model: "test"}}, nil
		}}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })
	if err := host.WaitForMCPTools(); err != nil {
		t.Fatal(err)
	}
	healthy := server.NewMCPServer("healthy", "1", server.WithToolCapabilities(true))
	healthy.AddTool(mcp.NewTool("greet"), func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("hello"), nil
	})
	if n, err := host.AddInProcessMCPServer(ctx, "healthy", healthy); err != nil || n != 1 {
		t.Fatalf("healthy: %d %v", n, err)
	}
	if n, err := host.AddInProcessMCPServer(ctx, "healthy__child", healthy); err != nil || n != 1 {
		t.Fatalf("child: %d %v", n, err)
	}
	for _, status := range host.ListMCPServers() {
		if status.ToolCount != 1 {
			t.Fatalf("exact ownership count: %+v", status)
		}
	}
	if err := host.RemoveMCPServer("healthy__child"); err != nil {
		t.Fatal(err)
	}
	before := host.GetToolNames()
	failedURL := discoveryHTTPFixture(t, true)
	n, err := host.AddMCPServer(ctx, "retry", MCPServerConfig{Type: "remote", URL: failedURL, NoOAuth: true})
	if n != 0 || err == nil || !strings.Contains(err.Error(), "failed to list tools") {
		t.Fatalf("failed add: %d %v", n, err)
	}
	statuses := host.ListMCPServers()
	if len(statuses) != 1 || statuses[0].Name != "healthy" || statuses[0].ToolCount != 1 {
		t.Fatalf("failed server remains: %+v", statuses)
	}
	if !slices.Equal(before, host.GetToolNames()) {
		t.Fatal("failed add changed healthy/native tools")
	}
	if len(host.agent.GetMCPPrompts()) != 0 || len(host.agent.GetMCPResources()) != 0 {
		t.Fatal("failed add published optional catalogs")
	}
	if err := host.RemoveMCPServer("retry"); err == nil {
		t.Fatal("rolled back server must be absent")
	}
	goodURL := discoveryHTTPFixture(t, false)
	if n, err := host.AddMCPServer(ctx, "retry", MCPServerConfig{Type: "remote", URL: goodURL, NoOAuth: true}); err != nil || n != 0 {
		t.Fatalf("retry: %d %v", n, err)
	}
	found := false
	for _, s := range host.ListMCPServers() {
		if s.Name == "retry" {
			found = true
			if s.ToolCount != 0 {
				t.Fatalf("empty server count=%d", s.ToolCount)
			}
		}
	}
	if !found {
		t.Fatal("successful empty server missing")
	}
	if _, err := host.AddMCPServer(ctx, "retry", MCPServerConfig{Type: "remote", URL: failedURL, NoOAuth: true}); err == nil || !strings.Contains(err.Error(), "already loaded") {
		t.Fatalf("duplicate: %v", err)
	}
	if err := host.RemoveMCPServer("retry"); err != nil {
		t.Fatal(err)
	}
	if len(host.ListMCPServers()) != 1 {
		t.Fatal("removal affected healthy server")
	}
	result, err := host.agent.GetMCPToolManager().ExecuteTool(ctx, "healthy__greet", "{}")
	if err != nil || !strings.Contains(result.Content, "hello") {
		t.Fatalf("healthy call: %+v %v", result, err)
	}
}
