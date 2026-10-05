package kit

import "github.com/mark3labs/kit/internal/agent"

// RetryPolicy limits retries for each model request, not for a whole turn.
// MaxAttempts includes the initial request; 1 disables retries. Zero or negative
// fields use defaults. Delays use exponential backoff with equal jitter.
// InitialDelay is capped by MaxDelay. Server retry hints are respected; a hint
// above MaxDelay stops retries rather than sending a request too early.
// MaxElapsed limits retry scheduling, not the duration of an active request.
// Use a context deadline to limit active requests.
// Streaming requests can retry only before any stream part is delivered.
// Tool execution and completed agent steps are never replayed by this policy.
type RetryPolicy = agent.RetryPolicy

// DefaultRetryPolicy returns four total attempts, a one-second initial delay,
// a 30-second maximum delay, and a two-minute retry scheduling limit.
func DefaultRetryPolicy() RetryPolicy { return agent.DefaultRetryPolicy() }

// WithRetryPolicy sets the retry limits for model requests. The policy is copied.
func WithRetryPolicy(policy RetryPolicy) Option {
	return func(o *Options) { p := policy; o.RetryPolicy = &p }
}
