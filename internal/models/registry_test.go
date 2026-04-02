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
