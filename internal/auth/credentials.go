package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CredentialStore holds stored credentials for Anthropic, OpenAI, GitHub
// Copilot, and any other provider that authenticates with a plain API key.
type CredentialStore struct {
	Anthropic *AnthropicCredentials `json:"anthropic,omitempty"`
	OpenAI    *OpenAICredentials    `json:"openai,omitempty"`
	Copilot   *CopilotCredentials   `json:"copilot,omitempty"`
	// Providers holds API-key credentials for every other provider, keyed
	// by the models.dev provider ID (e.g. "google", "openrouter", "groq").
	// Anthropic, OpenAI and Copilot keep their dedicated fields above because
	// they also support OAuth.
	Providers map[string]*ProviderCredentials `json:"providers,omitempty"`
}

// ProviderCredentials holds an API key for a generic provider.
type ProviderCredentials struct {
	Type      string    `json:"type"`              // always "api_key"
	APIKey    string    `json:"api_key,omitempty"` // the stored key
	CreatedAt time.Time `json:"created_at"`
}

// isEmpty reports whether the store holds no credentials at all, so the
// backing file can be removed instead of rewritten.
func (s *CredentialStore) isEmpty() bool {
	return s.Anthropic == nil && s.OpenAI == nil && s.Copilot == nil && len(s.Providers) == 0
}

// MissingCredentialsError is returned by the provider layer when no API key
// or OAuth token can be resolved for a provider. Callers use
// [IsMissingCredentials] to tell this apart from other provider failures
// (bad model, network, etc.) and, in interactive mode, start without a model
// so the user can add a key from inside the TUI.
type MissingCredentialsError struct {
	// Provider is the provider ID as used in model strings (e.g. "anthropic").
	Provider string
	// ProviderName is a human-friendly provider name, when known.
	ProviderName string
	// EnvVars lists the environment variables that would satisfy the
	// provider, in priority order. May be empty.
	EnvVars []string
	// Hint is an optional extra sentence (e.g. an OAuth login command).
	Hint string
}

// Error implements error.
func (e *MissingCredentialsError) Error() string {
	name := e.ProviderName
	if name == "" {
		name = e.Provider
	}
	msg := fmt.Sprintf("no API key configured for %s", name)
	if len(e.EnvVars) > 0 {
		msg += " (set " + strings.Join(e.EnvVars, " or ") + ", or store a key with 'kit auth login " + e.Provider + "')"
	} else {
		msg += " (store a key with 'kit auth login " + e.Provider + "')"
	}
	if e.Hint != "" {
		msg += ". " + e.Hint
	}
	return msg
}

// IsMissingCredentials reports whether err is (or wraps) a
// [MissingCredentialsError].
func IsMissingCredentials(err error) bool {
	var target *MissingCredentialsError
	return errors.As(err, &target)
}

// AsMissingCredentials returns the wrapped [MissingCredentialsError], or nil
// when err is not one.
func AsMissingCredentials(err error) *MissingCredentialsError {
	if target, ok := errors.AsType[*MissingCredentialsError](err); ok {
		return target
	}
	return nil
}

// AnthropicCredentials holds Anthropic API credentials supporting both OAuth
// and API key authentication methods. The Type field indicates which authentication
// method is being used. For OAuth, tokens are stored with expiration timestamps
// for automatic refresh. For API keys, only the key itself is stored.
type AnthropicCredentials struct {
	Type         string    `json:"type"`                    // "oauth" or "api_key"
	APIKey       string    `json:"api_key,omitempty"`       // For API key auth
	AccessToken  string    `json:"access_token,omitempty"`  // For OAuth
	RefreshToken string    `json:"refresh_token,omitempty"` // For OAuth
	ExpiresAt    int64     `json:"expires_at,omitempty"`    // For OAuth
	CreatedAt    time.Time `json:"created_at"`
}

// OpenAICredentials holds OpenAI API credentials supporting both OAuth
// and API key authentication methods. The Type field indicates which authentication
// method is being used. For OAuth, tokens are stored with expiration timestamps
// for automatic refresh. For API keys, only the key itself is stored.
type OpenAICredentials struct {
	Type         string    `json:"type"`                    // "oauth" or "api_key"
	APIKey       string    `json:"api_key,omitempty"`       // For API key auth
	AccessToken  string    `json:"access_token,omitempty"`  // For OAuth
	RefreshToken string    `json:"refresh_token,omitempty"` // For OAuth
	ExpiresAt    int64     `json:"expires_at,omitempty"`    // For OAuth
	AccountID    string    `json:"account_id,omitempty"`    // For OAuth (ChatGPT account ID)
	CreatedAt    time.Time `json:"created_at"`
}

