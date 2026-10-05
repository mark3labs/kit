package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"charm.land/fantasy"
)

// A transport timeout can wrap DeadlineExceeded without ending the turn context.
type requestTimeoutError struct{}

func (requestTimeoutError) Error() string   { return "request idle timeout" }
func (requestTimeoutError) Unwrap() error   { return context.DeadlineExceeded }
func (requestTimeoutError) Timeout() bool   { return true }
func (requestTimeoutError) Temporary() bool { return true }

type retryTestModel struct {
	fantasy.LanguageModel
	generate func(context.Context, fantasy.Call) (*fantasy.Response, error)
	stream   func(context.Context, fantasy.Call) (fantasy.StreamResponse, error)
}

func (m *retryTestModel) Generate(ctx context.Context, c fantasy.Call) (*fantasy.Response, error) {
	return m.generate(ctx, c)
}
func (m *retryTestModel) Stream(ctx context.Context, c fantasy.Call) (fantasy.StreamResponse, error) {
	return m.stream(ctx, c)
}
func (m *retryTestModel) Provider() string { return "test" }
func (m *retryTestModel) Model() string    { return "test" }
func fastRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 3, InitialDelay: time.Millisecond, MaxDelay: 4 * time.Millisecond, MaxElapsed: time.Second}
}
func retryTestError() error { return &fantasy.ProviderError{StatusCode: 429, Message: "rate limit"} }
func retryParts(parts ...fantasy.StreamPart) fantasy.StreamResponse {
	return func(yield func(fantasy.StreamPart) bool) {
		for _, p := range parts {
			if !yield(p) {
				return
			}
		}
	}
}
func collectRetryStream(t *testing.T, m *retryModel, ctx context.Context) []fantasy.StreamPart {
	t.Helper()
	stream, err := m.Stream(ctx, fantasy.Call{})
	if err != nil {
		t.Fatal(err)
	}
	var parts []fantasy.StreamPart
	for p := range stream {
		parts = append(parts, p)
	}
	return parts
}

func TestRetryGenerateBudgetAndEvents(t *testing.T) {
	calls := 0
	failure := retryTestError()
	model := &retryTestModel{generate: func(context.Context, fantasy.Call) (*fantasy.Response, error) { calls++; return nil, failure }}
	m := &retryModel{LanguageModel: model, policy: fastRetryPolicy()}
	var attempts []int
	ctx := context.WithValue(context.Background(), retryContextKey{}, retryNotification(func(attempt int, err error, delay time.Duration) {
		attempts = append(attempts, attempt)
		if err != failure || delay <= 0 || delay > m.policy.MaxDelay {
			t.Errorf("bad retry event: %d %v %v", attempt, err, delay)
		}
	}))
	_, err := m.Generate(ctx, fantasy.Call{})
	if !errors.Is(err, failure) || calls != 3 || fmt.Sprint(attempts) != "[1 2]" {
		t.Fatalf("calls=%d attempts=%v err=%v", calls, attempts, err)
	}
	// Each model request gets a fresh budget and retry counter.
	_, _ = m.Generate(ctx, fantasy.Call{})
	if calls != 6 || fmt.Sprint(attempts) != "[1 2 1 2]" {
		t.Fatalf("calls=%d attempts=%v", calls, attempts)
	}
}

func TestRetryStreamPreOutputBudget(t *testing.T) {
	calls := 0
	model := &retryTestModel{stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
		calls++
		switch calls {
		case 1:
			return nil, retryTestError()
		case 2:
			return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: retryTestError()}), nil
		default:
			return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish}), nil
		}
	}}
	m := &retryModel{LanguageModel: model, policy: fastRetryPolicy()}
	parts := collectRetryStream(t, m, context.Background())
	if calls != 3 || len(parts) != 1 || parts[0].Type != fantasy.StreamPartTypeFinish {
		t.Fatalf("calls=%d parts=%v", calls, parts)
	}
}

func TestRetryStreamNeverReplaysDeliveredParts(t *testing.T) {
	for _, typ := range []fantasy.StreamPartType{fantasy.StreamPartTypeTextStart, fantasy.StreamPartTypeTextDelta, fantasy.StreamPartTypeReasoningDelta, fantasy.StreamPartTypeToolCall} {
		t.Run(string(typ), func(t *testing.T) {
			calls := 0
			failure := retryTestError()
			model := &retryTestModel{stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				return retryParts(fantasy.StreamPart{Type: typ, Delta: "partial"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: failure}), nil
			}}
			m := &retryModel{LanguageModel: model, policy: fastRetryPolicy()}
			parts := collectRetryStream(t, m, context.Background())
			if calls != 1 || len(parts) != 2 || !errors.Is(parts[1].Error, failure) {
				t.Fatalf("calls=%d parts=%v", calls, parts)
			}
		})
	}
}

