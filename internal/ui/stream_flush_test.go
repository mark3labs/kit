package ui

import (
	"testing"
	"time"
)

func TestAdaptiveStreamFlushInterval(t *testing.T) {
	tests := []struct {
		name string
		cost time.Duration
		want time.Duration
	}{
		{"no render yet keeps base rate", 0, streamFlushInterval},
		{"cheap render keeps base rate", 2 * time.Millisecond, streamFlushInterval},
		{"costly render widens window", 30 * time.Millisecond, 60 * time.Millisecond},
		{"very costly render is capped", time.Second, maxStreamFlushInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adaptiveStreamFlushInterval(tt.cost); got != tt.want {
				t.Errorf("adaptiveStreamFlushInterval(%v) = %v, want %v", tt.cost, got, tt.want)
			}
		})
	}
}

// TestStreamFlushDelay_UsesStreamingItemCost checks that the flush window is
// taken from the message that is currently streaming, and falls back to the
// base rate when the last item is not a streaming message.
func TestStreamFlushDelay_UsesStreamingItemCost(t *testing.T) {
	m := &AppModel{}
	if got := m.streamFlushDelay(); got != streamFlushInterval {
		t.Fatalf("empty transcript: delay = %v, want %v", got, streamFlushInterval)
	}

	item := NewStreamingMessageItem("s1", "assistant", "model")
	item.renderCost = 40 * time.Millisecond
	m.messages = []MessageItem{item}
	if got, want := m.streamFlushDelay(), 80*time.Millisecond; got != want {
		t.Fatalf("streaming item: delay = %v, want %v", got, want)
	}
}

// TestStreamingMessageItem_RecordsRenderCost checks that a real render sets
// the cost the flush scheduler depends on.
func TestStreamingMessageItem_RecordsRenderCost(t *testing.T) {
	for _, role := range []string{"assistant", "reasoning"} {
		t.Run(role, func(t *testing.T) {
			item := NewStreamingMessageItem("s1", role, "model")
			item.AppendChunk("some **markdown** text\n\n- a\n- b\n")
			if item.RenderCost() != 0 {
				t.Fatalf("cost before render = %v, want 0", item.RenderCost())
			}
			_ = item.Render(80)
			if item.RenderCost() <= 0 {
				t.Fatalf("cost after render = %v, want > 0", item.RenderCost())
			}
		})
	}
}
