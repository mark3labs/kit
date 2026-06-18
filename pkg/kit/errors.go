package kit

import (
	"errors"
	"strings"
)

// Provider-error sentinels. Provider and turn execution paths wrap these via
// fmt.Errorf("%w: %s", …) so embedders can classify failures with errors.Is
// instead of brittle string matching. Use [ClassifyProviderError] to map an
// arbitrary provider error to one of these sentinels.
var (
	// ErrContextOverflow indicates the request exceeded the model's maximum
	// context window. Embedders typically respond by compacting and retrying.
	ErrContextOverflow = errors.New("context window exceeded")

	// ErrRateLimit indicates the provider throttled the request. Embedders
	// typically respond by backing off and retrying.
	ErrRateLimit = errors.New("rate limited by provider")

	// ErrAuth indicates a credential / authorization failure.
	ErrAuth = errors.New("provider authentication failed")

	// ErrProviderUnavailable indicates a transient provider/upstream failure
	// (5xx, network error, timeout).
	ErrProviderUnavailable = errors.New("provider unavailable")

	// ErrInvalidRequest indicates the request was structurally invalid and
	// retrying will not help.
	ErrInvalidRequest = errors.New("invalid request to provider")
)

// ClassifyProviderError inspects err and returns it wrapped with the matching
// provider-error sentinel ([ErrContextOverflow], [ErrRateLimit], [ErrAuth],
// [ErrProviderUnavailable], or [ErrInvalidRequest]) when the underlying cause
// can be recognized. The returned error satisfies errors.Is against both the
// sentinel and the original cause, so the full chain stays inspectable.
//
// When err is nil it returns nil. When the cause cannot be classified the
// original err is returned unchanged so callers never lose information.
//
// Classification is heuristic: it first honors any sentinel already present in
// the chain (so double-classification is idempotent), then falls back to
// matching common provider status codes and phrases in the error text.
func ClassifyProviderError(err error) error {
	if err == nil {
		return nil
	}
	// Already classified — keep as-is so the call is idempotent.
	for _, sentinel := range []error{
		ErrContextOverflow, ErrRateLimit, ErrAuth,
		ErrProviderUnavailable, ErrInvalidRequest,
	} {
		if errors.Is(err, sentinel) {
			return err
		}
	}

	if sentinel := classifyProviderErrorText(err.Error()); sentinel != nil {
		return wrapSentinel(sentinel, err)
	}
	return err
}

// wrapSentinel returns an error that satisfies errors.Is(_, sentinel) while
// keeping the original cause inspectable via errors.Is.
func wrapSentinel(sentinel, cause error) error {
	return &sentinelError{sentinel: sentinel, cause: cause}
}

type sentinelError struct {
	sentinel error
	cause    error
}

func (e *sentinelError) Error() string {
	return e.sentinel.Error() + ": " + e.cause.Error()
}

// Unwrap returns both the sentinel and the cause so errors.Is matches the
// sentinel and the underlying error chain stays reachable.
func (e *sentinelError) Unwrap() []error {
	return []error{e.sentinel, e.cause}
}

// classifyProviderErrorText returns the sentinel matching common provider
// error phrasings, or nil if none match.
func classifyProviderErrorText(msg string) error {
	m := strings.ToLower(msg)
	switch {
	case containsAny(m, "context_length_exceeded", "context window", "maximum context length", "too many tokens", "prompt is too long"):
		return ErrContextOverflow
	case containsAny(m, "rate limit", "rate_limit", "too many requests", "status 429", "429"):
		return ErrRateLimit
	case containsAny(m, "unauthorized", "authentication", "invalid api key", "invalid_api_key", "permission denied", "status 401", "status 403", "401", "403"):
		return ErrAuth
	case containsAny(m, "status 500", "status 502", "status 503", "status 504", "internal server error", "bad gateway", "service unavailable", "gateway timeout", "timeout", "connection refused", "no such host", "eof"):
		return ErrProviderUnavailable
	case containsAny(m, "status 400", "invalid request", "bad request", "unprocessable"):
		return ErrInvalidRequest
	default:
		return nil
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
