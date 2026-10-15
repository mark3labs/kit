package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type recorder struct {
	call   func(*http.Request) (*http.Response, error)
	closed bool
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) { return r.call(req) }
func (r *recorder) CloseIdleConnections()                               { r.closed = true }

func TestResolveUserAgent(t *testing.T) {
	for _, value := range []string{"", "KIT", "MyApp/1.0", "   ", " product / version "} {
		got, err := ResolveUserAgent(value)
		want := value
		if want == "" {
			want = "KIT"
		}
		if err != nil || got != want {
			t.Fatalf("resolve %q = %q, %v", value, got, err)
		}
	}
	for b := range 128 {
		if b >= 32 && b != 127 {
			continue
		}
		if _, err := ResolveUserAgent("x" + string(rune(b))); err == nil {
			t.Errorf("accepted control %d", b)
		}
	}
}

func TestClientPolicyAndImmutability(t *testing.T) {
	sentinel := errors.New("transport failure")
	body := io.NopCloser(strings.NewReader("stream"))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://example.test/", body)
	req.Header = http.Header{"user-agent": {"conflict"}, "USER-AGENT": {"other"}, "Authorization": {"Bearer fake"}, "Content-Type": {"application/json"}}
	req.GetBody = func() (io.ReadCloser, error) { return http.NoBody, nil }
	base := &recorder{call: func(got *http.Request) (*http.Response, error) {
		if got == req || got.Header.Get("User-Agent") != " exact / 1 " || len(got.Header) != 3 {
			t.Errorf("headers = %v", got.Header)
		}
		if got.Context() != ctx || got.Body != body || got.GetBody == nil || got.Method != req.Method || got.URL.String() != req.URL.String() {
			t.Error("request properties changed")
		}
		if got.Header.Get("Authorization") != "Bearer fake" || got.Header.Get("Content-Type") != "application/json" {
			t.Error("lost headers")
		}
		return nil, sentinel
	}}
	jar, _ := cookiejar.New(nil)
	redirect := func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	original := &http.Client{Transport: base, Timeout: time.Second, Jar: jar, CheckRedirect: redirect}
	client := WithUserAgent(original, " exact / 1 ")
	if client == original || client.Timeout != original.Timeout || client.Jar != jar || client.CheckRedirect == nil || original.Transport != base {
		t.Fatal("client not copied")
	}
	if _, err := client.Transport.RoundTrip(req); !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
	// These deliberately noncanonical keys must survive on the caller request.
	originalHeaders := map[string][]string(req.Header)
	if originalHeaders["user-agent"][0] != "conflict" || originalHeaders["USER-AGENT"][0] != "other" {
		t.Fatal("mutated original headers")
	}
	client.CloseIdleConnections()
	if !base.closed {
		t.Fatal("idle cleanup not forwarded")
	}
	data, _ := io.ReadAll(body)
	if string(data) != "stream" {
		t.Fatal("body pre-read")
	}
}

func TestRedirectAndCancellation(t *testing.T) {
	seen := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "KIT" {
			t.Errorf("identity = %q", r.Header.Get("User-Agent"))
		}
		seen++
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/next", http.StatusFound)
		}
	}))
	defer server.Close()
	client := WithUserAgent(&http.Client{Timeout: time.Second}, "")
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if seen != 2 {
		t.Fatalf("requests = %d", seen)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if _, err := client.Do(req); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}
