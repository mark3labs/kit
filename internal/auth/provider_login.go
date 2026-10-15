package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mark3labs/kit/internal/httpclient"
)

// LoginInstructions describes the browser authorization step.
type LoginInstructions struct {
	ProviderID  string
	URL         string
	RedirectURI string
}

// LoginOptions configures host interaction and credential storage.
type LoginOptions struct {
	// UserAgent identifies login HTTP requests independently of any Kit instance. Empty means "KIT".
	UserAgent         string
	CredentialManager *CredentialManager
	OnAuthorization   func(context.Context, LoginInstructions) error
}

// LoginProvider runs the provider login without browser or terminal side effects.
func LoginProvider(ctx context.Context, providerID string, options LoginOptions) error {
	return loginProvider(ctx, providerID, options, NewOpenAIOAuthClient())
}

// loginProvider coordinates authorization, exchange, and persistent credential storage.
// The client is private so SDK callers cannot redirect credentials to another endpoint.
func loginProvider(ctx context.Context, providerID string, options LoginOptions, client *OpenAIOAuthClient) error {
	identity, err := httpclient.ResolveUserAgent(options.UserAgent)
	if err != nil {
		return err
	}
	clientCopy := *client
	client = &clientCopy
	client.UserAgent = identity
	if ctx == nil {
		return errors.New("login context is required")
	}
	if providerID != "openai" {
		return fmt.Errorf("OAuth login is not supported for provider %q", providerID)
	}
	if options.OnAuthorization == nil {
		return errors.New("authorization callback is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("start login: %w", err)
	}
	cm := options.CredentialManager
	if cm == nil {
		var err error
		cm, err = NewCredentialManager()
		if err != nil {
			return fmt.Errorf("open credential store: %w", err)
		}
	}
	data, err := client.GetAuthorizationURL()
	if err != nil {
		return fmt.Errorf("create authorization URL: %w", err)
	}
	server, err := StartOpenAICallbackServer(data.State)
	if err != nil {
		return fmt.Errorf("start callback listener: %w", err)
	}
	defer server.Close()
	// Release the port even while a host callback is handling cancellation.
	stop := context.AfterFunc(ctx, server.Close)
	defer stop()
	if err := options.OnAuthorization(ctx, LoginInstructions{ProviderID: providerID, URL: data.URL, RedirectURI: client.RedirectURI}); err != nil {
		return fmt.Errorf("present authorization instructions: %w", err)
	}
	var code string
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait for authorization: %w", ctx.Err())
	case err := <-server.ErrorChan:
		return fmt.Errorf("receive authorization: %w", err)
	case code = <-server.CodeChan:
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("exchange authorization: %w", err)
	}
	creds, err := client.ExchangeCodeContext(ctx, code, data.Verifier)
	if err != nil {
		return fmt.Errorf("exchange authorization: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	if err := cm.SetOpenAIOAuthCredentials(creds); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	return nil
}
