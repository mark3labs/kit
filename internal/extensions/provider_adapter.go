package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/message"
)

// providerAdapter is compiled code; no interpreted type implements a host
// interface. The gate serializes one runtime without locking other runtimes.
type providerAdapter struct {
	provider     string
	owner        *ProviderOwner
	model        string
	capabilities ProviderCapabilities
	parameters   ProviderParameters
	runtime      ProviderRuntime
	gate         chan struct{}
	lifetime     context.Context
	cancel       context.CancelFunc
	closeOnce    sync.Once
	closeDone    chan struct{}
	closeErr     error
	callbackGID  atomic.Int64
	credential   string
}

// NewProviderModel creates a private compiled adapter. Callers validate owned
// definitions/collisions before this boundary. Credentials are only in Create.
func NewProviderModel(ctx context.Context, def ProviderDef, request ProviderCreateRequest) (model fantasy.LanguageModel, closer func(context.Context) error, err error) {
	if !def.owner.acquire() {
		return nil, nil, errors.New("extension provider generation is retired")
	}
	transferred := false
	defer func() {
		if !transferred {
			def.owner.release()
		}
	}()
	var capability ProviderCapabilities
	found := false
	for _, item := range def.Models {
		if item.ID == request.Model {
			capability = item.Capabilities
			if item.SettingsJSON != "" {
				request.Parameters.SettingsJSON = item.SettingsJSON
			}
			found = true
			break
		}
	}
	if !found {
		if !def.AllowUnknownModels {
			return nil, nil, fmt.Errorf("extension provider %s does not declare model %q", def.ID, request.Model)
		}
		capability = def.UnknownModelDefaults
	}
	defer func() {
		if recover() != nil {
			model = nil
			closer = nil
			err = fmt.Errorf("panic in extension provider %s Create", def.ID)
		}
	}()
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	request.Provider = def.ID
	request.Done = ctx.Done()
	if level := request.Parameters.ReasoningLevel; level != "" && level != "off" {
		allowed := false
		for _, supported := range capability.ReasoningLevels {
			if supported == level {
				allowed = true
			}
		}
		if !capability.Reasoning || !allowed {
			return nil, nil, errors.New("unsupported extension reasoning level")
		}
	}
	if def.ValidateSettings != nil {
		if failure := def.ValidateSettings(request.Parameters.SettingsJSON); failure != nil {
			return nil, nil, errors.New("extension provider settings validation failed")
		}
	}
	result := def.Create(request)
	if result.Error != nil {
		code := result.Error.Code
		if request.APIKey != "" {
			code = strings.ReplaceAll(code, request.APIKey, "[redacted]")
		}
		return nil, nil, fmt.Errorf("extension provider %s Create failed (%s)", def.ID, code)
	}
	if result.Runtime.Stream == nil {
		if result.Runtime.Close != nil {
			_ = result.Runtime.Close()
		}
		return nil, nil, fmt.Errorf("extension provider %s returned no Stream callback", def.ID)
	}
	lifetime, cancel := context.WithCancel(context.Background())
	transferred = true
	adapter := &providerAdapter{credential: request.APIKey, owner: def.owner, provider: def.ID, model: request.Model, capabilities: capability, parameters: request.Parameters, runtime: result.Runtime, gate: make(chan struct{}, 1), lifetime: lifetime, cancel: cancel, closeDone: make(chan struct{})}
	if !def.owner.track(adapter) {
		_ = adapter.closeContext(ctx)
		return nil, nil, errors.New("extension provider generation is shutting down")
	}
	return adapter, adapter.closeContext, nil
}

func (p *providerAdapter) Provider() string { return p.provider }
func (p *providerAdapter) Model() string    { return p.model }
func (p *providerAdapter) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("extension providers do not support object generation")
}
func (p *providerAdapter) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("extension providers do not support object streaming")
}

func (p *providerAdapter) closeContext(ctx context.Context) error {
	if p.callbackGID.Load() == goroutineID() {
		return errors.New("cannot close extension provider from its own callback")
	}
	p.closeOnce.Do(func() {
		p.cancel()
		go func() {
			defer close(p.closeDone)
			defer p.owner.release()
			defer p.owner.untrack(p)
			// An active noncooperative callback retains the owner until it returns.
			p.gate <- struct{}{}
			defer func() { <-p.gate }()
			defer func() {
				if recover() != nil {
					p.closeErr = errors.New("panic in extension provider Close")
				}
			}()
			if p.runtime.Close != nil {
				p.closeErr = p.runtime.Close()
				if p.closeErr != nil && p.credential != "" {
					p.closeErr = errors.New(strings.ReplaceAll(p.closeErr.Error(), p.credential, "[redacted]"))
				}
			}
		}()
	})
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	select {
	case <-p.closeDone:
		return p.closeErr
	case <-ctx.Done():
		return fmt.Errorf("extension provider %s/%s shutdown: %w", p.provider, p.model, ctx.Err())
	}
}

