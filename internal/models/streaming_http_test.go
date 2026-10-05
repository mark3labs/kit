package models

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func streamingTestClient(t *testing.T, idle time.Duration) *http.Client {
	t.Helper()
	transport := newStreamingTransport(false, StreamingHTTPConfig{BodyIdleTimeout: idle})
	t.Cleanup(transport.(*streamingTransport).base.(*http.Transport).CloseIdleConnections)
	return &http.Client{Transport: transport}
}

func TestStreamingHTTPHealthyData(t *testing.T) {
	const idle = 200 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for range 15 {
			if _, err := io.WriteString(w, ": heartbeat\n\n"); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			select {
			case <-time.After(30 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
		}
	}))
	defer server.Close()
	client := streamingTestClient(t, idle)
	if client.Timeout != 0 {
		t.Fatal("streaming client has a total timeout")
	}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()
	start := time.Now()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || time.Since(start) < 2*idle {
		t.Fatal("test did not stream beyond the idle limit")
	}
}

func TestStreamingHTTPStallAndCancel(t *testing.T) {
	for _, mode := range []string{"stall", "cancel", "deadline", "close"} {
		t.Run(mode, func(t *testing.T) {
			serverDone := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				close(serverDone)
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			idle := 100 * time.Millisecond
			if mode != "stall" {
				idle = time.Hour
			}
			if mode == "deadline" {
				var stop context.CancelFunc
				ctx, stop = context.WithTimeout(ctx, 100*time.Millisecond)
				defer stop()
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := streamingTestClient(t, idle).Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("close response body: %v", err)
				}
			}()
			result := make(chan error, 1)
			go func() {
				_, err := resp.Body.Read(make([]byte, 1))
				result <- err
			}()
			switch mode {
			case "cancel":
				cancel()
			case "close":
				if err := resp.Body.Close(); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-result:
				want := context.DeadlineExceeded
				switch mode {
				case "cancel":
					want = context.Canceled
				case "close":
					want = io.ErrClosedPipe
				}
				if !errors.Is(err, want) {
					t.Fatalf("read error = %v, want %v", err, want)
				}
				if mode == "stall" {
					var timeout net.Error
					if !errors.As(err, &timeout) || !timeout.Timeout() {
						t.Fatalf("idle error is not a timeout: %v", err)
					}
				}
			case <-time.After(3 * time.Second):
				t.Fatal("body read did not unblock")
			}
			select {
			case <-serverDone:
			case <-time.After(3 * time.Second):
				t.Fatal("server request was not canceled")
			}
		})
	}
}

type trackedStreamingBody struct {
	io.Reader
	closes atomic.Int32
}

func (b *trackedStreamingBody) Close() error { b.closes.Add(1); return nil }

func TestStreamingHTTPCleanup(t *testing.T) {
	for _, mode := range []string{"eof", "close", "unread"} {
		t.Run(mode, func(t *testing.T) {
			body := &trackedStreamingBody{Reader: http.NoBody}
			var requestContext context.Context
			transport := &streamingTransport{
				idleTimeout: 30 * time.Millisecond,
				base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requestContext = req.Context()
					return &http.Response{StatusCode: 200, Body: body}, nil
				}),
			}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com", nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := transport.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "eof":
				if _, err := io.ReadAll(resp.Body); err != nil {
					t.Fatal(err)
				}
			case "close":
				if err := resp.Body.Close(); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case <-requestContext.Done():
			case <-time.After(time.Second):
				t.Fatal("request context was not released")
			}
			time.Sleep(60 * time.Millisecond)
			wrapped := resp.Body.(*idleResponseBody)
			wrapped.mu.Lock()
			terminalErr := wrapped.err
			wrapped.mu.Unlock()
			if mode == "eof" && terminalErr != io.EOF {
				t.Fatalf("EOF replaced by timer: %v", terminalErr)
			}
			if mode == "close" && terminalErr != io.ErrClosedPipe {
				t.Fatalf("close replaced by timer: %v", terminalErr)
			}
			if mode == "unread" && !errors.Is(terminalErr, context.DeadlineExceeded) {
				t.Fatalf("unread body did not expire: %v", terminalErr)
			}
			if err := resp.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if body.closes.Load() != 1 {
				t.Fatalf("body closed %d times", body.closes.Load())
			}
		})
	}
}

func TestStreamingHTTPConfiguration(t *testing.T) {
	original := http.DefaultTransport.(*http.Transport)
	for _, skipVerify := range []bool{false, true} {
		for _, config := range []StreamingHTTPConfig{
			{},
			{DialTimeout: time.Second, TLSHandshakeTimeout: 2 * time.Second, ResponseHeaderTimeout: 3 * time.Second, BodyIdleTimeout: 4 * time.Second},
			{DialTimeout: -1, TLSHandshakeTimeout: -1, ResponseHeaderTimeout: -1, BodyIdleTimeout: -1},
		} {
			for _, client := range []*http.Client{
				createCodexHTTPClient(skipVerify, config),
				createCopilotHTTPClient("token", 0, skipVerify, config),
			} {
				if client.Timeout != 0 {
					t.Fatal("provider client has a total timeout")
				}
				var streaming *streamingTransport
				switch provider := client.Transport.(type) {
				case *codexTransport:
					streaming = provider.base.(*streamingTransport)
				case *copilotTransport:
					streaming = provider.base.(*streamingTransport)
				}
				base := streaming.base.(*http.Transport)
				if base == original || base.DialContext == nil || base.Proxy == nil {
					t.Fatal("transport did not preserve defaults in a separate clone")
				}
				if base.TLSHandshakeTimeout != streamingLimit(config.TLSHandshakeTimeout, 10*time.Second) ||
					base.ResponseHeaderTimeout != streamingLimit(config.ResponseHeaderTimeout, 120*time.Second) ||
					streaming.idleTimeout != streamingLimit(config.BodyIdleTimeout, 120*time.Second) {
					t.Fatal("configured timeout not applied")
				}
				insecure := base.TLSClientConfig != nil && base.TLSClientConfig.InsecureSkipVerify
				if insecure != skipVerify {
					t.Fatal("TLS verification setting not applied")
				}
			}
		}
	}
}

func TestStreamingHTTPHeaderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	transport := newStreamingTransport(false, StreamingHTTPConfig{ResponseHeaderTimeout: 50 * time.Millisecond})
	defer transport.(*streamingTransport).base.(*http.Transport).CloseIdleConnections()
	client := &http.Client{Transport: transport}
	resp, err := client.Get(server.URL)
	if resp != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("expected response-header timeout, got %v", err)
	}
}
