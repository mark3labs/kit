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
	a                *Agent
	cb               GenerateCallbacks
	ctx              context.Context
	messages         []fantasy.Message
	history          []fantasy.Message
	hadManagedPrompt bool

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

func (t *streamTurn) streamCall(prompt string, files []fantasy.FilePart) fantasy.AgentStreamCall {
	// Use the streaming agent
	zeroRetries := 0
	return fantasy.AgentStreamCall{
		MaxRetries: &zeroRetries,
		Prompt:     prompt,
		Files:      files,
		Messages:   t.history,

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

func (t *streamTurn) prepareStep(
	stepCtx context.Context,
	opts fantasy.PrepareStepFunctionOptions,
) (context.Context, fantasy.PrepareStepResult, error) {
	steerCh := steerChFromContext(t.ctx)
	onConsumed := steerConsumedFromContext(t.ctx)
	hasSteering := steerCh != nil
	hasPrepareStepHook := t.cb.OnPrepareStep != nil
	if err := stepCtx.Err(); err != nil {
		t.preparationErr = err
		return stepCtx, fantasy.PrepareStepResult{}, err
	}
	result := fantasy.PrepareStepResult{
		Model:    opts.Model,
		Messages: withSteerInjections(opts.Messages, t.injectedSteer),
	}

	// Phase 1: Drain steering channel (if present).
	if hasSteering {
		var steered []SteerMessage
		for {
			select {
			case msg := <-steerCh:
				steered = append(steered, msg)
			default:
				goto done
			}
		}
	done:
		if len(steered) > 0 {
			newMessages := make([]fantasy.Message, 0, len(steered))
			for _, sm := range steered {
				msg := fantasy.NewUserMessage(sm.Text, sm.Files...)
				newMessages = append(newMessages, msg)
				t.injectedSteer = append(t.injectedSteer, steerInjection{
					pos: len(opts.Messages),
					msg: msg,
				})
			}
			result.Messages = append(result.Messages, newMessages...)

			// The previous step's messages were already recorded in
			// OnStepFinish, so appending here keeps the order of the
			// conversation. Persist now so the steer message survives
			// a cancel, an error, and a session reload.
			t.completedStepMessages = append(t.completedStepMessages, newMessages...)
			t.persistStep(newMessages)
			if t.persistenceErr != nil {
				return stepCtx, result, t.persistenceErr
			}

			if onConsumed != nil {
				onConsumed(len(steered))
			}
		}
	}

	if err := stepCtx.Err(); err != nil {
		t.preparationErr = err
		return stepCtx, result, err
	}
	// Run hooks first, then capture live state. Explicit overrides win.
	var update *PrepareStepUpdate
	if hasPrepareStepHook {
		update = t.cb.OnPrepareStep(stepCtx, t.stepOffset+opts.StepNumber, result.Messages)
	}
	if update != nil && update.Error != nil {
		t.preparationErr = update.Error
		return stepCtx, result, t.preparationErr
	}
	if err := stepCtx.Err(); err != nil {
		t.preparationErr = err
		return stepCtx, result, err
	}
	var livePrompt string
	if t.cb.RequestSnapshot != nil {
		livePrompt, result.Tools = t.cb.RequestSnapshot()
	} else {
		livePrompt, result.Tools = t.a.RequestSnapshot()
	}
	if t.hadManagedPrompt && len(result.Messages) > 0 && result.Messages[0].Role == fantasy.MessageRoleSystem {
		result.Messages = append([]fantasy.Message(nil), result.Messages...)
		result.Messages[0] = fantasy.NewSystemMessage(livePrompt)
	} else if livePrompt != "" {
		result.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(livePrompt)}, result.Messages...)
	}
	if update != nil {
		if update.Messages != nil {
			result.Messages = update.Messages
		}
		if update.ToolChoice != nil {
			result.ToolChoice = update.ToolChoice
		}
		if update.Tools != nil {
			// An empty non-nil slice disables tools for this step.
			result.Tools = update.Tools
		}
	}

	// Remove images from the request when the active model cannot
	// read them (for example after a switch to a text-only model).
	// The session keeps them. See image_gate.go.
	if t.a.currentModel != nil {
		if supported, model := t.a.currentModel.imageInput(); !supported {
			result.Messages = withoutImages(result.Messages, model)
		}
	}

	// Apply message-level cache control for Anthropic models. The tool
	// list already carries a breakpoint, so reserve it from the budget.
	if t.a.messageCachingEnabled() {
		result.Messages = applyCacheControlToMessages(result.Messages, toolCacheBlocks)
	}

	if err := stepCtx.Err(); err != nil {
		t.preparationErr = err
		return stepCtx, result, err
	}
	return stepCtx, result, nil
}
