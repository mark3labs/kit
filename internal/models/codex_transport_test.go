package models

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/auth"
)

type codexTestRoundTripper func(*http.Request) (*http.Response, error)

func (f codexTestRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// These tests run in sequence because the OAuth client uses DefaultTransport.
func mockDefaultTransport(t *testing.T, f codexTestRoundTripper) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = f
	t.Cleanup(func() { http.DefaultTransport = old })
}

func jsonTestResponse(r *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}
}

func TestCodexTransportTokenFreshness(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cm, err := auth.NewCredentialManager()
	if err != nil {
		t.Fatal(err)
	}
	save := func(token, account string, expiry int64) {
		t.Helper()
		if err := cm.SetOpenAIOAuthCredentials(&auth.OpenAICredentials{
			Type: "oauth", AccessToken: token, AccountID: account,
			RefreshToken: "refresh-token", ExpiresAt: expiry,
		}); err != nil {
			t.Fatal(err)
		}
	}
	save("initial-token", "initial-account", time.Now().Add(time.Hour).Unix())
	refreshed := "header." + base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"refreshed-account"}}`)) + ".signature"
	wantToken, wantAccount := "initial-token", "initial-account"
	refreshes, requests := 0, 0
	mockDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == "https://auth.openai.com/oauth/token" {
			refreshes++
			return jsonTestResponse(r, fmt.Sprintf(`{"access_token":%q,"refresh_token":"new-refresh","expires_in":3600}`, refreshed)), nil
		}
		if !isCodexDestination(r.URL.String()) {
			t.Fatalf("unexpected destination: %s", r.URL)
		}
		requests++
		if r.Header.Get("Authorization") != "Bearer "+wantToken || r.Header.Get("ChatGPT-Account-ID") != wantAccount {
			t.Errorf("request did not use current credentials: %v", r.Header)
		}
		return jsonTestResponse(r, `{}`), nil
	})
	client := createCodexHTTPClient(false)
	send := func() {
		t.Helper()
		resp, err := client.Get("https://chatgpt.com/backend-api/codex/responses")
		if err != nil {
			t.Fatal(err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}
	send()
	// A long-lived client must see credentials replaced after its creation.
	wantToken, wantAccount = "replacement-token", "replacement-account"
	save(wantToken, wantAccount, time.Now().Add(time.Hour).Unix())
	send()
	// Expired credentials must refresh before the next backend request.
	save("expired-token", "old-account", time.Now().Add(-time.Hour).Unix())
	wantToken, wantAccount = refreshed, "refreshed-account"
	send()
	if refreshes != 1 || requests != 3 {
		t.Fatalf("refreshes = %d, requests = %d", refreshes, requests)
	}
}

func TestCodexTransportDestinationGuard(t *testing.T) {
	blocked := []string{
		"http://chatgpt.com/backend-api/codex/responses",
		"https://example.com/backend-api/codex/responses",
		"https://chatgpt.com.evil.example/backend-api/codex/responses",
		"https://chatgpt.com:444/backend-api/codex/responses",
		"https://user@chatgpt.com/backend-api/codex/responses",
		"https://chatgpt.com/backend-api/other",
		"https://chatgpt.com/backend-api/codex-other",
		"https://chatgpt.com/backend-api/codex/../other",
		"https://chatgpt.com/backend-api/codex/%2e%2e/other",
	}
	for _, destination := range blocked {
		t.Run(destination, func(t *testing.T) {
			transport := &codexTransport{base: codexTestRoundTripper(func(*http.Request) (*http.Response, error) {
				t.Fatal("blocked destination reached base transport")
				return nil, nil
			})}
			req, err := http.NewRequest(http.MethodGet, destination, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := transport.RoundTrip(req); err == nil {
				t.Fatal("expected destination error")
			}
		})
	}
}

func TestCodexTransportRedirectGuard(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cm, err := auth.NewCredentialManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := cm.SetOpenAIOAuthCredentials(&auth.OpenAICredentials{
		Type: "oauth", AccessToken: "token", AccountID: "account",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	for _, destination := range []string{
		"https://example.com/responses",
		"http://chatgpt.com/backend-api/codex/responses",
		"https://chatgpt.com/backend-api/other",
	} {
		t.Run(destination, func(t *testing.T) {
			calls := 0
			mockDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
				calls++
				if calls != 1 {
					t.Fatal("redirect reached base transport")
				}
				resp := jsonTestResponse(r, "")
				resp.StatusCode = http.StatusFound
				resp.Header.Set("Location", destination)
				return resp, nil
			})
			if _, err := createCodexHTTPClient(false).Get("https://chatgpt.com/backend-api/codex/responses"); err == nil {
				t.Fatal("expected redirect destination error")
			}
			if calls != 1 {
				t.Fatalf("base transport calls = %d", calls)
			}
		})
	}
}
