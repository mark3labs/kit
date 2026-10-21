package agent

import (
	"context"
	"crypto/rand"
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

func (t *streamTurn) streamCall(prompt string, files []fantasy.FilePart, history []fantasy.Message) fantasy.AgentStreamCall {
	// Use the streaming agent
	zeroRetries := 0
	return fantasy.AgentStreamCall{
		MaxRetries: &zeroRetries,
		Prompt:     prompt,
		Files:      files,
		Messages:   history,

		// Tool input streaming callbacks — fire during tool argument generation
		OnToolInputStart: t.onToolInputStart,
		OnToolInputDelta: t.onToolInputDelta,
		OnToolInputEnd:   t.onToolInputEnd,

		// Text start/end callbacks
		OnTextStart: t.onTextStart,
		OnTextEnd:   t.onTextEnd,

		// Reasoning start callback
		OnReasoningStart: t.onReasoningStart,

		// Reasoning/thinking streaming callback
		OnReasoningDelta: t.onReasoningDelta,

		// Reasoning/thinking complete callback
		OnReasoningEnd: t.onReasoningEnd,

		// Text streaming callback
		OnTextDelta: t.onTextDelta,

		// Warnings callback
		OnWarnings: t.onWarnings,

		// Source callback
		OnSource: t.onSource,

		// Stream finish callback (per-step stream completion)
		OnStreamFinish: t.onStreamFinish,

		// Error callback
		OnError: t.onError,

		// Step start callback
		OnStepStart: t.onStepStart,

		// Tool call complete - the tool has been parsed and is about to execute
		OnToolCall: t.onToolCall,

		// Tool result - tool execution completed
		OnToolResult: t.onToolResult,

		// Step callbacks for content that accompanies tool calls
		OnStepFinish: t.onStepFinish,
	}

}

func (t *streamTurn) onToolInputStart(id, toolName string) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	if t.cb.OnToolCallStart != nil {
		t.cb.OnToolCallStart(id, toolName)
	}
	return nil
}

func (t *streamTurn) onToolInputDelta(id, delta string) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	if t.cb.OnToolCallDelta != nil {
		t.cb.OnToolCallDelta(id, delta)
	}
	return nil
}

func (t *streamTurn) onToolInputEnd(id string) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	if t.cb.OnToolCallEnd != nil {
		t.cb.OnToolCallEnd(id)
	}
	return nil
}

func (t *streamTurn) onTextStart(id string) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	if t.cb.OnTextStart != nil {
		t.cb.OnTextStart(id)
	}
	return nil
}

func (t *streamTurn) onTextEnd(id string) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	if t.cb.OnTextEnd != nil {
		t.cb.OnTextEnd(id)
	}
	return nil
}

func (t *streamTurn) onReasoningStart(id string, _ fantasy.ReasoningContent) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	if t.cb.OnReasoningStart != nil {
		t.cb.OnReasoningStart(id)
	}
	return nil
}

func (t *streamTurn) onReasoningDelta(id, delta string) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	if t.cb.OnReasoningDelta != nil {
		t.cb.OnReasoningDelta(delta)
	}
	return nil
}

func (t *streamTurn) onReasoningEnd(id string, _ fantasy.ReasoningContent) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	if t.cb.OnReasoningComplete != nil {
		t.cb.OnReasoningComplete()
	}
	return nil
}

func (t *streamTurn) onTextDelta(id, text string) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	t.unfinished.WriteString(text)
	if t.cb.OnStreamingResponse != nil {
		t.cb.OnStreamingResponse(text)
	}
	return nil
}

func (t *streamTurn) onWarnings(warnings []fantasy.CallWarning) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	if t.cb.OnWarnings != nil {
		strs := make([]string, len(warnings))
		for i, w := range warnings {
			strs[i] = w.Message
		}
		t.cb.OnWarnings(strs)
	}
	return nil
}

func (t *streamTurn) onSource(source fantasy.SourceContent) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	if t.cb.OnSource != nil {
		t.cb.OnSource(string(source.SourceType), source.ID, source.URL, source.Title)
	}
	return nil
}

func (t *streamTurn) onStreamFinish(usage fantasy.Usage, finishReason fantasy.FinishReason, _ fantasy.ProviderMetadata) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	if t.cb.OnStreamFinish != nil {
		t.cb.OnStreamFinish(usage, string(finishReason))
	}
	return nil
}

