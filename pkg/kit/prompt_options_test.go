package kit

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestPromptWithOptionsModelOverride runs a prompt with a per-call model.
// applyPromptOptions holds promptOptsMu for the whole call. It used to call
// SetModel, which takes the same lock again, so the call never returned.
func TestPromptWithOptionsModelOverride(t *testing.T) {
	rec := &recordingFactory{}
	k := newProviderTestKit(t, &Options{
		Model:     "local/main",
		Providers: map[string]ProviderFactory{"local": rec.factory("local")},
	})

	type result struct {
		reply string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		reply, err := k.PromptWithOptions(context.Background(), "hello", PromptOptions{Model: "local/other"})
		done <- result{reply, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("PromptWithOptions: %v", r.err)
		}
		if !strings.Contains(r.reply, "reply from other") {
			t.Errorf("reply = %q, want the reply of the per-call model", r.reply)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("PromptWithOptions with a model override did not return")
	}

	if got := k.GetModelString(); got != "local/main" {
		t.Errorf("model after the call = %q, want local/main again", got)
	}
}
