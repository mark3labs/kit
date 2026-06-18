package kit_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mark3labs/kit/pkg/kit"
)

func TestClassifyProviderError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"nil", nil, nil},
		{"context overflow", errors.New("error: context_length_exceeded for this model"), kit.ErrContextOverflow},
		{"context window phrase", errors.New("the prompt is too long for the context window"), kit.ErrContextOverflow},
		{"rate limit", errors.New("HTTP status 429: rate limit exceeded"), kit.ErrRateLimit},
		{"auth 401", errors.New("status 401 unauthorized"), kit.ErrAuth},
		{"auth invalid key", errors.New("invalid api key provided"), kit.ErrAuth},
		{"unavailable 503", errors.New("status 503 service unavailable"), kit.ErrProviderUnavailable},
		{"invalid request", errors.New("status 400 bad request: malformed body"), kit.ErrInvalidRequest},
		{"unclassified", errors.New("something totally unexpected"), nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := kit.ClassifyProviderError(tc.in)
			if tc.in == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if tc.want == nil {
				// Unclassified errors are returned unchanged.
				if got.Error() != tc.in.Error() {
					t.Fatalf("expected unchanged error, got %v", got)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("errors.Is(%v, %v) = false", got, tc.want)
			}
			// Original cause must remain reachable.
			if !errors.Is(got, tc.in) {
				t.Fatalf("original cause not preserved in %v", got)
			}
		})
	}
}

func TestClassifyProviderErrorIdempotent(t *testing.T) {
	wrapped := fmt.Errorf("%w: upstream detail", kit.ErrRateLimit)
	got := kit.ClassifyProviderError(wrapped)
	if got != wrapped {
		t.Fatalf("already-classified error should be returned unchanged")
	}
	if !errors.Is(got, kit.ErrRateLimit) {
		t.Fatalf("expected ErrRateLimit to remain")
	}
}
