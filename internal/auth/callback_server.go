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
	listeners []net.Listener
}

// Close closes the listener and all active connections.
func (cs *CallbackServer) Close() {
	if cs.Server != nil {
		_ = cs.Server.Close()
	}
	for _, listener := range cs.listeners {
		_ = listener.Close()
	}
}

// StartOpenAICallbackServer binds the provider-required loopback port 1455.
// State is validated before either codes or provider errors are accepted.
func StartOpenAICallbackServer(expectedState string) (*CallbackServer, error) {
	if expectedState == "" {
		return nil, errors.New("OAuth state must not be empty")
	}
	var listeners []net.Listener
	var bindErrs []error
	for _, address := range []string{"127.0.0.1:1455", "[::1]:1455"} {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			bindErrs = append(bindErrs, err)
			continue
		}
		listeners = append(listeners, listener)
	}
	if len(listeners) == 0 {
		return nil, fmt.Errorf("bind OAuth callback port 1455: %w", errors.Join(bindErrs...))
	}
	cs := &CallbackServer{CodeChan: make(chan string, 1), ErrorChan: make(chan error, 1), State: expectedState, listeners: listeners}
	mux := http.NewServeMux()
	cs.Server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 10 * time.Second}
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
	// Serve owns the already-bound listeners; there is no close/rebind race.
	for _, listener := range listeners {
		go func() {
			if err := cs.Server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				fail(fmt.Errorf("OAuth callback server failed: %w", err))
			}
		}()
	}
	return cs, nil
}
