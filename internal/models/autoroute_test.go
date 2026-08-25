package models

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// TestNpmToWireProtocol documents the wire protocols that the auto-router
// understands. Provider-specific bundles that need bespoke auth or URL
// templating (azure, bedrock, openrouter, google-vertex*, @ai-sdk/gateway)
// are intentionally absent — they have native top-level cases in
// CreateProvider and never reach the auto-router.
func TestNpmToWireProtocol(t *testing.T) {
	want := map[string]wireProtocol{
		"@ai-sdk/openai":            wireOpenAI,
		"@ai-sdk/openai-compatible": wireOpenAICompat,
		"@ai-sdk/anthropic":         wireAnthropic,
		"@ai-sdk/google":            wireGoogle,

		// Thin OpenAI-compatible wrappers — routed via openaicompat using
		// the SDK's hard-coded default base URL (sdkDefaultBaseURL).
		"@ai-sdk/groq":                  wireOpenAICompat,
		"@ai-sdk/cerebras":              wireOpenAICompat,
		"@ai-sdk/perplexity":            wireOpenAICompat,
		"@ai-sdk/togetherai":            wireOpenAICompat,
		"@ai-sdk/xai":                   wireOpenAICompat,
		"@ai-sdk/deepinfra":             wireOpenAICompat,
		"@ai-sdk/mistral":               wireOpenAICompat,
		"@ai-sdk/cohere":                wireOpenAICompat,
		"@ai-sdk/vercel":                wireOpenAICompat,
		"@aihubmix/ai-sdk-provider":     wireOpenAICompat,
		"venice-ai-sdk-provider":        wireOpenAICompat,
		"merge-gateway-ai-sdk-provider": wireOpenAICompat,
	}
	for npm, wire := range want {
		if got := npmToWireProtocol[npm]; got != wire {
			t.Errorf("npmToWireProtocol[%q] = %d, want %d", npm, got, wire)
		}
	}

	// Bundle packages must NOT be in the table — they need bespoke auth or
	// URL templating that the auto-router cannot satisfy.
	for _, npm := range []string{
		"@ai-sdk/google-vertex",
		"@ai-sdk/google-vertex/anthropic",
		"@ai-sdk/amazon-bedrock",
		"@ai-sdk/azure",
		"@openrouter/ai-sdk-provider",
		"@ai-sdk/gateway",
	} {
		if _, ok := npmToWireProtocol[npm]; ok {
			t.Errorf("npmToWireProtocol unexpectedly contains bundle package %q", npm)
		}
	}
}

// newTestRegistry builds a registry containing a single proxy-style provider
// ("testproxy") with the given default npm, plus one model that carries the
// given per-model npm override.
func newTestRegistry(api, defaultNPM, modelID, modelNPMOverride string) *ModelsRegistry {
	return &ModelsRegistry{
		providers: map[string]ProviderInfo{
			"testproxy": {
				ID:   "testproxy",
				Name: "Test Proxy",
				Env:  []string{"TESTPROXY_API_KEY"},
				NPM:  defaultNPM,
				API:  api,
				Models: map[string]ModelInfo{
					modelID: {
						ID:          modelID,
						Name:        modelID,
						ProviderNPM: modelNPMOverride,
					},
				},
			},
		},
	}
}

// TestAutoRouteProvider_WireRouting verifies that autoRouteProvider routes each
// npm package to the correct fantasy provider implementation. This is the core
// regression test for issue #41: previously any npm that resolved to a
// non-openai/anthropic/openaicompat LLM provider (notably @ai-sdk/google) hit a
// dead `default` branch and failed with "has no LLM provider mapping".
func TestAutoRouteProvider_WireRouting(t *testing.T) {
	tests := []struct {
		name        string
		modelID     string
		defaultNPM  string
		overrideNPM string
		// wantType is the concrete fantasy LanguageModel type the model should
		// be routed to, identified by reflect type string.
		wantType string
	}{
		{
			name:       "openai-compatible default",
			modelID:    "test-model",
			defaultNPM: "@ai-sdk/openai-compatible",
			wantType:   "openai.languageModel",
		},
		{
			name:        "anthropic override",
			modelID:     "test-model",
			defaultNPM:  "@ai-sdk/openai-compatible",
			overrideNPM: "@ai-sdk/anthropic",
			wantType:    "anthropic.languageModel",
		},
		{
			name:        "openai (responses) override",
			modelID:     "gpt-4o",
			defaultNPM:  "@ai-sdk/openai-compatible",
			overrideNPM: "@ai-sdk/openai",
			wantType:    "openai.responsesLanguageModel",
		},
		{
			// The bug: opencode's gemini-* models override the default
			// openai-compatible npm with @ai-sdk/google.
			name:        "google override (issue #41)",
			modelID:     "gemini-3.5-flash",
			defaultNPM:  "@ai-sdk/openai-compatible",
			overrideNPM: "@ai-sdk/google",
			wantType:    "*google.languageModel",
		},
		{
			// Unknown npm but provider has an API URL → openai-compatible fallback.
			name:       "unknown npm with API URL falls back to openai-compat",
			modelID:    "test-model",
			defaultNPM: "@ai-sdk/some-future-thing",
			wantType:   "openai.languageModel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := newTestRegistry("https://proxy.example/v1", tt.defaultNPM, tt.modelID, tt.overrideNPM)
			config := &ProviderConfig{ProviderAPIKey: "test-key"}

			result, err := autoRouteProvider(context.Background(), config, "testproxy", tt.modelID, reg)
			if err != nil {
				t.Fatalf("autoRouteProvider returned error: %v", err)
			}
			if result == nil || result.Model == nil {
				t.Fatalf("autoRouteProvider returned nil model")
			}

			gotType := reflect.TypeOf(result.Model).String()
			if gotType != tt.wantType {
				t.Errorf("routed to %s, want %s", gotType, tt.wantType)
			}
		})
	}
}

