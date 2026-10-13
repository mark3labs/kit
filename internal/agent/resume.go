package agent

import (
	"context"
	"time"

	"charm.land/fantasy"
)

// Step resume restarts a streaming step that failed with a transient
// provider error, from the last completed step. It is the turn-level layer
// above retryModel: retryModel repeats one request only while no content was
// delivered, and resume also recovers a step that already streamed partial
// output.
//
// Safety rules:
//   - The stream library runs local tools only after a step stream ends
//     without error, so a failed step ran no local tools. Completed steps
//     and their tool results are never repeated.
//   - Partial output of the failed step is discarded, not persisted.
//     OnStepResume tells consumers to remove it from their display.
//   - Only errors that retryableRequestError accepts are resumed. Auth,
//     validation, cancellation, deadline and tool errors are not.
//   - A failed incremental write stops resumes, so history is never written
//     twice.
//   - The number of resumes for each turn is limited by
//     RetryPolicy.MaxResumes, and the step limit continues across resumes.

// resumePlan is a scheduled step resume.
type resumePlan struct {
	delay time.Duration
}

// resumeState is the turn state that planResume uses.
type resumeState struct {
	err            error
	resumes        int
	completedSteps int
	persistenceErr error
	// lastRole is the role of the last message of the replay prompt.
	lastRole fantasy.MessageRole
}

// planResume returns a plan when the failed step can be resumed, or nil.
func (a *Agent) planResume(ctx context.Context, s resumeState) *resumePlan {
	p := a.retryPolicy
	if s.err == nil || ctx.Err() != nil || s.persistenceErr != nil {
		return nil
	}
	if p.MaxResumes <= 0 || s.resumes >= p.MaxResumes {
		return nil
	}
	if a.maxSteps > 0 && s.completedSteps >= a.maxSteps {
		return nil
	}
	// The replay sends no new prompt, so the provider request needs a
	// user or tool message at the end. Otherwise the replay fails with a
	// validation error, which is worse than the original error.
	if s.lastRole != fantasy.MessageRoleUser && s.lastRole != fantasy.MessageRoleTool {
		return nil
	}
	if !retryableRequestError(s.err) {
		return nil
	}
	delay, ok := resumeDelay(p, s.resumes+1, s.err, time.Now())
	if !ok {
		return nil
	}
	if deadline, ok := ctx.Deadline(); ok && delay >= time.Until(deadline) {
		return nil
	}
	return &resumePlan{delay: delay}
}

// lastReplayRole returns the role of the last message of the replay prompt
// (the original messages followed by the completed step messages).
func lastReplayRole(messages, completed []fantasy.Message) fantasy.MessageRole {
	if n := len(completed); n > 0 {
		return completed[n-1].Role
	}
	if n := len(messages); n > 0 {
		return messages[n-1].Role
	}
	return ""
}

// waitResume waits for delay or until ctx ends.
func waitResume(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// addUsage returns the sum of two usage values.
func addUsage(a, b fantasy.Usage) fantasy.Usage {
	return fantasy.Usage{
		InputTokens:         a.InputTokens + b.InputTokens,
		OutputTokens:        a.OutputTokens + b.OutputTokens,
		TotalTokens:         a.TotalTokens + b.TotalTokens,
		ReasoningTokens:     a.ReasoningTokens + b.ReasoningTokens,
		CacheCreationTokens: a.CacheCreationTokens + b.CacheCreationTokens,
		CacheReadTokens:     a.CacheReadTokens + b.CacheReadTokens,
	}
}
