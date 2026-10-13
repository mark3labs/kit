package kit

import "github.com/mark3labs/kit/internal/agent"

// RetryPolicy limits retries for each model request and step resumes for
// each turn.
//
// Request retries: MaxAttempts includes the initial request; 1 disables
// retries. Delays use exponential backoff with equal jitter.
// InitialDelay is capped by MaxDelay. Server retry hints are respected; a hint
// above MaxDelay stops retries rather than sending a request too early.
// MaxElapsed limits retry scheduling, not the duration of an active request.
// Use a context deadline to limit active requests.
// Streaming requests can retry only before content is delivered.
//
// Step resumes: when a streaming step fails with a transient error, also
// after it streamed partial output, the step restarts from the last
// completed step. MaxResumes limits resumes for each turn; a negative value
// disables them. Delays start at ResumeInitialDelay and double up to
// ResumeMaxDelay. The failed step's partial output is discarded (see
// StepResumeEvent).
//
// Zero fields use defaults. Negative fields use defaults too, except
// MaxResumes. Tool execution and completed agent steps are never replayed.
type RetryPolicy = agent.RetryPolicy

// DefaultRetryPolicy returns four total attempts, a one-second initial delay,
// a 30-second maximum delay, a two-minute retry scheduling limit, and five
// step resumes for each turn with delays from 5 to 80 seconds.
func DefaultRetryPolicy() RetryPolicy { return agent.DefaultRetryPolicy() }

// WithRetryPolicy sets the retry limits for model requests. The policy is copied.
func WithRetryPolicy(policy RetryPolicy) Option {
	return func(o *Options) { p := policy; o.RetryPolicy = &p }
}
