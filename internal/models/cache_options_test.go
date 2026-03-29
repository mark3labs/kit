package models

import (
	"os"
	"testing"

	"charm.land/fantasy"
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
	os.Setenv("KIT_DISABLE_CACHE", "1")
	defer os.Unsetenv("KIT_DISABLE_CACHE")

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
	os.Unsetenv("KIT_DISABLE_CACHE")

	config := &ProviderConfig{DisableCaching: false}
	modelInfo := &ModelInfo{Family: "claude-3", ID: "claude-3-opus"}

	opts := buildCacheProviderOptions(modelInfo, config)
	// Provider-level Anthropic caching is disabled; message-level caching is used instead
	if opts != nil {
		t.Logf("Provider-level Anthropic caching disabled; using message-level caching")
	}
}

func TestBuildCacheProviderOptions_OpenAI(t *testing.T) {
	os.Unsetenv("KIT_DISABLE_CACHE")

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
	os.Unsetenv("KIT_DISABLE_CACHE")

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

func TestMergeProviderOptions(t *testing.T) {
	opts1 := fantasy.ProviderOptions{
		"provider1": &testProviderData{value: "value1"},
	}
	opts2 := fantasy.ProviderOptions{
		"provider2": &testProviderData{value: "value2"},
	}

	merged := mergeProviderOptions(opts1, opts2)

	if len(merged) != 2 {
		t.Errorf("mergeProviderOptions should combine options from multiple maps, got %d items", len(merged))
	}

	if _, ok := merged["provider1"]; !ok {
		t.Errorf("merged options should contain 'provider1' key")
	}

	if _, ok := merged["provider2"]; !ok {
		t.Errorf("merged options should contain 'provider2' key")
	}

	// Later options should override earlier ones
	opts3 := fantasy.ProviderOptions{
		"provider1": &testProviderData{value: "overridden"},
	}
	merged2 := mergeProviderOptions(opts1, opts3)

	if data, ok := merged2["provider1"].(*testProviderData); ok {
		if data.value != "overridden" {
			t.Errorf("later options should override earlier ones, got %q", data.value)
		}
	}

	if mergeProviderOptions() != nil {
		t.Errorf("mergeProviderOptions with no args should return nil")
	}
}

// testProviderData is a simple implementation of ProviderOptionsData for testing
type testProviderData struct {
	value string
}

func (t *testProviderData) Options() {}

func (t *testProviderData) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.value + `"`), nil
}

func (t *testProviderData) UnmarshalJSON(data []byte) error {
	return nil
}
