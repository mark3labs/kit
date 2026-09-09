package kit

import (
	"os"

	"github.com/mark3labs/kit/internal/auth"
)

// CredentialManager manages API keys and OAuth credentials.
type CredentialManager = auth.CredentialManager

// AnthropicCredentials holds Anthropic API credentials supporting both OAuth
// and API key authentication methods.
type AnthropicCredentials = auth.AnthropicCredentials

// OpenAICredentials holds OpenAI API credentials supporting both OAuth
// and API key authentication methods.
type OpenAICredentials = auth.OpenAICredentials

// CopilotCredentials holds GitHub OAuth and Copilot API credentials.
type CopilotCredentials = auth.CopilotCredentials

// CredentialStore holds all stored credentials for various providers.
type CredentialStore = auth.CredentialStore

// ProviderCredentials holds a stored API key for a generic provider.
type ProviderCredentials = auth.ProviderCredentials

// MissingCredentialsError is returned when no API key or OAuth token can be
// resolved for a provider. Check for it with IsMissingCredentialsError.
type MissingCredentialsError = auth.MissingCredentialsError

// IsMissingCredentialsError reports whether err is (or wraps) a
// MissingCredentialsError.
func IsMissingCredentialsError(err error) bool {
	return auth.IsMissingCredentials(err)
}

// SetProviderAPIKey stores an API key for providerID in the credentials file
// ($XDG_CONFIG_HOME/.kit/credentials.json, mode 0600). Stored keys take
// precedence over provider environment variables and are picked up by the
// next provider creation (e.g. SetModel). Anthropic and OpenAI keys go to
// their dedicated slots; GitHub Copilot is OAuth-only and is rejected.
func SetProviderAPIKey(providerID, apiKey string) error {
	cm, err := auth.NewCredentialManager()
	if err != nil {
		return err
	}
	return cm.SetProviderAPIKey(providerID, apiKey)
}

// GetProviderAPIKey returns the API key stored for providerID, or "" when
// none is stored.
func GetProviderAPIKey(providerID string) string {
	return auth.LookupStoredAPIKey(providerID)
}

// RemoveProviderCredentials deletes every stored credential for providerID.
func RemoveProviderCredentials(providerID string) error {
	cm, err := auth.NewCredentialManager()
	if err != nil {
		return err
	}
	return cm.RemoveProviderCredentials(providerID)
}

// HasProviderCredentials reports whether any credential (API key or OAuth
// token) is stored for providerID.
func HasProviderCredentials(providerID string) bool {
	cm, err := auth.NewCredentialManager()
	if err != nil {
		return false
	}
	has, err := cm.HasProviderCredentials(providerID)
	return err == nil && has
}

// NewCredentialManager creates a credential manager for secure storage and
// retrieval of authentication credentials.
func NewCredentialManager() (*CredentialManager, error) {
	return auth.NewCredentialManager()
}

// HasAnthropicCredentials checks if valid Anthropic credentials are stored
// (either OAuth token or API key).
func HasAnthropicCredentials() bool {
	cm, err := auth.NewCredentialManager()
	if err != nil {
		return false
	}
	has, err := cm.HasAnthropicCredentials()
	if err != nil {
		return false
	}
	return has
}

// GetAnthropicAPIKey resolves the Anthropic API key using the standard
// resolution order: stored credentials -> ANTHROPIC_API_KEY env var.
// Returns an empty string if no key is found.
func GetAnthropicAPIKey() string {
	key, _, err := auth.GetAnthropicAPIKey("")
	if err != nil {
		return ""
	}
	return key
}

// HasOpenAICredentials checks if valid OpenAI credentials are stored
// (either OAuth token or API key).
func HasOpenAICredentials() bool {
	cm, err := auth.NewCredentialManager()
	if err != nil {
		return false
	}
	has, err := cm.HasOpenAICredentials()
	if err != nil {
		return false
	}
	return has
}

// HasCopilotCredentials checks if valid GitHub Copilot credentials are stored.
func HasCopilotCredentials() bool {
	cm, err := auth.NewCredentialManager()
	if err != nil {
		return false
	}
	has, err := cm.HasCopilotCredentials()
	if err != nil {
		return false
	}
	return has
}

// GetCopilotCredentials retrieves stored GitHub Copilot credentials.
func GetCopilotCredentials() (*CopilotCredentials, error) {
	cm, err := auth.NewCredentialManager()
	if err != nil {
		return nil, err
	}
	return cm.GetCopilotCredentials()
}

// GetValidCopilotAccessToken returns a fresh GitHub Copilot access token.
func GetValidCopilotAccessToken() (string, error) {
	cm, err := auth.NewCredentialManager()
	if err != nil {
		return "", err
	}
	return cm.GetValidCopilotAccessToken()
}

// GetOpenAIAPIKey resolves the OpenAI API key using the standard
// resolution order: stored credentials -> OPENAI_API_KEY env var.
// Returns an empty string if no key is found.
//
// Deprecated: Use [HasOpenAICredentials] to check for credentials; the
// provider layer resolves the key itself. This function has no callers and
// will be removed in a future release.
func GetOpenAIAPIKey() string {
	cm, err := auth.NewCredentialManager()
	if err == nil {
		// Try to get valid access token (handles OAuth refresh)
		if token, err := cm.GetValidOpenAIAccessToken(); err == nil && token != "" {
			return token
		}
	}
	// Fall back to environment variable
	return os.Getenv("OPENAI_API_KEY")
}
