package kit

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

// redirectTransport sends every request to target and counts the calls.
type redirectTransport struct {
	base   http.RoundTripper
	target *url.URL
	calls  *atomic.Int32
	path   *atomic.Value
}

func (rt redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.calls.Add(1)
	rt.path.Store(req.URL.Host + req.URL.Path)
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host = rt.target.Scheme, rt.target.Host
	return rt.base.RoundTrip(req)
}

func TestKitDecideUsesHTTPTransport(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	var auth atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.Store(r.Header.Get("Authorization"))
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = io.WriteString(w, `{"model":"jev-1.14.0","answers":{"ok":{"type":"noul","noul":0.9}},"usage":{"input_tokens":10,"output_tokens":1}}`)
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)

	var calls atomic.Int32
	var path atomic.Value
	opts := &Options{Model: "custom/test", ProviderURL: "http://unused.test/v1", ProviderAPIKey: "fake", Bare: true}
	WithHTTPTransport(func(base http.RoundTripper) http.RoundTripper {
		return redirectTransport{base: base, target: target, calls: &calls, path: &path}
	})(opts)
	k := newProviderTestKit(t, opts)

	res, err := k.Decide(context.Background(), DecisionRequest{
		Model:     "openrouter/typesafe/jev-1.14",
		State:     DecisionState{Text: "all tests pass"},
		Questions: []DecisionQuestion{BooleanQuestion("ok", "Did the build pass?")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if a, err := res.Boolean("ok"); err != nil || a.Probability != 0.9 {
		t.Errorf("answer = %+v, %v", a, err)
	}
	if calls.Load() != 1 {
		t.Errorf("transport calls = %d, want 1", calls.Load())
	}
	if got := path.Load(); got != "openrouter.ai/api/v1/systemone" {
		t.Errorf("request went to %v, want openrouter.ai/api/v1/systemone", got)
	}
	// The custom/* endpoint overrides of the Kit do not apply to this model.
	if got := auth.Load(); got != "Bearer or-key" {
		t.Errorf("Authorization = %v, want the OpenRouter key", got)
	}
	if k.GetCurrentModel() != "custom/test" {
		t.Errorf("chat model changed to %s", k.GetCurrentModel())
	}
}

func TestDecideErrors(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	ctx := context.Background()
	q := []DecisionQuestion{BooleanQuestion("ok", "ok?")}

	if _, err := Decide(ctx, DecisionRequest{State: DecisionState{Text: "x"}, Questions: q}); !errors.Is(err, ErrInvalidDecisionRequest) {
		t.Errorf("no model: error = %v", err)
	}
	if _, err := Decide(ctx, DecisionRequest{Model: "anthropic/claude-sonnet-4-6", State: DecisionState{Text: "x"}, Questions: q}); !errors.Is(err, ErrNotDecisionModel) {
		t.Errorf("chat model: error = %v", err)
	}
	if _, err := Decide(ctx, DecisionRequest{Model: "openrouter/typesafe/jev-1.14", Questions: q}); !errors.Is(err, ErrInvalidDecisionRequest) {
		t.Errorf("empty state: error = %v", err)
	}
}

func TestGetDecisionModels(t *testing.T) {
	found := false
	for _, m := range GetDecisionModels() {
		if m.Model == "openrouter/typesafe/jev-1.14" {
			found = true
			if m.Wire != "systemone" || !m.AcceptsImages || m.Provider != "openrouter" || m.ID != "typesafe/jev-1.14" {
				t.Errorf("entry = %+v", m)
			}
		}
		if !strings.Contains(m.Model, "/") {
			t.Errorf("model %q is not provider/model", m.Model)
		}
	}
	if !found {
		t.Error("openrouter/typesafe/jev-1.14 is not listed")
	}
}

func TestKitDecideUsesConfiguredDecisionModel(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = io.WriteString(w, `{"model":"jev-1.14.0","answers":{"ok":{"type":"noul","noul":0.9}},"usage":{"input_tokens":10,"output_tokens":1}}`)
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)

	var calls atomic.Int32
	var path atomic.Value
	opts := &Options{Model: "custom/test", ProviderURL: "http://unused.test/v1", ProviderAPIKey: "fake", Bare: true}
	WithDecisionModel("openrouter/typesafe/jev-1.14")(opts)
	WithHTTPTransport(func(base http.RoundTripper) http.RoundTripper {
		return redirectTransport{base: base, target: target, calls: &calls, path: &path}
	})(opts)
	k := newProviderTestKit(t, opts)

	if got := k.GetDecisionModel(); got != "openrouter/typesafe/jev-1.14" {
		t.Fatalf("GetDecisionModel = %q", got)
	}
	req := DecisionRequest{
		State:     DecisionState{Text: "all tests pass"},
		Questions: []DecisionQuestion{BooleanQuestion("ok", "Did the build pass?")},
	}
	if _, err := k.Decide(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got := path.Load(); got != "openrouter.ai/api/v1/systemone" {
		t.Errorf("request went to %v, want openrouter.ai/api/v1/systemone", got)
	}

	// The value is live: clearing it makes Decide fail with no request sent.
	k.v.Set("decision-model", "")
	_, err := k.Decide(context.Background(), req)
	if !errors.Is(err, ErrNoDecisionModel) || !errors.Is(err, ErrInvalidDecisionRequest) {
		t.Errorf("no model: error = %v, want ErrNoDecisionModel and ErrInvalidDecisionRequest", err)
	}
	if calls.Load() != 1 {
		t.Errorf("transport calls = %d, want 1", calls.Load())
	}
}

func TestDecideWithoutModelIsErrNoDecisionModel(t *testing.T) {
	_, err := Decide(context.Background(), DecisionRequest{
		State:     DecisionState{Text: "x"},
		Questions: []DecisionQuestion{BooleanQuestion("ok", "ok?")},
	})
	if !errors.Is(err, ErrNoDecisionModel) {
		t.Errorf("error = %v, want ErrNoDecisionModel", err)
	}
}
