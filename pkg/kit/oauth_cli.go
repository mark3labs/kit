package kit

import (
	"context"
	"fmt"
	"io"
	"os"
)

// CLIMCPAuthHandler wraps a [DefaultMCPAuthHandler] and prints status messages
// to a writer (typically stderr) so the user knows what's happening during
// OAuth authorization. This is the handler used by the CLI/TUI binary.
//
// For TUI integration, set NotifyFunc to route messages through the TUI's
// event system instead of (or in addition to) the writer.
type CLIMCPAuthHandler struct {
	inner *DefaultMCPAuthHandler
	w     io.Writer

	// NotifyFunc, when set, is called with status messages instead of writing
	// to the writer. This allows the TUI to display system messages in the
	// chat stream. If nil, messages are written to w.
	NotifyFunc func(serverName, message string)
}

// NewCLIMCPAuthHandler creates a CLI auth handler that prints status messages
// to stderr and delegates the actual OAuth flow to a [DefaultMCPAuthHandler].
func NewCLIMCPAuthHandler() (*CLIMCPAuthHandler, error) {
	inner, err := NewDefaultMCPAuthHandler()
	if err != nil {
		return nil, err
	}
	return &CLIMCPAuthHandler{inner: inner, w: os.Stderr}, nil
}

// RedirectURI returns the OAuth redirect URI from the inner handler.
func (h *CLIMCPAuthHandler) RedirectURI() string {
	return h.inner.RedirectURI()
}

// HandleAuth prints status messages and delegates to the inner handler.
func (h *CLIMCPAuthHandler) HandleAuth(ctx context.Context, serverName string, authURL string) (string, error) {
	h.notify(serverName, fmt.Sprintf("🔐 MCP server %q requires authentication. Opening browser...", serverName))
	h.notify(serverName, fmt.Sprintf("   If the browser doesn't open, visit:\n   %s", authURL))

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
