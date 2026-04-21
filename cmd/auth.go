package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"charm.land/huh/v2"
	"github.com/mark3labs/kit/internal/auth"
	"github.com/mark3labs/kit/internal/ui"
	kit "github.com/mark3labs/kit/pkg/kit"
	"github.com/spf13/cobra"
)

// authCmd represents the auth command for managing AI provider authentication.
// This command provides subcommands for login, logout, and status checking
// of authentication credentials for various AI providers, with OAuth support
// for providers like Anthropic and OpenAI.
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication credentials for AI providers",
	Long: `Manage authentication credentials for AI providers.

This command allows you to securely authenticate and manage credentials for various AI providers
using OAuth flows. Stored credentials take precedence over environment variables.

Available providers:
  - anthropic: Anthropic Claude API (OAuth)
  - openai:    OpenAI API (OAuth and API key)

Examples:
  kit auth login anthropic
  kit auth login openai
  kit auth logout anthropic
  kit auth status`,
}

// authLoginCmd represents the login subcommand for authenticating with AI providers.
// It handles OAuth flow for supported providers, opening a browser for authentication
// and securely storing the resulting credentials for future use.
var authLoginCmd = &cobra.Command{
	Use:   "login [provider]",
	Short: "Authenticate with an AI provider using OAuth",
	Long: `Authenticate with an AI provider using OAuth flow.

This will open your browser to complete the OAuth authentication process.
Your credentials will be securely stored and will take precedence over 
environment variables when making API calls.

Available providers:
  - anthropic: Anthropic Claude API (OAuth)
  - openai:    OpenAI ChatGPT Plus/Pro (Codex OAuth)

Flags:
  --set-default   Set this provider's default model as the system default

Examples:
  kit auth login anthropic
  kit auth login openai
  kit auth login openai --set-default`,
	Args: cobra.ExactArgs(1),
	RunE: runAuthLogin,
}

// authLogoutCmd represents the logout subcommand for removing stored authentication credentials.
// This command removes stored API keys or OAuth tokens for specified providers,
// requiring the user to authenticate again or use environment variables.
var authLogoutCmd = &cobra.Command{
	Use:   "logout [provider]",
	Short: "Remove stored authentication credentials for a provider",
	Long: `Remove stored authentication credentials for an AI provider.

This will delete the stored API key or OAuth credentials for the specified provider. 
You will need to use environment variables or command-line flags for authentication after logout.

Available providers:
  - anthropic: Anthropic Claude API
  - openai:    OpenAI API

Example:
  kit auth logout anthropic
  kit auth logout openai`,
	Args: cobra.ExactArgs(1),
	RunE: runAuthLogout,
}

// authStatusCmd represents the status subcommand for checking authentication status.
// It displays which providers have stored credentials, their types (OAuth vs API key),
// creation dates, and expiration status without revealing the actual credentials.
var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status for all providers",
	Long: `Show the current authentication status for all supported AI providers.

This command displays which providers have stored credentials and when they were created.
It does not display the actual API keys for security reasons.

Example:
  kit auth status`,
	RunE: runAuthStatus,
}

var (
	loginSetDefault bool
)

// defaultModels maps providers to their recommended default models.
// These are used when --set-default flag is passed to auth login.
var defaultModels = map[string]string{
	"anthropic": "anthropic/claude-sonnet-4-5-20250929",
	"openai":    "openai/gpt-5.4",
}

