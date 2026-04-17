package kit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// MCPAuthHandler handles OAuth authorization for MCP servers.
// Implementations control the user experience — opening a browser, showing a
// prompt, displaying a URL, posting to a message bus, etc.
//
// [DefaultMCPAuthHandler] provides the transport mechanics (port reservation
// and callback server) but performs no user-facing I/O on its own; consumers
// wire presentation via [DefaultMCPAuthHandler.OnAuthURL] or implement
// MCPAuthHandler from scratch.
type MCPAuthHandler interface {
	// RedirectURI returns the OAuth redirect URI that the callback server
	// will listen on. This is called during MCP transport setup — before any
	// OAuth errors occur — so the redirect URI can be registered with the
	// authorization server.
	RedirectURI() string

	// HandleAuth is called when an MCP server requires OAuth authorization.
	// It receives the server name and an authorization URL that the user must
	// visit. The handler must:
	//   1. Direct the user to authURL (e.g. open browser, display URL)
	//   2. Listen for the OAuth callback on the redirect URI
	//   3. Return the full callback URL (with code and state query params)
	//
	// Return an error to abort the connection to this MCP server.
	// The context controls the overall timeout; implementations should
	// respect ctx.Done().
	HandleAuth(ctx context.Context, serverName string, authURL string) (callbackURL string, err error)
}

// DefaultMCPAuthHandler provides the transport mechanics of an OAuth flow —
// reserving a local TCP port and running a one-shot HTTP callback server —
// without making any user-experience decisions. It performs no browser opens,
// no printing, no TUI calls; consumers attach presentation by setting
// [DefaultMCPAuthHandler.OnAuthURL] or by wrapping the handler.
//
// The handler eagerly reserves a TCP port on construction so [RedirectURI] is
// stable for the lifetime of the handler. Create instances with
// [NewDefaultMCPAuthHandler] (random port) or [NewDefaultMCPAuthHandlerWithPort]
// (explicit port). Always call [DefaultMCPAuthHandler.Close] when done to
// release the port.
type DefaultMCPAuthHandler struct {
	listener net.Listener
	port     int
	mu       sync.Mutex // guards listener lifecycle

	// OnAuthURL, if set, is invoked exactly once per [HandleAuth] call with
	// the authorization URL the user must visit. This is where consumers
	// plug in their UX: open a browser, print to stderr, post to a TUI
	// stream, render a QR code, etc. The handler performs no I/O on the
	// URL itself; if OnAuthURL is nil the URL is silently dropped and the
	// user has no way to complete the flow.
	//
	// OnAuthURL is called synchronously before the handler blocks on the
	// callback. It must not block indefinitely — long-running work should
	// be dispatched to a goroutine.
	OnAuthURL func(serverName, authURL string)
}

// NewDefaultMCPAuthHandler creates a handler that listens on a random
// available port on localhost. The port is reserved immediately so
// [RedirectURI] returns a stable value. Call [DefaultMCPAuthHandler.Close]
// when the handler is no longer needed to release the port.
//
// The returned handler has no OnAuthURL hook configured and will therefore
// appear to hang on HandleAuth until the context deadline fires. Set
// OnAuthURL before using the handler, or use a higher-level wrapper such
// as [CLIMCPAuthHandler].
func NewDefaultMCPAuthHandler() (*DefaultMCPAuthHandler, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen for OAuth callback: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	return &DefaultMCPAuthHandler{listener: listener, port: port}, nil
}

// NewDefaultMCPAuthHandlerWithPort creates a handler that listens on the
// specified port on localhost. The port is reserved immediately. Pass 0 to
// let the OS pick a free port (equivalent to [NewDefaultMCPAuthHandler]).
// Call [DefaultMCPAuthHandler.Close] when the handler is no longer needed.
func NewDefaultMCPAuthHandlerWithPort(port int) (*DefaultMCPAuthHandler, error) {
	addr := fmt.Sprintf("localhost:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s for OAuth callback: %w", addr, err)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	return &DefaultMCPAuthHandler{listener: listener, port: actualPort}, nil
}

