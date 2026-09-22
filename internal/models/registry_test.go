package models

import (
	"strings"
	"testing"
)

func TestValidateModelString(t *testing.T) {
	registry := GetGlobalRegistry()

	tests := []struct {
		name      string
		model     string
		wantErr   bool
		errSubstr string // expected substring in error message (empty = don't check)
	}{
		{
			name:    "valid anthropic model",
			model:   "anthropic/claude-sonnet-4-6",
			wantErr: false,
		},
		{
			name:      "missing provider prefix",
			model:     "claude-sonnet-4-6",
			wantErr:   true,
			errSubstr: "invalid model format",
		},
		{
			name:      "empty string",
			model:     "",
			wantErr:   true,
			errSubstr: "invalid model format",
		},
		{
			name:      "unknown provider",
			model:     "fakeprovider/some-model",
			wantErr:   true,
			errSubstr: "unknown provider",
		},
		{
			name:    "ollama always valid",
			model:   "ollama/llama3",
			wantErr: false,
		},
		{
			name:    "custom always valid",
			model:   "custom/my-fine-tune",
			wantErr: false,
		},
		{
			name:      "empty provider",
			model:     "/claude-sonnet-4-6",
			wantErr:   true,
			errSubstr: "invalid model format",
		},
		{
			name:      "empty model name",
			model:     "anthropic/",
			wantErr:   true,
			errSubstr: "invalid model format",
		},
		{
			name:    "unknown model under known provider (no suggestions)",
			model:   "anthropic/totally-unknown-xyz-999",
			wantErr: false, // no suggestions → passes through
		},
		{
			name:      "typo model under known provider with suggestions",
			model:     "anthropic/claude-sonet", // misspelled "sonnet"
			wantErr:   true,
			errSubstr: "Did you mean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.ValidateModelString(tt.model)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateModelString(%q) = nil, want error", tt.model)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateModelString(%q) = %v, want nil", tt.model, err)
			}
			if tt.errSubstr != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("ValidateModelString(%q) error = %q, want substring %q",
						tt.model, err.Error(), tt.errSubstr)
				}
			}
		})
	}
}

// TestLookupModelProviderQualifiedKey guards the OpenRouter router models,
// whose catalog keys carry the provider prefix ("openrouter/auto"). A
// configured "openrouter/auto" parses into the bare model name "auto", which
// used to miss the catalog and print a warning that suggested the very same
// model back to the user.
func TestLookupModelProviderQualifiedKey(t *testing.T) {
	registry := GetGlobalRegistry()

	info := registry.LookupModel("openrouter", "auto")
	if info == nil {
		t.Fatal(`LookupModel("openrouter", "auto") = nil, want the "openrouter/auto" catalog entry`)
	}
	if info.ID != "openrouter/auto" {
		t.Errorf(`LookupModel("openrouter", "auto").ID = %q, want "openrouter/auto"`, info.ID)
	}

	// The fully qualified form keeps working.
	if got := registry.LookupModel("openrouter", "openrouter/auto"); got == nil {
		t.Error(`LookupModel("openrouter", "openrouter/auto") = nil, want the catalog entry`)
	}

	// Unknown models still report as unknown — the fallback must not invent
	// entries.
	if got := registry.LookupModel("openrouter", "totally-unknown-xyz-999"); got != nil {
		t.Errorf("LookupModel(openrouter, unknown) = %+v, want nil", got)
	}
}

// TestLookupModelPrefersBareKey verifies the provider-qualified fallback only
// runs after a direct hit fails, so providers with bare catalog keys keep
// their current behaviour.
func TestLookupModelPrefersBareKey(t *testing.T) {
	registry := &ModelsRegistry{
		providers: map[string]ProviderInfo{
			"testprov": {
				ID: "testprov",
				Models: map[string]ModelInfo{
					"m1":          {ID: "m1"},
					"testprov/m1": {ID: "testprov/m1"},
				},
			},
		},
	}

	info := registry.LookupModel("testprov", "m1")
	if info == nil {
		t.Fatal(`LookupModel("testprov", "m1") = nil, want the bare entry`)
	}
	if info.ID != "m1" {
		t.Errorf(`LookupModel("testprov", "m1").ID = %q, want "m1"`, info.ID)
	}
}