// setDefaultModelIfRequested sets the default model for the given provider
// if the --set-default flag was provided.
func setDefaultModelIfRequested(provider string) error {
	if !loginSetDefault {
		return nil
	}

	model, ok := defaultModels[provider]
	if !ok {
		return fmt.Errorf("no default model configured for provider: %s", provider)
	}

	if err := ui.SaveModelPreference(model); err != nil {
		return fmt.Errorf("failed to save model preference: %w", err)
	}

	fmt.Printf("\n✓ Set default model to: %s\n", model)
	return nil
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)

	authLoginCmd.Flags().BoolVar(&loginSetDefault, "set-default", false, "Set this provider's default model as the system default after login")
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	provider := strings.ToLower(args[0])

	switch provider {
	case "anthropic":
		return loginAnthropic()
	case "openai":
		return loginOpenAI()
	default:
		return fmt.Errorf("unsupported provider: %s. Available providers: anthropic, openai", provider)
	}
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	provider := strings.ToLower(args[0])

	switch provider {
	case "anthropic":
		return logoutAnthropic()
	case "openai":
		return logoutOpenAI()
	default:
		return fmt.Errorf("unsupported provider: %s. Available providers: anthropic, openai", provider)
	}
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	cm, err := kit.NewCredentialManager()
	if err != nil {
		return fmt.Errorf("failed to initialize credential manager: %w", err)
	}

	fmt.Println("Authentication Status")
	fmt.Println("====================")
	fmt.Printf("Credentials file: %s\n\n", cm.GetCredentialsPath())

	// Check Anthropic credentials
	fmt.Print("Anthropic Claude: ")
	if hasAnthropicCreds, err := cm.HasAnthropicCredentials(); err != nil {
		fmt.Printf("Error checking credentials: %v\n", err)
	} else if hasAnthropicCreds {
		if creds, err := cm.GetAnthropicCredentials(); err != nil {
			fmt.Printf("Error reading credentials: %v\n", err)
		} else {
			authType := "API Key"
			status := "✓ Authenticated"

			if creds.Type == "oauth" {
				authType = "OAuth"
				if creds.IsExpired() {
					status = "⚠️  Token expired (will refresh automatically)"
				} else if creds.NeedsRefresh() {
					status = "⚠️  Token expires soon (will refresh automatically)"
				}
			}

			fmt.Printf("%s (%s, stored %s)\n", status, authType, creds.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	} else {
		fmt.Println("✗ Not authenticated")
		// Check if environment variable is set
		if os.Getenv("ANTHROPIC_API_KEY") != "" {
			fmt.Println("  (ANTHROPIC_API_KEY environment variable is set)")
		}
	}

	// Check OpenAI credentials
	fmt.Print("\nOpenAI: ")
	if hasOpenAICreds, err := cm.HasOpenAICredentials(); err != nil {
		fmt.Printf("Error checking credentials: %v\n", err)
	} else if hasOpenAICreds {
		if creds, err := cm.GetOpenAICredentials(); err != nil {
			fmt.Printf("Error reading credentials: %v\n", err)
		} else {
			authType := "API Key"
			status := "✓ Authenticated"

			if creds.Type == "oauth" {
				authType = "OAuth (ChatGPT/Codex)"
				if creds.IsExpired() {
					status = "⚠️  Token expired (will refresh automatically)"
				} else if creds.NeedsRefresh() {
					status = "⚠️  Token expires soon (will refresh automatically)"
				}
			}

			accountInfo := ""
			if creds.Type == "oauth" && creds.AccountID != "" {
				accountInfo = fmt.Sprintf(" [%s]", creds.AccountID)
			}

			fmt.Printf("%s (%s%s, stored %s)\n", status, authType, accountInfo, creds.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	} else {
		fmt.Println("✗ Not authenticated")
		// Check if environment variable is set
		if os.Getenv("OPENAI_API_KEY") != "" {
			fmt.Println("  (OPENAI_API_KEY environment variable is set)")
		}
	}

	fmt.Println("\nTo authenticate with a provider:")
	fmt.Println("  kit auth login anthropic")
	fmt.Println("  kit auth login openai")

	return nil
}

func loginAnthropic() error {
	cm, err := kit.NewCredentialManager()
	if err != nil {
		return fmt.Errorf("failed to initialize credential manager: %w", err)
	}

	// Check if already authenticated
	if hasAuth, err := cm.HasAnthropicCredentials(); err == nil && hasAuth {
		var reauth bool
		err := huh.NewConfirm().
			Title("You are already authenticated with Anthropic").
			Description("Do you want to re-authenticate?").
			Affirmative("Yes").
			Negative("No").
			Value(&reauth).
			Run()
		if err != nil || !reauth {
			fmt.Println("Authentication cancelled.")
			return nil
		}
	}

	// Create OAuth client
	client := auth.NewOAuthClient()

	// Generate authorization URL
	fmt.Println("🔐 Starting OAuth authentication with Anthropic...")
	authData, err := client.GetAuthorizationURL()
	if err != nil {
		return fmt.Errorf("failed to generate authorization URL: %w", err)
	}

	// Display URL and try to open browser
	fmt.Println("\n📱 Opening your browser for authentication...")
	fmt.Println("If the browser doesn't open automatically, please visit this URL:")
	fmt.Printf("\n%s\n\n", authData.URL)

	// Try to open browser
	auth.TryOpenBrowser(authData.URL)

	// Wait for user to complete OAuth flow
	fmt.Println("After authorizing the application, you'll receive an authorization code.")

	var code string
	err = huh.NewInput().
		Title("Authorization code").
		Description("Paste the code from your browser").
		Value(&code).
		Run()
	if err != nil {
		return fmt.Errorf("failed to read authorization code: %w", err)
	}
	code = strings.TrimSpace(code)

	if code == "" {
		return fmt.Errorf("authorization code cannot be empty")
	}

	// Exchange code for tokens
	fmt.Println("\n🔄 Exchanging authorization code for access token...")
	creds, err := client.ExchangeCode(code, authData.Verifier)
	if err != nil {
		return fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Store the credentials
	if err := cm.SetOAuthCredentials(creds); err != nil {
		return fmt.Errorf("failed to store credentials: %w", err)
	}

	fmt.Println("✅ Successfully authenticated with Anthropic!")
	fmt.Printf("📁 Credentials stored in: %s\n", cm.GetCredentialsPath())
	fmt.Println("\n🎉 Your OAuth credentials will now be used for Anthropic API calls.")
	fmt.Println("💡 You can check your authentication status with: kit auth status")

	// Set default model if requested
	if err := setDefaultModelIfRequested("anthropic"); err != nil {
		return err
	}

	// Remind users how to set this as default if they didn't use --set-default
	if !loginSetDefault {
		fmt.Println("\n💡 To set Anthropic as your default model, run:")
		fmt.Println("   kit auth login anthropic --set-default")
	}

	return nil
}

func logoutAnthropic() error {
	cm, err := kit.NewCredentialManager()
	if err != nil {
		return fmt.Errorf("failed to initialize credential manager: %w", err)
	}

	// Check if authenticated
	hasAuth, err := cm.HasAnthropicCredentials()
	if err != nil {
		return fmt.Errorf("failed to check authentication status: %w", err)
	}

	if !hasAuth {
		fmt.Println("You are not currently authenticated with Anthropic.")
		return nil
	}

	// Confirm logout
	var confirm bool
	err = huh.NewConfirm().
		Title("Remove Anthropic credentials").
		Description("Are you sure you want to remove your stored credentials?").
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()
	if err != nil || !confirm {
		fmt.Println("Logout cancelled.")
		return nil
	}

	// Remove credentials
	if err := cm.RemoveAnthropicCredentials(); err != nil {
		return fmt.Errorf("failed to remove credentials: %w", err)
	}

	fmt.Println("✓ Successfully logged out from Anthropic!")
	fmt.Println("You will need to use environment variables or command-line flags for authentication.")

	return nil
}

func loginOpenAI() error {
	cm, err := kit.NewCredentialManager()
	if err != nil {
		return fmt.Errorf("failed to initialize credential manager: %w", err)
	}

	// Check if already authenticated
	if hasAuth, err := cm.HasOpenAICredentials(); err == nil && hasAuth {
		var reauth bool
		err := huh.NewConfirm().
			Title("You are already authenticated with OpenAI (ChatGPT/Codex)").
			Description("Do you want to re-authenticate?").
			Affirmative("Yes").
			Negative("No").
			Value(&reauth).
			Run()
		if err != nil || !reauth {
			fmt.Println("Authentication cancelled.")
			return nil
		}
	}

	// Create OAuth client
	client := auth.NewOpenAIOAuthClient()

	// Generate authorization URL
	fmt.Println("🔐 Starting OAuth authentication with OpenAI (ChatGPT/Codex)...")
	fmt.Println("This will open your browser to authenticate with your ChatGPT account.")
	fmt.Println()

	authData, err := client.GetAuthorizationURL()
	if err != nil {
		return fmt.Errorf("failed to generate authorization URL: %w", err)
	}

	// Start local callback server
	callbackServer, err := startOpenAICallbackServer(authData.State)
	if err != nil {
		fmt.Printf("⚠️  Could not start local callback server: %v\n", err)
		fmt.Println("Falling back to manual code entry.")
	}
	if callbackServer != nil {
		defer callbackServer.Close()
	}

	// Display URL and try to open browser
	fmt.Println("📱 Opening your browser for authentication...")
	fmt.Println("If the browser doesn't open automatically, please visit this URL:")
	fmt.Printf("\n%s\n\n", authData.URL)

	// Try to open browser
	auth.TryOpenBrowser(authData.URL)

	// Wait for callback or manual input
	var code string
	if callbackServer != nil {
		fmt.Println("Waiting for browser authentication...")
		select {
		case callbackCode := <-callbackServer.CodeChan:
			if callbackCode != "" {
				code = callbackCode
				fmt.Println("✓ Received authorization code from browser callback.")
			}
		case <-time.After(2 * time.Minute):
			fmt.Println("\n⏱️  Timeout waiting for browser callback.")
			callbackServer.Close()
		}
	}

	// If no code from callback, prompt for manual entry
	if code == "" {
		fmt.Println("\nAfter authorizing, paste the callback URL or authorization code below.")
		fmt.Println("(The callback URL will look like: http://localhost:1455/auth/callback?code=...&state=...)")
		fmt.Println()

		var input string
		err = huh.NewInput().
			Title("Callback URL or Code").
			Description("Paste the full callback URL or just the authorization code").
			Value(&input).
			Run()
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(input)

		if input == "" {
			return fmt.Errorf("authorization code cannot be empty")
		}

		// Parse the input (could be full URL or just code)
		parsedCode, parsedState := auth.ParseOpenAIAuthorizationInput(input)
		if parsedCode == "" {
			return fmt.Errorf("could not extract authorization code from input")
		}

		// Validate state if provided
		if parsedState != "" && parsedState != authData.State {
			return fmt.Errorf("state mismatch - possible security issue")
		}
		code = parsedCode
	}

	// Exchange code for tokens
	fmt.Println("\n🔄 Exchanging authorization code for access token...")
	creds, err := client.ExchangeCode(code, authData.Verifier)
	if err != nil {
		return fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Store the credentials
	if err := cm.SetOpenAIOAuthCredentials(creds); err != nil {
		return fmt.Errorf("failed to store credentials: %w", err)
	}

	fmt.Println("✅ Successfully authenticated with OpenAI (ChatGPT/Codex)!")
	fmt.Printf("📁 Credentials stored in: %s\n", cm.GetCredentialsPath())
	fmt.Printf("👤 Account ID: %s\n", creds.AccountID)
	fmt.Println("\n🎉 Your OAuth credentials will now be used for OpenAI API calls.")
	fmt.Println("💡 You can check your authentication status with: kit auth status")

	// Set default model if requested
	if err := setDefaultModelIfRequested("openai"); err != nil {
		return err
	}

	// Remind users how to set this as default if they didn't use --set-default
	if !loginSetDefault {
		fmt.Println("\n💡 To set OpenAI as your default model, run:")
		fmt.Println("   kit auth login openai --set-default")
	}

	return nil
}

// callbackServer holds the HTTP server and channel for receiving the OAuth callback
type callbackServer struct {
	Server   *http.Server
	CodeChan chan string
	State    string
}

// Close shuts down the callback server
func (cs *callbackServer) Close() {
	if cs.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cs.Server.Shutdown(ctx)
	}
}

// startOpenAICallbackServer starts a local HTTP server to receive the OAuth callback
func startOpenAICallbackServer(expectedState string) (*callbackServer, error) {
	codeChan := make(chan string, 1)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    "127.0.0.1:1455",
		Handler: mux,
	}

	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		// Check state
		state := r.URL.Query().Get("state")
		if state != expectedState {
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing authorization code", http.StatusBadRequest)
			return
		}

		// Send code to channel
		select {
		case codeChan <- code:
		default:
		}

		// Return success page
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1>&#10003; Authentication Successful</h1>
<p>You can close this window and return to the terminal.</p>
</body>
</html>`)
	})

	// Try to start server
	listener, err := net.Listen("tcp", "127.0.0.1:1455")
	if err != nil {
		return nil, fmt.Errorf("port 1455 not available: %w", err)
	}
	_ = listener.Close()

	go func() {
		_ = server.ListenAndServe()
	}()

	return &callbackServer{
		Server:   server,
		CodeChan: codeChan,
		State:    expectedState,
	}, nil
}

func logoutOpenAI() error {
	cm, err := kit.NewCredentialManager()
	if err != nil {
		return fmt.Errorf("failed to initialize credential manager: %w", err)
	}

	// Check if authenticated
	hasAuth, err := cm.HasOpenAICredentials()
	if err != nil {
		return fmt.Errorf("failed to check authentication status: %w", err)
	}

	if !hasAuth {
		fmt.Println("You are not currently authenticated with OpenAI.")
		return nil
	}

	// Confirm logout
	var confirm bool
	err = huh.NewConfirm().
		Title("Remove OpenAI credentials").
		Description("Are you sure you want to remove your stored credentials?").
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()
	if err != nil || !confirm {
		fmt.Println("Logout cancelled.")
		return nil
	}

	// Remove credentials
	if err := cm.RemoveOpenAICredentials(); err != nil {
		return fmt.Errorf("failed to remove credentials: %w", err)
	}

	fmt.Println("✓ Successfully logged out from OpenAI!")
	fmt.Println("You will need to use environment variables or command-line flags for authentication.")

	return nil
}