// RedirectURI returns the OAuth redirect URI pointing to the local callback
// server. This value is stable for the lifetime of the handler.
func (h *DefaultMCPAuthHandler) RedirectURI() string {
	return fmt.Sprintf("http://localhost:%d/oauth/callback", h.port)
}

// Port returns the TCP port the callback server is bound to.
func (h *DefaultMCPAuthHandler) Port() int {
	return h.port
}

// HandleAuth invokes [OnAuthURL] with the authorization URL (if configured)
// and waits for the OAuth callback on the local server. It returns the full
// callback URL including query parameters (code, state, etc.).
//
// If the context has no deadline, a default 2-minute timeout is applied.
// The callback server is started for each HandleAuth call and shut down
// before returning.
func (h *DefaultMCPAuthHandler) HandleAuth(ctx context.Context, serverName string, authURL string) (string, error) {
	h.mu.Lock()
	listener := h.listener
	h.mu.Unlock()

	if listener == nil {
		return "", fmt.Errorf("OAuth callback handler is closed")
	}

	// Apply default timeout if the context has no deadline.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
	}

	// Channel receives the full callback URL from the HTTP handler.
	callbackCh := make(chan string, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		// Reconstruct the full callback URL as the caller expects it.
		fullURL := fmt.Sprintf("http://localhost:%d%s", h.port, r.RequestURI)

		// Send the callback URL to the waiting goroutine (non-blocking).
		select {
		case callbackCh <- fullURL:
		default:
		}

		// Respond with a friendly HTML page so the user knows they can
		// close the browser tab.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, oauthSuccessHTML)
	})

	server := &http.Server{
		Handler: mux,
	}

	// Start serving on the pre-reserved listener. http.Server.Serve takes
	// ownership and closes the listener when Shutdown is called, so we
	// re-acquire a fresh listener on the same port in the deferred cleanup
	// below to keep the port reserved for subsequent HandleAuth calls.
	h.mu.Lock()
	serveListener := h.listener
	h.listener = nil
	h.mu.Unlock()

	if serveListener == nil {
		return "", fmt.Errorf("OAuth callback handler is closed")
	}

	// Start the HTTP server in a background goroutine.
	serverErrCh := make(chan error, 1)
	go func() {
		err := server.Serve(serveListener)
		if err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	// Re-acquire the listener after Serve completes (deferred).
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)

		// Re-reserve the port for future HandleAuth calls.
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.listener == nil {
			newListener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", h.port))
			if err == nil {
				h.listener = newListener
			}
			// If re-listen fails, the handler degrades gracefully — the
			// next HandleAuth call will return an error.
		}
	}()

	// Surface the authorization URL to the consumer. This is the single
	// presentation seam: the SDK itself does not open browsers, print,
	// or otherwise touch the user's environment.
	if h.OnAuthURL != nil {
		h.OnAuthURL(serverName, authURL)
	}

	// Wait for the callback, a server error, or context cancellation.
	select {
	case url := <-callbackCh:
		return url, nil
	case err := <-serverErrCh:
		return "", fmt.Errorf("OAuth callback server error for %q: %w", serverName, err)
	case <-ctx.Done():
		return "", fmt.Errorf("OAuth authorization timed out for %q: %w", serverName, ctx.Err())
	}
}

// Close releases the reserved port and shuts down the handler. After Close,
// HandleAuth will return an error. Close is safe to call multiple times.
func (h *DefaultMCPAuthHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.listener != nil {
		err := h.listener.Close()
		h.listener = nil
		return err
	}
	return nil
}

// oauthSuccessHTML is the HTML page returned to the browser after a
// successful OAuth callback.
const oauthSuccessHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Authorization Successful</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      display: flex;
      justify-content: center;
      align-items: center;
      min-height: 100vh;
      margin: 0;
      background: #f8f9fa;
      color: #333;
    }
    .container {
      text-align: center;
      padding: 2rem;
    }
    h1 { color: #22863a; }
    p { color: #586069; margin-top: 0.5rem; }
  </style>
</head>
<body>
  <div class="container">
    <h1>&#10003; Authorization Successful</h1>
    <p>You can close this tab and return to the terminal.</p>
  </div>
</body>
</html>`
