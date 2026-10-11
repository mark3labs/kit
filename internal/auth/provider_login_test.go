package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestProviderLoginFlow checks credential storage and failure handling with a fake token endpoint.
func TestProviderLoginFlow(t *testing.T) {
	// The production provider requires a fixed port. Package tests must not run in parallel.
	l, err := net.Listen("tcp", "127.0.0.1:1455")
	if err != nil {
		t.Skipf("callback port unavailable: %v", err)
	}
	_ = l.Close()
	for _, scenario := range []string{"success", "wrong_state", "denied", "missing_code", "exchange_failure", "storage_failure", "cancel_wait", "cancel_exchange", "deadline", "redirect"} {
		t.Run(scenario, func(t *testing.T) {
			cm := &CredentialManager{credentialsPath: filepath.Join(t.TempDir(), "credentials.json")}
			if scenario == "storage_failure" {
				cm.credentialsPath = t.TempDir()
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "deadline" {
				var stop context.CancelFunc
				ctx, stop = context.WithTimeout(ctx, 50*time.Millisecond)
				defer stop()
			}
			var challenge string
			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				hash := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
				if base64.RawURLEncoding.EncodeToString(hash[:]) != challenge {
					t.Error("PKCE verifier does not match challenge")
				}
				if r.Form.Get("code") != "test-code" || r.Form.Get("redirect_uri") != "http://localhost:1455/auth/callback" {
					t.Error("wrong exchange parameters")
				}
				switch scenario {
				case "exchange_failure":
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte("secret-token"))
					return
				case "redirect":
					w.Header().Set("Location", "https://example.invalid/steal")
					w.WriteHeader(http.StatusTemporaryRedirect)
					return
				case "cancel_exchange":
					cancel()
					<-r.Context().Done()
					return
				}
				payload := base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"account"}}`))
				_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "header." + payload + ".signature", "refresh_token": "refresh-secret", "expires_in": 3600})
			}))
			defer tokenServer.Close()
			client := NewOpenAIOAuthClient()
			client.TokenURL = tokenServer.URL
			err := loginProvider(ctx, "openai", LoginOptions{CredentialManager: cm, OnAuthorization: func(ctx context.Context, instructions LoginInstructions) error {
				u, err := url.Parse(instructions.URL)
				if err != nil {
					return err
				}
				q := u.Query()
				challenge = q.Get("code_challenge")
				if q.Get("code_challenge_method") != "S256" || q.Get("state") == "" {
					t.Error("missing OAuth security parameters")
				}
				if scenario == "cancel_wait" {
					cancel()
					return nil
				}
				if scenario == "deadline" {
					return nil
				}
				query := url.Values{"state": {q.Get("state")}, "code": {"test-code"}}
				switch scenario {
				case "wrong_state":
					query.Set("state", "wrong")
				case "denied":
					query.Set("error", "access_denied")
					query.Set("error_description", "secret-token")
				case "missing_code":
					query.Del("code")
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:1455/auth/callback?"+query.Encode(), nil)
				if err != nil {
					return err
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return err
				}
				if err := resp.Body.Close(); err != nil {
					return err
				}
				if scenario == "wrong_state" {
					if resp.StatusCode != http.StatusBadRequest {
						t.Error("invalid state was not rejected")
					}
					query.Set("state", q.Get("state"))
					req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:1455/auth/callback?"+query.Encode(), nil)
					if err != nil {
						return err
					}
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						return err
					}
					return resp.Body.Close()
				}
				return nil
			}}, client)
			if scenario == "success" || scenario == "wrong_state" {
				if err != nil {
					t.Fatal(err)
				}
				creds, err := cm.GetOpenAICredentials()
				if err != nil {
					t.Fatal(err)
				}
				if creds.Type != "oauth" || creds.AccountID != "account" || creds.RefreshToken != "refresh-secret" {
					t.Fatal("incompatible stored credentials")
				}
				info, err := os.Stat(cm.credentialsPath)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0600 {
					t.Fatalf("permissions: %o", info.Mode().Perm())
				}
			} else {
				if err == nil {
					t.Fatal("expected login error")
				}
				if strings.Contains(err.Error(), "secret-token") {
					t.Fatal("error disclosed endpoint content")
				}
				if strings.HasPrefix(scenario, "cancel_") && !errors.Is(err, context.Canceled) {
					t.Fatalf("expected cancellation, got %v", err)
				}
				if scenario == "deadline" && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected deadline, got %v", err)
				}
				if scenario != "storage_failure" {
					if _, statErr := os.Stat(cm.credentialsPath); !os.IsNotExist(statErr) {
						t.Fatal("failed login wrote credentials")
					}
				}
			}
			listener, err := net.Listen("tcp", "127.0.0.1:1455")
			if err != nil {
				t.Fatalf("callback listener leaked: %v", err)
			}
			_ = listener.Close()
		})
	}
}
