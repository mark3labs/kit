package models

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// TestSDKDefaultBaseURL_CoversAllWireMappedPackages enforces the invariant
// that every npm package recognised by the auto-router has a corresponding
// default base URL — otherwise a provider that omits its `api` field in the
// registry would silently fail to route at runtime.
func TestSDKDefaultBaseURL_CoversAllWireMappedPackages(t *testing.T) {
	for npm := range npmToWireProtocol {
		// @ai-sdk/openai-compatible is a wire family, not a single SDK with
		// a default URL — providers using it always supply their own `api`.
		if npm == "@ai-sdk/openai-compatible" {
			continue
		}
		if _, ok := sdkDefaultBaseURL[npm]; !ok {
			t.Errorf("npm %q is in npmToWireProtocol but has no sdkDefaultBaseURL entry — "+
				"providers using this npm with no `api` field cannot be routed", npm)
		}
	}
}

// TestSDKDefaultBaseURL_AllURLsAreAbsolute sanity-checks that every default
// URL is a well-formed absolute https endpoint (catches typos in the table).
func TestSDKDefaultBaseURL_AllURLsAreAbsolute(t *testing.T) {
	for npm, url := range sdkDefaultBaseURL {
		if !strings.HasPrefix(url, "https://") {
			t.Errorf("sdkDefaultBaseURL[%q] = %q is not an absolute https URL", npm, url)
		}
	}
}

// TestResolveProviderBaseURL_RegistryFirst verifies that the registry's `api`
// field wins over any SDK default.
func TestResolveProviderBaseURL_RegistryFirst(t *testing.T) {
	// xai is in the registry with no `api` field — its URL comes from the
	// SDK default. Use a synthetic registry-backed provider to test the
	// priority via the public registry instead.
	url, err := ResolveProviderBaseURL("openai")
	if err != nil {
		t.Fatalf("ResolveProviderBaseURL(openai): %v", err)
	}
	if url != "https://api.openai.com/v1" {
		t.Errorf("openai URL = %q, want https://api.openai.com/v1", url)
	}
}

// TestResolveProviderBaseURL_SDKDefaultFallback verifies that providers
// without an `api` field (groq, cerebras, xai, …) resolve to their SDK
// hard-coded default URL.
func TestResolveProviderBaseURL_SDKDefaultFallback(t *testing.T) {
	tests := map[string]string{
		"groq":       "https://api.groq.com/openai/v1",
		"cerebras":   "https://api.cerebras.ai/v1",
		"xai":        "https://api.x.ai/v1",
		"mistral":    "https://api.mistral.ai/v1",
		"perplexity": "https://api.perplexity.ai",
		"togetherai": "https://api.together.xyz/v1",
		"deepinfra":  "https://api.deepinfra.com/v1/openai",
		"cohere":     "https://api.cohere.com/compatibility/v1",
		"v0":         "https://api.v0.dev/v1",
		"aihubmix":   "https://aihubmix.com/v1",
		"venice":     "https://api.venice.ai/api/v1",
		"openrouter": "https://openrouter.ai/api/v1",
	}
	for providerID, wantURL := range tests {
		t.Run(providerID, func(t *testing.T) {
			got, err := ResolveProviderBaseURL(providerID)
			if err != nil {
				t.Fatalf("ResolveProviderBaseURL(%s): %v", providerID, err)
			}
			if got != wantURL {
				t.Errorf("%s URL = %q, want %q", providerID, got, wantURL)
			}
		})
	}
}

// TestResolveProviderBaseURL_TemplatedURL_MissingEnv verifies that providers
// whose URL contains "${VAR}" placeholders surface a targeted error when the
// environment variables are unset.
func TestResolveProviderBaseURL_TemplatedURL_MissingEnv(t *testing.T) {
	// cloudflare-workers-ai's api URL contains ${CLOUDFLARE_ACCOUNT_ID}.
	// Ensure the variable is unset for this test.
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
	t.Setenv("CF_ACCOUNT_ID", "")

	_, err := ResolveProviderBaseURL("cloudflare-workers-ai")
	if err == nil {
		t.Fatal("expected error for unset CLOUDFLARE_ACCOUNT_ID, got nil")
	}
	if !strings.Contains(err.Error(), "CLOUDFLARE_ACCOUNT_ID") {
		t.Errorf("error should name the missing env var, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--provider-url") {
		t.Errorf("error should suggest --provider-url override, got: %v", err)
	}
}

// TestResolveProviderBaseURL_TemplatedURL_Resolved verifies env-var
// substitution succeeds when the placeholder is set.
func TestResolveProviderBaseURL_TemplatedURL_Resolved(t *testing.T) {
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "test-acct-123")
	got, err := ResolveProviderBaseURL("cloudflare-workers-ai")
	if err != nil {
		t.Fatalf("ResolveProviderBaseURL: %v", err)
	}
	if !strings.Contains(got, "test-acct-123") {
		t.Errorf("resolved URL %q should contain test-acct-123", got)
	}
	if strings.Contains(got, "${") {
		t.Errorf("resolved URL %q still contains template placeholder", got)
	}
}

