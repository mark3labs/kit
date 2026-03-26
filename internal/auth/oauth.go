package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthClient handles OAuth 2.0 authentication flow with Anthropic using the
// PKCE (Proof Key for Code Exchange) extension for enhanced security in public clients.
// It manages the authorization URL generation, code exchange, and token refresh operations.
type OAuthClient struct {
	ClientID     string
	AuthorizeURL string
	TokenURL     string
	RedirectURI  string
	Scopes       string
}

// AuthData contains the authorization URL for user authentication and the PKCE
// verifier needed for the subsequent code exchange. The verifier must be stored
// securely and used when exchanging the authorization code for tokens.
type AuthData struct {
	URL      string
	Verifier string
	State    string // Optional state parameter for CSRF protection
}

// NewOAuthClient creates a new OAuth client configured for Anthropic's OAuth service.
// The client uses a public client ID (as per OAuth 2.0 public client specification)
// with PKCE for security. The configuration includes the authorization endpoint,
// token endpoint, redirect URI, and required scopes for API key creation and inference.
func NewOAuthClient() *OAuthClient {
	return &OAuthClient{
		// OAuth client ID is public by design for CLI applications (OAuth public clients).
		// Security is provided by PKCE flow, not by keeping the client ID secret.
		// This follows the same pattern as GitHub CLI, Google Cloud SDK, and other major CLI tools.
		ClientID:     "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
		AuthorizeURL: "https://claude.ai/oauth/authorize",
		TokenURL:     "https://console.anthropic.com/v1/oauth/token",
		RedirectURI:  "https://console.anthropic.com/oauth/code/callback",
		Scopes:       "org:create_api_key user:profile user:inference",
	}
}

// generatePKCE generates a cryptographically secure PKCE verifier and challenge pair
// for the OAuth 2.0 PKCE flow. The verifier is a random 32-byte string encoded as
// base64url, and the challenge is the SHA256 hash of the verifier, also base64url encoded.
// Returns the verifier (to be stored securely), challenge (to be sent with auth request),
// and any error encountered during generation.
func generatePKCE() (verifier, challenge string, err error) {
	// Generate 32 bytes of random data
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode verifier as base64url without padding
	verifier = base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Generate challenge by SHA256 hashing the verifier
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	return verifier, challenge, nil
}

// GetAuthorizationURL generates a complete authorization URL for the OAuth flow with
// PKCE parameters. The URL includes the client ID, redirect URI, requested scopes,
// and PKCE challenge. Returns an AuthData structure containing the URL for user
// authentication and the PKCE verifier for the subsequent code exchange.
func (c *OAuthClient) GetAuthorizationURL() (*AuthData, error) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE: %w", err)
	}

	params := url.Values{
		"code":                  {"true"},
		"client_id":             {c.ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {c.RedirectURI},
		"scope":                 {c.Scopes},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {verifier}, // Using verifier as state (following Python impl)
	}

	authURL := fmt.Sprintf("%s?%s", c.AuthorizeURL, params.Encode())

	return &AuthData{
		URL:      authURL,
		Verifier: verifier,
	}, nil
}

// ExchangeCode exchanges an authorization code for access and refresh tokens.
// The code parameter should be the authorization code received from the OAuth callback.
// The verifier parameter must be the same PKCE verifier generated during GetAuthorizationURL.
// Returns AnthropicCredentials containing the tokens and expiration information.
func (c *OAuthClient) ExchangeCode(code, verifier string) (*AnthropicCredentials, error) {
	// Parse code and state
	parsedCode, parsedState := c.parseCodeAndState(code)

	// Build request body
	reqBody := map[string]any{
		"code":          parsedCode,
		"grant_type":    "authorization_code",
		"client_id":     c.ClientID,
		"redirect_uri":  c.RedirectURI,
		"code_verifier": verifier,
	}

	// Include state if present
	if parsedState != "" {
		reqBody["state"] = parsedState
	}

	// Make request
	return c.makeTokenRequest(reqBody)
}

