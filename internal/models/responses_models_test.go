package models

import (
	"testing"

	"charm.land/fantasy/providers/openai"
)

func TestIsResponsesAPIModel(t *testing.T) {
	tests := []struct {
		modelID  string
		expected bool
	}{
		// Already in fantasy's list — always true
		{"gpt-5", true},
		{"gpt-4.1", true},
		{"o3", true},
		{"o4-mini", true},
		{"codex-mini-latest", true},

		// NOT in fantasy's list but matches our heuristic
		{"gpt-5.5", true},
		{"gpt-5.6-turbo", true},
		{"gpt-4.1-ultra", true},
		{"o3-jumbo", true},
		{"o4-mega", true},

		// Should NOT match
		{"gpt-3.5-turbo", true}, // actually IS in fantasy's responses list (legacy Chat Completions compat)
		{"llama-3", false},
		{"claude-opus-4-6", false},
		{"gemini-2.5-pro", false},
		{"random-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			got := isResponsesAPIModel(tt.modelID)
			if got != tt.expected {
				t.Errorf("isResponsesAPIModel(%q) = %v, want %v", tt.modelID, got, tt.expected)
			}
		})
	}
}

func TestIsResponsesReasoningModel(t *testing.T) {
	tests := []struct {
		modelID  string
		expected bool
	}{
		// In fantasy's reasoning list
		{"gpt-5", true},
		{"o3", true},
		{"o4-mini", true},

		// NOT in fantasy's list but matches reasoning heuristic (gpt-5 prefix)
		{"gpt-5.5", true},
		{"gpt-5.6-turbo", true},

		// Responses API but NOT reasoning
		{"gpt-4.1", false},
		{"gpt-4.1-mini", false},

		// Not OpenAI at all
		{"claude-opus-4-6", false},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			got := isResponsesReasoningModel(tt.modelID)
			if got != tt.expected {
				t.Errorf("isResponsesReasoningModel(%q) = %v, want %v", tt.modelID, got, tt.expected)
			}
		})
	}
}

func TestRegisterResponsesModels(t *testing.T) {
	// After RegisterResponsesModels() (called in init()),
	// any model matching our heuristic that's in the model database
	// should be queryable via openai.IsResponsesModel.

	// Models in the embedded database that are also in fantasy's list
	// should remain accessible.
	if !openai.IsResponsesModel("gpt-5") {
		t.Error("gpt-5 should be a responses model after registration")
	}

	// The registration should not break existing models.
	if openai.IsResponsesModel("random-nonexistent-model") {
		t.Error("random model should NOT be a responses model")
	}
}

func TestBuildOpenAIProviderOptions_NewModel(t *testing.T) {
	// A model like gpt-5.5 that isn't in fantasy's hardcoded list
	// but matches our heuristic should get ResponsesProviderOptions.
	config := &ProviderConfig{
		ModelString: "openai/gpt-5.5",
	}
	opts := buildOpenAIProviderOptions(config, "gpt-5.5")
	if opts == nil {
		t.Fatal("buildOpenAIProviderOptions should return non-nil for gpt-5.5")
	}
	v, ok := opts[openai.Name]
	if !ok {
		t.Fatal("should have openai key in provider options")
	}
	if _, ok := v.(*openai.ResponsesProviderOptions); !ok {
		t.Errorf("expected *ResponsesProviderOptions, got %T", v)
	}
}

func TestBuildOpenAIProviderOptions_NonResponsesModel(t *testing.T) {
	// A model that doesn't match any heuristic should get nil.
	config := &ProviderConfig{
		ModelString: "openai/some-old-model",
	}
	opts := buildOpenAIProviderOptions(config, "some-old-model")
	if opts != nil {
		t.Errorf("buildOpenAIProviderOptions should return nil for unknown model, got %v", opts)
	}
}