func (p *providerAdapter) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	request, err := p.convertRequest(call)
	if err != nil {
		return nil, err
	}
	return func(yield func(fantasy.StreamPart) bool) {
		requestCtx, cancel := context.WithCancel(ctx)
		stop := context.AfterFunc(p.lifetime, cancel)
		defer stop()
		defer cancel()
		select {
		case p.gate <- struct{}{}:
		case <-requestCtx.Done():
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: requestCtx.Err()})
			return
		}
		defer func() { <-p.gate }()
		if err := requestCtx.Err(); err != nil {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: err})
			return
		}
		p.callbackGID.Store(goroutineID())
		defer p.callbackGID.Store(0)
		consumerStopped := false
		forward := func(part fantasy.StreamPart) bool {
			if consumerStopped {
				return false
			}
			if !yield(part) {
				consumerStopped = true
				return false
			}
			return true
		}
		runtime := p.runtime
		stream := runtime.Stream
		runtime.Stream = func(request ProviderRequest, emit func(ProviderEvent) bool) ProviderResult {
			result := stream(request, emit)
			if result.Error != nil {
				copy := *result.Error
				if p.credential != "" {
					copy.Message = strings.ReplaceAll(copy.Message, p.credential, "[redacted]")
					copy.Code = strings.ReplaceAll(copy.Code, p.credential, "[redacted]")
				}
				result.Error = &copy
			}
			return result
		}
		if err := runProviderStream(requestCtx, runtime, request, forward); err != nil && !consumerStopped {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: err})
		}
	}, nil
}

