package extensions

import (
	"context"
	"errors"
	"testing"

	"charm.land/fantasy"
)

func TestProviderStreamProtocol(t *testing.T) {
	text := []ProviderEvent{{Type: ProviderTextStart, ID: "t"}, {Type: ProviderTextDelta, ID: "t", Delta: "hello"}, {Type: ProviderTextEnd, ID: "t"}, {Type: ProviderFinish, FinishReason: ProviderFinishStop}}
	tools := []ProviderEvent{{Type: ProviderToolStart, ID: "call/1", ToolName: "add"}, {Type: ProviderToolDelta, ID: "call/1", Delta: `{"a":2,`}, {Type: ProviderToolDelta, ID: "call/1", Delta: `"b":3}`}, {Type: ProviderToolEnd, ID: "call/1"}, {Type: ProviderFinish, FinishReason: ProviderFinishToolCalls}}
	tests := []struct {
		name       string
		events     []ProviderEvent
		result     ProviderResult
		valid      bool
		executable int
	}{
		{name: "greeting", events: text, valid: true},
		{name: "signature only reasoning", events: []ProviderEvent{{Type: ProviderReasoningStart, ID: "r"}, {Type: ProviderReasoningEnd, ID: "r", Signature: "signature"}, {Type: ProviderFinish, FinishReason: ProviderFinishStop}}, valid: true},
		{name: "empty", events: []ProviderEvent{{Type: ProviderFinish, FinishReason: ProviderFinishStop}}, valid: true},
		{name: "tools", events: tools, valid: true, executable: 1},
		{name: "delta before start", events: text[1:]},
		{name: "missing finish", events: text[:3]},
		{name: "post finish", events: append(append([]ProviderEvent{}, text...), ProviderEvent{Type: ProviderTextStart, ID: "x"})},
		{name: "repeated finish", events: []ProviderEvent{{Type: ProviderFinish, FinishReason: ProviderFinishStop}, {Type: ProviderFinish, FinishReason: ProviderFinishStop}}},
		{name: "duplicate block", events: []ProviderEvent{{Type: ProviderTextStart, ID: "t"}, {Type: ProviderTextStart, ID: "t"}}},
		{name: "wrong block", events: []ProviderEvent{{Type: ProviderTextStart, ID: "t"}, {Type: ProviderToolDelta, ID: "t"}}},
		{name: "open block", events: []ProviderEvent{{Type: ProviderTextStart, ID: "t"}, {Type: ProviderFinish, FinishReason: ProviderFinishStop}}},
		{name: "unknown tool", events: []ProviderEvent{{Type: ProviderToolStart, ID: "c", ToolName: "hidden"}}},
		{name: "invalid arguments", events: []ProviderEvent{{Type: ProviderToolStart, ID: "c", ToolName: "add"}, {Type: ProviderToolDelta, ID: "c", Delta: `[]`}, {Type: ProviderToolEnd, ID: "c"}}},
		{name: "late error", events: tools, result: ProviderResult{Error: &ProviderError{Code: "transport", Retryable: true}}},
		{name: "usage decreases", events: []ProviderEvent{{Type: ProviderUsageEvent, Usage: ProviderUsage{InputTokens: 2}}, {Type: ProviderUsageEvent, Usage: ProviderUsage{InputTokens: 1}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := ProviderRuntime{Stream: func(_ ProviderRequest, emit func(ProviderEvent) bool) ProviderResult {
				for _, event := range tt.events {
					emit(event)
				}
				return tt.result
			}}
			var got []fantasy.StreamPart
			err := runProviderStream(context.Background(), runtime, ProviderRequest{Tools: []ProviderTool{{Name: "add"}}}, func(p fantasy.StreamPart) bool { got = append(got, p); return true })
			if (err == nil) != tt.valid {
				t.Fatalf("err=%v valid=%v", err, tt.valid)
			}
			calls, finishes := 0, 0
			for _, part := range got {
				if part.Type == fantasy.StreamPartTypeToolCall {
					calls++
				}
				if part.Type == fantasy.StreamPartTypeFinish {
					finishes++
				}
			}
			if calls != tt.executable || (!tt.valid && finishes != 0) {
				t.Fatalf("executable=%d finish=%d", calls, finishes)
			}
		})
	}
}

func TestProviderStreamConcurrentEmitRejected(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	runtime := ProviderRuntime{Stream: func(_ ProviderRequest, emit func(ProviderEvent) bool) ProviderResult {
		done := make(chan struct{})
		go func() { defer close(done); emit(ProviderEvent{Type: ProviderTextStart, ID: "one"}) }()
		<-entered
		if emit(ProviderEvent{Type: ProviderTextStart, ID: "two"}) {
			t.Error("concurrent emit accepted")
		}
		close(release)
		<-done
		return ProviderResult{}
	}}
	err := runProviderStream(context.Background(), runtime, ProviderRequest{}, func(fantasy.StreamPart) bool { close(entered); <-release; return true })
	if err == nil {
		t.Fatal("concurrent emit did not fail")
	}
}

func TestProviderStreamConsumerStopCancelsDone(t *testing.T) {
	observed := false
	runtime := ProviderRuntime{Stream: func(req ProviderRequest, emit func(ProviderEvent) bool) ProviderResult {
		if emit(ProviderEvent{Type: ProviderTextStart, ID: "t"}) {
			t.Error("consumer stop ignored")
		}
		select {
		case <-req.Done:
			observed = true
		default:
			t.Error("Done not canceled")
		}
		if emit(ProviderEvent{Type: ProviderFinish, FinishReason: ProviderFinishStop}) {
			t.Error("accepted after stop")
		}
		return ProviderResult{}
	}}
	err := runProviderStream(context.Background(), runtime, ProviderRequest{}, func(fantasy.StreamPart) bool { return false })
	if !errors.Is(err, context.Canceled) || !observed {
		t.Fatalf("err=%v observed=%v", err, observed)
	}
}
