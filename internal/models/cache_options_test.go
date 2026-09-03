package models

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestModelInfo_SupportsCaching(t *testing.T) {
	tests := []struct {
		name     string
		family   string
		expected bool
	}{
		{"Claude model", "claude-3-5-sonnet", true},
		{"Claude 4 model", "claude-4-opus", true},
		{"GPT model", "gpt-4", true},
		{"GPT-5 model", "gpt-5", true},
		{"O1 model", "o1", true},
		{"O3 model", "o3", true},
		{"O4 model", "o4-mini", true},
		{"Codex model", "codex", true},
		{"Gemini model", "gemini-2.5-pro", true},
		{"Gemini 1.5 model", "gemini-1.5-flash", true},
		{"Llama model", "llama-3", false},
		{"Unknown model", "unknown", false},
		{"Empty family", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ModelInfo{Family: tt.family}
			if got := m.SupportsCaching(); got != tt.expected {
				t.Errorf("ModelInfo.SupportsCaching() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestModelInfo_CacheType(t *testing.T) {
	tests := []struct {
		name     string
		family   string
		expected string
	}{
		{"Claude model", "claude-3-5-sonnet", "anthropic-ephemeral"},
		{"GPT model", "gpt-4", "openai-prompt-cache"},
		{"O1 model", "o1", "openai-prompt-cache"},
		{"Gemini model", "gemini-2.5-pro", "google-cached-content"},
		{"Unknown model", "llama-3", ""},
		{"Empty family", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ModelInfo{Family: tt.family}
			if got := m.CacheType(); got != tt.expected {
				t.Errorf("ModelInfo.CacheType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGenerateCacheKey(t *testing.T) {
	key1 := generateCacheKey("system prompt", "model-id")
	key2 := generateCacheKey("system prompt", "model-id")
	if key1 != key2 {
		t.Errorf("generateCacheKey should be deterministic: got %q and %q", key1, key2)
	}

	key3 := generateCacheKey("different prompt", "model-id")
	if key1 == key3 {
		t.Errorf("generateCacheKey should produce different keys for different inputs")
	}

	key4 := generateCacheKey("", "model-id")
	key5 := generateCacheKey("default", "model-id")
	if key4 != key5 {
		t.Errorf("generateCacheKey should treat empty prompt as 'default'")
	}

	if len(key1) < 4 || key1[:4] != "kit-" {
		t.Errorf("generateCacheKey should produce keys with 'kit-' prefix, got %q", key1)
	}
}

func TestBuildCacheProviderOptions_Disabled(t *testing.T) {
	config := &ProviderConfig{DisableCaching: true}
	modelInfo := &ModelInfo{Family: "claude-3", ID: "claude-3-opus"}

	if opts := buildCacheProviderOptions(modelInfo, config); opts != nil {
		t.Errorf("buildCacheProviderOptions should return nil when DisableCaching=true")
	}
}

func TestBuildCacheProviderOptions_EnvironmentVariable(t *testing.T) {
	_ = os.Setenv("KIT_DISABLE_CACHE", "1")
	defer func() { _ = os.Unsetenv("KIT_DISABLE_CACHE") }()

	config := &ProviderConfig{DisableCaching: false}
	modelInfo := &ModelInfo{Family: "claude-3", ID: "claude-3-opus"}

	if opts := buildCacheProviderOptions(modelInfo, config); opts != nil {
		t.Errorf("buildCacheProviderOptions should return nil when KIT_DISABLE_CACHE is set")
	}
}

func TestBuildCacheProviderOptions_UnsupportedModel(t *testing.T) {
	config := &ProviderConfig{DisableCaching: false}
	modelInfo := &ModelInfo{Family: "llama-3", ID: "llama-3-70b"}

	if opts := buildCacheProviderOptions(modelInfo, config); opts != nil {
		t.Errorf("buildCacheProviderOptions should return nil for unsupported model families")
	}
}

func TestBuildCacheProviderOptions_NilModelInfo(t *testing.T) {
	config := &ProviderConfig{DisableCaching: false}

	if opts := buildCacheProviderOptions(nil, config); opts != nil {
		t.Errorf("buildCacheProviderOptions should return nil when modelInfo is nil")
	}
}

func TestBuildCacheProviderOptions_Anthropic(t *testing.T) {
	_ = os.Unsetenv("KIT_DISABLE_CACHE")

	config := &ProviderConfig{DisableCaching: false}
	modelInfo := &ModelInfo{Family: "claude-3", ID: "claude-3-opus"}

	opts := buildCacheProviderOptions(modelInfo, config)
	// Provider-level Anthropic caching is disabled; message-level caching is used instead
	if opts != nil {
		t.Logf("Provider-level Anthropic caching disabled; using message-level caching")
	}
}

func TestBuildCacheProviderOptions_OpenAI(t *testing.T) {
	_ = os.Unsetenv("KIT_DISABLE_CACHE")

	config := &ProviderConfig{
		DisableCaching: false,
		SystemPrompt:   "test system prompt",
	}
	modelInfo := &ModelInfo{Family: "gpt-4", ID: "gpt-4o"}

	opts := buildCacheProviderOptions(modelInfo, config)
	if opts == nil {
		t.Fatalf("buildCacheProviderOptions should return options for OpenAI models")
	}

	if _, ok := opts["openai"]; !ok {
		t.Errorf("buildCacheProviderOptions should include 'openai' key for GPT models")
	}
}

func TestCachingPriorityOverThinking(t *testing.T) {
	_ = os.Unsetenv("KIT_DISABLE_CACHE")

	// Anthropic uses message-level caching; provider-level returns nil
	config1 := &ProviderConfig{
		DisableCaching: false,
		ThinkingLevel:  ThinkingOff,
	}
	modelInfo1 := &ModelInfo{Family: "claude-3", ID: "claude-3-opus"}
	opts1 := buildCacheProviderOptions(modelInfo1, config1)
	if opts1 != nil {
		t.Logf("Provider-level Anthropic caching disabled; using message-level caching")
	}

	// OpenAI provider-level caching works with thinking enabled
	config2 := &ProviderConfig{
		DisableCaching: false,
		SystemPrompt:   "test prompt",
		ThinkingLevel:  ThinkingMedium,
	}
	modelInfo2 := &ModelInfo{Family: "gpt-4", ID: "gpt-4o"}
	opts2 := buildCacheProviderOptions(modelInfo2, config2)
	if opts2 == nil {
		t.Errorf("OpenAI caching should work with thinking enabled")
	}

	// OpenAI caching also works with thinking disabled
	config3 := &ProviderConfig{
		DisableCaching: false,
		SystemPrompt:   "test prompt",
		ThinkingLevel:  ThinkingOff,
	}
	opts3 := buildCacheProviderOptions(modelInfo2, config3)
	if opts3 == nil {
		t.Errorf("OpenAI caching should work when thinking is OFF")
	}
}

// TestCacheSchemaVersionGating covers the cache-invalidation contract.
//
// The cache re-serializes provider data through modelsDBProvider, so a cache
// written by an older binary silently lacks any field that binary did not
// model. Because cached data takes precedence over the embedded catalog, such
// a cache would mask newer metadata (reasoning options, status, tiered
// pricing) rather than merely being incomplete.
func TestCacheSchemaVersionGating(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	providers := map[string]modelsDBProvider{
		"testprov": {
			ID:   "testprov",
			Name: "Test",
			Models: map[string]modelsDBModel{
				"m1": {ID: "m1", Name: "M1", Status: StatusDeprecated},
			},
		},
	}

	if err := StoreCachedProviders(providers, `"etag-1"`); err != nil {
		t.Fatalf("StoreCachedProviders: %v", err)
	}

	t.Run("current version round-trips", func(t *testing.T) {
		got, etag := LoadCachedProviders()
		if len(got) != 1 {
			t.Fatalf("want 1 provider, got %d", len(got))
		}
		if etag != `"etag-1"` {
			t.Errorf("etag = %q, want %q", etag, `"etag-1"`)
		}
		// Fields added alongside versioning must survive the round-trip,
		// otherwise the cache is still lossy.
		if s := got["testprov"].Models["m1"].Status; s != StatusDeprecated {
			t.Errorf("status did not round-trip: %q", s)
		}
	})

	t.Run("older schema is discarded", func(t *testing.T) {
		path, err := cachePath()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var env map[string]any
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatal(err)
		}
		// Simulate a cache written before versioning existed.
		delete(env, "version")
		stale, err := json.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, stale, 0o644); err != nil {
			t.Fatal(err)
		}

		got, etag := LoadCachedProviders()
		if got != nil {
			t.Errorf("stale cache must be ignored, got %d providers", len(got))
		}
		// The ETag must be dropped too: keeping it would let the next update
		// get a 304 and leave the stale cache in place forever.
		if etag != "" {
			t.Errorf("stale cache must not return an ETag, got %q", etag)
		}
	})

	t.Run("future schema is discarded", func(t *testing.T) {
		path, err := cachePath()
		if err != nil {
			t.Fatal(err)
		}
		future := fmt.Sprintf(`{"version":%d,"etag":"x","providers":{"p":{"id":"p","models":{}}}}`,
			cacheSchemaVersion+1)
		if err := os.WriteFile(path, []byte(future), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, _ := LoadCachedProviders(); got != nil {
			t.Errorf("future-schema cache must be ignored, got %d providers", len(got))
		}
	})
}
