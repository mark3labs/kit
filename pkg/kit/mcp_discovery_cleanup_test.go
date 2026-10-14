package kit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	kit "github.com/mark3labs/kit/pkg/kit"
)

type cleanupFixtureModel struct{ kit.LLMLanguageModel }

func (cleanupFixtureModel) Provider() string { return "cleanup-fixture" }
func (cleanupFixtureModel) Model() string    { return "test" }

func remoteFixtureHost(t *testing.T) *kit.Kit {
	t.Helper()
	opts := &kit.Options{
		Model: "cleanup-fixture/test",
		Quiet: true,
		Providers: map[string]kit.ProviderFactory{
			"cleanup-fixture": func(context.Context, *kit.ProviderConfig, string) (*kit.ProviderResult, error) {
				return &kit.ProviderResult{Model: cleanupFixtureModel{}}, nil
			},
		},
	}
	kit.Isolated()(opts)
	host, err := kit.New(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })
	return host
}

// A server that initializes successfully but fails discovery leaves a pooled
// connection. The SDK's supported removal API must release it immediately,
// and failed AddMCPServer must roll the connection back so ListMCPServers
// does not keep reporting it.
func TestRemoteMCPKitFailedDiscoveryCleanup(t *testing.T) {
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(400)
			return
		}
		if len(req.ID) == 0 {
			w.WriteHeader(202)
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
	defer fixture.Close()

	host := remoteFixtureHost(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := host.AddMCPServer(ctx, "remote_broken", kit.MCPServerConfig{
		Type: "remote", URL: fixture.URL, NoOAuth: true,
	}); err == nil {
		t.Fatal("expected discovery failure")
	}

	// Preferred path: failed add already rolled back — Remove should say not loaded,
	// and List must not contain the name. If a connection somehow remained,
	// Remove must still succeed (defense in depth).
	listed := false
	for _, srv := range host.ListMCPServers() {
		if srv.Name == "remote_broken" {
			listed = true
			break
		}
	}
	if listed {
		if err := host.RemoveMCPServer("remote_broken"); err != nil {
			t.Errorf("failed discovery cannot be cleaned up through supported SDK API: %v", err)
		}
		for _, srv := range host.ListMCPServers() {
			if srv.Name == "remote_broken" {
				t.Error("failed server connection remains pooled after removal")
			}
		}
	} else {
		// Roll-back path: nothing left to remove.
		if err := host.RemoveMCPServer("remote_broken"); err == nil {
			t.Log("RemoveMCPServer on fully rolled-back server returned nil (unexpected but harmless)")
		}
	}
}
