package kit

import "github.com/mark3labs/kit/internal/auth"

// CredentialManager manages API keys and OAuth credentials.
type CredentialManager = auth.CredentialManager

// AnthropicCredentials holds Anthropic API credentials supporting both OAuth
// and API key authentication methods.
type AnthropicCredentials = auth.AnthropicCredentials

// CredentialStore holds all stored credentials for various providers.
type CredentialStore = auth.CredentialStore

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