// CopilotCredentials holds GitHub OAuth credentials and the short-lived
// GitHub Copilot API token derived from them.
type CopilotCredentials struct {
	Type               string    `json:"type"`                           // "oauth"
	GitHubToken        string    `json:"github_token,omitempty"`         // GitHub device-flow OAuth token
	CopilotAccessToken string    `json:"copilot_access_token,omitempty"` // Short-lived Copilot API token
	ExpiresAt          int64     `json:"expires_at,omitempty"`           // Copilot token expiry
	CreatedAt          time.Time `json:"created_at"`
}

// oauthTokenExpired reports whether an OAuth token with the given type and
// expiry unix timestamp is past its expiry. Returns false for API key
// credentials or when no expiry is set.
func oauthTokenExpired(credType string, expiresAt int64) bool {
	if credType != "oauth" || expiresAt == 0 {
		return false
	}
	return time.Now().Unix() >= expiresAt
}

// oauthTokenNeedsRefresh reports whether an OAuth token will expire within the
// next 5 minutes, allowing proactive refresh before it becomes invalid.
// Returns false for API key credentials or when no expiry is set.
func oauthTokenNeedsRefresh(credType string, expiresAt int64) bool {
	if credType != "oauth" || expiresAt == 0 {
		return false
	}
	return time.Now().Unix() >= (expiresAt - 300) // 5 minutes buffer
}

// IsExpired checks if the OAuth token is expired based on the ExpiresAt timestamp.
// Returns false for API key authentication or if no expiration is set.
func (c *AnthropicCredentials) IsExpired() bool {
	return oauthTokenExpired(c.Type, c.ExpiresAt)
}

// NeedsRefresh checks if the OAuth token needs refresh, returning true if the token
// will expire within the next 5 minutes. This allows for proactive token refresh
// to avoid authentication failures during operations. Returns false for API key
// authentication or if no expiration is set.
func (c *AnthropicCredentials) NeedsRefresh() bool {
	return oauthTokenNeedsRefresh(c.Type, c.ExpiresAt)
}

// IsExpired checks if the OAuth token is expired based on the ExpiresAt timestamp.
// Returns false for API key authentication or if no expiration is set.
func (c *OpenAICredentials) IsExpired() bool {
	return oauthTokenExpired(c.Type, c.ExpiresAt)
}

// NeedsRefresh checks if the OAuth token needs refresh, returning true if the token
// will expire within the next 5 minutes. This allows for proactive token refresh
// to avoid authentication failures during operations. Returns false for API key
// authentication or if no expiration is set.
func (c *OpenAICredentials) NeedsRefresh() bool {
	return oauthTokenNeedsRefresh(c.Type, c.ExpiresAt)
}

// IsExpired checks if the Copilot API token is expired.
func (c *CopilotCredentials) IsExpired() bool {
	return oauthTokenExpired(c.Type, c.ExpiresAt)
}

// NeedsRefresh reports whether the Copilot API token should be renewed.
func (c *CopilotCredentials) NeedsRefresh() bool {
	return oauthTokenNeedsRefresh(c.Type, c.ExpiresAt)
}

// CredentialManager handles secure storage and retrieval of authentication credentials.
// It manages a JSON file stored in the user's config directory with appropriate
// file permissions for security.
type CredentialManager struct {
	credentialsPath string
}

// NewCredentialManager creates a new credential manager instance. It determines
// the appropriate credentials path based on XDG_CONFIG_HOME or falls back to
// ~/.config/.kit/credentials.json. Returns an error if the home directory
// cannot be determined.
func NewCredentialManager() (*CredentialManager, error) {
	credentialsPath, err := getCredentialsPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine credentials path: %w", err)
	}

	return &CredentialManager{
		credentialsPath: credentialsPath,
	}, nil
}

// getCredentialsPath returns the path to the credentials file
func getCredentialsPath() (string, error) {
	// Try XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, ".kit", "credentials.json"), nil
	}

	// Fall back to ~/.config/.kit/credentials.json
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".config", ".kit", "credentials.json"), nil
}

