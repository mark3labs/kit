package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/mark3labs/kit/internal/models"
)

// maxRetries is the number of retries after a transient failure (429, 529,
// 5xx or a network error).
const maxRetries = 2

// maxResponseBytes limits the size of a response body.
const maxResponseBytes = 16 << 20

// requestTimeout bounds one attempt, so a stalled provider cannot block a
// caller whose context has no deadline. A timed-out attempt is retried.
const requestTimeout = 2 * time.Minute

// APIError is an HTTP error from a decision provider.
type APIError struct {
	StatusCode int
	// Message is the provider's error message, when the body has one.
	Message string
	// Body is the raw response body.
	Body string
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	return fmt.Sprintf("decision request failed: status %d: %s", e.StatusCode, msg)
}

// Decide validates req, sends it to ep and returns the normalized result.
func Decide(ctx context.Context, ep *models.DecisionEndpoint, req Request) (*Result, error) {
	if err := validate(ep, req); err != nil {
		return nil, err
	}

	var (
		body        []byte
		contentType string
		headers     map[string]string
		err         error
	)
	switch ep.Wire {
	case models.DecisionWireSystemOne, models.DecisionWireWorkersAI:
		body, contentType, headers, err = encodeSystemOne(ep, req)
	case models.DecisionWireGateway:
		body, contentType, headers, err = encodeGateway(ep, req)
	default:
		return nil, fmt.Errorf("%w: %q", models.ErrUnknownDecisionWire, ep.Wire)
	}
	if err != nil {
		return nil, err
	}

	data, err := send(ctx, ep, body, contentType, headers)
	if err != nil {
		return nil, err
	}

	if ep.Wire == models.DecisionWireGateway {
		return decodeGateway(req, data)
	}
	return decodeSystemOne(ep, req, data)
}

// send posts the body and retries transient failures with backoff.
func send(ctx context.Context, ep *models.DecisionEndpoint, body []byte, contentType string, headers map[string]string) ([]byte, error) {
	client := ep.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	var lastErr error
	for attempt := 0; ; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		httpReq, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, ep.URL, bytes.NewReader(body))
		if err != nil {
			cancel()
			return nil, fmt.Errorf("create decision request: %w", err)
		}
		httpReq.Header.Set("Content-Type", contentType)
		httpReq.Header.Set("Accept", "application/json")
		if ep.APIKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+ep.APIKey)
		}
		for k, v := range headers {
			httpReq.Header.Set(k, v)
		}

		data, retryAfter, err := do(client, httpReq) // do reads the whole body
		cancel()
		if err == nil {
			return data, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt >= maxRetries || !retryable(err) {
			return nil, lastErr
		}

		wait := time.Duration(1<<attempt) * time.Second
		if retryAfter > 0 {
			wait = min(retryAfter, 30*time.Second)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// do sends one request. It returns the body of a 2xx response, or an
// error and the server's Retry-After delay.
func do(client *http.Client, req *http.Request) ([]byte, time.Duration, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("decision request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("read decision response: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return data, 0, nil
	}
	var retryAfter time.Duration
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
		retryAfter = time.Duration(s) * time.Second
	}
	return nil, retryAfter, &APIError{StatusCode: resp.StatusCode, Message: errorMessage(data), Body: string(data)}
}

// retryable reports whether a failed request may succeed when sent again.
func retryable(err error) bool {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		switch apiErr.StatusCode {
		case http.StatusTooManyRequests, 529,
			http.StatusInternalServerError, http.StatusBadGateway,
			http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
		return false
	}
	// Network errors.
	return true
}

// errorMessage extracts the message from the error bodies of the supported
// wires: {"message"}, {"error": "..."}, {"error": {"message"}}, {"detail"}
// and Cloudflare's {"errors": [{"message"}]}.
func errorMessage(data []byte) string {
	var body struct {
		Message string          `json:"message"`
		Error   json.RawMessage `json:"error"`
		Detail  json.RawMessage `json:"detail"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(data, &body) != nil {
		return ""
	}
	if body.Message != "" {
		return body.Message
	}
	if len(body.Error) > 0 {
		var s string
		if json.Unmarshal(body.Error, &s) == nil && s != "" {
			return s
		}
		var obj struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(body.Error, &obj) == nil && obj.Message != "" {
			return obj.Message
		}
	}
	if len(body.Errors) > 0 && body.Errors[0].Message != "" {
		return body.Errors[0].Message
	}
	if len(body.Detail) > 0 {
		var s string
		if json.Unmarshal(body.Detail, &s) == nil {
			return s
		}
		return string(body.Detail)
	}
	return ""
}
