package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/message"
)

// providerStreamError is also consumed by the agent retry layer via a private
// behavioral interface. Never replay after any visible content or tool event.
type providerStreamError struct {
	cause     error
	retryable bool
	exposed   bool
}

func (e *providerStreamError) Error() string            { return e.cause.Error() }
func (e *providerStreamError) Unwrap() error            { return e.cause }
func (e *providerStreamError) ExtensionRetryable() bool { return e.retryable && !e.exposed }

func usageSnapshot(u ProviderUsage) fantasy.Usage {
	return fantasy.Usage{InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, TotalTokens: u.TotalTokens, ReasoningTokens: u.ReasoningTokens, CacheCreationTokens: u.CacheCreationTokens, CacheReadTokens: u.CacheReadTokens}
}
func monotonicUsage(prior, next ProviderUsage) bool {
	return next.InputTokens >= prior.InputTokens && next.OutputTokens >= prior.OutputTokens && next.TotalTokens >= prior.TotalTokens && next.ReasoningTokens >= prior.ReasoningTokens && next.CacheCreationTokens >= prior.CacheCreationTokens && next.CacheReadTokens >= prior.CacheReadTokens
}

type providerBlock struct {
	kind ProviderEventType
	name string
	args string
	open bool
}

// runProviderStream executes on the consumer goroutine. It has no event queue
// or producer goroutine. Only final executable calls and finish are buffered.
func runProviderStream(ctx context.Context, runtime ProviderRuntime, request ProviderRequest, yield func(fantasy.StreamPart) bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	request.Done = ctx.Done()
	var emitting atomic.Bool
	var stopped atomic.Bool
	var mu sync.Mutex
	var first error
	setError := func(err error) {
		mu.Lock()
		if first == nil {
			first = err
		}
		mu.Unlock()
		stopped.Store(true)
		cancel()
	}
	getError := func() error { mu.Lock(); defer mu.Unlock(); return first }
	blocks := map[string]*providerBlock{}
	var calls []fantasy.StreamPart
	var usage ProviderUsage
	var finish fantasy.FinishReason
	var exposed bool
	send := func(part fantasy.StreamPart) bool {
		if !yield(part) {
			setError(context.Canceled)
			return false
		}
		return true
	}
	protocol := func() bool { setError(errors.New("extension provider stream protocol violation")); return false }
	emit := func(event ProviderEvent) bool {
		if stopped.Load() {
			return false
		}
		if !emitting.CompareAndSwap(false, true) {
			return protocol()
		}
		defer emitting.Store(false)
		if ctx.Err() != nil {
			setError(ctx.Err())
			return false
		}
		if finish != "" {
			return protocol()
		}
		metadata := &message.ExtensionMetadata{Provider: request.Provider, Signature: event.Signature}
		for _, replay := range event.Replay {
			if replay.Version != 1 || replay.Provider != request.Provider || !json.Valid([]byte(replay.JSON)) {
				setError(errors.New("unsupported extension replay metadata"))
				return false
			}
			metadata.Replay = append(metadata.Replay, message.ReplayData{Version: replay.Version, Provider: replay.Provider, JSON: replay.JSON})
		}
		options := fantasy.ProviderMetadata{"kit-extension": metadata}
		if event.Type == ProviderUsageEvent {
			if !monotonicUsage(usage, event.Usage) {
				return protocol()
			}
			usage = event.Usage
			return true
		}
		if event.Type == ProviderFinish {
			for _, block := range blocks {
				if block.open {
					return protocol()
				}
			}
			switch event.FinishReason {
			case ProviderFinishStop:
				finish = fantasy.FinishReasonStop
			case ProviderFinishLength:
				finish = fantasy.FinishReasonLength
			case ProviderFinishToolCalls:
				finish = fantasy.FinishReasonToolCalls
			default:
				return protocol()
			}
			if (len(calls) > 0) != (event.FinishReason == ProviderFinishToolCalls) {
				return protocol()
			}
			return true
		}
		if event.ID == "" {
			return protocol()
		}
		block := blocks[event.ID]
		part := fantasy.StreamPart{ID: event.ID, Delta: event.Delta, ProviderMetadata: options}
		switch event.Type {
		case ProviderTextStart, ProviderReasoningStart, ProviderToolStart:
			if block != nil || event.Delta != "" {
				return protocol()
			}
			block = &providerBlock{kind: event.Type, name: event.ToolName, open: true}
			blocks[event.ID] = block
			switch event.Type {
			case ProviderTextStart:
				part.Type = fantasy.StreamPartTypeTextStart
			case ProviderReasoningStart:
				part.Type = fantasy.StreamPartTypeReasoningStart
			case ProviderToolStart:
				found := false
				for _, tool := range request.Tools {
					if tool.Name == event.ToolName {
						found = true
					}
				}
				if !found || request.ToolChoice.Type == ProviderToolNone || (request.ToolChoice.Type == ProviderToolNamed && request.ToolChoice.Name != event.ToolName) {
					return protocol()
				}
				part.Type = fantasy.StreamPartTypeToolInputStart
				part.ToolCallName = event.ToolName
				exposed = true
			}
		case ProviderTextDelta, ProviderReasoningDelta, ProviderToolDelta, ProviderTextEnd, ProviderReasoningEnd, ProviderToolEnd:
			if block == nil || !block.open {
				return protocol()
			}
			switch event.Type {
			case ProviderTextDelta:
				if block.kind != ProviderTextStart {
					return protocol()
				}
				part.Type = fantasy.StreamPartTypeTextDelta
				exposed = exposed || event.Delta != ""
			case ProviderReasoningDelta:
				if block.kind != ProviderReasoningStart {
					return protocol()
				}
				part.Type = fantasy.StreamPartTypeReasoningDelta
				exposed = exposed || event.Delta != ""
			case ProviderToolDelta:
				if block.kind != ProviderToolStart {
					return protocol()
				}
				part.Type = fantasy.StreamPartTypeToolInputDelta
				block.args += event.Delta
				exposed = true
			case ProviderTextEnd:
				if block.kind != ProviderTextStart || event.Delta != "" {
					return protocol()
				}
				part.Type = fantasy.StreamPartTypeTextEnd
				block.open = false
			case ProviderReasoningEnd:
				if block.kind != ProviderReasoningStart || event.Delta != "" {
					return protocol()
				}
				part.Type = fantasy.StreamPartTypeReasoningEnd
				block.open = false
			case ProviderToolEnd:
				if block.kind != ProviderToolStart || event.Delta != "" {
					return protocol()
				}
				var object map[string]json.RawMessage
				if json.Unmarshal([]byte(block.args), &object) != nil || object == nil {
					return protocol()
				}
				block.open = false
				part.Type = fantasy.StreamPartTypeToolInputEnd
				calls = append(calls, fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: event.ID, ToolCallName: block.name, ToolCallInput: block.args, ProviderMetadata: options})
			}
		default:
			return protocol()
		}
		return send(part)
	}
	var result ProviderResult
	func() {
		defer func() {
			if recover() != nil {
				setError(errors.New("panic in extension provider Stream"))
			}
		}()
		result = runtime.Stream(request, emit)
	}()
	stopped.Store(true)
	if err := getError(); err != nil {
		return &providerStreamError{cause: err, exposed: exposed}
	}
	if ctx.Err() != nil {
		return &providerStreamError{cause: ctx.Err(), exposed: exposed}
	}
	if result.Error != nil {
		// The host bridge must redact resolved credentials before surfacing text.
		retryable := result.Error.Retryable && (result.Error.Code == "transport" || result.Error.Code == "rate_limit")
		return &providerStreamError{cause: fmt.Errorf("extension provider %s/%s: %s", request.Provider, request.Model, result.Error.Code), retryable: retryable, exposed: exposed}
	}
	if finish == "" {
		return &providerStreamError{cause: errors.New("extension provider stream missing finish"), exposed: exposed}
	}
	for _, call := range calls {
		if !send(call) {
			return getError()
		}
	}
	if !send(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: finish, Usage: usageSnapshot(usage)}) {
		return getError()
	}
	return nil
}
