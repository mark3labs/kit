package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/server"
)

func TestRemoteMCPIdentity(t *testing.T) {
	for _, kind := range []string{"sse", "streamable"} {
		for _, identity := range []string{"", "MCPApp/1"} {
			t.Run(kind+identity, func(t *testing.T) {
				want := identity
				if want == "" {
					want = "KIT"
				}
				mcpServer := server.NewMCPServer("identity", "1", server.WithToolCapabilities(false))
				var handler http.Handler
				path := "/mcp"
				if kind == "sse" {
					handler = server.NewSSEServer(mcpServer)
					path = "/sse"
				} else {
					handler = server.NewStreamableHTTPServer(mcpServer)
				}
				var requests atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if r.Header.Get("User-Agent") != want || r.Header.Get("Authorization") != "Bearer fake" {
						t.Errorf("headers = %v", r.Header)
					}
					handler.ServeHTTP(w, r)
				}))
				defer srv.Close()
				manager := NewMCPToolManager()
				manager.SetUserAgent(identity)
				defer func() { _ = manager.Close() }()
				cfg := config.MCPServerConfig{Type: kind, URL: srv.URL + path, NoOAuth: true, Headers: []string{"uSeR-aGeNt: conflict", "Authorization: Bearer fake"}}
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				if _, err := manager.AddServer(ctx, "local", cfg); err != nil {
					t.Fatal(err)
				}
				if err := manager.RemoveServer("local"); err != nil {
					t.Fatal(err)
				}
				if _, err := manager.AddServer(ctx, "local", cfg); err != nil {
					t.Fatal(err)
				}
				if requests.Load() < 4 {
					t.Fatalf("requests = %d", requests.Load())
				}
				if cfg.Headers[0] != "uSeR-aGeNt: conflict" {
					t.Fatal("caller config mutated")
				}
				if err := manager.LoadTools(ctx, &config.Config{}); err != nil {
					t.Fatal(err)
				}
				if manager.connectionPool.config.UserAgent != identity {
					t.Fatal("pool recreation lost identity")
				}
			})
		}
	}
}

func TestConnectionPoolSnapshotsIdentity(t *testing.T) {
	cfg := DefaultConnectionPoolConfig()
	cfg.UserAgent = "original"
	pool := NewMCPConnectionPool(cfg, false, nil, nil)
	defer func() { _ = pool.Close() }()
	cfg.UserAgent = "changed"
	if pool.config.UserAgent != "original" {
		t.Fatal("pool shares caller configuration")
	}
}

type identityAuthHandler struct{}

func (identityAuthHandler) RedirectURI() string { return "http://127.0.0.1/callback" }
func (identityAuthHandler) HandleAuth(context.Context, string, string) (string, error) {
	return "", nil
}

func TestMCPOAuthIdentity(t *testing.T) {
	for _, identity := range []string{"", "OAuthMCP/1"} {
		t.Run(identity, func(t *testing.T) {
			want := identity
			if want == "" {
				want = "KIT"
			}
			seen := map[string]int{}
			var base string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen[r.URL.Path]++
				if r.Header.Get("User-Agent") != want {
					t.Errorf("OAuth identity = %q", r.Header.Get("User-Agent"))
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/.well-known/oauth-authorization-server":
					_ = json.NewEncoder(w).Encode(map[string]any{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "registration_endpoint": base + "/register", "response_types_supported": []string{"code"}, "token_endpoint_auth_methods_supported": []string{"none"}})
				case "/register":
					_, _ = w.Write([]byte(`{"client_id":"fake-client"}`))
				case "/token":
					_, _ = w.Write([]byte(`{"access_token":"fake-access","token_type":"Bearer","refresh_token":"fake-refresh","expires_in":3600}`))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()
			base = srv.URL
			cfg := DefaultConnectionPoolConfig()
			cfg.UserAgent = identity
			pool := NewMCPConnectionPool(cfg, false, identityAuthHandler{}, func(string) (transport.TokenStore, error) { return transport.NewMemoryTokenStore(), nil })
			defer func() { _ = pool.Close() }()
			oauth, err := pool.buildOAuthConfig(config.MCPServerConfig{URL: base})
			if err != nil {
				t.Fatal(err)
			}
			if oauth.HTTPClient.Timeout != 30*time.Second {
				t.Fatal("OAuth timeout changed")
			}
			handler := transport.NewOAuthHandler(*oauth)
			handler.SetBaseURL(base)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			if err := handler.RegisterClient(ctx, "test"); err != nil {
				t.Fatal(err)
			}
			handler.SetExpectedState("state")
			if err := handler.ProcessAuthorizationResponse(ctx, "code", "state", "verifier"); err != nil {
				t.Fatal(err)
			}
			// Refresh the stored token so the handler can verify its issuer.
			token, err := oauth.TokenStore.GetToken(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := handler.RefreshToken(ctx, token.RefreshToken); err != nil {
				t.Fatal(err)
			}
			if seen["/.well-known/oauth-authorization-server"] == 0 || seen["/register"] != 1 || seen["/token"] != 2 {
				t.Fatalf("OAuth requests = %v", seen)
			}
		})
	}
}
