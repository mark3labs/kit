package agent

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"charm.land/fantasy"
)

// RetryPolicy limits retries for each model request. MaxAttempts includes the
// initial request. Zero delays use defaults; MaxAttempts=1 disables retries.
// MaxElapsed bounds retry scheduling, not the duration of an active request.
// The caller's context bounds active requests.
//
// MaxResumes limits step resumes for each turn. A resume restarts a failed
// streaming step from the last completed step after a transient error, also
// when the step already sent partial output. Zero uses the default; a negative
// value disables resumes. Resume delays start at ResumeInitialDelay and double
// up to ResumeMaxDelay.
type RetryPolicy struct {
	MaxAttempts        int
	InitialDelay       time.Duration
	MaxDelay           time.Duration
	MaxElapsed         time.Duration
	MaxResumes         int
	ResumeInitialDelay time.Duration
	ResumeMaxDelay     time.Duration
}

// DefaultRetryPolicy returns bounded defaults with three retries and five
// step resumes for each turn (about 5, 10, 20, 40 and 80 seconds apart).
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 4, InitialDelay: time.Second, MaxDelay: 30 * time.Second, MaxElapsed: 2 * time.Minute,
		MaxResumes: 5, ResumeInitialDelay: 5 * time.Second, ResumeMaxDelay: 80 * time.Second,
	}
}

func normalizeRetryPolicy(p *RetryPolicy) RetryPolicy {
	d := DefaultRetryPolicy()
	if p == nil {
		return d
	}
	if p.MaxAttempts > 0 {
		d.MaxAttempts = p.MaxAttempts
	}
	if p.InitialDelay > 0 {
		d.InitialDelay = p.InitialDelay
	}
	if p.MaxDelay > 0 {
		d.MaxDelay = p.MaxDelay
	}
	if p.MaxElapsed > 0 {
		d.MaxElapsed = p.MaxElapsed
	}
	// After normalization, zero MaxResumes means "disabled".
	switch {
	case p.MaxResumes > 0:
		d.MaxResumes = p.MaxResumes
	case p.MaxResumes < 0:
		d.MaxResumes = 0
	}
	if p.ResumeInitialDelay > 0 {
		d.ResumeInitialDelay = p.ResumeInitialDelay
	}
	if p.ResumeMaxDelay > 0 {
		d.ResumeMaxDelay = p.ResumeMaxDelay
	}
	d.InitialDelay = min(d.InitialDelay, d.MaxDelay)
	d.ResumeInitialDelay = min(d.ResumeInitialDelay, d.ResumeMaxDelay)
	return d
}

// resumeDelay returns the wait before step resume number resume (1-based).
// It uses the same jitter and server retry hints as request retries. A hint
// above ResumeMaxDelay stops the resume.
func resumeDelay(p RetryPolicy, resume int, err error, now time.Time) (time.Duration, bool) {
	return retryDelay(RetryPolicy{InitialDelay: p.ResumeInitialDelay, MaxDelay: p.ResumeMaxDelay}, resume, err, now)
}

type retryNotification func(attempt int, err error, delay time.Duration)
type retryContextKey struct{}

// retryModel retries only model requests, never tool execution or agent steps.
// Once content has reached the agent it cannot be replayed safely. Such failures
// are returned unchanged, so partial text and tool calls are not repeated.
// Start events and warnings are buffered until content arrives.
type retryModel struct {
	fantasy.LanguageModel
	policy RetryPolicy
}

// retryableStreamMessage matches known transient failures, not general retry
// advice. The processing error suffix can contain a different request ID.
func retryableStreamMessage(message string) bool {
	message = strings.ToLower(strings.Join(strings.Fields(message), " "))
	return strings.Contains(message, "stream transport error: internal_error (received from peer)") ||
		strings.Contains(message, "our servers are currently overloaded. please try again later.") ||
		strings.Contains(message, "an error occurred while processing your request. you can retry your request, or contact us through our help center at help.openai.com if the error persists.")
}

func retryableRequestError(err error) bool {
	if err == nil || err == context.DeadlineExceeded || errors.Is(err, context.Canceled) {
		return false
	}
	streamMessage := retryableStreamMessage(err.Error())
	if pe, ok := errors.AsType[*fantasy.ProviderError](err); ok {
		if pe.AuthError || pe.StatusCode == http.StatusUnauthorized || pe.StatusCode == http.StatusForbidden {
			return false
		}
		if pe.IsRetryable() {
			return true
		}
		// Some compatible providers omit the transient error type in SSE
		// errors. Match known transient messages only when no HTTP failure
		// status is available. Do not turn validation or auth errors into retries.
		if (pe.StatusCode == 0 || pe.StatusCode == http.StatusOK) &&
			retryableStreamMessage(pe.Message) {
			return true
		}
		var timeout net.Error
		return errors.As(pe.Cause, &timeout) && timeout != context.DeadlineExceeded && timeout.Timeout()
	}
	if ne, ok := errors.AsType[net.Error](err); ok {
		// Some providers return temporary DNS errors without wrapping them.
		// Keep that legacy signal alongside timeout and transport classification.
		//nolint:staticcheck // SA1019: needed for temporary, non-timeout DNS failures.
		return ne != context.DeadlineExceeded && (ne.Timeout() || ne.Temporary() || fantasy.IsTransportError(err) || streamMessage)
	}
	return !errors.Is(err, context.DeadlineExceeded) && (fantasy.IsTransportError(err) || streamMessage)
}

