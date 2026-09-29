package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/media"
)

// rejectingServer serves scripted SSE turns, but answers any request that
// carries an image with the given HTTP status and body. This simulates a
// provider that rejects a media tool result (413 request too large, or 400
// for a model without vision support).
type rejectingServer struct {
	*httptest.Server
	mu       sync.Mutex
	bodies   [][]byte
	payloads []string
	turn     int
	rejected int
}

func newRejectingServer(t *testing.T, status int, errBody string, payloads ...string) *rejectingServer {
	t.Helper()
	rs := &rejectingServer{payloads: payloads}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rs.mu.Lock()
		rs.bodies = append(rs.bodies, body)
		if strings.Contains(string(body), `"image_url"`) {
			rs.rejected++
			rs.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = io.WriteString(w, errBody)
			return
		}
		payload := answerSSE
		if rs.turn < len(rs.payloads) {
			payload = rs.payloads[rs.turn]
		}
		rs.turn++
		rs.mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		rc := http.NewResponseController(w)
		for event := range strings.SplitSeq(payload, "\n") {
			if event = strings.TrimSpace(event); event == "" {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", event)
			_ = rc.Flush()
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		_ = rc.Flush()
	}))
	t.Cleanup(rs.Close)
	return rs
}

const requestTooLargeBody = `{"error":{"type":"request_too_large","message":"Request exceeds the maximum size"}}`

// mediaResultsIn counts media and rejected-media tool results in msgs.
func mediaResultsIn(msgs []fantasy.Message) (mediaCount, rejected int, rejectedText string) {
	for _, m := range msgs {
		for _, part := range m.Content {
			trp, ok := part.(fantasy.ToolResultPart)
			if !ok {
				continue
			}
			switch out := trp.Output.(type) {
			case fantasy.ToolResultOutputContentMedia:
				mediaCount++
			case fantasy.ToolResultOutputContentError:
				if strings.Contains(out.Error.Error(), mediaRejectedPrefix) {
					rejected++
					rejectedText = out.Error.Error()
				}
			}
		}
	}
	return mediaCount, rejected, rejectedText
}

// TestE2EReadImageRejectedByProviderRecovers checks that when the provider
// rejects the request that carries an image (413 request too large), the
// turn does not fail: the image is replaced by an error result that the
// model can read, and the turn continues.
func TestE2EReadImageRejectedByProviderRecovers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		advice string
	}{
		{"413 request too large", http.StatusRequestEntityTooLarge, requestTooLargeBody, "The request was too large"},
		{"400 image not supported", http.StatusBadRequest, `{"error":{"type":"invalid_request_error","message":"image input is not supported"}}`, "may not accept this media"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			path := writeTestPNG(t, t.TempDir(), "shot.png", 64, 64, false)

			srv := newRejectingServer(t, tc.status, tc.body, toolCallSSE(path), answerSSE)
			a := newImageAgent(ctx, t, srv.Server, media.Limits{})

			var persisted []fantasy.Message
			var warnings []string
			var turnErrors []error
			result, err := a.GenerateWithCallbacks(ctx,
				[]fantasy.Message{fantasy.NewUserMessage("look at shot.png")},
				GenerateCallbacks{
					OnStepMessages: func(msgs []fantasy.Message) { persisted = append(persisted, msgs...) },
					OnWarnings:     func(w []string) { warnings = append(warnings, w...) },
					OnError:        func(err error) { turnErrors = append(turnErrors, err) },
				})
			if err != nil {
				t.Fatalf("generate: %v; a rejected image must not end the turn", err)
			}
			if srv.rejected != 1 {
				t.Errorf("rejected requests = %d, want 1", srv.rejected)
			}
			if len(turnErrors) != 0 {
				t.Errorf("OnError fired %d times for a recovered rejection: %v", len(turnErrors), turnErrors)
			}
			if len(warnings) != 1 {
				t.Errorf("warnings = %v, want one about the rejected media", warnings)
			}

			// The model must see the reason in the replayed request.
			last := string(srv.bodies[len(srv.bodies)-1])
			if !strings.Contains(last, mediaRejectedPrefix) {
				t.Errorf("replayed request does not tell the model why the image failed:\n%s", last)
			}
			if strings.Contains(last, `"image_url"`) {
				t.Error("replayed request still carries the rejected image")
			}

			// The session must never hold the rejected image, or every later
			// turn would replay it and fail again.
			if n, _, _ := mediaResultsIn(persisted); n != 0 {
				t.Errorf("persisted media results = %d, want 0", n)
			}
			if _, n, text := mediaResultsIn(persisted); n != 1 || !strings.Contains(text, "shot.png") {
				t.Errorf("persisted rejected results = %d (%q), want 1 that names the file", n, text)
			} else if !strings.Contains(text, tc.advice) {
				t.Errorf("rejected result text = %q, want advice %q", text, tc.advice)
			}

			// The result and the persisted count must agree, so the SDK does
			// not persist anything twice.
			newMsgs := len(result.ConversationMessages) - 1
			if result.PersistedMessageCount != newMsgs {
				t.Errorf("PersistedMessageCount = %d, new messages = %d", result.PersistedMessageCount, newMsgs)
			}
			if n, _, _ := mediaResultsIn(result.ConversationMessages); n != 0 {
				t.Errorf("result still holds %d media results", n)
			}
			if got := result.FinalResponse.Content.Text(); got != "I can see the image." {
				t.Errorf("final response = %q", got)
			}
		})
	}
}

