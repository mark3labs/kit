package models

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/auth"
)

// TestResolveAPIKeyPrecedence checks flag > stored key > environment.
func TestResolveAPIKeyPrecedence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GROQ_API_KEY", "from-env")

	if got := resolveAPIKey("", "groq", []string{"GROQ_API_KEY"}); got != "from-env" {
		t.Fatalf("env fallback: got %q", got)
	}

	cm, err := auth.NewCredentialManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := cm.SetProviderAPIKey("groq", "from-store"); err != nil {
		t.Fatal(err)
	}
	if got := resolveAPIKey("", "groq", []string{"GROQ_API_KEY"}); got != "from-store" {
		t.Errorf("stored key must beat env: got %q", got)
	}
	if got := resolveAPIKey("from-flag", "groq", []string{"GROQ_API_KEY"}); got != "from-flag" {
		t.Errorf("explicit key must win: got %q", got)
	}
	if got := resolveAPIKey("", "", []string{"GROQ_API_KEY"}); got != "from-env" {
		t.Errorf("empty provider skips store: got %q", got)
	}
}

func TestOpenAICredentialPrecedence(t *testing.T) {
	cases := []struct {
		name      string
		configKey string
		envKey    string
		stored    *auth.OpenAICredentials
		wantToken string
		wantOAuth bool
		endpoint  string
	}{
		{"oauth beats config and environment", "config-key", "env-key", &auth.OpenAICredentials{Type: "oauth", AccessToken: "oauth-token", AccountID: "account-id"}, "oauth-token", true, ""},
		{"oauth beats environment", "", "env-key", &auth.OpenAICredentials{Type: "oauth", AccessToken: "oauth-token", AccountID: "account-id"}, "oauth-token", true, ""},
		{"config beats stored API key", "config-key", "env-key", &auth.OpenAICredentials{Type: "api_key", APIKey: "stored-key"}, "config-key", false, ""},
		{"stored API key beats environment", "", "env-key", &auth.OpenAICredentials{Type: "api_key", APIKey: "stored-key"}, "stored-key", false, ""},
		{"environment fallback", "", "env-key", nil, "env-key", false, ""},
		{"empty OAuth falls back to config", "config-key", "env-key", &auth.OpenAICredentials{Type: "oauth"}, "config-key", false, ""},
		{"trusted explicit endpoint uses OAuth", "config-key", "env-key", &auth.OpenAICredentials{Type: "oauth", AccessToken: "oauth-token", AccountID: "account-id"}, "oauth-token", true, "https://chatgpt.com/backend-api/codex"},
		{"custom HTTPS uses config key", "config-key", "env-key", &auth.OpenAICredentials{Type: "oauth", AccessToken: "oauth-token"}, "config-key", false, "https://example.com/v1"},
		{"custom endpoint uses environment", "", "env-key", &auth.OpenAICredentials{Type: "oauth", AccessToken: "oauth-token"}, "env-key", false, "https://example.com/v1"},
		{"plaintext ChatGPT uses config key", "config-key", "env-key", &auth.OpenAICredentials{Type: "oauth", AccessToken: "oauth-token"}, "config-key", false, "http://chatgpt.com/backend-api/codex"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("OPENAI_API_KEY", tc.envKey)
			if tc.stored != nil {
				cm, err := auth.NewCredentialManager()
				if err != nil {
					t.Fatal(err)
				}
				tc.stored.ExpiresAt = time.Now().Add(time.Hour).Unix()
				if err := cm.SetOpenAIOAuthCredentials(tc.stored); err != nil {
					t.Fatal(err)
				}
			}
			headers := make(chan http.Header, 1)
			mockDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
				headers <- r.Header.Clone()
				if tc.wantOAuth && !isCodexDestination(r.URL.String()) {
					t.Errorf("unexpected OAuth destination: %s", r.URL)
				}
				return jsonTestResponse(r, `{"id":"test","object":"response","status":"completed","output":[]}`), nil
			})
			result, err := createOpenAIProvider(context.Background(), &ProviderConfig{
				ProviderAPIKey: tc.configKey,
				ProviderURL:    tc.endpoint,
			}, "gpt-5")
			if err != nil {
				t.Fatal(err)
			}
			if result.SkipMaxOutputTokens != tc.wantOAuth {
				t.Fatalf("OAuth route = %v, want %v", result.SkipMaxOutputTokens, tc.wantOAuth)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := result.Model.Generate(ctx, fantasy.Call{}); err != nil {
				t.Fatalf("generate: %v", err)
			}
			select {
			case got := <-headers:
				if got.Get("Authorization") != "Bearer "+tc.wantToken {
					t.Errorf("wrong Authorization header: got %q", got.Get("Authorization"))
				}
				if tc.wantOAuth && got.Get("ChatGPT-Account-ID") != "account-id" {
					t.Errorf("wrong OAuth account header: got %q", got.Get("ChatGPT-Account-ID"))
				}
				if !tc.wantOAuth && got.Get("ChatGPT-Account-ID") != "" {
					t.Error("API-key request includes OAuth account header")
				}
			default:
				t.Fatal("no request received")
			}
		})
	}
}

