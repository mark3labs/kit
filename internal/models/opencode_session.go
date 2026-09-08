package models

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
)

// opencodeSessionHeader is the header the opencode zen API uses to keep a
// conversation sticky to one upstream deployment and to reuse cached prompts
// across requests (see https://opencode.ai/docs/go/). The opencode client
// sends it for every provider whose ID starts with "opencode" (this covers
// both opencode and opencode-go). The fantasy SDK does not add this header,
// so Kit injects it here.
const opencodeSessionHeader = "x-opencode-session"

// isOpencodeZenProvider reports whether the provider talks to the opencode
// zen API and therefore wants the x-opencode-session header. Matches the
// opencode client's own condition (provider ID prefix "opencode"), but
// anchored on "-" so an unrelated provider such as "opencodex" does not
// match.
func isOpencodeZenProvider(id string) bool {
	return id == "opencode" || strings.HasPrefix(id, "opencode-")
}

// processSessionID returns a stable per-process fallback session identifier.
// It is used when no SessionIDFunc is configured or when the func returns ""
// (e.g. --no-session runs). A stable random ID still lets the zen API route
// and cache all requests from this process as one conversation; without the
// header the server falls back to the client IP, which is worse.
var processSessionID = sync.OnceValue(func() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand.Read failing is near-impossible; a fixed marker keeps the
		// header present and the process functional.
		return "kit-unknown-session"
	}
	return hex.EncodeToString(b[:])
})

// sessionHeaderRoundTripper injects a session header into every request. The
// value is resolved at request time (not at provider creation) through the
// ProviderConfig pointer, so a session that is created, loaded, or switched
// after the provider is built is picked up immediately. A header explicitly
// set on the request is not overwritten.
type sessionHeaderRoundTripper struct {
	base   http.RoundTripper
	header string
	config *ProviderConfig
}

func (s *sessionHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if _, exists := req.Header[http.CanonicalHeaderKey(s.header)]; !exists {
		value := ""
		if fn := s.config.SessionIDFunc; fn != nil {
			value = fn()
		}
		if value == "" {
			value = processSessionID()
		}
		req = req.Clone(req.Context())
		req.Header.Set(s.header, value)
	}
	return s.base.RoundTrip(req)
}

// withOpencodeSessionHeader wraps client so every request carries the
// x-opencode-session header when info identifies an opencode zen provider
// (opencode, opencode-go). Returns client unchanged for all other providers.
// A nil client with a matching provider produces a fresh client.
func withOpencodeSessionHeader(client *http.Client, config *ProviderConfig, info *ProviderInfo) *http.Client {
	if info == nil || !isOpencodeZenProvider(info.ID) {
		return client
	}
	if client == nil {
		client = &http.Client{}
	}
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	client.Transport = &sessionHeaderRoundTripper{
		base:   base,
		header: opencodeSessionHeader,
		config: config,
	}
	return client
}