// TestE2EReadImageAcceptedIsPersisted checks the normal path: an accepted
// image is persisted after the provider accepts it, exactly once.
func TestE2EReadImageAcceptedIsPersisted(t *testing.T) {
	ctx := context.Background()
	path := writeTestPNG(t, t.TempDir(), "ok.png", 32, 32, false)

	srv := newRecordingServer(t, toolCallSSE(path), answerSSE)
	a := newImageAgent(ctx, t, srv.Server, media.Limits{})

	var persisted []fantasy.Message
	result, err := a.GenerateWithCallbacks(ctx,
		[]fantasy.Message{fantasy.NewUserMessage("look at ok.png")},
		GenerateCallbacks{
			OnStepMessages: func(msgs []fantasy.Message) { persisted = append(persisted, msgs...) },
		})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if n, _, _ := mediaResultsIn(persisted); n != 1 {
		t.Errorf("persisted media results = %d, want 1", n)
	}
	if got, want := result.PersistedMessageCount, len(result.ConversationMessages)-1; got != want {
		t.Errorf("PersistedMessageCount = %d, want %d", got, want)
	}
	if len(persisted) != result.PersistedMessageCount {
		t.Errorf("persisted %d messages, count says %d", len(persisted), result.PersistedMessageCount)
	}
}

// TestE2EAuthErrorDoesNotDropImage checks that an error not caused by the
// request content still fails the turn and keeps the image.
func TestE2EAuthErrorDoesNotDropImage(t *testing.T) {
	ctx := context.Background()
	path := writeTestPNG(t, t.TempDir(), "keep.png", 32, 32, false)

	srv := newRejectingServer(t, http.StatusUnauthorized, `{"error":{"message":"bad key"}}`, toolCallSSE(path))
	a := newImageAgent(ctx, t, srv.Server, media.Limits{})

	result, err := a.GenerateWithCallbacks(ctx,
		[]fantasy.Message{fantasy.NewUserMessage("look at keep.png")},
		GenerateCallbacks{OnStepMessages: func([]fantasy.Message) {}})
	if err == nil {
		t.Fatal("generate succeeded, want the auth error")
	}
	if result == nil {
		t.Fatal("no partial result")
	}
	if n, _, _ := mediaResultsIn(result.ConversationMessages); n != 1 {
		t.Errorf("media results in partial result = %d, want 1", n)
	}
}

func TestIsMediaRejection(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"413", &fantasy.ProviderError{StatusCode: 413}, true},
		{"400", &fantasy.ProviderError{StatusCode: 400}, true},
		{"context too large", &fantasy.ProviderError{StatusCode: 400, ContextTooLargeErr: true}, true},
		{"wrapped 413", fmt.Errorf("x: %w", &fantasy.ProviderError{StatusCode: 413}), true},
		{"401", &fantasy.ProviderError{StatusCode: 401}, false},
		{"403", &fantasy.ProviderError{StatusCode: 403}, false},
		{"429", &fantasy.ProviderError{StatusCode: 429}, false},
		{"500", &fantasy.ProviderError{StatusCode: 500}, false},
		{"transient", &fantasy.ProviderError{StatusCode: 400, TransientError: true}, false},
		{"plain error", errors.New("boom"), false},
		{"cancel", context.Canceled, false},
	}
	for _, tc := range tests {
		if got := isMediaRejection(tc.err); got != tc.want {
			t.Errorf("%s: isMediaRejection = %v, want %v", tc.name, got, tc.want)
		}
	}
}
