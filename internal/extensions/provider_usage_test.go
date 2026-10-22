package extensions

import (
	"context"
	"testing"

	"charm.land/fantasy"
)

func TestProviderCumulativeUsageCountedOnce(t *testing.T) {
	runtime := ProviderRuntime{Stream: func(_ ProviderRequest, emit func(ProviderEvent) bool) ProviderResult {
		emit(ProviderEvent{Type: ProviderUsageEvent, Usage: ProviderUsage{InputTokens: 10, OutputTokens: 1, CacheReadTokens: 3}})
		emit(ProviderEvent{Type: ProviderUsageEvent, Usage: ProviderUsage{InputTokens: 10, OutputTokens: 2, CacheReadTokens: 3}})
		emit(ProviderEvent{Type: ProviderFinish, FinishReason: ProviderFinishStop})
		return ProviderResult{}
	}}
	finishes := 0
	err := runProviderStream(context.Background(), runtime, ProviderRequest{}, func(part fantasy.StreamPart) bool {
		if part.Type == fantasy.StreamPartTypeFinish {
			finishes++
			if part.Usage.InputTokens != 10 || part.Usage.OutputTokens != 2 || part.Usage.CacheReadTokens != 3 {
				t.Errorf("usage=%+v", part.Usage)
			}
		} else if part.Usage != (fantasy.Usage{}) {
			t.Error("usage forwarded outside finish")
		}
		return true
	})
	if err != nil || finishes != 1 {
		t.Fatalf("err=%v finishes=%d", err, finishes)
	}
}