// TestLookupModelWire verifies the per-model wire pins that correct catalogs
// which mix wires under a single npm package (opencode zen).
func TestLookupModelWire(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		wantWire wireProtocol
		wantOK   bool
	}{
		{"opencode", "grok-4.5", wireOpenAI, true},
		{"opencode", "grok-4.6", wireOpenAI, true},
		{"opencode", "grok-build-0.1", wireOpenAI, true},
		{"opencode", "gpt-5.4", wireOpenAI, true},
		{"opencode", "GPT-5-Nano", wireOpenAI, true},
		// Open-weight and native-wire families keep the npm heuristic.
		{"opencode", "kimi-k2.5", wireUnknown, false},
		{"opencode", "claude-opus-4-5", wireUnknown, false},
		{"opencode", "gemini-3.5-flash", wireUnknown, false},
		{"opencode", "big-pickle", wireUnknown, false},
		// Other providers are untouched.
		{"xai", "grok-4.5", wireUnknown, false},
	}

	for _, tt := range tests {
		gotWire, gotOK := lookupModelWire(tt.provider, tt.model)
		if gotWire != tt.wantWire || gotOK != tt.wantOK {
			t.Errorf("lookupModelWire(%q, %q) = (%d, %v), want (%d, %v)",
				tt.provider, tt.model, gotWire, gotOK, tt.wantWire, tt.wantOK)
		}
	}
}

// TestAutoRouteProvider_ModelWirePin is the regression test for opencode's
// grok-* and gpt-* models. models.dev declares @ai-sdk/openai-compatible for
// the whole opencode catalog, which routed these models to /chat/completions —
// an endpoint the zen gateway answers with HTTP 500/503 for these families.
// The per-model pin must route them to the Responses API instead, whatever the
// model ID looks like.
func TestAutoRouteProvider_ModelWirePin(t *testing.T) {
	tests := []struct {
		modelID  string
		wantType string
	}{
		{"grok-4.5", "openai.responsesLanguageModel"},
		{"grok-4.6", "openai.responsesLanguageModel"},
		{"gpt-5.4", "openai.responsesLanguageModel"},
		// Open-weight models stay on chat completions.
		{"kimi-k2.5", "openai.languageModel"},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			reg := &ModelsRegistry{
				providers: map[string]ProviderInfo{
					"opencode": {
						ID:   "opencode",
						Name: "opencode",
						Env:  []string{"OPENCODE_API_KEY"},
						NPM:  "@ai-sdk/openai-compatible",
						API:  "https://opencode.ai/zen/v1",
						Models: map[string]ModelInfo{
							tt.modelID: {ID: tt.modelID, Name: tt.modelID},
						},
					},
				},
			}
			config := &ProviderConfig{ProviderAPIKey: "test-key"}

			result, err := autoRouteProvider(context.Background(), config, "opencode", tt.modelID, reg)
			if err != nil {
				t.Fatalf("autoRouteProvider returned error: %v", err)
			}
			if gotType := reflect.TypeOf(result.Model).String(); gotType != tt.wantType {
				t.Errorf("routed %s to %s, want %s", tt.modelID, gotType, tt.wantType)
			}
		})
	}
}

// TestAutoRouteProvider_WireFlagBeatsModelPin verifies that --provider-wire
// still overrides a per-model pin.
func TestAutoRouteProvider_WireFlagBeatsModelPin(t *testing.T) {
	reg := &ModelsRegistry{
		providers: map[string]ProviderInfo{
			"opencode": {
				ID:     "opencode",
				Name:   "opencode",
				Env:    []string{"OPENCODE_API_KEY"},
				NPM:    "@ai-sdk/openai-compatible",
				API:    "https://opencode.ai/zen/v1",
				Models: map[string]ModelInfo{"grok-4.6": {ID: "grok-4.6", Name: "grok-4.6"}},
			},
		},
	}
	config := &ProviderConfig{ProviderAPIKey: "test-key", ProviderWire: WireNameOpenAICompat}

	result, err := autoRouteProvider(context.Background(), config, "opencode", "grok-4.6", reg)
	if err != nil {
		t.Fatalf("autoRouteProvider returned error: %v", err)
	}
	if gotType := reflect.TypeOf(result.Model).String(); gotType != "openai.languageModel" {
		t.Errorf("routed to %s, want openai.languageModel", gotType)
	}
}