// TestResolveProviderBaseURL_UnknownProvider verifies the not-in-registry error.
func TestResolveProviderBaseURL_UnknownProvider(t *testing.T) {
	_, err := ResolveProviderBaseURL("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error should say 'unknown provider', got: %v", err)
	}
}

// TestAutoRouteProvider_SDKDefaultURLFallback verifies that providers whose
// registry entry omits the `api` field (groq, mistral, xai, etc.) are still
// auto-routed by falling back to the SDK's hard-coded default URL.
func TestAutoRouteProvider_SDKDefaultURLFallback(t *testing.T) {
	tests := []struct {
		name       string
		npmPackage string
		wantInURL  string
	}{
		{"groq", "@ai-sdk/groq", "groq.com"},
		{"cerebras", "@ai-sdk/cerebras", "cerebras.ai"},
		{"xai", "@ai-sdk/xai", "x.ai"},
		{"mistral", "@ai-sdk/mistral", "mistral.ai"},
		{"v0", "@ai-sdk/vercel", "v0.dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &ModelsRegistry{
				providers: map[string]ProviderInfo{
					"testfallback": {
						ID:   "testfallback",
						Name: "Test Fallback",
						Env:  []string{"TESTFALLBACK_API_KEY"},
						NPM:  tt.npmPackage,
						// API intentionally omitted — must fall back to SDK default.
						Models: map[string]ModelInfo{
							"any-model": {ID: "any-model", Name: "any-model"},
						},
					},
				},
			}
			config := &ProviderConfig{ProviderAPIKey: "test-key"}

			result, err := autoRouteProvider(context.Background(), config, "testfallback", "any-model", reg)
			if err != nil {
				t.Fatalf("autoRouteProvider returned error: %v", err)
			}
			if result == nil || result.Model == nil {
				t.Fatal("autoRouteProvider returned nil model")
			}
			// Verify the SDK default URL was picked up.
			if !strings.Contains(config.ProviderURL, tt.wantInURL) {
				t.Errorf("config.ProviderURL = %q, want substring %q (SDK default)",
					config.ProviderURL, tt.wantInURL)
			}
			// All these wrappers route through the openai-compat wire.
			gotType := reflect.TypeOf(result.Model).String()
			if gotType != "openai.languageModel" {
				t.Errorf("model type = %q, want openai.languageModel", gotType)
			}
		})
	}
}

// TestResolveTemplatedAPIURL_NoPlaceholders verifies that URLs without
// placeholders are returned as-is (the caller keeps using the original).
func TestResolveTemplatedAPIURL_NoPlaceholders(t *testing.T) {
	got, err := resolveTemplatedAPIURL("https://api.example.com/v1", &ProviderInfo{ID: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string for URL with no placeholders", got)
	}
}

// TestResolveTemplatedAPIURL_AltEnvVar verifies that the alternative env-var
// names (e.g. CF_ACCOUNT_ID for CLOUDFLARE_ACCOUNT_ID) are honoured.
func TestResolveTemplatedAPIURL_AltEnvVar(t *testing.T) {
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
	t.Setenv("CF_ACCOUNT_ID", "alt-name-123")

	got, err := resolveTemplatedAPIURL(
		"https://api.cloudflare.com/client/v4/accounts/${CLOUDFLARE_ACCOUNT_ID}/ai/v1",
		&ProviderInfo{ID: "cloudflare-workers-ai"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "alt-name-123") {
		t.Errorf("resolved URL %q should have picked up CF_ACCOUNT_ID alternative", got)
	}
}