// RefreshToken refreshes an expired or expiring access token using a refresh token.
// Returns new AnthropicCredentials with updated access token, refresh token (may be
// rotated), and new expiration timestamp. Returns an error if the refresh fails or
// the refresh token is invalid.
func (c *OAuthClient) RefreshToken(refreshToken string) (*AnthropicCredentials, error) {
	reqBody := map[string]any{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     c.ClientID,
	}

	return c.makeTokenRequest(reqBody)
}

// makeTokenRequest makes a token request to the OAuth server
func (c *OAuthClient) makeTokenRequest(body map[string]any) (*AnthropicCredentials, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", c.TokenURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			return nil, fmt.Errorf("token request failed: %v", errorResp)
		}
		return nil, fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &AnthropicCredentials{
		Type:         "oauth",
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Unix() + int64(tokenResp.ExpiresIn),
		CreatedAt:    time.Now(),
	}, nil
}

// parseCodeAndState parses the authorization code and state from the callback
func (c *OAuthClient) parseCodeAndState(code string) (parsedCode, parsedState string) {
	splits := strings.Split(code, "#")
	parsedCode = splits[0]
	if len(splits) > 1 {
		parsedState = splits[1]
	}
	return
}

// OpenAIOAuthClient handles OAuth 2.0 authentication flow with OpenAI Codex (ChatGPT Plus/Pro).
// This uses OpenAI's auth0-based OAuth service for ChatGPT account authentication.
type OpenAIOAuthClient struct {
	ClientID     string
	AuthorizeURL string
	TokenURL     string
	RedirectURI  string
	Scopes       string
}

// NewOpenAIOAuthClient creates a new OAuth client configured for OpenAI Codex OAuth.
// This uses the public client ID for CLI applications with PKCE for security.
func NewOpenAIOAuthClient() *OpenAIOAuthClient {
	return &OpenAIOAuthClient{
		// Public client ID for OpenAI Codex CLI OAuth
		ClientID:     "app_EMoamEEZ73f0CkXaXp7hrann",
		AuthorizeURL: "https://auth.openai.com/oauth/authorize",
		TokenURL:     "https://auth.openai.com/oauth/token",
		RedirectURI:  "http://localhost:1455/auth/callback",
		Scopes:       "openid profile email offline_access",
	}
}

// GetAuthorizationURL generates a complete authorization URL for the OAuth flow with
// PKCE parameters. Returns an AuthData structure containing the URL for user
// authentication and the PKCE verifier for the subsequent code exchange.
func (c *OpenAIOAuthClient) GetAuthorizationURL() (*AuthData, error) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE: %w", err)
	}

	// Generate random state
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}
	state := fmt.Sprintf("%x", stateBytes)

	params := url.Values{
		"response_type":              {"code"},
		"client_id":                  {c.ClientID},
		"redirect_uri":               {c.RedirectURI},
		"scope":                      {c.Scopes},
		"code_challenge":             {challenge},
		"code_challenge_method":      {"S256"},
		"state":                      {state},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
		"originator":                 {"kit"},
	}

	authURL := fmt.Sprintf("%s?%s", c.AuthorizeURL, params.Encode())

	return &AuthData{
		URL:      authURL,
		Verifier: verifier,
		State:    state,
	}, nil
}

// ExchangeCode exchanges an authorization code for access and refresh tokens.
// The code parameter should be the authorization code received from the OAuth callback.
// The verifier parameter must be the same PKCE verifier generated during GetAuthorizationURL.
// Returns OpenAICredentials containing the tokens, expiration, and account ID.
func (c *OpenAIOAuthClient) ExchangeCode(code, verifier string) (*OpenAICredentials, error) {
	return c.exchangeAuthorizationCode(code, verifier, c.RedirectURI)
}

// exchangeAuthorizationCode performs the token exchange with the OAuth server
func (c *OpenAIOAuthClient) exchangeAuthorizationCode(code, verifier, redirectUri string) (*OpenAICredentials, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {c.ClientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectUri},
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", c.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		IDToken      string `json:"id_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("token response missing required fields")
	}

	// Extract account ID from JWT token
	accountID := extractOpenAIAccountID(tokenResp.AccessToken)
	if accountID == "" {
		return nil, fmt.Errorf("failed to extract account ID from token")
	}

	return &OpenAICredentials{
		Type:         "oauth",
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Unix() + int64(tokenResp.ExpiresIn),
		CreatedAt:    time.Now(),
		AccountID:    accountID,
	}, nil
}

