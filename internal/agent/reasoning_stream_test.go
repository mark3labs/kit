package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"

	"github.com/mark3labs/kit/internal/message"
)

// Regression guard for issue #124: some OpenAI-compatible providers (e.g.
// glm-5.3-flash, DeepSeek, vLLM, SGLang) put "reasoning_content": null on
// every chunk that carries answer text, a tool call or the finish reason. A
// provider that reads a present-but-null key as "reasoning is still running"
// never closes the reasoning block, so:
//
//   - OnReasoningComplete (and therefore ReasoningCompleteEvent) never fires,
//     and
//   - the assistant message carries no reasoning part, so sessions persist
//     nothing and the reasoning the user watched stream live disappears on
//     reload.
//
// The stream hook lives in the provider library, so these tests pin the
// behaviour Kit depends on: they drive Kit's own callback plumbing and message
// conversion over a replayed SSE stream and fail loudly if a dependency bump
// reintroduces the leak.

// glmFlashNullReasoningSSE replays the shape reported in issue #124: reasoning
// deltas, then an answer chunk that repeats the reasoning key with a null
// value, then a finish chunk that does the same.
const glmFlashNullReasoningSSE = `{"id":"x","created":1,"model":"glm-flash","choices":[{"index":0,"delta":{"role":"assistant","content":"","reasoning_content":"17 times 23"},"finish_reason":null}]}
{"id":"x","created":1,"model":"glm-flash","choices":[{"index":0,"delta":{"content":null,"reasoning_content":" is 391."},"finish_reason":null}]}
{"id":"x","created":1,"model":"glm-flash","choices":[{"index":0,"delta":{"content":"391","reasoning_content":null},"finish_reason":null}]}
{"id":"x","created":1,"model":"glm-flash","choices":[{"index":0,"delta":{"content":"","reasoning_content":null},"finish_reason":"stop"}]}`

// glmOmittedReasoningSSE replays the control shape (glm-5.2 in the issue): the
// reasoning key is simply absent once the answer starts.
const glmOmittedReasoningSSE = `{"id":"x","created":1,"model":"glm","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"17 times 23"},"finish_reason":null}]}
{"id":"x","created":1,"model":"glm","choices":[{"index":0,"delta":{"reasoning_content":" is 391."},"finish_reason":null}]}
{"id":"x","created":1,"model":"glm","choices":[{"index":0,"delta":{"content":"391"},"finish_reason":null}]}
{"id":"x","created":1,"model":"glm","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}]}`