func (t *streamTurn) onError(err error) {
	if t.preparationErr != nil {
		if t.cb.OnError != nil {
			t.cb.OnError(t.preparationErr)
		}
		return
	}
	// A rejected media tool result is recovered below and the
	// model is told about it, so it is not a turn error.
	if t.canRecoverMedia(err) {
		return
	}
	// A resumable failure is not a turn error either. The plan is
	// made here, so the error is reported exactly once: either
	// as a resume or as an error.
	t.plannedResume = t.a.planResume(t.ctx, resumeState{
		err:            err,
		resumes:        t.resumes,
		completedSteps: t.completedSteps,
		persistenceErr: t.persistenceErr,
		lastRole:       lastReplayRole(t.messages, t.completedStepMessages),
	})
	if t.plannedResume != nil {
		return
	}
	if t.cb.OnError != nil {
		t.cb.OnError(err)
	}
}

func (t *streamTurn) onStepStart(stepNumber int) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	t.stepCounter = t.stepOffset + stepNumber
	t.attemptID = rand.Text()
	if t.cb.OnStepStart != nil {
		t.cb.OnStepStart(t.stepCounter)
	}
	return nil
}

func (t *streamTurn) onToolCall(tc fantasy.ToolCallContent) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	t.toolCallArgsMu.Lock()
	t.toolCallArgs[tc.ToolCallID] = tc.Input
	t.toolCallArgsMu.Unlock()

	// Notify about the tool call
	if t.cb.OnToolCall != nil {
		t.cb.OnToolCall(tc.ToolCallID, tc.ToolName, tc.Input)
	}

	// Notify tool execution starting
	if t.cb.OnToolExecution != nil {
		t.cb.OnToolExecution(tc.ToolCallID, tc.ToolName, tc.Input, true)
	}

	return nil
}

func (t *streamTurn) onToolResult(tr fantasy.ToolResultContent) error {
	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	// Look up the args recorded for this specific tool call. Delete
	// the entry so the map doesn't accumulate across steps.
	t.toolCallArgsMu.Lock()
	args := t.toolCallArgs[tr.ToolCallID]
	delete(t.toolCallArgs, tr.ToolCallID)
	t.toolCallArgsMu.Unlock()

	// Notify tool execution finished
	if t.cb.OnToolExecution != nil {
		t.cb.OnToolExecution(tr.ToolCallID, tr.ToolName, args, false)
	}

	if t.cb.OnToolResult != nil {
		// Extract result text and error status
		resultText, isError := extractToolResultText(tr)
		t.cb.OnToolResult(tr.ToolCallID, tr.ToolName, args, resultText, tr.ClientMetadata, isError)
	}

	return nil
}

func (t *streamTurn) onStepFinish(step fantasy.StepResult) error {
	// Accumulate messages from completed steps so they can be
	// persisted even if a later step is cancelled.
	t.completedStepMessages = append(t.completedStepMessages, step.Messages...)
	t.unfinished.Reset()
	t.completedSteps++
	t.completedUsage = addUsage(t.completedUsage, step.Usage)

	// This step completed, so the provider accepted the request
	// that carried any held media tool results. Persist them.
	t.flushPending()

	// Persist step messages incrementally so progress is saved
	// as it happens rather than only at the end of the turn.
	t.persistStep(step.Messages)
	if t.persistenceErr != nil {
		return t.persistenceErr
	}

	if t.ctx.Err() != nil {
		return t.ctx.Err()
	}
	// Check if step has text content alongside tool calls
	text := step.Content.Text()
	toolCalls := step.Content.ToolCalls()
	if text != "" && len(toolCalls) > 0 && t.cb.OnToolCallContent != nil {
		t.cb.OnToolCallContent(text)
	}
	// Emit step usage for real-time cost tracking
	if t.cb.OnStepUsage != nil {
		t.cb.OnStepUsage(step.Usage.InputTokens, step.Usage.OutputTokens,
			step.Usage.CacheReadTokens, step.Usage.CacheCreationTokens)
	}
	// Emit unified step finish event
	if t.cb.OnStepFinish != nil {
		t.cb.OnStepFinish(t.stepCounter, len(toolCalls) > 0, string(step.FinishReason), step.Usage)
	}
	return nil
}
