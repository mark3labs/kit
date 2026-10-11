package kit_test

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/kit/pkg/kit"
)

func TestLoginProviderValidation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, providerID := range []string{"anthropic", "copilot", "unknown"} {
		t.Run(providerID, func(t *testing.T) {
			called := false
			err := kit.LoginProvider(context.Background(), providerID, kit.LoginOptions{
				OnAuthorization: func(context.Context, kit.LoginInstructions) error {
					called = true
					return nil
				},
			})
			if err == nil || called {
				t.Fatalf("unsupported provider: err=%v, callback called=%v", err, called)
			}
		})
	}
	if err := kit.LoginProvider(context.Background(), "openai", kit.LoginOptions{}); err == nil {
		t.Fatal("expected an error for missing OnAuthorization")
	}
}

func TestLoginProviderCanceledContext(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := kit.LoginProvider(ctx, "openai", kit.LoginOptions{
		OnAuthorization: func(context.Context, kit.LoginInstructions) error {
			called = true
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("canceled login: err=%v, callback called=%v", err, called)
	}
}

func TestLoginProviderAuthorization(t *testing.T) {
	// The provider requires a fixed port. Do not run these cases in parallel.
	listener, err := net.Listen("tcp", "127.0.0.1:1455")
	if err != nil {
		t.Skipf("OpenAI callback port is in use: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cm, err := kit.NewCredentialManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := cm.SetProviderAPIKey("openai", "existing-key"); err != nil {
		t.Fatal(err)
	}
	// Change the default store to check that the supplied manager is used.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	sentinel := errors.New("host authorization failed")
	for _, stopWithError := range []bool{true, false} {
		name := "cancel"
		if stopWithError {
			name = "callback_error"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			called := false
			err := kit.LoginProvider(ctx, "openai", kit.LoginOptions{
				CredentialManager: cm,
				OnAuthorization: func(flowCtx context.Context, instructions kit.LoginInstructions) error {
					called = true
					if instructions.ProviderID != "openai" || instructions.RedirectURI != "http://localhost:1455/auth/callback" {
						t.Errorf("unexpected instructions: %+v", instructions)
					}
					authorizationURL, err := url.Parse(instructions.URL)
					if err != nil {
						t.Fatalf("parse authorization URL: %v", err)
					}
					if authorizationURL.Scheme != "https" || authorizationURL.Host == "" || authorizationURL.Query().Get("redirect_uri") != instructions.RedirectURI {
						t.Errorf("unexpected authorization URL: %s", instructions.URL)
					}
					deadline, ok := flowCtx.Deadline()
					if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 5*time.Minute {
						t.Errorf("expected a maximum five-minute flow deadline: %v", deadline)
					}
					if stopWithError {
						return sentinel
					}
					cancel()
					return nil
				},
			})
			want := sentinel
			if !stopWithError {
				want = context.Canceled
			}
			if !called || !errors.Is(err, want) {
				t.Fatalf("callback called=%v, expected wrapped %v, got %v", called, want, err)
			}
			creds, err := cm.GetOpenAICredentials()
			if err != nil {
				t.Fatal(err)
			}
			if creds.APIKey != "existing-key" {
				t.Fatal("failed login changed existing credentials")
			}
			if kit.HasOpenAICredentials() {
				t.Fatal("failed login wrote to the default store")
			}
		})
	}
}

func TestLoginProviderBusyPort(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	listener, err := net.Listen("tcp", "127.0.0.1:1455")
	if err != nil {
		t.Skipf("OpenAI callback port is in use: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	}()
	ipv6, ipv6Err := net.Listen("tcp", "[::1]:1455")
	if ipv6Err == nil {
		defer func() { _ = ipv6.Close() }()
	}
	called := false
	err = kit.LoginProvider(context.Background(), "openai", kit.LoginOptions{
		OnAuthorization: func(context.Context, kit.LoginInstructions) error {
			called = true
			return nil
		},
	})
	if err == nil || called || !strings.Contains(err.Error(), "1455") {
		t.Fatalf("busy port: err=%v, callback called=%v", err, called)
	}
}