// retryAfter accepts case-insensitive HTTP headers and rejects unsafe numeric
// conversions. Oversized server delays stop retries instead of retrying early.
func retryAfter(err error, now time.Time) (time.Duration, bool) {
	var pe *fantasy.ProviderError
	if !errors.As(err, &pe) {
		return 0, false
	}
	headers := make(map[string]string, len(pe.ResponseHeaders))
	for k, v := range pe.ResponseHeaders {
		headers[strings.ToLower(k)] = strings.TrimSpace(v)
	}
	for _, item := range []struct {
		name string
		unit time.Duration
	}{{"retry-after-ms", time.Millisecond}, {"retry-after", time.Second}} {
		value, ok := headers[item.name]
		if !ok {
			continue
		}
		if n, e := strconv.ParseFloat(value, 64); e == nil {
			if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
				continue
			}
			ns := n * float64(item.unit)
			if ns >= float64(math.MaxInt64) {
				return time.Duration(math.MaxInt64), true
			}
			return time.Duration(ns), true
		}
		if item.name == "retry-after" {
			if t, e := http.ParseTime(value); e == nil {
				return max(0, t.Sub(now)), true
			}
		}
	}
	return 0, false
}

func retryDelay(p RetryPolicy, attempt int, err error, now time.Time) (time.Duration, bool) {
	capDelay := p.InitialDelay
	for i := 1; i < attempt && capDelay < p.MaxDelay; i++ {
		if capDelay > p.MaxDelay/2 {
			capDelay = p.MaxDelay
		} else {
			capDelay *= 2
		}
	}
	// Equal jitter avoids both synchronized retries and zero-delay retry loops.
	delay := max(time.Nanosecond, capDelay/2+time.Duration(rand.Float64()*float64(capDelay-capDelay/2)))
	if server, ok := retryAfter(err, now); ok {
		delay = max(delay, server)
	}
	return delay, delay <= p.MaxDelay
}

func (m *retryModel) wait(ctx context.Context, started time.Time, attempt int, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if attempt >= m.policy.MaxAttempts || !retryableRequestError(err) {
		return err
	}
	now := time.Now()
	delay, ok := retryDelay(m.policy, attempt, err, now)
	remaining := m.policy.MaxElapsed - now.Sub(started)
	if !ok || remaining <= 0 || delay >= remaining {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok && delay >= time.Until(deadline) {
		return err
	}
	if notify, ok := ctx.Value(retryContextKey{}).(retryNotification); ok {
		notify(attempt, err, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (m *retryModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	started := time.Now()
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		response, err := m.LanguageModel.Generate(ctx, call)
		if err == nil {
			return response, nil
		}
		if err = m.wait(ctx, started, attempt, err); err != nil {
			return nil, err
		}
	}
}

func (m *retryModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	// Keep stream creation and pre-output failures in the same attempt budget.
	return func(yield func(fantasy.StreamPart) bool) {
		started := time.Now()
		for attempt := 1; ; attempt++ {
			err := ctx.Err()
			emitted, finished := false, false
			var pending []fantasy.StreamPart
			if err == nil {
				var stream fantasy.StreamResponse
				stream, err = m.LanguageModel.Stream(ctx, call)
				if err == nil {
					for part := range stream {
						if ctx.Err() != nil {
							err = ctx.Err()
							break
						}
						if part.Type == fantasy.StreamPartTypeError {
							err = part.Error
							break
						}
						// These events do not deliver content. Hold them until the
						// attempt succeeds or sends content, so a retry does not
						// repeat start callbacks or warnings from a failed attempt.
						switch part.Type {
						case fantasy.StreamPartTypeKeepalive:
							if !yield(part) {
								return
							}
							continue
						case fantasy.StreamPartTypeWarnings, fantasy.StreamPartTypeTextStart, fantasy.StreamPartTypeReasoningStart, fantasy.StreamPartTypeToolInputStart:
							if !emitted {
								pending = append(pending, part)
								continue
							}
						}
						for _, buffered := range pending {
							if !yield(buffered) {
								return
							}
						}
						pending = nil
						emitted = true
						if part.Type == fantasy.StreamPartTypeFinish {
							finished = true
						}
						if !yield(part) {
							return
						}
					}
					if err == nil && !finished {
						err = fantasy.NewIncompleteStreamError()
					}
				}
			}
			if err == nil {
				return
			}
			if !emitted {
				err = m.wait(ctx, started, attempt, err)
				if err == nil {
					continue
				}
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: err})
			return
		}
	}, nil
}
