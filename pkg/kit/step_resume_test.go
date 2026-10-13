package kit

import (
	"context"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
)

// resumeModel fails its first stream after partial output (with an open
// <think> tag) and answers on the next call.
type resumeModel struct {
	echoModel
	mu    sync.Mutex
	calls int
}

func (m *resumeModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.calls++
	first := m.calls == 1
	m.mu.Unlock()
	parts := []fantasy.StreamPart{
		{Type: fantasy.StreamPartTypeTextStart, ID: "a"},
		{Type: fantasy.StreamPartTypeTextDelta, ID: "a", Delta: "final answer"},
		{Type: fantasy.StreamPartTypeTextEnd, ID: "a"},
		{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
	}
	if first {
		parts = []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeTextStart, ID: "a"},
			{Type: fantasy.StreamPartTypeTextDelta, ID: "a", Delta: "broken <think>half"},
			{Type: fantasy.StreamPartTypeError, Error: &fantasy.ProviderError{Title: "stream error", Message: "Our servers are currently overloaded. Please try again later."}},
		}
	}
	return iter.Seq[fantasy.StreamPart](func(yield func(fantasy.StreamPart) bool) {
		for _, p := range parts {
			if !yield(p) {
				return
			}
		}
	}), nil
}

func TestPromptResumesFailedStep(t *testing.T) {
	model := &resumeModel{}
	model.provider, model.model = "resume", "m"
	policy := RetryPolicy{MaxAttempts: 1, MaxResumes: 2, ResumeInitialDelay: time.Millisecond, ResumeMaxDelay: 2 * time.Millisecond}
	k := newProviderTestKit(t, &Options{Model: "resume/m", RetryPolicy: &policy, Providers: map[string]ProviderFactory{
		"resume": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
			return &ProviderResult{Model: model}, nil
		},
	}})
	var mu sync.Mutex
	var resumes []StepResumeEvent
	var errorEvents int
	k.Subscribe(func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		switch ev := e.(type) {
		case StepResumeEvent:
			resumes = append(resumes, ev)
		case ErrorEvent:
			errorEvents++
		}
	})
	result, err := k.PromptResult(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(resumes) != 1 || resumes[0].StepNumber != 0 || resumes[0].Resume != 1 || resumes[0].Error == nil || errorEvents != 0 {
		t.Fatalf("resumes=%+v errors=%d", resumes, errorEvents)
	}
	if result.Incomplete || result.PartialText != "" {
		t.Fatalf("result = %#v", result)
	}
	// TurnResult.Stream holds only the restarted step, and the <think>
	// parser state of the failed step did not leak into it.
	var text, reasoning strings.Builder
	for _, e := range result.Stream {
		text.WriteString(e.Text)
		reasoning.WriteString(e.Reasoning)
	}
	if text.String() != "final answer" || reasoning.String() != "" {
		t.Fatalf("stream text=%q reasoning=%q", text.String(), reasoning.String())
	}
	for _, msg := range k.session.GetMessages() {
		for _, part := range msg.Content {
			if tp, ok := part.(fantasy.TextPart); ok && strings.Contains(tp.Text, "broken") {
				t.Fatal("failed step output entered history")
			}
		}
	}
}
