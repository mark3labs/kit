package extensions

import (
	"context"
	"errors"
	"testing"

	"charm.land/fantasy"
)

func TestProviderRetryOnlyBeforeExposure(t *testing.T) {
	for _, kind := range []ProviderEventType{"", ProviderTextStart, ProviderReasoningStart, ProviderToolStart} {
		t.Run(string(kind), func(t *testing.T) {
			runtime := ProviderRuntime{Stream: func(_ ProviderRequest, emit func(ProviderEvent) bool) ProviderResult {
				if kind != "" {
					emit(ProviderEvent{Type: kind, ID: "block", ToolName: "add"})
					if kind == ProviderTextStart {
						emit(ProviderEvent{Type: ProviderTextDelta, ID: "block", Delta: "visible"})
					}
				}
				return ProviderResult{Error: &ProviderError{Code: "transport", Retryable: true}}
			}}
			err := runProviderStream(context.Background(), runtime, ProviderRequest{Tools: []ProviderTool{{Name: "add"}}}, func(fantasy.StreamPart) bool { return true })
			var classified interface{ ExtensionRetryable() bool }
			if !errors.As(err, &classified) {
				t.Fatalf("unclassified %v", err)
			}
			if classified.ExtensionRetryable() != (kind == "") {
				t.Fatalf("retryable=%v kind=%s", classified.ExtensionRetryable(), kind)
			}
		})
	}
}
