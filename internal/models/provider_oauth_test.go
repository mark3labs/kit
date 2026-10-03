package models

import (
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/auth"
)

func TestProviderOAuthState(t *testing.T) {
	cases := []struct {
		name, model, key, endpoint string
		want                       bool
	}{
		{"anthropic OAuth", "anthropic/claude-sonnet-4-5", "", "", true},
		{"anthropic flag wins", "anthropic/claude-sonnet-4-5", "explicit-key", "", false},
		{"anthropic custom endpoint uses OAuth", "anthropic/claude-sonnet-4-5", "", "https://example.com", true},
		{"openai OAuth wins over flag", "openai/gpt-5", "explicit-key", "", true},
		{"openai custom endpoint uses flag", "openai/gpt-5", "explicit-key", "https://example.com/v1", false},
		{"copilot OAuth", "copilot/gpt-5.5", "explicit-key", "", true},
		{"copilot catalog alias", "github-copilot/gpt-5.5", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			cm, err := auth.NewCredentialManager()
			if err != nil {
				t.Fatal(err)
			}
			expiry := time.Now().Add(time.Hour).Unix()
			store, err := cm.LoadCredentials()
			if err != nil {
				t.Fatal(err)
			}
			store.Anthropic = &auth.AnthropicCredentials{Type: "oauth", AccessToken: "anthropic-token", ExpiresAt: expiry}
			store.OpenAI = &auth.OpenAICredentials{Type: "oauth", AccessToken: "openai-token", AccountID: "account", ExpiresAt: expiry}
			store.Copilot = &auth.CopilotCredentials{Type: "oauth", GitHubToken: "github-token", CopilotAccessToken: "copilot-token", ExpiresAt: expiry}
			if err := cm.SaveCredentials(store); err != nil {
				t.Fatal(err)
			}
			result, err := CreateProvider(t.Context(), &ProviderConfig{ModelString: tc.model, ProviderAPIKey: tc.key, ProviderURL: tc.endpoint})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsOAuth != tc.want {
				t.Fatalf("IsOAuth = %v, want %v", result.IsOAuth, tc.want)
			}
		})
	}
}