func TestRetryStreamIncompleteAndConsumerStop(t *testing.T) {
	calls := 0
	model := &retryTestModel{stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) { calls++; return retryParts(), nil }}
	m := &retryModel{LanguageModel: model, policy: fastRetryPolicy()}
	parts := collectRetryStream(t, m, context.Background())
	if calls != 3 || len(parts) != 1 || parts[0].Error == nil {
		t.Fatalf("calls=%d parts=%v", calls, parts)
	}
	calls = 0
	model.stream = func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
		calls++
		return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart}), nil
	}
	stream, _ := m.Stream(context.Background(), fantasy.Call{})
	for range stream {
		break
	}
	if calls != 1 {
		t.Fatalf("consumer stop retried %d times", calls)
	}
	parts = collectRetryStream(t, m, context.Background())
	if calls != 2 || len(parts) != 2 || parts[1].Error == nil {
		t.Fatalf("incomplete output retried: calls=%d parts=%v", calls, parts)
	}
}

func TestRetryClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{retryTestError(), true}, {&fantasy.ProviderError{StatusCode: 503}, true},
		{fmt.Errorf("wrapped: %w", retryTestError()), true},
		{&fantasy.ProviderError{StatusCode: 401, TransientError: true}, false},
		{&fantasy.ProviderError{StatusCode: 429, AuthError: true}, false},
		{&fantasy.ProviderError{StatusCode: 400}, false},
		{&fantasy.ProviderError{StatusCode: 403, TransientError: true}, false},
		{fmt.Errorf("wrapped deadline: %w", context.DeadlineExceeded), false},
		{requestTimeoutError{}, true},
		{context.Canceled, false}, {context.DeadlineExceeded, false}, {errors.New("validation"), false},
	} {
		if got := retryableRequestError(tc.err); got != tc.want {
			t.Errorf("%v: got %v want %v", tc.err, got, tc.want)
		}
	}
}

func TestRetryAfterAndBounds(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, tc := range []struct {
		headers map[string]string
		want    time.Duration
		valid   bool
	}{
		{map[string]string{"ReTrY-AfTeR": "2"}, 2 * time.Second, true},
		{map[string]string{"retry-after-ms": "1500", "retry-after": "3"}, 1500 * time.Millisecond, true},
		{map[string]string{"Retry-After": now.Add(5 * time.Second).Format(http.TimeFormat)}, 5 * time.Second, true},
		{map[string]string{"retry-after": "NaN"}, 0, false},
		{map[string]string{"retry-after": "-1"}, 0, false},
		{map[string]string{"retry-after": "nonsense"}, 0, false},
	} {
		err := &fantasy.ProviderError{StatusCode: 429, ResponseHeaders: tc.headers}
		got, valid := retryAfter(err, now)
		if got != tc.want || valid != tc.valid {
			t.Errorf("%v: got %v,%v want %v,%v", tc.headers, got, valid, tc.want, tc.valid)
		}
	}
	p := fastRetryPolicy()
	if _, ok := retryDelay(p, 1, &fantasy.ProviderError{StatusCode: 429, ResponseHeaders: map[string]string{"retry-after": "1e100"}}, now); ok {
		t.Fatal("oversized hint accepted")
	}
	for attempt := 1; attempt < 100; attempt++ {
		delay, ok := retryDelay(p, attempt, retryTestError(), now)
		if !ok || delay < p.InitialDelay/2 || delay > p.MaxDelay {
			t.Fatalf("attempt %d: %v %v", attempt, delay, ok)
		}
	}
}

func TestRetryCancellationAndSchedulingLimits(t *testing.T) {
	failure := retryTestError()
	for _, mode := range []string{"disabled", "elapsed", "deadline", "canceled", "cancel-wait"} {
		t.Run(mode, func(t *testing.T) {
			p := fastRetryPolicy()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			switch mode {
			case "disabled":
				p.MaxAttempts = 1
			case "elapsed":
				p.MaxElapsed = time.Nanosecond
			case "deadline":
				var c context.CancelFunc
				ctx, c = context.WithTimeout(ctx, 100*time.Microsecond)
				defer c()
				p.InitialDelay = time.Second
				p.MaxDelay = time.Second
			case "canceled":
				cancel()
			case "cancel-wait":
				ctx = context.WithValue(ctx, retryContextKey{}, retryNotification(func(int, error, time.Duration) { cancel() }))
			}
			model := &retryTestModel{generate: func(context.Context, fantasy.Call) (*fantasy.Response, error) { calls++; return nil, failure }}
			m := &retryModel{LanguageModel: model, policy: p}
			_, err := m.Generate(ctx, fantasy.Call{})
			if err == nil || calls > 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			if (mode == "canceled" || mode == "cancel-wait") && !errors.Is(err, context.Canceled) {
				t.Fatalf("want cancellation, got %v", err)
			}
		})
	}
}

