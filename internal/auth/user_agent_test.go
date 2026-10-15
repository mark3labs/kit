package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOAuthUserAgents(t *testing.T) {
	for _, identity := range []string{"", "AuthApp/1"} {
		t.Run(identity, func(t *testing.T) {
			want := identity
			if want == "" {
				want = "KIT"
			}
			seen := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen++
				if r.Header.Get("User-Agent") != want {
					t.Errorf("identity = %q", r.Header.Get("User-Agent"))
				}
				w.Header().Set("Content-Type", "application/json")
				token := "header." + base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"fake-account"}}`)) + ".signature"
				_, _ = fmt.Fprintf(w, `{"access_token":%q,"refresh_token":"fake-refresh","expires_in":3600,"token":"fake-copilot","expires_at":9999999999,"device_code":"device","user_code":"code","verification_uri":"http://example.test","interval":1}`, token)
			}))
			defer srv.Close()
			a := NewOAuthClient()
			a.TokenURL = srv.URL
			a.UserAgent = identity
			if _, err := a.ExchangeCode("code", "verifier"); err != nil {
				t.Fatal(err)
			}
			if _, err := a.RefreshToken("refresh"); err != nil {
				t.Fatal(err)
			}
			o := NewOpenAIOAuthClient()
			o.TokenURL = srv.URL
			o.UserAgent = identity
			if _, err := o.ExchangeCodeContext(t.Context(), "code", "verifier"); err != nil {
				t.Fatal(err)
			}
			if _, err := o.RefreshToken("refresh"); err != nil {
				t.Fatal(err)
			}
			c := NewCopilotOAuthClient()
			c.DeviceURL = srv.URL
			c.TokenURL = srv.URL
			c.CopilotURL = srv.URL
			c.UserAgent = identity
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			code, err := c.StartDeviceFlow(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.PollDeviceToken(ctx, code); err != nil {
				t.Fatal(err)
			}
			if _, err := c.ExchangeGitHubToken(ctx, "fake"); err != nil {
				t.Fatal(err)
			}
			if seen != 7 {
				t.Fatalf("requests = %d", seen)
			}
		})
	}
}

func TestCredentialManagerIdentityCopies(t *testing.T) {
	cm := &CredentialManager{credentialsPath: "temporary-credentials"}
	a := cm.WithUserAgent("A")
	b := cm.WithUserAgent("B")
	if cm.userAgent != "" || a.userAgent != "A" || b.userAgent != "B" || a.credentialsPath != cm.credentialsPath || b.credentialsPath != cm.credentialsPath {
		t.Fatal("manager identity not isolated")
	}
}

func TestLoginRejectsInvalidIdentity(t *testing.T) {
	called := false
	err := LoginProvider(t.Context(), "openai", LoginOptions{UserAgent: "bad\nvalue", OnAuthorization: func(context.Context, LoginInstructions) error { called = true; return nil }})
	if err == nil || called {
		t.Fatal("invalid identity reached authorization")
	}
}
