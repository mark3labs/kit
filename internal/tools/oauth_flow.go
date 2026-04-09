package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
)

// MCPAuthHandler is the internal interface for handling MCP OAuth flows.
// The SDK-level kit.MCPAuthHandler is adapted to this interface in cmd/root.go
// or pkg/kit/kit.go, keeping the tools package decoupled from the SDK.
type MCPAuthHandler interface {
	// RedirectURI returns the OAuth redirect URI for transport setup.
	RedirectURI() string
	// HandleAuth is called when a server requires OAuth authorization.
	// It receives the server name and the authorization URL the user must visit.
	// It returns the full callback URL (containing code and state query params)
	// after the user completes authorization.
	HandleAuth(ctx context.Context, serverName string, authURL string) (callbackURL string, err error)
}

// TokenStoreFactory creates a transport.TokenStore for a given MCP server URL.
// When provided to the connection pool, it is called once per remote MCP server
// instead of using the default file-based token store. Implementations can
// return any transport.TokenStore — in-memory, database-backed, encrypted, etc.
type TokenStoreFactory func(serverURL string) (transport.TokenStore, error)

// OAuthFlowRunner handles the OAuth authorization flow when an MCP server
// returns an OAuthAuthorizationRequiredError. It coordinates dynamic client
// registration, PKCE generation, user authorization (via MCPAuthHandler),
// and token exchange.
type OAuthFlowRunner struct {
	handler MCPAuthHandler
}

// NewOAuthFlowRunner creates a new OAuthFlowRunner with the given auth handler.
func NewOAuthFlowRunner(handler MCPAuthHandler) *OAuthFlowRunner {
	return &OAuthFlowRunner{handler: handler}
}

// RunAuthFlow executes the OAuth authorization flow for the given server.
// It extracts the OAuthHandler from the error, performs dynamic client registration
// if needed, generates PKCE parameters, delegates to the MCPAuthHandler for user
// interaction, and exchanges the authorization code for a token.
func (r *OAuthFlowRunner) RunAuthFlow(ctx context.Context, serverName string, authErr error) error {
	// Extract the OAuthHandler from the authorization-required error.
	oauthHandler := client.GetOAuthHandler(authErr)
	if oauthHandler == nil {
		return fmt.Errorf("oauth flow: failed to extract OAuth handler from error: %w", authErr)
	}

	// Perform dynamic client registration if no client ID is configured yet.
	if oauthHandler.GetClientID() == "" {
		if err := oauthHandler.RegisterClient(ctx, "kit"); err != nil {
			return fmt.Errorf("oauth flow: dynamic client registration failed: %w", err)
		}
	}

	// Generate PKCE code verifier and challenge.
	codeVerifier, err := client.GenerateCodeVerifier()
	if err != nil {
		return fmt.Errorf("oauth flow: failed to generate code verifier: %w", err)
	}
	codeChallenge := client.GenerateCodeChallenge(codeVerifier)

	// Generate a random state parameter for CSRF protection.
	state, err := client.GenerateState()
	if err != nil {
		return fmt.Errorf("oauth flow: failed to generate state: %w", err)
	}

	// Build the authorization URL the user needs to visit.
	authURL, err := oauthHandler.GetAuthorizationURL(ctx, state, codeChallenge)
	if err != nil {
		return fmt.Errorf("oauth flow: failed to get authorization URL: %w", err)
	}

	// Delegate to the MCPAuthHandler for user-facing authorization (e.g. open
	// browser, wait for redirect). It returns the full callback URL containing
	// the authorization code and state.
	callbackURL, err := r.handler.HandleAuth(ctx, serverName, authURL)
	if err != nil {
		return fmt.Errorf("oauth flow: user authorization failed: %w", err)
	}

	// Parse the callback URL to extract the authorization code and state.
	parsed, err := url.Parse(callbackURL)
	if err != nil {
		return fmt.Errorf("oauth flow: failed to parse callback URL: %w", err)
	}

	code := parsed.Query().Get("code")
	returnedState := parsed.Query().Get("state")

	if code == "" {
		return fmt.Errorf("oauth flow: callback URL missing 'code' parameter")
	}
	if returnedState == "" {
		return fmt.Errorf("oauth flow: callback URL missing 'state' parameter")
	}

	// Exchange the authorization code for an access token.
	if err := oauthHandler.ProcessAuthorizationResponse(ctx, code, returnedState, codeVerifier); err != nil {
		return fmt.Errorf("oauth flow: token exchange failed: %w", err)
	}

	return nil
}

// IsOAuthError returns true if the error is an OAuthAuthorizationRequiredError.
func IsOAuthError(err error) bool {
	return client.IsOAuthAuthorizationRequiredError(err)
}
