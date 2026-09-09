package models

import (
	"context"
	"errors"
	"testing"

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
