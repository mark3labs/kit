package extbridge

import (
	"context"
	"strings"
	"testing"

	kitpkg "github.com/mark3labs/kit/pkg/kit"
)

// TestBaseContext_GetSystemPrompt verifies that the bridge returns the
// composed prompt together with the configured source, and that the text
// follows a context file added at runtime.
func TestBaseContext_GetSystemPrompt(t *testing.T) {
	const inline = "You are a bridge-test persona. Marker: SYSPROMPT-OK"

	host, err := kitpkg.New(context.Background(), &kitpkg.Options{
		Quiet:          true,
		NoSession:      true,
		NoExtensions:   true,
		SkipConfig:     true,
		Model:          testModel,
		ProviderAPIKey: "sk-ant-test-not-a-real-key",
		SystemPrompt:   inline,
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	t.Cleanup(func() { _ = host.Close() })

	extCtx := BaseContext(context.Background(), host)
	if extCtx.GetSystemPrompt == nil {
		t.Fatal("BaseContext did not wire GetSystemPrompt")
	}

	text, source := extCtx.GetSystemPrompt()
	if source != inline {
		t.Errorf("source = %q; want the inline prompt %q", source, inline)
	}
	// The text is the composed prompt, not the configured value alone.
	if !strings.Contains(text, inline) || !strings.Contains(text, "Current working directory:") {
		t.Errorf("text = %q; want the inline prompt composed with the runtime context", text)
	}

	if _, err := host.AddContextFileContent("/bridge/AGENTS.md", "BRIDGE-MARKER"); err != nil {
		t.Fatalf("AddContextFileContent: %v", err)
	}
	updated, _ := extCtx.GetSystemPrompt()
	if !strings.Contains(updated, "BRIDGE-MARKER") {
		t.Error("GetSystemPrompt does not report a context file added at runtime")
	}
}
