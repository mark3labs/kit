package models

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestParseWire documents the accepted wire protocol names and aliases.
func TestParseWire(t *testing.T) {
	tests := []struct {
		name string
		want wireProtocol
		ok   bool
	}{
		{"openai", wireOpenAI, true},
		{"openai-responses", wireOpenAI, true},
		{"openai-compat", wireOpenAICompat, true},
		{"openai-compatible", wireOpenAICompat, true},
		{"openai-chat", wireOpenAICompat, true},
		{"anthropic", wireAnthropic, true},
		{"google", wireGoogle, true},
		{"gemini", wireGoogle, true},
		{"Anthropic", wireAnthropic, true},   // case-insensitive
		{" anthropic ", wireAnthropic, true}, // trimmed
		{"", wireUnknown, false},
		{"bogus", wireUnknown, false},
	}
	for _, tt := range tests {
		got, ok := parseWire(tt.name)
		if got != tt.want || ok != tt.ok {
			t.Errorf("parseWire(%q) = (%d, %v), want (%d, %v)", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

// newWireTestRegistry builds a registry with a single provider carrying an
// explicit Wire declaration (as produced by a `providers` config override).
func newWireTestRegistry(wire, api string) *ModelsRegistry {
	return &ModelsRegistry{
		providers: map[string]ProviderInfo{
			"wireproxy": {
				ID:     "wireproxy",
				Name:   "Wire Proxy",
				Env:    []string{"WIREPROXY_API_KEY"},
				API:    api,
				Wire:   wire,
				Models: map[string]ModelInfo{},
			},
		},
	}
}

// TestAutoRouteProvider_ExplicitWire verifies that a provider-level Wire
// declaration routes to the right fantasy implementation without any npm
// package information.
func TestAutoRouteProvider_ExplicitWire(t *testing.T) {
	tests := []struct {
		wire     string
		model    string
		wantType string
	}{
		{"openai", "gpt-4o", "openai.responsesLanguageModel"},
		{"openai-compat", "some-model", "openai.languageModel"},
		{"anthropic", "some-model", "anthropic.languageModel"},
		{"google", "some-model", "*google.languageModel"},
	}
	for _, tt := range tests {
		t.Run(tt.wire, func(t *testing.T) {
			reg := newWireTestRegistry(tt.wire, "https://proxy.example/v1")
			config := &ProviderConfig{ProviderAPIKey: "test-key"}

			result, err := autoRouteProvider(context.Background(), config, "wireproxy", tt.model, reg)
			if err != nil {
				t.Fatalf("autoRouteProvider returned error: %v", err)
			}
			gotType := reflect.TypeOf(result.Model).String()
			if gotType != tt.wantType {
				t.Errorf("routed to %s, want %s", gotType, tt.wantType)
			}
		})
	}
}

// TestAutoRouteProvider_WireFlagBeatsRegistry verifies that
// config.ProviderWire (--provider-wire) takes precedence over both the
// registry Wire declaration and the npm heuristic.
func TestAutoRouteProvider_WireFlagBeatsRegistry(t *testing.T) {
	// Registry says anthropic, flag says openai-compat.
	reg := newWireTestRegistry("anthropic", "https://proxy.example/v1")
	config := &ProviderConfig{ProviderAPIKey: "test-key", ProviderWire: "openai-compat"}

	result, err := autoRouteProvider(context.Background(), config, "wireproxy", "some-model", reg)
	if err != nil {
		t.Fatalf("autoRouteProvider returned error: %v", err)
	}
	if gotType := reflect.TypeOf(result.Model).String(); gotType != "openai.languageModel" {
		t.Errorf("routed to %s, want openai.languageModel", gotType)
	}
}

// TestAutoRouteProvider_WireBeatsNpm verifies that an explicit Wire wins over
// a conflicting npm package mapping.
func TestAutoRouteProvider_WireBeatsNpm(t *testing.T) {
	reg := &ModelsRegistry{
		providers: map[string]ProviderInfo{
			"wireproxy": {
				ID:     "wireproxy",
				Name:   "Wire Proxy",
				Env:    []string{"WIREPROXY_API_KEY"},
				API:    "https://proxy.example/v1",
				NPM:    "@ai-sdk/openai-compatible", // heuristic would say openai-compat
				Wire:   "anthropic",                 // explicit declaration wins
				Models: map[string]ModelInfo{},
			},
		},
	}
	config := &ProviderConfig{ProviderAPIKey: "test-key"}

	result, err := autoRouteProvider(context.Background(), config, "wireproxy", "some-model", reg)
	if err != nil {
		t.Fatalf("autoRouteProvider returned error: %v", err)
	}
	if gotType := reflect.TypeOf(result.Model).String(); gotType != "anthropic.languageModel" {
		t.Errorf("routed to %s, want anthropic.languageModel", gotType)
	}
}

// TestAutoRouteProvider_SynthesizedProvider verifies that a provider absent
// from the registry is synthesized when both --provider-url and
// --provider-wire are supplied.
func TestAutoRouteProvider_SynthesizedProvider(t *testing.T) {
	reg := &ModelsRegistry{providers: map[string]ProviderInfo{}}
	config := &ProviderConfig{
		ProviderAPIKey: "test-key",
		ProviderURL:    "https://llm.internal.corp/api",
		ProviderWire:   "anthropic",
	}

	result, err := autoRouteProvider(context.Background(), config, "corp-llm", "some-model", reg)
	if err != nil {
		t.Fatalf("autoRouteProvider returned error: %v", err)
	}
	if gotType := reflect.TypeOf(result.Model).String(); gotType != "anthropic.languageModel" {
		t.Errorf("routed to %s, want anthropic.languageModel", gotType)
	}
}

// TestAutoRouteProvider_SynthesizedProviderRequiresBoth verifies that an
// unknown provider without url+wire still errors with guidance.
func TestAutoRouteProvider_SynthesizedProviderRequiresBoth(t *testing.T) {
	reg := &ModelsRegistry{providers: map[string]ProviderInfo{}}

	for _, config := range []*ProviderConfig{
		{ProviderAPIKey: "test-key", ProviderURL: "https://x.example"}, // wire missing
		{ProviderAPIKey: "test-key", ProviderWire: "anthropic"},        // url missing
		{ProviderAPIKey: "test-key"},                                   // both missing
	} {
		_, err := autoRouteProvider(context.Background(), config, "corp-llm", "some-model", reg)
		if err == nil {
			t.Fatalf("expected error for config %+v, got nil", config)
		}
		if !strings.Contains(err.Error(), "not found in model database") {
			t.Errorf("unexpected error message: %v", err)
		}
	}
}

// TestAutoRouteProvider_InvalidWireFlag verifies the error for a bogus
// --provider-wire value.
func TestAutoRouteProvider_InvalidWireFlag(t *testing.T) {
	reg := newWireTestRegistry("", "https://proxy.example/v1")
	config := &ProviderConfig{ProviderAPIKey: "test-key", ProviderWire: "carrier-pigeon"}

	_, err := autoRouteProvider(context.Background(), config, "wireproxy", "some-model", reg)
	if err == nil {
		t.Fatal("expected error for invalid wire, got nil")
	}
	if !strings.Contains(err.Error(), "carrier-pigeon") || !strings.Contains(err.Error(), WireNameOpenAICompat) {
		t.Errorf("error should name the bad wire and list valid ones, got: %v", err)
	}
}

// TestApplyProviderOverrides_PatchExisting verifies field-level merge onto a
// provider that came from the database.
func TestApplyProviderOverrides_PatchExisting(t *testing.T) {
	providers := map[string]ProviderInfo{
		"minimax": {
			ID:   "minimax",
			Name: "MiniMax",
			Env:  []string{"MINIMAX_API_KEY"},
			NPM:  "@ai-sdk/openai-compatible",
			API:  "https://api.minimax.io/v1",
			Models: map[string]ModelInfo{
				"minimax-m2": {ID: "minimax-m2"},
			},
		},
	}

	applyProviderOverrides(providers, map[string]ProviderOverrideConfig{
		"minimax": {Wire: "anthropic"},
	})

	got := providers["minimax"]
	if got.Wire != "anthropic" {
		t.Errorf("Wire = %q, want anthropic", got.Wire)
	}
	// Unset override fields must inherit database values.
	if got.API != "https://api.minimax.io/v1" {
		t.Errorf("API = %q, want inherited database URL", got.API)
	}
	if len(got.Env) != 1 || got.Env[0] != "MINIMAX_API_KEY" {
		t.Errorf("Env = %v, want inherited [MINIMAX_API_KEY]", got.Env)
	}
	if len(got.Models) != 1 {
		t.Errorf("Models lost during override merge: %v", got.Models)
	}
}

// TestApplyProviderOverrides_DeclareNew verifies that an unknown provider ID
// is registered fresh with an empty model map.
func TestApplyProviderOverrides_DeclareNew(t *testing.T) {
	providers := map[string]ProviderInfo{}

	applyProviderOverrides(providers, map[string]ProviderOverrideConfig{
		"corp-llm": {
			Name:      "Corp LLM Gateway",
			Wire:      "anthropic",
			BaseURL:   "https://llm.internal.corp/api",
			APIKeyEnv: []string{"CORP_LLM_KEY", "LLM_GATEWAY_KEY"},
			Headers:   map[string]string{"X-Team": "platform"},
		},
	})

	got, ok := providers["corp-llm"]
	if !ok {
		t.Fatal("corp-llm not registered")
	}
	if got.Name != "Corp LLM Gateway" || got.Wire != "anthropic" || got.API != "https://llm.internal.corp/api" {
		t.Errorf("unexpected provider info: %+v", got)
	}
	if len(got.Env) != 2 || got.Env[0] != "CORP_LLM_KEY" {
		t.Errorf("Env = %v, want [CORP_LLM_KEY LLM_GATEWAY_KEY]", got.Env)
	}
	if got.Headers["X-Team"] != "platform" {
		t.Errorf("Headers = %v, want X-Team: platform", got.Headers)
	}
	if got.Models == nil {
		t.Error("Models map must be initialized (advisory lookups)")
	}
}

// TestApplyProviderOverrides_InvalidWireIgnored verifies that an override
// with a bogus wire is skipped entirely (no partial application).
func TestApplyProviderOverrides_InvalidWireIgnored(t *testing.T) {
	providers := map[string]ProviderInfo{
		"minimax": {ID: "minimax", Name: "MiniMax", API: "https://api.minimax.io/v1"},
	}

	applyProviderOverrides(providers, map[string]ProviderOverrideConfig{
		"minimax": {Wire: "smoke-signals", BaseURL: "https://evil.example"},
	})

	got := providers["minimax"]
	if got.Wire != "" || got.API != "https://api.minimax.io/v1" {
		t.Errorf("invalid-wire override must be skipped entirely, got: %+v", got)
	}
}

// TestLoadProviderOverridesFrom verifies config parsing from a viper store.
func TestLoadProviderOverridesFrom(t *testing.T) {
	v := viper.New()
	v.Set("providers", map[string]any{
		"minimax": map[string]any{"wire": "anthropic"},
		"corp-llm": map[string]any{
			"name":      "Corp LLM",
			"wire":      "openai-compat",
			"baseUrl":   "https://llm.internal.corp/api",
			"apiKeyEnv": []string{"CORP_LLM_KEY"},
			"headers":   map[string]string{"X-Team": "platform"},
		},
	})

	overrides := loadProviderOverridesFrom(v)
	if len(overrides) != 2 {
		t.Fatalf("got %d overrides, want 2", len(overrides))
	}
	if overrides["minimax"].Wire != "anthropic" {
		t.Errorf("minimax.Wire = %q", overrides["minimax"].Wire)
	}
	corp := overrides["corp-llm"]
	if corp.Name != "Corp LLM" || corp.BaseURL != "https://llm.internal.corp/api" {
		t.Errorf("corp-llm parsed wrong: %+v", corp)
	}
	if len(corp.APIKeyEnv) != 1 || corp.APIKeyEnv[0] != "CORP_LLM_KEY" {
		t.Errorf("corp-llm.APIKeyEnv = %v", corp.APIKeyEnv)
	}
	if corp.Headers["X-Team"] != "platform" {
		t.Errorf("corp-llm.Headers = %v", corp.Headers)
	}

	// Absent key → nil.
	if got := loadProviderOverridesFrom(viper.New()); got != nil {
		t.Errorf("expected nil for absent providers key, got %v", got)
	}
}

// TestIsProviderLLMSupported_ExplicitWire verifies that a wire-only override
// (no npm, no API URL) marks a provider as LLM-supported.
func TestIsProviderLLMSupported_ExplicitWire(t *testing.T) {
	info := &ProviderInfo{ID: "corp-llm", Wire: "anthropic"}
	if !isProviderLLMSupported("corp-llm", info) {
		t.Error("expected wire-declared provider to be LLM-supported")
	}
}

// headerRecordingRoundTripper captures the headers of the request it receives.
type headerRecordingRoundTripper struct{ got http.Header }

func (r *headerRecordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.got = req.Header.Clone()
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("{}")),
		Header:     make(http.Header),
	}, nil
}

// TestHeaderRoundTripper verifies default-header injection semantics:
// missing headers are added, existing request headers are not overwritten,
// and the original request is not mutated.
func TestHeaderRoundTripper(t *testing.T) {
	rec := &headerRecordingRoundTripper{}
	tr := &headerRoundTripper{
		base:    rec,
		headers: map[string]string{"X-Team": "platform", "Authorization": "Bearer default"},
	}

	req, err := http.NewRequest(http.MethodPost, "https://llm.internal.corp/api", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer user-supplied")

	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}

	if got := rec.got.Get("X-Team"); got != "platform" {
		t.Errorf("X-Team = %q, want platform", got)
	}
	if got := rec.got.Get("Authorization"); got != "Bearer user-supplied" {
		t.Errorf("Authorization = %q, existing header must not be overwritten", got)
	}
	if got := req.Header.Get("X-Team"); got != "" {
		t.Errorf("original request mutated: X-Team = %q", got)
	}
}

// TestWithDefaultHeaders verifies the nil-client contract.
func TestWithDefaultHeaders(t *testing.T) {
	if got := withDefaultHeaders(nil, nil); got != nil {
		t.Error("nil client + no headers must stay nil")
	}
	if got := withDefaultHeaders(nil, map[string]string{"X": "y"}); got == nil {
		t.Error("nil client + headers must produce a client")
	}
	base := &http.Client{}
	if got := withDefaultHeaders(base, nil); got != base {
		t.Error("client + no headers must be returned unchanged")
	}
	wrapped := withDefaultHeaders(&http.Client{}, map[string]string{"X": "y"})
	if _, ok := wrapped.Transport.(*headerRoundTripper); !ok {
		t.Errorf("transport not wrapped: %T", wrapped.Transport)
	}
}
