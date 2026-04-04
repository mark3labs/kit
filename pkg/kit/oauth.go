package kit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// MCPAuthHandler handles OAuth authorization for MCP servers.
// Implementations control the user experience — opening a browser, showing a
// prompt, displaying a URL, etc.
//
// The default implementation ([DefaultMCPAuthHandler]) opens the system browser
// and starts a local HTTP callback server to receive the authorization code.
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

// DefaultMCPAuthHandler opens the system browser and starts a local HTTP
// callback server to receive the OAuth authorization code. It eagerly reserves
// a TCP port on construction so [RedirectURI] is stable for the lifetime of
// the handler.
//
// Create instances with [NewDefaultMCPAuthHandler] (random port) or
// [NewDefaultMCPAuthHandlerWithPort] (explicit port).
type DefaultMCPAuthHandler struct {
	listener net.Listener
	port     int
	mu       sync.Mutex // guards listener lifecycle
}

// NewDefaultMCPAuthHandler creates a handler that listens on a random
// available port on localhost. The port is reserved immediately so
// [RedirectURI] returns a stable value. Call [DefaultMCPAuthHandler.Close]
// when the handler is no longer needed to release the port.
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

// HandleAuth opens the system browser to authURL and waits for the OAuth
// callback on the local server. It returns the full callback URL including
// query parameters (code, state, etc.).
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

	// Start serving on the pre-reserved listener. We need to create a new
	// listener on the same port because http.Server.Serve takes ownership
	// and closes the listener when done. The original listener is kept open
	// to reserve the port; we create a second listener via SO_REUSEADDR
	// semantics (Go's default on most platforms) or, more reliably, we
	// temporarily release and re-acquire.
	//
	// Strategy: use the held listener directly for Serve. After Serve
	// returns (due to Shutdown), re-acquire the listener to keep the port
	// reserved for future HandleAuth calls.
	h.mu.Lock()
	serveListener := h.listener
	h.listener = nil // Serve will close it
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

	// Open the system browser.
	if err := openBrowser(authURL); err != nil {
		// Browser open is best-effort; the user can still navigate manually.
		_ = err
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

// openBrowser opens the default system browser to the given URL. This is a
// best-effort operation — errors are returned but callers typically ignore
// them since the user can navigate manually.
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