func (p *providerAdapter) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	stream, err := p.Stream(ctx, call)
	if err != nil {
		return nil, err
	}
	response := &fantasy.Response{}
	texts := map[string]*fantasy.TextContent{}
	reasoning := map[string]*fantasy.ReasoningContent{}
	for event := range stream {
		switch event.Type {
		case fantasy.StreamPartTypeTextStart:
			item := &fantasy.TextContent{}
			texts[event.ID] = item
			response.Content = append(response.Content, item)
		case fantasy.StreamPartTypeTextDelta:
			texts[event.ID].Text += event.Delta
		case fantasy.StreamPartTypeTextEnd:
			texts[event.ID].ProviderMetadata = event.ProviderMetadata
		case fantasy.StreamPartTypeReasoningStart:
			item := &fantasy.ReasoningContent{}
			reasoning[event.ID] = item
			response.Content = append(response.Content, item)
		case fantasy.StreamPartTypeReasoningDelta:
			reasoning[event.ID].Text += event.Delta
		case fantasy.StreamPartTypeReasoningEnd:
			reasoning[event.ID].ProviderMetadata = event.ProviderMetadata
		case fantasy.StreamPartTypeToolCall:
			response.Content = append(response.Content, fantasy.ToolCallContent{ToolCallID: event.ID, ToolName: event.ToolCallName, Input: event.ToolCallInput, ProviderMetadata: event.ProviderMetadata})
		case fantasy.StreamPartTypeFinish:
			response.ProviderMetadata = event.ProviderMetadata
			response.FinishReason = event.FinishReason
			response.Usage = event.Usage
		case fantasy.StreamPartTypeError:
			return nil, event.Error
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if response.FinishReason == "" {
		return nil, errors.New("extension provider stream canceled without finish")
	}
	return response, nil
}

func (p *providerAdapter) convertRequest(call fantasy.Call) (ProviderRequest, error) {
	request := ProviderRequest{Provider: p.provider, Model: p.model, ToolChoice: ProviderToolChoice{Type: ProviderToolAuto}, Parameters: ProviderParameters{MaxOutputTokens: call.MaxOutputTokens, Temperature: call.Temperature, TopP: call.TopP, TopK: call.TopK, PresencePenalty: call.PresencePenalty, FrequencyPenalty: call.FrequencyPenalty}}
	request.Parameters.ReasoningLevel = p.parameters.ReasoningLevel
	request.Parameters.SettingsJSON = p.parameters.SettingsJSON
	request.Parameters.StopSequences = append([]string(nil), p.parameters.StopSequences...)
	seenTools := map[string]bool{}
	for _, tool := range call.Tools {
		function, ok := tool.(fantasy.FunctionTool)
		if !ok || !p.capabilities.Tools {
			return request, errors.New("extension model does not support exposed tools")
		}
		if function.Name == "" || seenTools[function.Name] {
			return request, errors.New("invalid or duplicate exposed tool name")
		}
		seenTools[function.Name] = true
		if function.InputSchema["type"] != "object" {
			return request, errors.New("tool schema must be an object")
		}
		schema, err := json.Marshal(function.InputSchema)
		if err != nil {
			return request, fmt.Errorf("invalid tool schema: %w", err)
		}
		request.Tools = append(request.Tools, ProviderTool{Name: function.Name, Description: function.Description, SchemaJSON: string(schema)})
	}
	if call.ToolChoice != nil {
		switch *call.ToolChoice {
		case fantasy.ToolChoiceAuto:
			request.ToolChoice.Type = ProviderToolAuto
		case fantasy.ToolChoiceNone:
			request.ToolChoice.Type = ProviderToolNone
		case fantasy.ToolChoiceRequired:
			request.ToolChoice.Type = ProviderToolRequired
		default:
			request.ToolChoice = ProviderToolChoice{Type: ProviderToolNamed, Name: string(*call.ToolChoice)}
			found := false
			for _, tool := range request.Tools {
				if tool.Name == request.ToolChoice.Name {
					found = true
				}
			}
			if !found {
				return request, errors.New("named tool is not exposed")
			}
		}
	}
	for _, msg := range call.Prompt {
		converted := ProviderMessage{Role: string(msg.Role)}
		if metadata := message.ExtensionOptions(msg.ProviderOptions); metadata != nil {
			if metadata.Provider != p.provider {
				return request, errors.New("unsupported extension replay namespace")
			}
			for _, replay := range metadata.Replay {
				if replay.Version != 1 || replay.Provider != p.provider || !json.Valid([]byte(replay.JSON)) {
					return request, errors.New("unsupported extension message replay metadata")
				}
				converted.Replay = append(converted.Replay, ProviderReplayData{Version: replay.Version, Provider: replay.Provider, JSON: replay.JSON})
			}
		}
		for _, part := range msg.Content {
			var item ProviderPart
			switch v := part.(type) {
			case fantasy.TextPart:
				item = ProviderPart{Type: ProviderPartText, Text: v.Text}
			case fantasy.ReasoningPart:
				if !p.capabilities.Reasoning {
					return request, errors.New("extension model does not support reasoning history")
				}
				item = ProviderPart{Type: ProviderPartReasoning, Text: v.Text}
			case fantasy.FilePart:
				allowed := false
				for _, modality := range p.capabilities.InputModalities {
					if modality == "file" || (modality == "image" && len(v.MediaType) > 6 && v.MediaType[:6] == "image/") {
						allowed = true
					}
				}
				if !allowed {
					return request, errors.New("extension model does not support input modality")
				}
				item = ProviderPart{Type: ProviderPartFile, Data: append([]byte(nil), v.Data...), MIMEType: v.MediaType, Filename: v.Filename}
			case fantasy.ToolCallPart:
				item = ProviderPart{Type: ProviderPartToolCall, CallID: v.ToolCallID, ToolName: v.ToolName, ArgumentsJSON: v.Input}
			case fantasy.ToolResultPart:
				item = ProviderPart{Type: ProviderPartToolResult, CallID: v.ToolCallID}
				switch output := v.Output.(type) {
				case fantasy.ToolResultOutputContentText:
					item.Text = output.Text
				case fantasy.ToolResultOutputContentError:
					item.IsError = true
					item.Text = output.Error.Error()
				default:
					return request, errors.New("unsupported extension tool result modality")
				}
			default:
				return request, errors.New("unsupported extension message part")
			}
			if metadata := message.ExtensionOptions(part.Options()); metadata != nil {
				if metadata.Provider != p.provider {
					return request, errors.New("unsupported extension replay namespace")
				}
				item.Signature = metadata.Signature
				for _, replay := range metadata.Replay {
					if replay.Version != 1 || replay.Provider != p.provider || !json.Valid([]byte(replay.JSON)) {
						return request, errors.New("unsupported extension replay metadata")
					}
					item.Replay = append(item.Replay, ProviderReplayData{Version: replay.Version, Provider: replay.Provider, JSON: replay.JSON})
				}
			} else if len(part.Options()) > 0 {
				return request, errors.New("unsupported extension replay metadata")
			}
			converted.Parts = append(converted.Parts, item)
		}
		request.Messages = append(request.Messages, converted)
	}
	return request, nil
}
