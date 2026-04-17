package kit

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

// CLIMCPAuthHandler is the MCP OAuth handler for CLI/TUI consumers. It wraps
// a [DefaultMCPAuthHandler] and layers standard CLI behavior on top of the
// underlying transport mechanics:
//
//   - Opens the authorization URL in the system browser
//   - Prints status messages (or routes them to a TUI via [NotifyFunc])
//
// Non-CLI consumers (web apps, daemons, custom TUIs) should not use this
// handler; implement [MCPAuthHandler] directly or configure a
// [DefaultMCPAuthHandler] with a custom OnAuthURL instead.
type CLIMCPAuthHandler struct {
	inner *DefaultMCPAuthHandler
	w     io.Writer

	// NotifyFunc, when set, is called with status messages instead of
	// writing to the writer. This allows the TUI to display system
	// messages in the chat stream. If nil, messages are written to w.
	NotifyFunc func(serverName, message string)
}

// NewCLIMCPAuthHandler creates a CLI auth handler that prints status messages
// to stderr, opens the authorization URL in the system browser, and delegates
// the callback-server mechanics to a [DefaultMCPAuthHandler].
func NewCLIMCPAuthHandler() (*CLIMCPAuthHandler, error) {
	inner, err := NewDefaultMCPAuthHandler()
	if err != nil {
		return nil, err
	}
	h := &CLIMCPAuthHandler{inner: inner, w: os.Stderr}
	// Wire the CLI presentation policy into the inner handler's hook.
	// This is the one place in the codebase where OAuth triggers a
	// browser open; the SDK core remains I/O-free.
	inner.OnAuthURL = func(serverName, authURL string) {
		h.notify(serverName, fmt.Sprintf("🔐 MCP server %q requires authentication. Opening browser...", serverName))
		h.notify(serverName, fmt.Sprintf("   If the browser doesn't open, visit:\n   %s", authURL))
		// Browser open is best-effort; the user can still navigate manually.
		_ = openBrowser(authURL)
	}
	return h, nil
}

// RedirectURI returns the OAuth redirect URI from the inner handler.
func (h *CLIMCPAuthHandler) RedirectURI() string {
	return h.inner.RedirectURI()
}

// HandleAuth delegates to the inner handler (which invokes OnAuthURL, runs
// the callback server, and returns the full callback URL) and emits a final
// success or failure notification.
func (h *CLIMCPAuthHandler) HandleAuth(ctx context.Context, serverName string, authURL string) (string, error) {
	callbackURL, err := h.inner.HandleAuth(ctx, serverName, authURL)
	if err != nil {
		h.notify(serverName, fmt.Sprintf("✗ Authentication failed for %q: %v", serverName, err))
		return "", err
	}
	h.notify(serverName, fmt.Sprintf("✓ Authenticated with %q", serverName))
	return callbackURL, nil
}

// Close releases the inner handler's resources.
func (h *CLIMCPAuthHandler) Close() error {
	return h.inner.Close()
}

// notify sends a message through NotifyFunc if set, otherwise writes to w.
func (h *CLIMCPAuthHandler) notify(serverName, message string) {
	if h.NotifyFunc != nil {
		h.NotifyFunc(serverName, message)
		return
	}
	_, _ = fmt.Fprintln(h.w, message)
}

// openBrowser opens the system default browser at url. Intentionally
// unexported: browser opening is CLI policy, not SDK surface. Consumers
// that need similar behavior for their own UX should bring their own
// helper (or use a third-party package like github.com/pkg/browser).
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
