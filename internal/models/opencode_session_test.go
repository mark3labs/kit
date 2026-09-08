package models

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsOpencodeZenProvider(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"opencode", true},
		{"opencode-go", true},
		{"opencode-anything", true},
		{"opencodex", false},
		{"openai", false},
		{"anthropic", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isOpencodeZenProvider(tt.id); got != tt.want {
			t.Errorf("isOpencodeZenProvider(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

// newHeaderCaptureServer returns a test server that records the
// x-opencode-session header of each request into got.
func newHeaderCaptureServer(got *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = append(*got, r.Header.Get(opencodeSessionHeader))
	}))
}

func TestOpencodeSessionHeader_FromSessionIDFunc(t *testing.T) {
	var got []string
	srv := newHeaderCaptureServer(&got)
	defer srv.Close()

	config := &ProviderConfig{SessionIDFunc: func() string { return "sess-123" }}
	info := &ProviderInfo{ID: "opencode-go"}
	client := withOpencodeSessionHeader(nil, config, info)
	if client == nil {
		t.Fatal("expected non-nil client for opencode provider")
	}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if len(got) != 1 || got[0] != "sess-123" {
		t.Fatalf("expected header %q, got %v", "sess-123", got)
	}
}

// TestOpencodeSessionHeader_LateBoundSessionID verifies that the session ID
// is resolved per request through the ProviderConfig pointer, so a
// SessionIDFunc bound after provider creation takes effect (this is how
// Kit.New wires the real session onto an already-built provider).
func TestOpencodeSessionHeader_LateBoundSessionID(t *testing.T) {
	var got []string
	srv := newHeaderCaptureServer(&got)
	defer srv.Close()

	config := &ProviderConfig{} // no SessionIDFunc yet
	info := &ProviderInfo{ID: "opencode"}
	client := withOpencodeSessionHeader(nil, config, info)

	// First request: falls back to the stable per-process ID.
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	// Late-bind the real session supplier, as Kit.New does.
	config.SessionIDFunc = func() string { return "real-session-uuid" }

	resp, err = client.Get(srv.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if len(got) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(got))
	}
	if got[0] == "" {
		t.Error("first request should carry the per-process fallback ID, got empty header")
	}
	if got[0] != processSessionID() {
		t.Errorf("first request header = %q, want process fallback %q", got[0], processSessionID())
	}
	if got[1] != "real-session-uuid" {
		t.Errorf("second request header = %q, want %q (late-bound func must win)", got[1], "real-session-uuid")
	}
}

func TestOpencodeSessionHeader_EmptySessionIDFallsBack(t *testing.T) {
	var got []string
	srv := newHeaderCaptureServer(&got)
	defer srv.Close()

	config := &ProviderConfig{SessionIDFunc: func() string { return "" }}
	info := &ProviderInfo{ID: "opencode"}
	client := withOpencodeSessionHeader(nil, config, info)

	for range 2 {
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		_ = resp.Body.Close()
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(got))
	}
	if got[0] == "" || got[0] != got[1] {
		t.Errorf("fallback ID must be non-empty and stable across requests, got %q then %q", got[0], got[1])
	}
}

func TestOpencodeSessionHeader_ExplicitHeaderNotOverwritten(t *testing.T) {
	var got []string
	srv := newHeaderCaptureServer(&got)
	defer srv.Close()

	config := &ProviderConfig{SessionIDFunc: func() string { return "from-func" }}
	info := &ProviderInfo{ID: "opencode"}
	client := withOpencodeSessionHeader(nil, config, info)

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set(opencodeSessionHeader, "explicit")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if len(got) != 1 || got[0] != "explicit" {
		t.Fatalf("explicitly set header must win, got %v", got)
	}
}

func TestWithOpencodeSessionHeader_NonOpencodeUnchanged(t *testing.T) {
	config := &ProviderConfig{SessionIDFunc: func() string { return "sess" }}

	if client := withOpencodeSessionHeader(nil, config, &ProviderInfo{ID: "openai"}); client != nil {
		t.Error("nil client must stay nil for non-opencode providers")
	}
	if client := withOpencodeSessionHeader(nil, config, nil); client != nil {
		t.Error("nil client must stay nil when info is nil")
	}

	existing := &http.Client{}
	if client := withOpencodeSessionHeader(existing, config, &ProviderInfo{ID: "groq"}); client != existing || client.Transport != nil {
		t.Error("existing client must pass through unwrapped for non-opencode providers")
	}
}

// TestAutoRouteHTTPClient_OpencodeGetsSessionTransport verifies the
// auto-route client builder produces a session-header transport for
// opencode providers even when no other HTTP customization is configured.
func TestAutoRouteHTTPClient_OpencodeGetsSessionTransport(t *testing.T) {
	config := &ProviderConfig{}
	client := autoRouteHTTPClient(config, &ProviderInfo{ID: "opencode-go"})
	if client == nil {
		t.Fatal("expected non-nil client for opencode-go")
	}
	if _, ok := client.Transport.(*sessionHeaderRoundTripper); !ok {
		t.Fatalf("expected *sessionHeaderRoundTripper transport, got %T", client.Transport)
	}

	// Non-opencode provider with no TLS/header config stays nil (SDK default).
	if client := autoRouteHTTPClient(config, &ProviderInfo{ID: "groq"}); client != nil {
		t.Errorf("expected nil client for plain non-opencode provider, got %T", client.Transport)
	}
}

// TestAutoRouteHTTPClient_OpencodeStacksWithDefaultHeaders verifies the
// session transport wraps (not replaces) the configured default headers.
func TestAutoRouteHTTPClient_OpencodeStacksWithDefaultHeaders(t *testing.T) {
	var session, team []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session = append(session, r.Header.Get(opencodeSessionHeader))
		team = append(team, r.Header.Get("X-Team"))
	}))
	defer srv.Close()

	config := &ProviderConfig{SessionIDFunc: func() string { return "sess-9" }}
	info := &ProviderInfo{
		ID:      "opencode",
		Headers: map[string]string{"X-Team": "platform"},
	}
	client := autoRouteHTTPClient(config, info)

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if len(session) != 1 || session[0] != "sess-9" {
		t.Errorf("session header = %v, want [sess-9]", session)
	}
	if len(team) != 1 || team[0] != "platform" {
		t.Errorf("default header = %v, want [platform]", team)
	}
}