// serveReasoningSSE starts a server that answers a streaming chat-completions
// POST with the given SSE payload, one event per line.
func serveReasoningSSE(t *testing.T, payload string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		rc := http.NewResponseController(w)
		for event := range strings.SplitSeq(payload, "\n") {
			event = strings.TrimSpace(event)
			if event == "" {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", event)
			_ = rc.Flush()
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		_ = rc.Flush()
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newSSEAgent builds an Agent backed by the OpenAI-compatible provider pointed
// at srv. It bypasses NewAgent (and therefore the model registry and network)
// while keeping the real provider, the real fantasy agent, and Kit's own
// callback and message-conversion code in the path.
func newSSEAgent(ctx context.Context, t *testing.T, srv *httptest.Server) *Agent {
	t.Helper()
	provider, err := openaicompat.New(
		openaicompat.WithBaseURL(srv.URL),
		openaicompat.WithAPIKey("test"),
		openaicompat.WithName("test-compat"),
	)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	model, err := provider.LanguageModel(ctx, "test-model")
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	return &Agent{
		fantasyAgent:     fantasy.NewAgent(model),
		model:            model,
		maxSteps:         1,
		streamingEnabled: true,
		providerType:     "test-compat",
	}
}

// reasoningRun holds what one generation observed, so both SSE shapes can be
// asserted the same way.
type reasoningRun struct {
	starts    int
	completes int
	deltas    string
	response  string
	// reasoningParts counts fantasy.ReasoningPart values in assistant
	// messages — this is what a SessionManager persists.
	reasoningParts int
	// blockText is the reasoning text on the converted message blocks.
	blockText string
}

func runReasoningStream(ctx context.Context, t *testing.T, payload string) reasoningRun {
	t.Helper()
	a := newSSEAgent(ctx, t, serveReasoningSSE(t, payload))

	var run reasoningRun
	var deltas strings.Builder
	result, err := a.GenerateWithCallbacks(ctx,
		[]fantasy.Message{fantasy.NewUserMessage("what is 17*23?")},
		GenerateCallbacks{
			OnReasoningStart:    func(string) { run.starts++ },
			OnReasoningDelta:    func(d string) { deltas.WriteString(d) },
			OnReasoningComplete: func() { run.completes++ },
		})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	run.deltas = deltas.String()
	run.response = result.FinalResponse.Content.Text()

	for _, m := range result.ConversationMessages {
		if m.Role != fantasy.MessageRoleAssistant {
			continue
		}
		for _, part := range m.Content {
			if _, ok := part.(fantasy.ReasoningPart); ok {
				run.reasoningParts++
			}
		}
	}
	for _, m := range result.Messages {
		if m.Role != message.RoleAssistant {
			continue
		}
		run.blockText += m.Reasoning().Thinking
	}
	return run
}

// TestReasoningClosesOnNullReasoningField covers the reported failure: a null
// reasoning field on the answer chunk must not hold the reasoning block open.
func TestReasoningClosesOnNullReasoningField(t *testing.T) {
	t.Parallel()
	assertReasoningComplete(t, runReasoningStream(t.Context(), t, glmFlashNullReasoningSSE))
}

// TestReasoningClosesOnOmittedReasoningField is the control case: providers
// that drop the key entirely once the answer starts already worked.
func TestReasoningClosesOnOmittedReasoningField(t *testing.T) {
	t.Parallel()
	assertReasoningComplete(t, runReasoningStream(t.Context(), t, glmOmittedReasoningSSE))
}

func assertReasoningComplete(t *testing.T, run reasoningRun) {
	t.Helper()
	const wantReasoning = "17 times 23 is 391."
	if run.starts != 1 {
		t.Errorf("OnReasoningStart fired %d times, want 1", run.starts)
	}
	if run.deltas != wantReasoning {
		t.Errorf("reasoning deltas = %q, want %q", run.deltas, wantReasoning)
	}
	if run.completes != 1 {
		t.Errorf("OnReasoningComplete fired %d times, want 1 "+
			"(reasoning block never closed — see issue #124)", run.completes)
	}
	if run.reasoningParts != 1 {
		t.Errorf("assistant messages carry %d reasoning parts, want 1 "+
			"(reasoning would not be persisted — see issue #124)", run.reasoningParts)
	}
	if run.blockText != wantReasoning {
		t.Errorf("converted reasoning block = %q, want %q", run.blockText, wantReasoning)
	}
	if run.response != "391" {
		t.Errorf("response = %q, want %q", run.response, "391")
	}
}

// TestHasReasoningFieldNullContract documents the provider-side rule Kit
// relies on: a null reasoning value means "no reasoning in this delta", while
// an empty string still counts as reasoning.
func TestHasReasoningFieldNullContract(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{`{"content":"391","reasoning_content":null}`, false},
		{`{"content":"391","reasoning":null}`, false},
		{`{"content":"391"}`, false},
		{`{"reasoning_content":""}`, true},
		{`{"reasoning_content":"thinking"}`, true},
	} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(tc.raw), &fields); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.raw, err)
		}
		got := false
		for _, key := range []string{"reasoning_content", "reasoning"} {
			if v, ok := fields[key]; ok && strings.TrimSpace(string(v)) != "null" {
				got = true
				break
			}
		}
		if got != tc.want {
			t.Errorf("reasoning field present for %s = %v, want %v", tc.raw, got, tc.want)
		}
	}
}
