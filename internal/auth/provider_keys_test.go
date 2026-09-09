package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestCredentialManager(t *testing.T) *CredentialManager {
	t.Helper()
	return &CredentialManager{credentialsPath: filepath.Join(t.TempDir(), "credentials.json")}
}

func TestProviderAPIKeyRoundTrip(t *testing.T) {
	cm := newTestCredentialManager(t)

	if key, err := cm.GetProviderAPIKey("groq"); err != nil || key != "" {
		t.Fatalf("expected no key initially, got %q err=%v", key, err)
	}
	if has, _ := cm.HasProviderCredentials("groq"); has {
		t.Fatal("expected no credentials initially")
	}

	if err := cm.SetProviderAPIKey("groq", "  gsk_test_123  "); err != nil {
		t.Fatalf("SetProviderAPIKey: %v", err)
	}

	key, err := cm.GetProviderAPIKey("groq")
	if err != nil {
		t.Fatalf("GetProviderAPIKey: %v", err)
	}
	if key != "gsk_test_123" {
		t.Errorf("key = %q, want trimmed gsk_test_123", key)
	}
	if has, _ := cm.HasProviderCredentials("groq"); !has {
		t.Error("HasProviderCredentials = false after set")
	}

	// File permissions must stay owner-only.
	info, err := os.Stat(cm.credentialsPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials file mode = %o, want 600", perm)
	}

	// The generic entry lives under "providers" in the JSON.
	data, _ := os.ReadFile(cm.credentialsPath)
	if !strings.Contains(string(data), `"providers"`) || !strings.Contains(string(data), `"groq"`) {
		t.Errorf("unexpected file content:\n%s", data)
	}

	ids, err := cm.StoredProviderIDs()
	if err != nil {
		t.Fatalf("StoredProviderIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != "groq" {
		t.Errorf("StoredProviderIDs = %v, want [groq]", ids)
	}

	if err := cm.RemoveProviderCredentials("groq"); err != nil {
		t.Fatalf("RemoveProviderCredentials: %v", err)
	}
	if _, err := os.Stat(cm.credentialsPath); !os.IsNotExist(err) {
		t.Error("expected credentials file removed when store became empty")
	}
}

func TestProviderAPIKeyDedicatedSlots(t *testing.T) {
	cm := newTestCredentialManager(t)

	// Anthropic goes to its dedicated slot and is validated.
	if err := cm.SetProviderAPIKey("anthropic", "not-valid"); err == nil {
		t.Error("expected Anthropic key validation error")
	}
	if err := cm.SetProviderAPIKey("anthropic", "sk-ant-test-key-12345678901234567890"); err != nil {
		t.Fatalf("SetProviderAPIKey(anthropic): %v", err)
	}
	store, _ := cm.LoadCredentials()
	if store.Anthropic == nil || store.Anthropic.Type != "api_key" {
		t.Fatalf("anthropic key not stored in dedicated slot: %+v", store.Anthropic)
	}
	if len(store.Providers) != 0 {
		t.Errorf("anthropic key leaked into generic map: %v", store.Providers)
	}
	if key, _ := cm.GetProviderAPIKey("anthropic"); key != "sk-ant-test-key-12345678901234567890" {
		t.Errorf("GetProviderAPIKey(anthropic) = %q", key)
	}

	// OpenAI too.
	if err := cm.SetProviderAPIKey("openai", "sk-openai-test"); err != nil {
		t.Fatalf("SetProviderAPIKey(openai): %v", err)
	}
	store, _ = cm.LoadCredentials()
	if store.OpenAI == nil || store.OpenAI.APIKey != "sk-openai-test" {
		t.Fatalf("openai key not stored in dedicated slot: %+v", store.OpenAI)
	}
	if has, _ := cm.HasProviderCredentials("openai"); !has {
		t.Error("HasProviderCredentials(openai) = false")
	}

	// Copilot is OAuth-only.
	if err := cm.SetProviderAPIKey("copilot", "ghp_x"); err == nil {
		t.Error("expected error storing an API key for copilot")
	}

	// OAuth Anthropic credentials are not reported as a stored API key.
	store.Anthropic = &AnthropicCredentials{Type: "oauth", AccessToken: "tok"}
	if err := cm.SaveCredentials(store); err != nil {
		t.Fatal(err)
	}
	if key, _ := cm.GetProviderAPIKey("anthropic"); key != "" {
		t.Errorf("OAuth credential reported as API key: %q", key)
	}
	if has, _ := cm.HasProviderCredentials("anthropic"); !has {
		t.Error("HasProviderCredentials should see OAuth credentials")
	}
}

func TestProviderAPIKeyAliases(t *testing.T) {
	cm := newTestCredentialManager(t)
	if err := cm.SetProviderAPIKey("Gemini", "g-key"); err != nil {
		t.Fatal(err)
	}
	if key, _ := cm.GetProviderAPIKey("google"); key != "g-key" {
		t.Errorf("gemini alias not mapped to google: %q", key)
	}
	if err := cm.SetProviderAPIKey("", "x"); err == nil {
		t.Error("expected error for empty provider")
	}
	if err := cm.SetProviderAPIKey("groq", "   "); err == nil {
		t.Error("expected error for empty key")
	}
}

func TestLookupStoredAPIKeyUsesXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if got := LookupStoredAPIKey("groq"); got != "" {
		t.Fatalf("expected empty key, got %q", got)
	}
	cm, err := NewCredentialManager()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cm.GetCredentialsPath(), dir) {
		t.Fatalf("credentials path %q not under XDG_CONFIG_HOME %q", cm.GetCredentialsPath(), dir)
	}
	if err := cm.SetProviderAPIKey("groq", "gsk_1"); err != nil {
		t.Fatal(err)
	}
	if got := LookupStoredAPIKey("groq"); got != "gsk_1" {
		t.Errorf("LookupStoredAPIKey = %q, want gsk_1", got)
	}
}

func TestMissingCredentialsError(t *testing.T) {
	err := &MissingCredentialsError{Provider: "groq", ProviderName: "Groq", EnvVars: []string{"GROQ_API_KEY"}}
	wrapped := fmt.Errorf("failed to create model provider: %w", err)

	if !IsMissingCredentials(wrapped) {
		t.Error("IsMissingCredentials(wrapped) = false")
	}
	if IsMissingCredentials(errors.New("other")) {
		t.Error("IsMissingCredentials(other) = true")
	}
	if got := AsMissingCredentials(wrapped); got == nil || got.Provider != "groq" {
		t.Errorf("AsMissingCredentials = %+v", got)
	}
	msg := err.Error()
	for _, want := range []string{"Groq", "GROQ_API_KEY", "kit auth login groq"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q lacks %q", msg, want)
		}
	}

	// GetAnthropicAPIKey returns the typed error when nothing is configured.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "")
	if _, _, err := GetAnthropicAPIKey(""); !IsMissingCredentials(err) {
		t.Errorf("GetAnthropicAPIKey error = %v, want MissingCredentialsError", err)
	}
}
