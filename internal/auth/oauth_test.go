package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCopilotStartDeviceFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		if r.Form.Get("client_id") != "client-id" {
			t.Fatalf("expected client id, got %q", r.Form.Get("client_id"))
		}
		if r.Form.Get("scope") != "read:user" {
			t.Fatalf("expected scope, got %q", r.Form.Get("scope"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "device-code",
			"user_code":        "USER-CODE",
			"verification_uri": "https://github.com/login/device",
			"expires_in":       600,
			"interval":         1,
		})
	}))
	defer server.Close()

	client := NewCopilotOAuthClient()
	client.ClientID = "client-id"
	client.DeviceURL = server.URL

	code, err := client.StartDeviceFlow(context.Background())
	if err != nil {
		t.Fatalf("StartDeviceFlow failed: %v", err)
	}
	if code.DeviceCode != "device-code" || code.UserCode != "USER-CODE" || code.Interval != 1 {
		t.Fatalf("unexpected device code: %#v", code)
	}
}

func TestCopilotPollDeviceToken(t *testing.T) {
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polls++
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" {
			t.Fatalf("unexpected grant type: %q", r.Form.Get("grant_type"))
		}
		if polls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "authorization_pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "github-token"})
	}))
	defer server.Close()

	client := NewCopilotOAuthClient()
	client.ClientID = "client-id"
	client.TokenURL = server.URL
	client.PollTimeout = 5 * time.Second
	client.ClientTimeout = time.Second

	token, err := client.PollDeviceToken(context.Background(), &CopilotDeviceCode{
		DeviceCode: "device-code",
		ExpiresIn:  10,
		Interval:   1,
	})
	if err != nil {
		t.Fatalf("PollDeviceToken failed: %v", err)
	}
	if token != "github-token" {
		t.Fatalf("expected github-token, got %q", token)
	}
	if polls != 2 {
		t.Fatalf("expected 2 polls, got %d", polls)
	}
}

func TestCopilotRefreshToken(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "token github-token" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("User-Agent") != "kit" {
			t.Fatalf("unexpected user agent: %q", r.Header.Get("User-Agent"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "copilot-token",
			"expires_at": expiresAt,
		})
	}))
	defer server.Close()

	client := NewCopilotOAuthClient()
	client.CopilotURL = server.URL

	creds, err := client.RefreshCopilotToken(context.Background(), "github-token")
	if err != nil {
		t.Fatalf("RefreshCopilotToken failed: %v", err)
	}
	if creds.GitHubToken != "github-token" || creds.CopilotAccessToken != "copilot-token" {
		t.Fatalf("unexpected credentials: %#v", creds)
	}
	if creds.ExpiresAt != expiresAt {
		t.Fatalf("expected expires_at %d, got %d", expiresAt, creds.ExpiresAt)
	}
}
