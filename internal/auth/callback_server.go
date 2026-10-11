package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// CallbackServer receives an OAuth callback on a loopback listener.
type CallbackServer struct {
	Server    *http.Server
	CodeChan  chan string
	ErrorChan chan error
	State     string
	listener  net.Listener
}

// Close closes the listener and all active connections.
func (cs *CallbackServer) Close() {
	if cs.Server != nil {
		_ = cs.Server.Close()
	}
	if cs.listener != nil {
		_ = cs.listener.Close()
	}
}

// StartOpenAICallbackServer binds the provider-required loopback port 1455.
// State is validated before either codes or provider errors are accepted.
func StartOpenAICallbackServer(expectedState string) (*CallbackServer, error) {
	if expectedState == "" {
		return nil, errors.New("OAuth state must not be empty")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:1455")
	if err != nil {
		return nil, fmt.Errorf("bind OAuth callback port 1455: %w", err)
	}
	cs := &CallbackServer{CodeChan: make(chan string, 1), ErrorChan: make(chan error, 1), State: expectedState}
	mux := http.NewServeMux()
	cs.Server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	var once sync.Once
	fail := func(err error) { once.Do(func() { cs.ErrorChan <- err }) }
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(expectedState)) != 1 {
			http.Error(w, "State mismatch", http.StatusBadRequest)
			fail(errors.New("OAuth callback state mismatch"))
			return
		}
		if q.Get("error") != "" {
			http.Error(w, "Authorization denied", http.StatusBadRequest)
			fail(errors.New("OAuth authorization denied"))
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "Missing authorization code", http.StatusBadRequest)
			fail(errors.New("OAuth callback missing authorization code"))
			return
		}
		once.Do(func() { cs.CodeChan <- code })
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Authentication received. You can close this window."))
	})
	// Serve owns the already-bound listener; there is no close/rebind race.
	go func() {
		if err := cs.Server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fail(fmt.Errorf("OAuth callback server failed: %w", err))
		}
	}()
	// Close must also release the listener if Serve has not started yet.
	cs.listener = listener
	return cs, nil
}
