package models

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/auth"
)

func TestNativeProviderUserAgents(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, identity := range []string{"", "App/1.0"} {
		for _, model := range []string{"openai/gpt-4o", "anthropic/claude-sonnet-4-5", "google/gemini-2.5-flash", "azure/gpt-4o", "openrouter/test", "vercel/test", "custom/test"} {
			t.Run(model+identity, func(t *testing.T) {
				want := identity
				if want == "" {
					want = "KIT"
				}
				seen := 0
				mockDefaultTransport(t, func(req *http.Request) (*http.Response, error) {
					seen++
					if req.Header.Get("User-Agent") != want {
						t.Errorf("identity = %q", req.Header.Get("User-Agent"))
					}
					return jsonTestResponse(req, `{}`), nil
				})
				result, err := CreateProvider(t.Context(), &ProviderConfig{ModelString: model, ProviderAPIKey: "fake", ProviderURL: "http://example.test/v1", UserAgent: identity})
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				_, _ = result.Model.Generate(ctx, fantasy.Call{Prompt: []fantasy.Message{fantasy.NewUserMessage("hi")}, Headers: map[string]string{"user-agent": "per-call-conflict"}})
				if seen == 0 {
					t.Fatal("no outbound request")
				}
			})
		}
	}
}

func TestAutoRoutedUserAgents(t *testing.T) {
	for _, wire := range []string{"openai", "openai-compat", "anthropic", "google"} {
		for _, identity := range []string{"", "App/1.0"} {
			t.Run(wire+identity, func(t *testing.T) {
				want := identity
				if want == "" {
					want = "KIT"
				}
				seen := false
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					seen = true
					if r.Header.Get("User-Agent") != want || r.Header.Get("X-Keep") != "keep" {
						t.Errorf("headers = %v", r.Header)
					}
					if wire == "google" && strings.Contains(r.URL.Path, "v1beta") {
						t.Errorf("unrewritten proxy path %s", r.URL.Path)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{}`)
				}))
				defer srv.Close()
				wrappedRequests := 0
				cfg := &ProviderConfig{ProviderAPIKey: "fake", ProviderURL: srv.URL + "/v1", ProviderWire: wire, UserAgent: identity,
					HTTPTransport: func(base http.RoundTripper) http.RoundTripper {
						return roundTripFunc(func(req *http.Request) (*http.Response, error) {
							wrappedRequests++
							if req.Header.Get("User-Agent") != want || req.Header.Get("X-Keep") != "keep" {
								t.Errorf("wrapper headers = %v", req.Header)
							}
							if wire == "google" && strings.Contains(req.URL.Path, "v1beta") {
								t.Errorf("wrapper saw unrewritten path %s", req.URL.Path)
							}
							return base.RoundTrip(req)
						})
					},
				}
				reg := newTestRegistry(srv.URL+"/v1", "@ai-sdk/openai-compatible", "test-model", "")
				info := reg.providers["testproxy"]
				info.Headers = map[string]string{"uSeR-aGeNt": "catalog-conflict", "X-Keep": "keep"}
				reg.providers["testproxy"] = info
				result, err := autoRouteProvider(t.Context(), cfg, "testproxy", "test-model", reg)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				_, _ = result.Model.Generate(ctx, fantasy.Call{Prompt: []fantasy.Message{fantasy.NewUserMessage("hi")}, Headers: map[string]string{"User-Agent": "call-conflict"}})
				if !seen || wrappedRequests == 0 {
					t.Fatal("no wrapped request")
				}
			})
		}
	}
}

func TestProviderStreamingIdentity(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "StreamApp/1" {
			t.Errorf("identity = %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"}}]}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	cfg := &ProviderConfig{ProviderAPIKey: "fake", ProviderURL: srv.URL + "/v1", UserAgent: "StreamApp/1"}
	result, err := createAutoRoutedOpenAICompatProvider(ctx, cfg, "test", &ProviderInfo{ID: "test", Name: "test"})
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	stream, err := result.Model.Stream(ctx, fantasy.Call{Prompt: []fantasy.Message{fantasy.NewUserMessage("hi")}})
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	first := false
	for part := range stream {
		if part.Type == fantasy.StreamPartTypeTextDelta {
			first = true
			close(release)
			break
		}
	}
	if !first {
		close(release)
		t.Fatal("no streaming event before response completion")
	}
}

func TestOllamaSetupIdentity(t *testing.T) {
	seen := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "OllamaApp/1" {
			t.Errorf("identity = %q", r.Header.Get("User-Agent"))
		}
		seen[r.URL.Path] = true
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/show":
			w.WriteHeader(http.StatusNotFound)
		case "/api/pull":
			_, _ = io.WriteString(w, `{"status":"success"}`)
		default:
			_, _ = io.WriteString(w, `{"done":true}`)
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err := loadOllamaModelWithFallback(ctx, srv.URL, "test", &ProviderConfig{UserAgent: "OllamaApp/1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/show", "/api/pull", "/api/generate"} {
		if !seen[path] {
			t.Errorf("missing %s", path)
		}
	}
}

func TestCopilotRefreshIdentity(t *testing.T) {
	for _, identity := range []string{"", "CopilotApp/1"} {
		t.Run(identity, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cm, err := auth.NewCredentialManager()
			if err != nil {
				t.Fatal(err)
			}
			if err := cm.SetCopilotOAuthCredentials(&auth.CopilotCredentials{Type: "oauth", GitHubToken: "fake-github", CopilotAccessToken: "expired", ExpiresAt: time.Now().Add(-time.Hour).Unix()}); err != nil {
				t.Fatal(err)
			}
			refreshes := 0
			mockDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
				refreshes++
				want := identity
				if want == "" {
					want = "KIT"
				}
				if r.Header.Get("User-Agent") != want || r.Header.Get("Authorization") != "token fake-github" {
					t.Errorf("refresh headers = %v", r.Header)
				}
				return jsonTestResponse(r, `{"token":"fresh-token","expires_at":9999999999}`), nil
			})
			transport := &copilotTransport{userAgent: identity, token: "expired", expiresAt: time.Now().Add(-time.Hour).Unix(), base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("User-Agent") != copilotUserAgent || r.Header.Get("Authorization") != "Bearer fresh-token" {
					t.Errorf("inference headers = %v", r.Header)
				}
				return jsonTestResponse(r, `{}`), nil
			})}
			req, _ := http.NewRequestWithContext(t.Context(), "POST", "http://example.test", nil)
			resp, err := transport.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if refreshes != 1 {
				t.Fatalf("refreshes = %d", refreshes)
			}
		})
	}
}

func TestCloudProviderIdentityAndAuth(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	credentialsPath := filepath.Join(t.TempDir(), "google.json")
	if err := os.WriteFile(credentialsPath, []byte(`{"type":"authorized_user","client_id":"fake","client_secret":"fake","refresh_token":"fake","token_uri":"http://credentials.test/token"}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credentialsPath)
	t.Setenv("GOOGLE_VERTEX_PROJECT", "fake-project")
	t.Setenv("GOOGLE_VERTEX_LOCATION", "us-central1")
	t.Setenv("AWS_ACCESS_KEY_ID", "fake-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fake-secret")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, model := range []string{"google-vertex/gemini-2.5-flash", "google-vertex-anthropic/claude-sonnet-4-5", "amazon-bedrock/anthropic.claude-v2"} {
		t.Run(model, func(t *testing.T) {
			requests := 0
			mockDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
				if strings.Contains(r.URL.Host, "credentials.test") || strings.Contains(r.URL.Host, "oauth2") {
					return jsonTestResponse(r, `{"access_token":"fake-google-token","token_type":"Bearer","expires_in":3600}`), nil
				}
				requests++
				if r.Header.Get("User-Agent") != "CloudApp/1" {
					t.Errorf("identity = %q", r.Header.Get("User-Agent"))
				}
				if r.Header.Get("Authorization") == "" {
					t.Error("cloud authorization lost")
				}
				return jsonTestResponse(r, `{}`), nil
			})
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			result, err := CreateProvider(ctx, &ProviderConfig{ModelString: model, UserAgent: "CloudApp/1"})
			if err != nil {
				t.Fatal(err)
			}
			_, _ = result.Model.Generate(ctx, fantasy.Call{Prompt: []fantasy.Message{fantasy.NewUserMessage("hi")}})
			if requests == 0 {
				t.Fatal("no model request")
			}
		})
	}
}

func TestNonStreamingIdentityResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "KIT" || r.Header.Get("Authorization") != "Bearer fake" {
			t.Errorf("headers = %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"fixture","object":"chat.completion","model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	result, err := createAutoRoutedOpenAICompatProvider(ctx, &ProviderConfig{ProviderAPIKey: "fake", ProviderURL: srv.URL + "/v1"}, "test", &ProviderInfo{ID: "test", Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	response, err := result.Model.Generate(ctx, fantasy.Call{Prompt: []fantasy.Message{fantasy.NewUserMessage("hi")}})
	if err != nil {
		t.Fatal(err)
	}
	if response.Content.Text() != "hello" {
		t.Fatalf("response = %v", response.Content)
	}
}
