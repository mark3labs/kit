package kit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

type tracingTransport struct {
	base  http.RoundTripper
	calls *atomic.Int32
}

func (rt tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.calls.Add(1)
	return rt.base.RoundTrip(req)
}

func TestHTTPTransportInheritedAndRebuilt(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"}}]}\n\ndata: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	var calls atomic.Int32
	opts := &Options{Model: "custom/test", ProviderURL: server.URL + "/v1", ProviderAPIKey: "fake", Bare: true}
	WithHTTPTransport(func(base http.RoundTripper) http.RoundTripper { return tracingTransport{base: base, calls: &calls} })(opts)
	k := newProviderTestKit(t, opts)
	if _, err := k.Prompt(t.Context(), "hello"); err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	if before == 0 {
		t.Fatal("initial provider not wrapped")
	}
	if err := k.SetModel(t.Context(), "custom/other"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Prompt(t.Context(), "again"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() <= before {
		t.Fatal("rebuilt provider not wrapped")
	}
	before = calls.Load()
	if _, err := k.Subagent(t.Context(), SubagentConfig{Prompt: "child", Tools: []Tool{}, NoSession: true}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() <= before {
		t.Fatal("subagent did not inherit wrapper")
	}
}
