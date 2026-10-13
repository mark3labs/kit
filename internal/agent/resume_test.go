package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
)

func resumeTestPolicy(maxResumes int) RetryPolicy {
	p := fastRetryPolicy()
	p.MaxAttempts = 1 // isolate the resume layer from request retries
	p.MaxResumes = maxResumes
	p.ResumeInitialDelay = time.Millisecond
	p.ResumeMaxDelay = 2 * time.Millisecond
	return p
}

func overloadError() error {
	return &fantasy.ProviderError{Title: "stream error", Message: "Our servers are currently overloaded. Please try again later."}
}

func toolStep(id string) fantasy.StreamResponse {
	return retryParts(
		fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: id, ToolCallName: "work", ToolCallInput: "{}"},
		fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls},
	)
}

func partialFailure(text string, err error) fantasy.StreamResponse {
	return retryParts(
		fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "t"},
		fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "t", Delta: text},
		fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: err},
	)
}

func answerStep(text string) fantasy.StreamResponse {
	return retryParts(
		fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "a"},
		fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "a", Delta: text},
		fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "a"},
		fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
	)
}

type resumeHarness struct {
	mu         sync.Mutex
	calls      int
	executions int
	prompts    [][]fantasy.Message
}

func newResumeAgent(h *resumeHarness, policy RetryPolicy, maxSteps int, stream func(call int) fantasy.StreamResponse) *Agent {
	model := &retryTestModel{stream: func(_ context.Context, c fantasy.Call) (fantasy.StreamResponse, error) {
		h.mu.Lock()
		h.calls++
		n := h.calls
		h.prompts = append(h.prompts, append([]fantasy.Message(nil), c.Prompt...))
		h.mu.Unlock()
		return stream(n), nil
	}}
	tool := fantasy.NewAgentTool("work", "test", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		h.mu.Lock()
		h.executions++
		h.mu.Unlock()
		return fantasy.NewTextResponse("ok"), nil
	})
	a := &Agent{model: model, retryPolicy: policy, maxSteps: maxSteps, coreTools: []fantasy.AgentTool{tool}, streamingEnabled: true}
	a.rebuildFantasyAgent()
	return a
}

