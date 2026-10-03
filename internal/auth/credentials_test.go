package auth

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCredentialManager(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "kit-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a credential manager with a test path
	cm := &CredentialManager{
		credentialsPath: filepath.Join(tempDir, "credentials.json"),
	}

	// Test initial state - no credentials
	hasAuth, err := cm.HasAnthropicCredentials()
	if err != nil {
		t.Fatalf("HasAnthropicCredentials failed: %v", err)
	}
	if hasAuth {
		t.Error("Expected no credentials initially")
	}

	// Test setting credentials
	testAPIKey := "sk-ant-test-key-12345678901234567890"
	err = cm.SetAnthropicCredentials(testAPIKey)
	if err != nil {
		t.Fatalf("SetAnthropicCredentials failed: %v", err)
	}

	// Test that credentials are now present
	hasAuth, err = cm.HasAnthropicCredentials()
	if err != nil {
		t.Fatalf("HasAnthropicCredentials failed: %v", err)
	}
	if !hasAuth {
		t.Error("Expected credentials to be present")
	}

	// Test retrieving credentials
	creds, err := cm.GetAnthropicCredentials()
	if err != nil {
		t.Fatalf("GetAnthropicCredentials failed: %v", err)
	}
	if creds == nil {
		t.Fatal("Expected credentials to be returned")
		return
	}
	if creds.APIKey != testAPIKey {
		t.Errorf("Expected API key %s, got %s", testAPIKey, creds.APIKey)
	}
	if creds.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	// Test removing credentials
	err = cm.RemoveAnthropicCredentials()
	if err != nil {
		t.Fatalf("RemoveAnthropicCredentials failed: %v", err)
	}

	// Test that credentials are gone
	hasAuth, err = cm.HasAnthropicCredentials()
	if err != nil {
		t.Fatalf("HasAnthropicCredentials failed: %v", err)
	}
	if hasAuth {
		t.Error("Expected no credentials after removal")
	}

	// Test that file is removed when empty
	if _, err := os.Stat(cm.credentialsPath); !os.IsNotExist(err) {
		t.Error("Expected credentials file to be removed when empty")
	}
}

func TestValidateAnthropicAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid key",
			apiKey:  "sk-ant-test-key-12345678901234567890",
			wantErr: false,
		},
		{
			name:    "empty key",
			apiKey:  "",
			wantErr: true,
		},
		{
			name:    "wrong prefix",
			apiKey:  "sk-test-key-12345678901234567890",
			wantErr: true,
		},
		{
			name:    "too short",
			apiKey:  "sk-ant-short",
			wantErr: true,
		},
		{
			name:    "with whitespace",
			apiKey:  "  sk-ant-test-key-12345678901234567890  ",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAnthropicAPIKey(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAnthropicAPIKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetAnthropicAPIKey(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "kit-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Save original environment
	originalAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	originalXDGConfig := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		_ = os.Setenv("ANTHROPIC_API_KEY", originalAPIKey)
		_ = os.Setenv("XDG_CONFIG_HOME", originalXDGConfig)
	}()

	// Set up test environment
	_ = os.Setenv("XDG_CONFIG_HOME", tempDir)
	_ = os.Unsetenv("ANTHROPIC_API_KEY")

	// Test 1: Flag value takes precedence
	flagKey := "sk-ant-flag-key-12345678901234567890"
	apiKey, source, err := GetAnthropicAPIKey(flagKey)
	if err != nil {
		t.Fatalf("GetAnthropicAPIKey failed: %v", err)
	}
	if apiKey != flagKey {
		t.Errorf("Expected flag key %s, got %s", flagKey, apiKey)
	}
	if source != "command-line flag" {
		t.Errorf("Expected source 'command-line flag', got %s", source)
	}

	// Test 2: Stored credentials when no flag
	cm, err := NewCredentialManager()
	if err != nil {
		t.Fatalf("NewCredentialManager failed: %v", err)
	}

	storedKey := "sk-ant-stored-key-12345678901234567890"
	err = cm.SetAnthropicCredentials(storedKey)
	if err != nil {
		t.Fatalf("SetAnthropicCredentials failed: %v", err)
	}

	apiKey, source, err = GetAnthropicAPIKey("")
	if err != nil {
		t.Fatalf("GetAnthropicAPIKey failed: %v", err)
	}
	if apiKey != storedKey {
		t.Errorf("Expected stored key %s, got %s", storedKey, apiKey)
	}
	if source != "stored API key" {
		t.Errorf("Expected source 'stored API key', got %s", source)
	}

	// Test 3: Environment variable when no flag or stored credentials
	err = cm.RemoveAnthropicCredentials()
	if err != nil {
		t.Fatalf("RemoveAnthropicCredentials failed: %v", err)
	}

	envKey := "sk-ant-env-key-12345678901234567890"
	_ = os.Setenv("ANTHROPIC_API_KEY", envKey)

	apiKey, source, err = GetAnthropicAPIKey("")
	if err != nil {
		t.Fatalf("GetAnthropicAPIKey failed: %v", err)
	}
	if apiKey != envKey {
		t.Errorf("Expected env key %s, got %s", envKey, apiKey)
	}
	if source != "ANTHROPIC_API_KEY environment variable" {
		t.Errorf("Expected source 'ANTHROPIC_API_KEY environment variable', got %s", source)
	}

	// Test 4: No credentials available
	_ = os.Unsetenv("ANTHROPIC_API_KEY")

	_, _, err = GetAnthropicAPIKey("")
	if err == nil {
		t.Error("Expected error when no credentials available")
	}
}

func TestCredentialStorePersistence(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "kit-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	defer func() { _ = os.RemoveAll(tempDir) }()

	credentialsPath := filepath.Join(tempDir, "credentials.json")

	// Create first manager and store credentials
	cm1 := &CredentialManager{credentialsPath: credentialsPath}
	testAPIKey := "sk-ant-test-key-12345678901234567890"

	err = cm1.SetAnthropicCredentials(testAPIKey)
	if err != nil {
		t.Fatalf("SetAnthropicCredentials failed: %v", err)
	}

	// Create second manager and verify credentials persist
	cm2 := &CredentialManager{credentialsPath: credentialsPath}

	creds, err := cm2.GetAnthropicCredentials()
	if err != nil {
		t.Fatalf("GetAnthropicCredentials failed: %v", err)
	}
	if creds == nil {
		t.Fatal("Expected credentials to persist")
		return
	}
	if creds.APIKey != testAPIKey {
		t.Errorf("Expected API key %s, got %s", testAPIKey, creds.APIKey)
	}

	// Verify file permissions
	info, err := os.Stat(credentialsPath)
	if err != nil {
		t.Fatalf("Failed to stat credentials file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %v", info.Mode().Perm())
	}
}

func TestCopilotCredentials(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "kit-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	cm := &CredentialManager{
		credentialsPath: filepath.Join(tempDir, "credentials.json"),
	}

	creds := &CopilotCredentials{
		Type:               "oauth",
		GitHubToken:        "github-token",
		CopilotAccessToken: "copilot-token",
		ExpiresAt:          time.Now().Add(time.Hour).Unix(),
		CreatedAt:          time.Now(),
	}

	if err := cm.SetCopilotOAuthCredentials(creds); err != nil {
		t.Fatalf("SetCopilotOAuthCredentials failed: %v", err)
	}

	hasAuth, err := cm.HasCopilotCredentials()
	if err != nil {
		t.Fatalf("HasCopilotCredentials failed: %v", err)
	}
	if !hasAuth {
		t.Fatal("Expected Copilot credentials")
	}

	token, err := cm.GetValidCopilotAccessToken()
	if err != nil {
		t.Fatalf("GetValidCopilotAccessToken failed: %v", err)
	}
	if token != creds.CopilotAccessToken {
		t.Fatalf("Expected Copilot token %q, got %q", creds.CopilotAccessToken, token)
	}

	if err := cm.RemoveCopilotCredentials(); err != nil {
		t.Fatalf("RemoveCopilotCredentials failed: %v", err)
	}
	hasAuth, err = cm.HasCopilotCredentials()
	if err != nil {
		t.Fatalf("HasCopilotCredentials after removal failed: %v", err)
	}
	if hasAuth {
		t.Fatal("Expected no Copilot credentials after removal")
	}
}

func TestRemoveCredentialsPreservesOtherProviders(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "kit-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	cm := &CredentialManager{
		credentialsPath: filepath.Join(tempDir, "credentials.json"),
	}

	if err := cm.SetOpenAIOAuthCredentials(&OpenAICredentials{
		Type:         "oauth",
		AccessToken:  "openai-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		AccountID:    "account",
		CreatedAt:    time.Now(),
	}); err != nil {
		t.Fatalf("SetOpenAIOAuthCredentials failed: %v", err)
	}
	if err := cm.SetCopilotOAuthCredentials(&CopilotCredentials{
		Type:               "oauth",
		GitHubToken:        "github-token",
		CopilotAccessToken: "copilot-token",
		ExpiresAt:          time.Now().Add(time.Hour).Unix(),
		CreatedAt:          time.Now(),
	}); err != nil {
		t.Fatalf("SetCopilotOAuthCredentials failed: %v", err)
	}

	if err := cm.RemoveCopilotCredentials(); err != nil {
		t.Fatalf("RemoveCopilotCredentials failed: %v", err)
	}

	hasOpenAI, err := cm.HasOpenAICredentials()
	if err != nil {
		t.Fatalf("HasOpenAICredentials failed: %v", err)
	}
	if !hasOpenAI {
		t.Fatal("Expected OpenAI credentials to remain after removing Copilot credentials")
	}
}

