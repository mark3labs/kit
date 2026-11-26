package models

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"charm.land/fantasy"
)

// Exercise the real provider and SDK, not just the path rewrite in isolation.
func TestGeminiAPIVersion(t *testing.T) {
	for _, route := range []string{"direct", "auto", "auto-prefix", "proxy-v1", "proxy-beta"} {
		for _, stream := range []bool{false, true} {
			t.Run(route+map[bool]string{false: "/generate", true: "/stream"}[stream], func(t *testing.T) {
				t.Setenv("GOOGLE_GENAI_API_VERSION", "v1")
				baseURL := "https://example.test"
				wantBase := "/v1beta"
				switch route {
				case "auto-prefix":
					baseURL += "/gateway"
					wantBase = "/gateway/v1beta"
				case "proxy-v1":
					baseURL += "/zen/v1"
					wantBase = "/zen/v1"
				case "proxy-beta":
					baseURL += "/google/v1beta"
					wantBase = "/google/v1beta"
				}
				method := ":generateContent"
				if stream {
					method = ":streamGenerateContent"
				}
				wantPath := wantBase + "/models/gemini-flash-latest" + method
				seen := 0
				mockDefaultTransport(t, func(req *http.Request) (*http.Response, error) {
					seen++
					if req.URL.Path != wantPath {
						t.Errorf("path = %q, want %q", req.URL.Path, wantPath)
					}
					if req.Header.Get("x-goog-api-key") != "fake" {
						t.Errorf("missing API key: %v", req.Header)
					}
					response := jsonTestResponse(req, `{"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`)
					if stream {
						response.Header.Set("Content-Type", "text/event-stream")
						response.Body = io.NopCloser(strings.NewReader(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}` + "\n\n"))
					}
					return response, nil
				})
				cfg := &ProviderConfig{ProviderAPIKey: "fake", ProviderURL: baseURL}
				var result *ProviderResult
				var err error
				if route == "direct" {
					result, err = createGoogleProvider(t.Context(), cfg, "gemini-flash-latest")
				} else {
					reg := newTestRegistry(baseURL, "@ai-sdk/google", "gemini-flash-latest", "")
					result, err = autoRouteProvider(t.Context(), cfg, "testproxy", "gemini-flash-latest", reg)
				}
				if err != nil {
					t.Fatal(err)
				}
				call := fantasy.Call{Prompt: []fantasy.Message{fantasy.NewUserMessage("hi")}}
				if stream {
					events, err := result.Model.Stream(t.Context(), call)
					if err != nil {
						t.Fatal(err)
					}
					for event := range events {
						if event.Type == fantasy.StreamPartTypeError {
							t.Fatalf("stream error: %v", event.Error)
						}
					}
				} else if _, err := result.Model.Generate(t.Context(), call); err != nil {
					t.Fatal(err)
				}
				if seen != 1 {
					t.Fatalf("requests = %d, want 1", seen)
				}
			})
		}
	}
}

func TestGeminiBetaTransport(t *testing.T) {
	for _, path := range []string{"/gateway/v1/models/x", "/gateway/v1beta/models/x", "/other/v1/models/x", "/gateway/v10/models/x"} {
		t.Run(path, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "https://example.test"+path+"?alt=sse", nil)
			if err != nil {
				t.Fatal(err)
			}
			want := path
			if path == "/gateway/v1/models/x" {
				want = "/gateway/v1beta/models/x"
			}
			transport := &geminiBetaTransport{basePath: "/gateway", base: roundTripFunc(func(got *http.Request) (*http.Response, error) {
				if got.URL.Path != want || got.URL.RawQuery != "alt=sse" {
					t.Errorf("URL = %s", got.URL)
				}
				return jsonTestResponse(got, `{}`), nil
			})}
			response, err := transport.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if req.URL.Path != path {
				t.Errorf("original request changed: %s", req.URL.Path)
			}
		})
	}
}