// LoadCredentials loads credentials from the JSON file. If the file doesn't exist,
// it returns an empty CredentialStore instead of an error, allowing for graceful
// initialization. Returns an error if the file exists but cannot be read or parsed.
func (cm *CredentialManager) LoadCredentials() (*CredentialStore, error) {
	// If file doesn't exist, return empty store
	if _, err := os.Stat(cm.credentialsPath); os.IsNotExist(err) {
		return &CredentialStore{}, nil
	}

	data, err := os.ReadFile(cm.credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var store CredentialStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}

	return &store, nil
}

// SaveCredentials saves credentials to the JSON file with secure permissions (0600).
// It creates the parent directory if it doesn't exist. The file is written atomically
// to prevent corruption. Returns an error if the directory cannot be created or the
// file cannot be written.
func (cm *CredentialManager) SaveCredentials(store *CredentialStore) error {
	// Ensure directory exists
	dir := filepath.Dir(cm.credentialsPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Write with restrictive permissions (read/write for owner only)
	if err := os.WriteFile(cm.credentialsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// SetAnthropicCredentials stores Anthropic API key credentials. It validates the
// API key format before storing. The API key must start with "sk-ant-" and be
// at least 20 characters long. Returns an error if the API key is invalid or
// if storage fails.
func (cm *CredentialManager) SetAnthropicCredentials(apiKey string) error {
	if err := validateAnthropicAPIKey(apiKey); err != nil {
		return err
	}

	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}

	store.Anthropic = &AnthropicCredentials{
		Type:      "api_key",
		APIKey:    apiKey,
		CreatedAt: time.Now(),
	}

	return cm.SaveCredentials(store)
}

// GetAnthropicCredentials retrieves stored Anthropic credentials. Returns nil if
// no credentials are stored. The returned credentials may be either OAuth or API
// key type, check the Type field to determine which.
func (cm *CredentialManager) GetAnthropicCredentials() (*AnthropicCredentials, error) {
	store, err := cm.LoadCredentials()
	if err != nil {
		return nil, err
	}

	return store.Anthropic, nil
}

// RemoveAnthropicCredentials removes stored Anthropic credentials from storage.
// If this was the only credential stored, the entire credentials file is removed.
// Returns an error if the removal fails.
func (cm *CredentialManager) RemoveAnthropicCredentials() error {
	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}

	store.Anthropic = nil
	return cm.saveOrRemove(store)
}

// saveOrRemove writes the store, or deletes the credentials file when the
// store no longer holds any credential.
func (cm *CredentialManager) saveOrRemove(store *CredentialStore) error {
	if store.isEmpty() {
		if err := os.Remove(cm.credentialsPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove credentials file: %w", err)
		}
		return nil
	}
	return cm.SaveCredentials(store)
}

// HasAnthropicCredentials checks if valid Anthropic credentials are stored.
// Returns true if either a non-empty OAuth access token or API key is present,
// false otherwise. Returns an error if credentials cannot be loaded.
func (cm *CredentialManager) HasAnthropicCredentials() (bool, error) {
	creds, err := cm.GetAnthropicCredentials()
	if err != nil {
		return false, err
	}
	if creds == nil {
		return false, nil
	}

	// Check based on credential type
	switch creds.Type {
	case "oauth":
		return creds.AccessToken != "", nil
	case "api_key":
		return creds.APIKey != "", nil
	default:
		return false, nil
	}
}

// GetOpenAICredentials retrieves stored OpenAI credentials. Returns nil if
// no credentials are stored. The returned credentials may be either OAuth or API
// key type, check the Type field to determine which.
func (cm *CredentialManager) GetOpenAICredentials() (*OpenAICredentials, error) {
	store, err := cm.LoadCredentials()
	if err != nil {
		return nil, err
	}

	return store.OpenAI, nil
}

// RemoveOpenAICredentials removes stored OpenAI credentials from storage.
// If this was the only credential stored, the entire credentials file is removed.
// Returns an error if the removal fails.
func (cm *CredentialManager) RemoveOpenAICredentials() error {
	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}

	store.OpenAI = nil
	return cm.saveOrRemove(store)
}

// GetCopilotCredentials retrieves stored GitHub Copilot credentials.
func (cm *CredentialManager) GetCopilotCredentials() (*CopilotCredentials, error) {
	store, err := cm.LoadCredentials()
	if err != nil {
		return nil, err
	}

	return store.Copilot, nil
}

