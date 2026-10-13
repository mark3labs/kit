package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
)

func TestRetryStreamOverloadAfterMetadata(t *testing.T) {
	for _, typ := range []fantasy.StreamPartType{fantasy.StreamPartTypeWarnings, fantasy.StreamPartTypeKeepalive, fantasy.StreamPartTypeTextStart, fantasy.StreamPartTypeReasoningStart, fantasy.StreamPartTypeToolInputStart} {
		t.Run(string(typ), func(t *testing.T) {
			calls := 0
			failure := &fantasy.ProviderError{Title: "stream error", Message: "Our servers are currently overloaded. Please try again later."}
			model := &retryTestModel{stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				if calls == 1 {
					return retryParts(fantasy.StreamPart{Type: typ, ID: "failed"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: failure}), nil
				}
				return retryParts(
					fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "answer"},
					fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "answer", Delta: "done"},
					fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish},
				), nil
			}}
			m := &retryModel{LanguageModel: model, policy: fastRetryPolicy()}
			parts := collectRetryStream(t, m, context.Background())
			if calls != 2 {
				t.Fatalf("calls=%d parts=%v", calls, parts)
			}
			if typ == fantasy.StreamPartTypeKeepalive {
				parts = parts[1:]
			}
			if len(parts) != 3 || parts[0].ID != "answer" || parts[1].Delta != "done" || parts[2].Type != fantasy.StreamPartTypeFinish {
				t.Fatalf("unexpected parts: %v", parts)
			}
		})
	}
}