// Exercise the real agent loop: retries in step two must not execute step one's
// tool a second time. Rebuild must retain the policy, and per-call zero retries
// must prevent a second library retry layer after the Kit budget is exhausted.
func TestRetryPerStepDoesNotReplayTools(t *testing.T) {
	for _, exhaust := range []bool{false, true} {
		t.Run(fmt.Sprint(exhaust), func(t *testing.T) {
			calls, executions := 0, 0
			failure := retryTestError()
			model := &retryTestModel{stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				if calls == 1 {
					return retryParts(
						fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "work-1", ToolCallName: "work", ToolCallInput: "{}"},
						fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls},
					), nil
				}
				if exhaust || calls == 2 {
					return nil, failure
				}
				return retryParts(
					fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "answer"},
					fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "answer", Delta: "done"},
					fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "answer"},
					fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
				), nil
			}}
			tool := fantasy.NewAgentTool("work", "test", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
				executions++
				return fantasy.NewTextResponse("ok"), nil
			})
			a := &Agent{model: model, retryPolicy: fastRetryPolicy(), coreTools: []fantasy.AgentTool{tool}, streamingEnabled: true}
			a.rebuildFantasyAgent()
			var events []int
			var text string
			_, err := a.GenerateWithCallbacks(context.Background(), []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{
				OnRetryScheduled:    func(attempt int, err error, delay time.Duration) { events = append(events, attempt) },
				OnStreamingResponse: func(chunk string) { text += chunk },
			})
			if executions != 1 {
				t.Fatalf("tool executed %d times", executions)
			}
			if exhaust {
				if err == nil || calls != 4 || fmt.Sprint(events) != "[1 2]" {
					t.Fatalf("calls=%d events=%v err=%v", calls, events, err)
				}
			} else if err != nil || calls != 3 || text != "done" || fmt.Sprint(events) != "[1]" {
				t.Fatalf("calls=%d events=%v text=%q err=%v", calls, events, text, err)
			}
		})
	}
}

func TestNormalizeRetryPolicy(t *testing.T) {
	if got := normalizeRetryPolicy(nil); got != DefaultRetryPolicy() {
		t.Fatalf("defaults: %v", got)
	}
	p := RetryPolicy{MaxAttempts: 1, InitialDelay: time.Minute, MaxDelay: time.Second}
	got := normalizeRetryPolicy(&p)
	if got.MaxAttempts != 1 || got.InitialDelay != time.Second || got.MaxElapsed != DefaultRetryPolicy().MaxElapsed {
		t.Fatalf("normalized: %v", got)
	}
}

func TestRetryAgentDoesNotReplayMidStream(t *testing.T) {
	calls := 0
	model := &retryTestModel{stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
		calls++
		return retryParts(
			fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "answer"},
			fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "answer", Delta: "partial"},
			fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: retryTestError()},
		), nil
	}}
	a := &Agent{model: model, retryPolicy: fastRetryPolicy(), streamingEnabled: true}
	a.rebuildFantasyAgent()
	var text string
	retries := 0
	_, err := a.GenerateWithCallbacks(context.Background(), []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{
		OnStreamingResponse: func(chunk string) { text += chunk },
		OnRetry:             func(int, error) { retries++ },
	})
	if err == nil || calls != 1 || retries != 0 || text != "partial" {
		t.Fatalf("calls=%d retries=%d text=%q err=%v", calls, retries, text, err)
	}
}

func TestRetrySimpleAgentUsesSingleBudget(t *testing.T) {
	calls := 0
	failure := retryTestError()
	model := &retryTestModel{generate: func(context.Context, fantasy.Call) (*fantasy.Response, error) { calls++; return nil, failure }}
	a := &Agent{model: model, retryPolicy: fastRetryPolicy()}
	a.rebuildFantasyAgent()
	_, err := a.GenerateWithCallbacks(context.Background(), []fantasy.Message{fantasy.NewUserMessage("go")}, GenerateCallbacks{})
	if err == nil || calls != 3 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