// RemoveCopilotCredentials removes stored GitHub Copilot credentials.
func (cm *CredentialManager) RemoveCopilotCredentials() error {
	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}

	store.Copilot = nil
	return cm.saveOrRemove(store)
}

// HasCopilotCredentials checks if valid GitHub Copilot credentials are stored.
func (cm *CredentialManager) HasCopilotCredentials() (bool, error) {
	creds, err := cm.GetCopilotCredentials()
	if err != nil {
		return false, err
	}
	if creds == nil {
		return false, nil
	}

	return creds.Type == "oauth" && creds.GitHubToken != "", nil
}

// SetCopilotOAuthCredentials stores GitHub Copilot OAuth credentials.
func (cm *CredentialManager) SetCopilotOAuthCredentials(creds *CopilotCredentials) error {
	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}

	store.Copilot = creds
	return cm.SaveCredentials(store)
}

// GetValidCopilotAccessToken returns a fresh Copilot API token, renewing it
// with the stored GitHub OAuth token when needed.
func (cm *CredentialManager) GetValidCopilotAccessToken() (string, error) {
	return cm.GetValidCopilotAccessTokenContext(context.Background())
}

// GetValidCopilotAccessTokenContext returns a fresh Copilot API token, renewing
// it with the stored GitHub OAuth token when needed.
func (cm *CredentialManager) GetValidCopilotAccessTokenContext(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	creds, err := cm.GetCopilotCredentials()
	if err != nil {
		return "", err
	}
	if creds == nil {
		return "", fmt.Errorf("no Copilot credentials found")
	}
	if creds.Type != "oauth" {
		return "", fmt.Errorf("unknown credential type: %s", creds.Type)
	}
	if creds.GitHubToken == "" {
		return "", fmt.Errorf("GitHub OAuth token missing from Copilot credentials")
	}

	if creds.CopilotAccessToken == "" || creds.NeedsRefresh() {
		client := NewCopilotOAuthClient()
		newCreds, err := client.RefreshCopilotToken(ctx, creds.GitHubToken)
		if err != nil {
			return "", fmt.Errorf("failed to refresh Copilot token: %w", err)
		}
		newCreds.CreatedAt = creds.CreatedAt

		if err := cm.SetCopilotOAuthCredentials(newCreds); err != nil {
			return "", fmt.Errorf("failed to save refreshed Copilot token: %w", err)
		}

		return newCreds.CopilotAccessToken, nil
	}

	return creds.CopilotAccessToken, nil
}

// HasOpenAICredentials checks if valid OpenAI credentials are stored.
// Returns true if either a non-empty OAuth access token or API key is present,
// false otherwise. Returns an error if credentials cannot be loaded.
func (cm *CredentialManager) HasOpenAICredentials() (bool, error) {
	creds, err := cm.GetOpenAICredentials()
	if err != nil {
		return false, err
	}
	if creds == nil {
		return false, nil
	}

	// Check based on credential type
	switch creds.Type {
	case "oauth":
		return creds.AccessToken != "", nil
	case "api_key":
		return creds.APIKey != "", nil
	default:
		return false, nil
	}
}

// SetOpenAIOAuthCredentials stores OpenAI OAuth credentials in the credential manager's secure storage.
// The credentials should include access token, refresh token, and expiration information.
// Returns an error if the credentials cannot be saved.
func (cm *CredentialManager) SetOpenAIOAuthCredentials(creds *OpenAICredentials) error {
	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}

	store.OpenAI = creds
	return cm.SaveCredentials(store)
}

// GetValidOpenAIAccessToken returns a valid access token for API requests. For OAuth credentials,
// it automatically refreshes the token if it's expired or about to expire. For API key
// credentials, it simply returns the API key. Returns an error if no credentials are found,
// if token refresh fails, or if the credential type is unknown.
func (cm *CredentialManager) GetValidOpenAIAccessToken() (string, error) {
	creds, err := cm.GetOpenAICredentials()
	if err != nil {
		return "", err
	}

	if creds == nil {
		return "", fmt.Errorf("no credentials found")
	}

	// For API key auth, return the API key
	if creds.Type == "api_key" {
		return creds.APIKey, nil
	}

	// For OAuth, check if token needs refresh
	if creds.Type == "oauth" {
		if creds.NeedsRefresh() {
			// Refresh the token
			client := NewOpenAIOAuthClient()
			newCreds, err := client.RefreshToken(creds.RefreshToken)
			if err != nil {
				return "", fmt.Errorf("failed to refresh token: %w", err)
			}

			// Update stored credentials
			if err := cm.SetOpenAIOAuthCredentials(newCreds); err != nil {
				return "", fmt.Errorf("failed to save refreshed token: %w", err)
			}

			return newCreds.AccessToken, nil
		}

		return creds.AccessToken, nil
	}

	return "", fmt.Errorf("unknown credential type: %s", creds.Type)
}