func TestGetValidOpenAICredentialsSnapshot(t *testing.T) {
	for _, credType := range []string{"oauth", "api_key"} {
		t.Run(credType, func(t *testing.T) {
			cm := &CredentialManager{credentialsPath: filepath.Join(t.TempDir(), "credentials.json")}
			original := &OpenAICredentials{
				Type: credType, APIKey: "api-key", AccessToken: "token", AccountID: "account",
				ExpiresAt: time.Now().Add(time.Hour).Unix(),
			}
			if err := cm.SetOpenAIOAuthCredentials(original); err != nil {
				t.Fatal(err)
			}
			snapshot, err := cm.GetValidOpenAICredentials()
			if err != nil {
				t.Fatal(err)
			}
			if *snapshot != *original {
				t.Fatal("snapshot differs from stored credentials")
			}
			token, err := cm.GetValidOpenAIAccessToken()
			if err != nil {
				t.Fatal(err)
			}
			want := original.AccessToken
			if credType == "api_key" {
				want = original.APIKey
			}
			if token != want {
				t.Fatal("access token helper returned the wrong credential field")
			}
			if err := cm.SetOpenAIOAuthCredentials(&OpenAICredentials{
				Type: "oauth", AccessToken: "replacement-token", AccountID: "replacement-account",
			}); err != nil {
				t.Fatal(err)
			}
			if *snapshot != *original {
				t.Fatal("credential replacement changed the snapshot")
			}
		})
	}
}

func TestGetValidOpenAICredentialsErrors(t *testing.T) {
	cm := &CredentialManager{credentialsPath: filepath.Join(t.TempDir(), "credentials.json")}
	for _, creds := range []*OpenAICredentials{nil, {Type: "unknown"}} {
		if err := cm.SaveCredentials(&CredentialStore{OpenAI: creds}); err != nil {
			t.Fatal(err)
		}
		if snapshot, err := cm.GetValidOpenAICredentials(); err == nil || snapshot != nil {
			t.Fatal("expected no snapshot and an error")
		}
		if _, err := cm.GetValidOpenAIAccessToken(); err == nil {
			t.Fatal("expected access token helper error")
		}
	}
}

type openAICredentialTestTransport func(*http.Request) (*http.Response, error)

func (f openAICredentialTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetValidOpenAICredentialsRefreshSnapshot(t *testing.T) {
	cm := &CredentialManager{credentialsPath: filepath.Join(t.TempDir(), "credentials.json")}
	if err := cm.SetOpenAIOAuthCredentials(&OpenAICredentials{
		Type: "oauth", AccessToken: "expired-token", AccountID: "old-account",
		RefreshToken: "old-refresh", ExpiresAt: time.Now().Add(-time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	refreshedToken := "header." + base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"new-account"}}`)) + ".signature"
	oldTransport := http.DefaultTransport
	http.DefaultTransport = openAICredentialTestTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://auth.openai.com/oauth/token" {
			t.Fatalf("unexpected refresh destination: %s", req.URL)
		}
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if req.Form.Get("refresh_token") != "old-refresh" {
			t.Fatal("wrong refresh token")
		}
		return &http.Response{
			StatusCode: http.StatusOK, Header: make(http.Header), Request: req,
			Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"access_token":%q,"refresh_token":"new-refresh","expires_in":3600}`, refreshedToken))),
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	snapshot, err := cm.GetValidOpenAICredentials()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.AccessToken != refreshedToken || snapshot.AccountID != "new-account" || snapshot.RefreshToken != "new-refresh" || snapshot.NeedsRefresh() {
		t.Fatal("snapshot does not contain the refreshed credentials")
	}
	stored, err := cm.GetOpenAICredentials()
	if err != nil {
		t.Fatal(err)
	}
	// JSON storage does not retain the monotonic part of CreatedAt.
	if !stored.CreatedAt.Equal(snapshot.CreatedAt) {
		t.Fatal("stored creation time differs from the refreshed snapshot")
	}
	stored.CreatedAt = snapshot.CreatedAt
	if *stored != *snapshot {
		t.Fatal("stored credentials differ from the refreshed snapshot")
	}
	if err := cm.SetOpenAIOAuthCredentials(&OpenAICredentials{
		Type: "oauth", AccessToken: "login-token", AccountID: "login-account",
	}); err != nil {
		t.Fatal(err)
	}
	if snapshot.AccessToken != refreshedToken || snapshot.AccountID != "new-account" {
		t.Fatal("login changed the refreshed snapshot")
	}
}
