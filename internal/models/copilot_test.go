package models

import (
	"net/http"
	"testing"
	"time"
)

func TestCopilotProviderAliasUsesCatalog(t *testing.T) {
	registry := NewModelsRegistry()

	models, err := registry.GetModelsForProvider("copilot")
	if err != nil {
		t.Fatalf("GetModelsForProvider(copilot) failed: %v", err)
	}
	if len(models) == 0 {
		t.Fatal("expected copilot alias to return github-copilot catalog models")
	}
	if registry.LookupModel("copilot", "gpt-5.5") == nil {
		t.Fatal("expected copilot/gpt-5.5 to resolve through github-copilot catalog")
	}
	if registry.GetProviderInfo("copilot") == nil {
		t.Fatal("expected copilot alias to return github-copilot provider info")
	}
}

func TestCopilotRejectsNonGPTModels(t *testing.T) {
	_, err := CreateProvider(t.Context(), &ProviderConfig{ModelString: "copilot/claude-sonnet-4.6"})
	if err == nil {
		t.Fatal("expected non-GPT Copilot model to be rejected")
	}
}

func TestCopilotHTTPClientCachesToken(t *testing.T) {
	client := createCopilotHTTPClient("cached-token", time.Now().Add(time.Hour).Unix(), false)
	transport, ok := client.Transport.(*copilotTransport)
	if !ok {
		t.Fatal("expected *copilotTransport")
	}

	token := transport.cachedToken(t.Context())
	if token != "cached-token" {
		t.Fatalf("expected cached token, got %q", token)
	}
}

func TestCopilotTransportHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	transport := &copilotTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("Authorization") != "Bearer cached-token" {
				t.Fatalf("unexpected Authorization header: %q", req.Header.Get("Authorization"))
			}
			if req.Header.Get("Copilot-Integration-Id") != copilotIntegrationID {
				t.Fatalf("unexpected Copilot-Integration-Id header: %q", req.Header.Get("Copilot-Integration-Id"))
			}
			if req.Header.Get("Editor-Version") != copilotEditorVersion {
				t.Fatalf("unexpected Editor-Version header: %q", req.Header.Get("Editor-Version"))
			}
			if req.Header.Get("User-Agent") != copilotUserAgent {
				t.Fatalf("unexpected User-Agent header: %q", req.Header.Get("User-Agent"))
			}
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		}),
		token:     "cached-token",
		expiresAt: time.Now().Add(time.Hour).Unix(),
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}
	_ = resp.Body.Close()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
