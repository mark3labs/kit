package models

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/httpclient"
)

func TestProviderHTTPTransport(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	for _, model := range []string{"anthropic/claude-sonnet-4-5", "openai/gpt-4o", "custom/test"} {
		t.Run(model, func(t *testing.T) {
			seen := 0
			cfg := &ProviderConfig{ModelString: model, ProviderAPIKey: "fake", ProviderURL: server.URL + "/v1", UserAgent: "TraceApp/1", HTTPTransport: func(base http.RoundTripper) http.RoundTripper {
				if base == nil {
					t.Fatal("nil base")
				}
				return roundTripFunc(func(req *http.Request) (*http.Response, error) {
					seen++
					if req.Header.Get("User-Agent") != "TraceApp/1" {
						t.Errorf("identity = %q", req.Header.Get("User-Agent"))
					}
					return base.RoundTrip(req)
				})
			}}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			result, err := CreateProvider(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = result.Model.Generate(ctx, fantasy.Call{Prompt: []fantasy.Message{fantasy.NewUserMessage("hello")}})
			if seen == 0 {
				t.Fatal("wrapper saw no requests")
			}
		})
	}
}

func TestProviderHTTPClientCopiesBase(t *testing.T) {
	base := &http.Client{Timeout: time.Second}
	wrapped := providerHTTPClient(base, &ProviderConfig{HTTPTransport: func(rt http.RoundTripper) http.RoundTripper {
		if rt != http.DefaultTransport {
			t.Error("wrong default base")
		}
		return rt
	}})
	if base.Transport != nil || wrapped == base || wrapped.Timeout != base.Timeout {
		t.Fatal("base client mutated or settings lost")
	}
	// A nil wrapper must be equivalent to the existing identity-only client.
	got := providerHTTPClient(base, &ProviderConfig{UserAgent: "App"})
	want := httpclient.WithUserAgent(base, "App")
	if !reflect.DeepEqual(got, want) {
		t.Fatal("nil wrapper changed the identity-only client")
	}
}