// TestValidateEnvironmentSeesStoredKey checks that a stored key satisfies the
// registry check used by the model picker.
func TestValidateEnvironmentSeesStoredKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GROQ_API_KEY", "")

	r := GetGlobalRegistry()
	if err := r.ValidateEnvironment("groq", ""); err == nil {
		t.Fatal("expected groq to be unconfigured")
	}
	cm, _ := auth.NewCredentialManager()
	if err := cm.SetProviderAPIKey("groq", "gsk_x"); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateEnvironment("groq", ""); err != nil {
		t.Errorf("stored key not honoured: %v", err)
	}
}

// TestCreateProviderMissingKeyIsTyped checks that the provider layer reports
// missing credentials with the typed error for native and auto-routed
// providers.
func TestCreateProviderMissingKeyIsTyped(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, env := range []string{"GROQ_API_KEY", "OPENROUTER_API_KEY", "GOOGLE_API_KEY", "GEMINI_API_KEY", "GOOGLE_GENERATIVE_AI_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "VERCEL_API_KEY", "AZURE_OPENAI_API_KEY"} {
		t.Setenv(env, "")
	}

	cases := []struct {
		model    string
		provider string
	}{
		{"groq/llama-3.1-8b-instant", "groq"},
		{"openrouter/anthropic/claude-sonnet-4", "openrouter"},
		{"google/gemini-2.5-flash", "google"},
		{"openai/gpt-4o", "openai"},
		{"anthropic/claude-sonnet-4-5", "anthropic"},
		{"vercel/anthropic/claude-sonnet-4", "vercel"},
	}
	for _, tc := range cases {
		_, err := CreateProvider(context.Background(), &ProviderConfig{ModelString: tc.model})
		if err == nil {
			t.Errorf("%s: expected error", tc.model)
			continue
		}
		mc := auth.AsMissingCredentials(err)
		if mc == nil {
			t.Errorf("%s: error %v is not MissingCredentialsError", tc.model, err)
			continue
		}
		if mc.Provider != tc.provider {
			t.Errorf("%s: Provider = %q, want %q", tc.model, mc.Provider, tc.provider)
		}
	}
}

func TestUnavailableModel(t *testing.T) {
	cause := errors.New("no key")
	var m fantasy.LanguageModel = NewUnavailableModel("groq", "llama", cause)

	if m.Provider() != "groq" || m.Model() != "llama" {
		t.Errorf("identity = %s/%s", m.Provider(), m.Model())
	}
	if _, err := m.Generate(context.Background(), fantasy.Call{}); !errors.Is(err, cause) {
		t.Errorf("Generate err = %v", err)
	}
	if _, err := m.Stream(context.Background(), fantasy.Call{}); !errors.Is(err, cause) {
		t.Errorf("Stream err = %v", err)
	}
	if _, err := m.GenerateObject(context.Background(), fantasy.ObjectCall{}); !errors.Is(err, cause) {
		t.Errorf("GenerateObject err = %v", err)
	}
	if _, err := m.StreamObject(context.Background(), fantasy.ObjectCall{}); !errors.Is(err, cause) {
		t.Errorf("StreamObject err = %v", err)
	}
}

// TestAzureStoredKeyUsesSelectedProviderID checks that a key stored under
// the azure-cognitive-services registry ID is found when that ID is used in
// the model string (both IDs route to createAzureProvider).
func TestAzureStoredKeyUsesSelectedProviderID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AZURE_OPENAI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_BASE_URL", "https://example.openai.azure.com")

	cm, err := auth.NewCredentialManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := cm.SetProviderAPIKey("azure-cognitive-services", "azure-key"); err != nil {
		t.Fatal(err)
	}

	_, err = CreateProvider(context.Background(), &ProviderConfig{ModelString: "azure-cognitive-services/gpt-4o"})
	if err != nil {
		t.Fatalf("azure-cognitive-services/gpt-4o: CreateProvider failed: %v", err)
	}

	// The plain "azure" ID must not see that key.
	_, err = CreateProvider(context.Background(), &ProviderConfig{ModelString: "azure/gpt-4o"})
	if mc := auth.AsMissingCredentials(err); mc == nil || mc.Provider != "azure" {
		t.Fatalf("azure/gpt-4o: err = %v, want MissingCredentialsError for azure", err)
	}
}