// TestAutoRouteProvider_UnknownNpmNoAPI verifies the improved error message for
// a provider whose npm has no known wire protocol and that has no API URL to
// fall back on.
func TestAutoRouteProvider_UnknownNpmNoAPI(t *testing.T) {
	reg := newTestRegistry("", "@ai-sdk/unmapped", "test-model", "")
	config := &ProviderConfig{ProviderAPIKey: "test-key"}

	_, err := autoRouteProvider(context.Background(), config, "testproxy", "test-model", reg)
	if err == nil {
		t.Fatal("expected error for unknown npm with no API URL, got nil")
	}
	if !strings.Contains(err.Error(), "cannot auto-route provider testproxy") {
		t.Errorf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "--provider-url") {
		t.Errorf("error should suggest --provider-url, got: %v", err)
	}
}

// TestAutoRouteProvider_UnknownProvider verifies the not-in-database error.
func TestAutoRouteProvider_UnknownProvider(t *testing.T) {
	reg := newTestRegistry("https://proxy.example/v1", "@ai-sdk/openai-compatible", "test-model", "")
	config := &ProviderConfig{ProviderAPIKey: "test-key"}

	_, err := autoRouteProvider(context.Background(), config, "does-not-exist", "test-model", reg)
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "not found in model database") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestIsProviderLLMSupported_Google verifies that a provider whose npm is
// @ai-sdk/google is reported as supported (it now maps to a wire protocol).
func TestIsProviderLLMSupported_Google(t *testing.T) {
	info := &ProviderInfo{ID: "testproxy", NPM: "@ai-sdk/google"}
	if !isProviderLLMSupported("testproxy", info) {
		t.Error("expected @ai-sdk/google provider to be LLM-supported")
	}
}

// TestVersionedBasePath verifies detection of proxy base URLs that already
// carry an API version segment (which collides with the genai SDK's injected
// version).
func TestVersionedBasePath(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		{"https://opencode.ai/zen/v1", "/zen/v1"},
		{"https://opencode.ai/zen/v1/", "/zen/v1"},
		{"https://example.com/api/v1beta", "/api/v1beta"},
		{"https://example.com/api/v2alpha", "/api/v2alpha"},
		{"https://generativelanguage.googleapis.com", ""},
		{"https://proxy.example/openai", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := versionedBasePath(tt.rawURL); got != tt.want {
			t.Errorf("versionedBasePath(%q) = %q, want %q", tt.rawURL, got, tt.want)
		}
	}
}

// recordingRoundTripper captures the path of the request it receives.
type recordingRoundTripper struct{ gotPath string }

func (r *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.gotPath = req.URL.Path
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("{}")),
		Header:     make(http.Header),
	}, nil
}

// TestGeminiProxyTransport_StripsInjectedVersion verifies that the transport
// collapses the genai-injected "/v1beta" segment that follows a proxy base
// URL which already carries its own version segment. This is the second-order
// fix that makes opencode/gemini-* actually reach the proxy (issue #41).
func TestGeminiProxyTransport_StripsInjectedVersion(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		reqPath  string
		wantPath string
	}{
		{
			name:     "strips doubled v1beta after /zen/v1",
			basePath: "/zen/v1",
			reqPath:  "/zen/v1/v1beta/models/gemini-3.5-flash:generateContent",
			wantPath: "/zen/v1/models/gemini-3.5-flash:generateContent",
		},
		{
			name:     "strips doubled v1beta1 after /zen/v1",
			basePath: "/zen/v1",
			reqPath:  "/zen/v1/v1beta1/models/gemini-3.5-flash:generateContent",
			wantPath: "/zen/v1/models/gemini-3.5-flash:generateContent",
		},
		{
			name:     "leaves non-matching path untouched",
			basePath: "/zen/v1",
			reqPath:  "/other/v1beta/models/x:generateContent",
			wantPath: "/other/v1beta/models/x:generateContent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recordingRoundTripper{}
			tr := &geminiProxyTransport{base: rec, basePath: tt.basePath}
			req, err := http.NewRequest(http.MethodPost, "https://host"+tt.reqPath, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			if _, err := tr.RoundTrip(req); err != nil {
				t.Fatalf("RoundTrip: %v", err)
			}
			if rec.gotPath != tt.wantPath {
				t.Errorf("forwarded path = %q, want %q", rec.gotPath, tt.wantPath)
			}
		})
	}
}