// RefreshToken refreshes an expired or expiring access token using a refresh token.
// Returns new OpenAICredentials with updated access token, refresh token (may be
// rotated), and new expiration timestamp. Returns an error if the refresh fails or
// the refresh token is invalid.
func (c *OpenAIOAuthClient) RefreshToken(refreshToken string) (*OpenAICredentials, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {c.ClientID},
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", c.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make refresh request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("refresh response missing required fields")
	}

	// Extract account ID from JWT token
	accountID := extractOpenAIAccountID(tokenResp.AccessToken)
	if accountID == "" {
		return nil, fmt.Errorf("failed to extract account ID from refreshed token")
	}

	return &OpenAICredentials{
		Type:         "oauth",
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Unix() + int64(tokenResp.ExpiresIn),
		CreatedAt:    time.Now(),
		AccountID:    accountID,
	}, nil
}

// extractOpenAIAccountID extracts the ChatGPT account ID from a JWT access token.
// The account ID is stored in the claim path https://api.openai.com/auth.chatgpt_account_id
func extractOpenAIAccountID(token string) string {
	// JWT tokens are base64-encoded JSON payloads
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}

	// Decode payload (second part)
	payload := parts[1]
	// Add padding if needed
	if len(payload)%4 != 0 {
		payload += strings.Repeat("=", 4-len(payload)%4)
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}

	var claims map[string]any
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}

	// Navigate to the claim path: https://api.openai.com/auth.chatgpt_account_id
	authPath, ok := claims["https://api.openai.com/auth"].(map[string]any)
	if !ok {
		return ""
	}

	accountID, ok := authPath["chatgpt_account_id"].(string)
	if !ok {
		return ""
	}

	return accountID
}

// ParseOpenAIAuthorizationInput parses various forms of authorization input:
// - Full callback URL: http://localhost:1455/auth/callback?code=xxx&state=yyy
// - Code#State format: abc123#state456
// - Query string: code=abc123&state=state456
// - Just the code: abc123
func ParseOpenAIAuthorizationInput(input string) (code, state string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", ""
	}

	// Try parsing as URL
	if strings.HasPrefix(input, "http") {
		if u, err := url.Parse(input); err == nil {
			return u.Query().Get("code"), u.Query().Get("state")
		}
	}

	// Try code#state format
	if strings.Contains(input, "#") {
		parts := strings.SplitN(input, "#", 2)
		return parts[0], parts[1]
	}

	// Try query string format
	if strings.Contains(input, "code=") {
		if values, err := url.ParseQuery(input); err == nil {
			return values.Get("code"), values.Get("state")
		}
	}

	// Assume it's just the code
	return input, ""
}

// SetOAuthCredentials stores OAuth credentials in the credential manager's secure storage.
// The credentials should include access token, refresh token, and expiration information.
// Returns an error if the credentials cannot be saved.
func (cm *CredentialManager) SetOAuthCredentials(creds *AnthropicCredentials) error {
	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}

	store.Anthropic = creds
	return cm.SaveCredentials(store)
}

// GetValidAccessToken returns a valid access token for API requests. For OAuth credentials,
// it automatically refreshes the token if it's expired or about to expire. For API key
// credentials, it simply returns the API key. Returns an error if no credentials are found,
// if token refresh fails, or if the credential type is unknown.
func (cm *CredentialManager) GetValidAccessToken() (string, error) {
	creds, err := cm.GetAnthropicCredentials()
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
			client := NewOAuthClient()
			newCreds, err := client.RefreshToken(creds.RefreshToken)
			if err != nil {
				return "", fmt.Errorf("failed to refresh token: %w", err)
			}

			// Update stored credentials
			if err := cm.SetOAuthCredentials(newCreds); err != nil {
				return "", fmt.Errorf("failed to save refreshed token: %w", err)
			}

			return newCreds.AccessToken, nil
		}

		return creds.AccessToken, nil
	}

	return "", fmt.Errorf("unknown credential type: %s", creds.Type)
}