// A step that streamed partial text and then failed is restarted from the
// last completed step. The completed tool step is not repeated, the partial
// text is discarded, and step numbers stay turn-wide.
func TestStepResumeAfterPartialOutput(t *testing.T) {
	h := &resumeHarness{}
	a := newResumeAgent(h, resumeTestPolicy(3), 0, func(n int) fantasy.StreamResponse {
		switch n {
		case 1:
			return toolStep("work-1")
		case 2:
			return partialFailure("half an ans", overloadError())
		default:
			return answerStep("full answer")
		}
	})
	var starts []int
	var resumes []string
	var errs []error
	var persisted []fantasy.Message
	var text string
	res, err := a.GenerateWithCallbacks(context.Background(), []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{
		OnStepStart: func(n int) { starts = append(starts, n) },
		OnStepResume: func(step, resume int, err error, delay time.Duration) {
			resumes = append(resumes, fmt.Sprintf("%d/%d", step, resume))
			if err == nil || delay <= 0 {
				t.Errorf("bad resume event: %v %v", err, delay)
			}
		},
		OnError:             func(err error) { errs = append(errs, err) },
		OnStreamingResponse: func(s string) { text += s },
		OnStepMessages:      func(m []fantasy.Message) { persisted = append(persisted, m...) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.executions != 1 || h.calls != 3 {
		t.Fatalf("executions=%d calls=%d", h.executions, h.calls)
	}
	if fmt.Sprint(starts) != "[0 1 1]" || fmt.Sprint(resumes) != "[1/1]" || len(errs) != 0 {
		t.Fatalf("starts=%v resumes=%v errs=%v", starts, resumes, errs)
	}
	if text != "half an ansfull answer" {
		t.Fatalf("stream text = %q", text)
	}
	if res.FinalResponse.Content.Text() != "full answer" || res.PartialText != "" {
		t.Fatalf("final=%q partial=%q", res.FinalResponse.Content.Text(), res.PartialText)
	}
	// user + assistant tool call + tool result + assistant answer
	if len(res.ConversationMessages) != 4 || res.PersistedMessageCount != 3 || len(persisted) != 3 {
		t.Fatalf("messages=%d persisted=%d/%d", len(res.ConversationMessages), res.PersistedMessageCount, len(persisted))
	}
	// The resumed request carries the completed tool step and no partial text.
	last := h.prompts[2]
	if got := last[len(last)-1].Role; got != fantasy.MessageRoleTool {
		t.Fatalf("resumed prompt ends with %s", got)
	}
	for _, m := range last {
		for _, p := range m.Content {
			if tp, ok := p.(fantasy.TextPart); ok && tp.Text == "half an ans" {
				t.Fatal("partial text was sent again")
			}
		}
	}
}

func TestStepResumeBudget(t *testing.T) {
	h := &resumeHarness{}
	failure := overloadError()
	a := newResumeAgent(h, resumeTestPolicy(2), 0, func(int) fantasy.StreamResponse { return partialFailure("x", failure) })
	var resumes, errs int
	res, err := a.GenerateWithCallbacks(context.Background(), []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{
		OnStepResume: func(int, int, error, time.Duration) { resumes++ },
		OnError:      func(error) { errs++ },
	})
	if !errors.Is(err, failure) || h.calls != 3 || resumes != 2 || errs != 1 {
		t.Fatalf("err=%v calls=%d resumes=%d errs=%d", err, h.calls, resumes, errs)
	}
	if !res.Incomplete || res.PartialText != "x" {
		t.Fatalf("incomplete=%v partial=%q", res.Incomplete, res.PartialText)
	}
}

func TestStepResumeSkipsPermanentErrors(t *testing.T) {
	for name, failure := range map[string]error{
		"validation": &fantasy.ProviderError{StatusCode: 400, Message: "bad request"},
		"auth":       &fantasy.ProviderError{StatusCode: 401, Message: "Our servers are currently overloaded. Please try again later."},
		"tool":       &fantasy.ToolExecutionError{ToolName: "work", Err: overloadError()},
	} {
		t.Run(name, func(t *testing.T) {
			h := &resumeHarness{}
			a := newResumeAgent(h, resumeTestPolicy(3), 0, func(int) fantasy.StreamResponse { return partialFailure("x", failure) })
			resumes := 0
			_, err := a.GenerateWithCallbacks(context.Background(), []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{
				OnStepResume: func(int, int, error, time.Duration) { resumes++ },
			})
			if err == nil || h.calls != 1 || resumes != 0 {
				t.Fatalf("err=%v calls=%d resumes=%d", err, h.calls, resumes)
			}
		})
	}
}

func TestStepResumeDisabled(t *testing.T) {
	for _, policy := range []RetryPolicy{resumeTestPolicy(0), normalizeRetryPolicy(&RetryPolicy{MaxResumes: -1})} {
		h := &resumeHarness{}
		policy.MaxAttempts = 1
		a := newResumeAgent(h, policy, 0, func(int) fantasy.StreamResponse { return partialFailure("x", overloadError()) })
		if _, err := a.GenerateWithCallbacks(context.Background(), []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{}); err == nil || h.calls != 1 {
			t.Fatalf("err=%v calls=%d", err, h.calls)
		}
	}
}

func TestStepResumeCancelDuringWait(t *testing.T) {
	h := &resumeHarness{}
	p := resumeTestPolicy(3)
	p.ResumeInitialDelay, p.ResumeMaxDelay = time.Hour, time.Hour
	a := newResumeAgent(h, p, 0, func(int) fantasy.StreamResponse { return partialFailure("x", overloadError()) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := a.GenerateWithCallbacks(ctx, []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{
		OnStepResume: func(int, int, error, time.Duration) { cancel() },
	})
	if !errors.Is(err, context.Canceled) || h.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, h.calls)
	}
}

func TestStepResumeSkipsWaitBeyondDeadline(t *testing.T) {
	h := &resumeHarness{}
	p := resumeTestPolicy(3)
	p.ResumeInitialDelay, p.ResumeMaxDelay = time.Hour, time.Hour
	a := newResumeAgent(h, p, 0, func(int) fantasy.StreamResponse { return partialFailure("x", overloadError()) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errs := 0
	_, err := a.GenerateWithCallbacks(ctx, []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{
		OnError: func(error) { errs++ },
	})
	if err == nil || h.calls != 1 || errs != 1 {
		t.Fatalf("err=%v calls=%d errs=%d", err, h.calls, errs)
	}
}

// A failed incremental write stops resumes, so history is not written twice.
func TestStepResumeStopsAfterPersistenceError(t *testing.T) {
	h := &resumeHarness{}
	a := newResumeAgent(h, resumeTestPolicy(3), 0, func(n int) fantasy.StreamResponse {
		if n == 1 {
			return toolStep("work-1")
		}
		return partialFailure("x", overloadError())
	})
	_, err := a.GenerateWithCallbacks(context.Background(), []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{
		OnStepMessagesChecked: func([]fantasy.Message) (int, error) { return 0, errors.New("disk full") },
	})
	if err == nil || h.calls != 2 {
		t.Fatalf("err=%v calls=%d", err, h.calls)
	}
}

// The step limit applies to the whole turn, also across resumes.
func TestStepResumeKeepsStepLimit(t *testing.T) {
	h := &resumeHarness{}
	a := newResumeAgent(h, resumeTestPolicy(3), 2, func(n int) fantasy.StreamResponse {
		if n == 2 {
			return partialFailure("x", overloadError())
		}
		return toolStep(fmt.Sprintf("work-%d", n))
	})
	_, err := a.GenerateWithCallbacks(context.Background(), []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	// Step 1 (tool), step 2 failed, step 2 resumed (tool), then the limit.
	if h.calls != 3 || h.executions != 2 {
		t.Fatalf("calls=%d executions=%d", h.calls, h.executions)
	}
}

func TestStepResumeTotalUsage(t *testing.T) {
	h := &resumeHarness{}
	a := newResumeAgent(h, resumeTestPolicy(3), 0, func(n int) fantasy.StreamResponse {
		switch n {
		case 1:
			return retryParts(
				fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "w", ToolCallName: "work", ToolCallInput: "{}"},
				fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls, Usage: fantasy.Usage{InputTokens: 10, OutputTokens: 1}},
			)
		case 2:
			return partialFailure("x", overloadError())
		default:
			return retryParts(
				fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "a"},
				fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "a", Delta: "done"},
				fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "a"},
				fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop, Usage: fantasy.Usage{InputTokens: 20, OutputTokens: 2}},
			)
		}
	})
	res, err := a.GenerateWithCallbacks(context.Background(), []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalUsage.InputTokens != 30 || res.TotalUsage.OutputTokens != 3 {
		t.Fatalf("usage = %v", res.TotalUsage)
	}
}

func TestPlanResumeRequiresReplayablePrompt(t *testing.T) {
	a := &Agent{retryPolicy: resumeTestPolicy(3)}
	base := resumeState{err: overloadError()}
	for role, want := range map[fantasy.MessageRole]bool{
		fantasy.MessageRoleUser: true, fantasy.MessageRoleTool: true,
		fantasy.MessageRoleAssistant: false, "": false,
	} {
		s := base
		s.lastRole = role
		if got := a.planResume(context.Background(), s) != nil; got != want {
			t.Errorf("%q: got %v want %v", role, got, want)
		}
	}
}

func TestNormalizeResumePolicy(t *testing.T) {
	d := normalizeRetryPolicy(nil)
	if d.MaxResumes != 5 || d.ResumeInitialDelay != 5*time.Second || d.ResumeMaxDelay != 80*time.Second {
		t.Fatalf("defaults: %+v", d)
	}
	if got := normalizeRetryPolicy(&RetryPolicy{MaxResumes: -1}); got.MaxResumes != 0 {
		t.Fatalf("disable: %+v", got)
	}
	got := normalizeRetryPolicy(&RetryPolicy{MaxResumes: 2, ResumeInitialDelay: time.Minute, ResumeMaxDelay: time.Second})
	if got.MaxResumes != 2 || got.ResumeInitialDelay != time.Second {
		t.Fatalf("normalized: %+v", got)
	}
}
