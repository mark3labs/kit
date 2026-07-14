package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// CallbackServer holds the local HTTP server and channel used to receive an
// OAuth authorization-code callback from the browser.
type CallbackServer struct {
	Server   *http.Server
	CodeChan chan string
	State    string
}

// Close shuts down the callback server.
func (cs *CallbackServer) Close() {
	if cs.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = cs.Server.Shutdown(ctx)
	}
}

// StartOpenAICallbackServer starts a local HTTP server on 127.0.0.1:1455 to
// receive the OpenAI OAuth callback. The received authorization code is sent
// on CodeChan after the state parameter is validated against expectedState.
func StartOpenAICallbackServer(expectedState string) (*CallbackServer, error) {
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

	return &CallbackServer{
		Server:   server,
		CodeChan: codeChan,
		State:    expectedState,
	}, nil
}
