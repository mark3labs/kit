package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/app"
)

func testExtensionResponse[T comparable](t *testing.T, response, cancelled T) {
	t.Helper()
	ch := make(chan T, 1)
	ch <- response
	if got := waitExtensionResponse(context.Background(), ch, cancelled); got != response {
		t.Fatalf("response = %#v, want %#v", got, response)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan T, 1)
	go func() { done <- waitExtensionResponse(ctx, ch, cancelled) }()
	cancel()
	select {
	case got := <-done:
		if got != cancelled {
			t.Fatalf("cancelled response = %#v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("response wait ignored shutdown")
	}
}

func TestExtensionResponseShutdown(t *testing.T) {
	// PromptResponse contains slices, so test the comparable overlay response
	// through the shared helper, and a prompt cancellation separately.
	testExtensionResponse(t, app.OverlayResponse{Action: "ok"}, app.OverlayResponse{Cancelled: true})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := waitExtensionResponse(ctx, make(chan app.PromptResponse), app.PromptResponse{Cancelled: true}); !got.Cancelled {
		t.Fatal("prompt response was not cancelled")
	}
}
