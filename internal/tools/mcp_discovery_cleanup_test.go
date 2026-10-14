package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/mcp-go/server"
)

// streamableFixture answers initialize successfully but fails tools/list.
func streamableBrokenDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Notifications (no id) get 202.
		if len(req.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		if req.Method == "initialize" {
			response["result"] = map[string]any{
				"protocolVersion": "2025-03-26",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "broken-fixture", "version": "1"},
			}
		} else {
			response["error"] = map[string]any{"code": -32601, "message": "discovery unavailable"}
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
}

func TestAddServer_FailedDiscovery_RollsBackConnection(t *testing.T) {
	fixture := streamableBrokenDiscoveryServer(t)
	defer fixture.Close()

	manager := NewMCPToolManager()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := manager.AddServer(ctx, "remote_broken", config.MCPServerConfig{
		Type:    "remote",
		URL:     fixture.URL,
		NoOAuth: true,
	})
	if err == nil {
		t.Fatal("expected discovery failure")
	}

	if names := manager.GetLoadedServerNames(); len(names) != 0 {
		t.Fatalf("failed discovery left pooled connection(s): %v", names)
	}
	if clients := manager.connectionPool.GetClients(); len(clients) != 0 {
		t.Fatalf("connection pool not empty after failed discovery: %v", clients)
	}
}

func TestRemoveServer_FailedDiscovery_CanCleanup(t *testing.T) {
	// Simulate the pre-fix leak path: connection in pool, no tools registered.
	manager := NewMCPToolManager()
	manager.ensureConnectionPool()

	fixture := streamableBrokenDiscoveryServer(t)
	defer fixture.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Put a connection in the pool without going through AddServer success.
	_, err := manager.connectionPool.GetConnection(ctx, "remote_broken", config.MCPServerConfig{
		Type:    "remote",
		URL:     fixture.URL,
		NoOAuth: true,
	})
	if err != nil {
		t.Fatalf("GetConnection (initialize-only) failed: %v", err)
	}
	if len(manager.GetLoadedServerNames()) == 0 {
		t.Fatal("expected pooled connection before RemoveServer")
	}

	if err := manager.RemoveServer("remote_broken"); err != nil {
		t.Fatalf("RemoveServer should release zero-tool pooled connection: %v", err)
	}
	for _, name := range manager.GetLoadedServerNames() {
		if name == "remote_broken" {
			t.Error("failed server connection remains pooled after removal")
		}
	}
}

func TestRemoveServer_EmptyToolServer(t *testing.T) {
	// Successful initialize + empty tools/list must still be removable.
	srv := server.NewMCPServer("empty-tools", "1.0.0", server.WithToolCapabilities(true))
	// No tools registered.

	manager := NewMCPToolManager()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := manager.AddServer(ctx, "empty", config.MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: srv,
	})
	if err != nil {
		t.Fatalf("AddServer empty-tools failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 tools, got %d", count)
	}
	if len(manager.GetLoadedServerNames()) != 1 {
		t.Fatalf("expected empty-tools server listed, got %v", manager.GetLoadedServerNames())
	}

	if err := manager.RemoveServer("empty"); err != nil {
		t.Fatalf("RemoveServer empty-tools: %v", err)
	}
	if names := manager.GetLoadedServerNames(); len(names) != 0 {
		t.Fatalf("expected no servers after remove, got %v", names)
	}

}
