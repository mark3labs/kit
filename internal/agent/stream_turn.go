package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"charm.land/fantasy"
)

// streamTurn holds the state of one generateStreaming call: one user turn,
// across media-recovery and resume replays.
// The goroutine running Stream owns all mutable fields except toolCallArgs.
// Tool callbacks can access toolCallArgs concurrently; toolCallArgsMu protects it.
// The calling goroutine accesses turn state only between or after Stream calls.
type streamTurn struct {
	a        *Agent
	cb       GenerateCallbacks
	ctx      context.Context
	messages []fantasy.Message

	toolCallArgsMu        sync.Mutex
	toolCallArgs          map[string]string
	completedStepMessages []fantasy.Message
	completedUsage        fantasy.Usage
	completedSteps        int
	stepOffset            int
	stepCounter           int
	persistedCount        int
	persistenceErr        error
	pendingMedia          []fantasy.Message
	unfinished            strings.Builder
	attemptID             string
	mediaRetries          int
	preparationErr        error
	resumes               int
	plannedResume         *resumePlan
	injectedSteer         []steerInjection
}

func (t *streamTurn) writeStep(msgs []fantasy.Message) {
	if t.persistenceErr != nil {
		return
	}
	if t.cb.OnStepMessagesChecked != nil {
		n, err := t.cb.OnStepMessagesChecked(msgs)
		if n < 0 || n > len(msgs) {
			n = 0
			err = errors.Join(err, fmt.Errorf("invalid confirmed write count"))
		}
		if err == nil && n != len(msgs) {
			err = fmt.Errorf("only %d of %d step messages were persisted", n, len(msgs))
		}
		t.persistedCount += n
		t.persistenceErr = err
	} else if t.cb.OnStepMessages != nil {
		t.cb.OnStepMessages(msgs)
		t.persistedCount += len(msgs)
	}
}

func (t *streamTurn) persistStep(msgs []fantasy.Message) {
	if len(msgs) == 0 {
		return
	}
	if len(t.pendingMedia) > 0 || hasMediaToolResult(msgs) {
		t.pendingMedia = append(t.pendingMedia, msgs...)
		return
	}
	t.writeStep(msgs)
}

func (t *streamTurn) flushPending() {
	if len(t.pendingMedia) == 0 {
		return
	}
	held := t.pendingMedia
	t.pendingMedia = nil
	t.writeStep(held)
}

func (t *streamTurn) canRecoverMedia(err error) bool {
	return t.preparationErr == nil && t.mediaRetries < maxMediaRetries &&
		hasMediaToolResult(t.pendingMedia) &&
		isMediaRejection(err)
}