// GetCredentialsPath returns the absolute path to the credentials JSON file.
// This is useful for debugging or displaying the storage location to users.
func (cm *CredentialManager) GetCredentialsPath() string {
	return cm.credentialsPath
}

// validateAnthropicAPIKey validates the format of an Anthropic API key
func validateAnthropicAPIKey(apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// Anthropic API keys typically start with "sk-ant-" and are quite long
	if !strings.HasPrefix(apiKey, "sk-ant-") {
		return fmt.Errorf("invalid Anthropic API key format (should start with 'sk-ant-')")
	}

	if len(apiKey) < 20 {
		return fmt.Errorf("API key appears to be too short")
	}

	return nil
}

// CredentialSourceOAuth is the source description returned by
// GetAnthropicAPIKey when the key resolves to stored OAuth credentials.
// Consumers should compare against this constant (or use IsAnthropicOAuth)
// rather than matching the string literal.
const CredentialSourceOAuth = "stored OAuth credentials"

// IsAnthropicOAuth reports whether the active Anthropic credential resolves
// to a stored OAuth token (in which case the user is not billed per-token).
// flagValue is the --provider-api-key flag value (may be empty).
func IsAnthropicOAuth(flagValue string) bool {
	_, source, err := GetAnthropicAPIKey(flagValue)
	return err == nil && source == CredentialSourceOAuth
}

// GetAnthropicAPIKey retrieves an Anthropic API key from multiple sources in priority order:
// 1. Command-line flag value (highest priority)
// 2. Stored credentials (OAuth or API key)
// 3. ANTHROPIC_API_KEY environment variable (lowest priority)
// Returns the API key, a description of its source, and any error encountered.
// For OAuth credentials, it automatically refreshes expired tokens.
func GetAnthropicAPIKey(flagValue string) (string, string, error) {
	// 1. Check flag value first (highest priority)
	if flagValue != "" {
		return flagValue, "command-line flag", nil
	}

	// 2. Check stored credentials
	cm, err := NewCredentialManager()
	if err == nil {
		if creds, err := cm.GetAnthropicCredentials(); err == nil && creds != nil {
			if creds.Type == "oauth" && creds.AccessToken != "" {
				// For OAuth, get a valid access token (may refresh if needed)
				token, err := cm.GetValidAccessToken()
				if err != nil {
					return "", "", fmt.Errorf("failed to get valid OAuth token: %w", err)
				}
				return token, CredentialSourceOAuth, nil
			} else if creds.Type == "api_key" && creds.APIKey != "" {
				return creds.APIKey, "stored API key", nil
			}
		}
	}

	// 3. Fall back to environment variable
	if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
		return envKey, "ANTHROPIC_API_KEY environment variable", nil
	}

	missing := &MissingCredentialsError{
		Provider:     "anthropic",
		ProviderName: "Anthropic",
		EnvVars:      []string{"ANTHROPIC_API_KEY"},
	}

	// Check if OpenAI credentials exist to provide a helpful suggestion
	if cm != nil {
		hasOpenAI, _ := cm.HasOpenAICredentials()
		if hasOpenAI {
			missing.Hint = "OpenAI credentials were detected: run with --model openai/gpt-5.4 or set it as default with 'kit auth login openai --set-default'"
		}
	}

	return "", "", missing
}

// ---------------------------------------------------------------------------
// Generic provider API keys
// ---------------------------------------------------------------------------

// providerKeyAlias maps user-facing provider aliases onto the ID used as the
// storage key so "copilot" and "github-copilot" (or "gemini" and "google")
// resolve to the same entry.
func providerKeyAlias(providerID string) string {
	switch strings.ToLower(strings.TrimSpace(providerID)) {
	case "gemini":
		return "google"
	case "github-copilot":
		return "copilot"
	default:
		return strings.ToLower(strings.TrimSpace(providerID))
	}
}

