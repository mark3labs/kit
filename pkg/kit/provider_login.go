package kit

import (
	"context"
	"fmt"

	"github.com/mark3labs/kit/internal/auth"
)

// LoginInstructions contains the information needed to authorize a provider.
// The host application must present URL to the user or open it in a browser.
type LoginInstructions struct {
	ProviderID  string `json:"provider_id,omitempty"`
	URL         string `json:"url,omitempty"`
	RedirectURI string `json:"redirect_uri,omitempty"`
}

// LoginOptions configures a provider OAuth login.
type LoginOptions struct {
	// CredentialManager selects the credential store. If nil, login uses the
	// standard store at $XDG_CONFIG_HOME/.kit/credentials.json, or
	// ~/.config/.kit/credentials.json when XDG_CONFIG_HOME is not set.
	CredentialManager *CredentialManager

	// OnAuthorization is required. It receives the authorization URL after
	// the local callback listener is ready. The host handles browser opening
	// or URL display; the SDK does neither. Return an error to stop login.
	// The callback must respect its context and return when it is canceled.
	OnAuthorization func(context.Context, LoginInstructions) error
}

// LoginProvider authorizes a provider with OAuth and stores its credentials.
// Only providerID "openai" is supported. This is ChatGPT/Codex subscription
// authentication, not OAuth for the general OpenAI API. Anthropic and Copilot
// login are not yet supported by this function.
//
// OpenAI login requires a fixed loopback listener at 127.0.0.1:1455 and uses
// the redirect URI http://localhost:1455/auth/callback. The browser must be
// able to reach this listener. There is no manual code-entry fallback; login
// fails if the port is not available. On a remote or headless host, arrange
// loopback port forwarding from the browser machine to the login host before
// login. Without a reachable callback, the flow ends at its deadline.
//
// The flow has a maximum timeout of five minutes, or the caller's earlier
// deadline. OnAuthorization must respect the supplied context. Cancellation
// and callback errors are returned to the caller.
//
// Credentials are stored with the manager in options, or the standard store
// when no manager is supplied. The credential file has mode 0600 and must
// remain writable so normal provider token refresh can persist updates.
// Normal Kit construction never starts this interactive login automatically.
// An existing Kit instance does not reload its
// provider after login. Call SetModel or create a new Kit instance to use the
// stored credentials.
func LoginProvider(ctx context.Context, providerID string, options LoginOptions) error {
	internalOptions := auth.LoginOptions{CredentialManager: options.CredentialManager}
	if options.OnAuthorization != nil {
		internalOptions.OnAuthorization = func(ctx context.Context, instructions auth.LoginInstructions) error {
			return options.OnAuthorization(ctx, LoginInstructions{
				ProviderID:  instructions.ProviderID,
				URL:         instructions.URL,
				RedirectURI: instructions.RedirectURI,
			})
		}
	}
	if err := auth.LoginProvider(ctx, providerID, internalOptions); err != nil {
		return fmt.Errorf("login provider %q: %w", providerID, err)
	}
	return nil
}
