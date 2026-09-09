package agent

import (
	"context"
	"testing"

	"github.com/mark3labs/kit/internal/auth"
	"github.com/mark3labs/kit/internal/models"
)

// TestNewAgentMissingCredentials checks that a missing API key is fatal by
// default and tolerated when AllowMissingCredentials is set, in which case
// the agent reports the cause through ProviderError.
func TestNewAgentMissingCredentials(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GROQ_API_KEY", "")

	cfg := &models.ProviderConfig{ModelString: "groq/llama-3.1-8b-instant"}

	// Default: fail fast.
	if _, err := NewAgent(context.Background(), &AgentConfig{ModelConfig: cfg}); err == nil {
		t.Fatal("expected error without AllowMissingCredentials")
	}

	// Tolerant: agent starts with a placeholder model.
	a, err := NewAgent(context.Background(), &AgentConfig{ModelConfig: cfg, AllowMissingCredentials: true})
	if err != nil {
		t.Fatalf("NewAgent with AllowMissingCredentials: %v", err)
	}
	defer func() { _ = a.Close() }()

	perr := a.ProviderError()
	if perr == nil {
		t.Fatal("ProviderError = nil, want missing-credentials error")
	}
	if !auth.IsMissingCredentials(perr) {
		t.Errorf("ProviderError = %v, want MissingCredentialsError", perr)
	}
	if _, ok := a.GetModel().(*models.UnavailableModel); !ok {
		t.Errorf("model = %T, want *models.UnavailableModel", a.GetModel())
	}

	// A non-credential failure is still fatal even when tolerant.
	bad := &models.ProviderConfig{ModelString: "not-a-model-string"}
	if _, err := NewAgent(context.Background(), &AgentConfig{ModelConfig: bad, AllowMissingCredentials: true}); err == nil {
		t.Error("expected parse error to stay fatal")
	}

	// Once a key is stored, SetModel installs a real provider and clears
	// the error.
	cm, err := auth.NewCredentialManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := cm.SetProviderAPIKey("groq", "gsk_test"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetModel(context.Background(), &models.ProviderConfig{ModelString: "groq/llama-3.1-8b-instant"}); err != nil {
		t.Fatalf("SetModel after storing key: %v", err)
	}
	if a.ProviderError() != nil {
		t.Errorf("ProviderError after SetModel = %v, want nil", a.ProviderError())
	}
	if _, ok := a.GetModel().(*models.UnavailableModel); ok {
		t.Error("model is still the placeholder after SetModel")
	}
}