// SetProviderAPIKey stores an API key for any provider. Anthropic and OpenAI
// are routed to their dedicated credential slots so the existing resolution
// paths (which also handle OAuth) pick the key up; every other provider is
// written to the generic Providers map. Copilot only supports OAuth and is
// rejected.
func (cm *CredentialManager) SetProviderAPIKey(providerID, apiKey string) error {
	providerID = providerKeyAlias(providerID)
	apiKey = strings.TrimSpace(apiKey)
	if providerID == "" {
		return fmt.Errorf("provider ID cannot be empty")
	}
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	switch providerID {
	case "anthropic":
		return cm.SetAnthropicCredentials(apiKey)
	case "openai":
		store, err := cm.LoadCredentials()
		if err != nil {
			return err
		}
		store.OpenAI = &OpenAICredentials{Type: "api_key", APIKey: apiKey, CreatedAt: time.Now()}
		return cm.SaveCredentials(store)
	case "copilot":
		return fmt.Errorf("GitHub Copilot does not use API keys; run 'kit auth login copilot'")
	}

	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}
	if store.Providers == nil {
		store.Providers = make(map[string]*ProviderCredentials)
	}
	store.Providers[providerID] = &ProviderCredentials{
		Type:      "api_key",
		APIKey:    apiKey,
		CreatedAt: time.Now(),
	}
	return cm.SaveCredentials(store)
}

// GetProviderAPIKey returns the stored API key for a provider, or "" when
// none is stored. For Anthropic and OpenAI only an api_key-type credential
// is returned; OAuth tokens are resolved by the provider-specific helpers.
func (cm *CredentialManager) GetProviderAPIKey(providerID string) (string, error) {
	providerID = providerKeyAlias(providerID)
	store, err := cm.LoadCredentials()
	if err != nil {
		return "", err
	}

	switch providerID {
	case "anthropic":
		if store.Anthropic != nil && store.Anthropic.Type == "api_key" {
			return store.Anthropic.APIKey, nil
		}
		return "", nil
	case "openai":
		if store.OpenAI != nil && store.OpenAI.Type == "api_key" {
			return store.OpenAI.APIKey, nil
		}
		return "", nil
	}

	if creds, ok := store.Providers[providerID]; ok && creds != nil && creds.Type == "api_key" {
		return creds.APIKey, nil
	}
	return "", nil
}

// HasProviderCredentials reports whether any credential (API key or OAuth)
// is stored for the provider.
func (cm *CredentialManager) HasProviderCredentials(providerID string) (bool, error) {
	switch providerKeyAlias(providerID) {
	case "anthropic":
		return cm.HasAnthropicCredentials()
	case "openai":
		return cm.HasOpenAICredentials()
	case "copilot":
		return cm.HasCopilotCredentials()
	}
	key, err := cm.GetProviderAPIKey(providerID)
	return key != "", err
}

// RemoveProviderCredentials deletes every stored credential for a provider.
func (cm *CredentialManager) RemoveProviderCredentials(providerID string) error {
	providerID = providerKeyAlias(providerID)
	switch providerID {
	case "anthropic":
		return cm.RemoveAnthropicCredentials()
	case "openai":
		return cm.RemoveOpenAICredentials()
	case "copilot":
		return cm.RemoveCopilotCredentials()
	}

	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}
	delete(store.Providers, providerID)
	if len(store.Providers) == 0 {
		store.Providers = nil
	}
	return cm.saveOrRemove(store)
}

// StoredProviderIDs returns the IDs of every provider with a stored
// credential (dedicated slots and generic entries), sorted.
func (cm *CredentialManager) StoredProviderIDs() ([]string, error) {
	store, err := cm.LoadCredentials()
	if err != nil {
		return nil, err
	}
	var ids []string
	if store.Anthropic != nil {
		ids = append(ids, "anthropic")
	}
	if store.OpenAI != nil {
		ids = append(ids, "openai")
	}
	if store.Copilot != nil {
		ids = append(ids, "copilot")
	}
	for id, creds := range store.Providers {
		if creds != nil && creds.APIKey != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// LookupStoredAPIKey is a best-effort helper for the provider layer: it
// returns the stored API key for providerID, or "" when the credential store
// is unavailable or holds nothing for that provider. It never returns an
// error so callers can chain it with environment-variable fallbacks.
func LookupStoredAPIKey(providerID string) string {
	cm, err := NewCredentialManager()
	if err != nil {
		return ""
	}
	key, err := cm.GetProviderAPIKey(providerID)
	if err != nil {
		return ""
	}
	return key
}
