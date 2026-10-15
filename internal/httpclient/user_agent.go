// Package httpclient provides per-client HTTP identity policy.
package httpclient

import (
	"fmt"
	"net/http"
	"strings"
)

// DefaultUserAgent identifies Kit-controlled requests.
const DefaultUserAgent = "KIT"

// ResolveUserAgent validates identity without trimming valid input.
func ResolveUserAgent(value string) (string, error) {
	if value == "" {
		return DefaultUserAgent, nil
	}
	for _, b := range []byte(value) {
		if b < 32 || b == 127 {
			return "", fmt.Errorf("user agent contains an HTTP control character")
		}
	}
	return value, nil
}

// WithUserAgent copies a client and enforces identity on each outgoing request.
// Callers must validate non-empty values with ResolveUserAgent first.
func WithUserAgent(client *http.Client, value string) *http.Client {
	if value == "" {
		value = DefaultUserAgent
	}
	copy := http.Client{}
	if client != nil {
		copy = *client
	}
	base := copy.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	copy.Transport = &userAgentTransport{base: base, value: value}
	return &copy
}

type userAgentTransport struct {
	base  http.RoundTripper
	value string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if clone.Header == nil {
		clone.Header = make(http.Header)
	}
	for key := range clone.Header {
		if strings.EqualFold(key, "User-Agent") {
			delete(clone.Header, key)
		}
	}
	clone.Header.Set("User-Agent", t.value)
	return t.base.RoundTrip(clone)
}

func (t *userAgentTransport) CloseIdleConnections() {
	if closer, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}
