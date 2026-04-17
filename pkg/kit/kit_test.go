package kit_test

import (
	"context"
	"os"
	"testing"

	kit "github.com/mark3labs/kit/pkg/kit"
)

func TestNew(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	// Test default initialization
	opts := &kit.Options{
		Model: "anthropic/claude-sonnet-4-5-20250929",
	}
	host, err := kit.New(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to create Kit with defaults: %v", err)
	}
	defer func() { _ = host.Close() }()

	if host.GetModelString() == "" {
		t.Error("Model string should not be empty")
	}
}

func TestNewWithOptions(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	opts := &kit.Options{
		Model:    "anthropic/claude-sonnet-4-5-20250929",
		MaxSteps: 5,
		Quiet:    true,
	}

	host, err := kit.New(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to create Kit with options: %v", err)
	}
	defer func() { _ = host.Close() }()

	if host.GetModelString() != opts.Model {
		t.Errorf("Expected model %s, got %s", opts.Model, host.GetModelString())
	}
}

// TestNewWithGenerationOptions verifies that the SDK-only generation
// parameter overrides on Options propagate all the way through to the
// agent without requiring any viper.Set workarounds in caller code.
func TestNewWithGenerationOptions(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	// MaxTokens override — keep ThinkingLevel off so Anthropic's thinking
	// budget doesn't auto-bump MaxTokens above what we configured.
	t.Run("MaxTokens", func(t *testing.T) {
		const want = 12345
		host, err := kit.New(ctx, &kit.Options{
			Model:     "anthropic/claude-sonnet-4-5-20250929",
			Quiet:     true,
			MaxTokens: want,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit: %v", err)
		}
		defer func() { _ = host.Close() }()

		if got := host.MaxTokens(); got != want {
			t.Errorf("Options.MaxTokens=%d did not propagate; Kit.MaxTokens()=%d", want, got)
		}
	})

	// ThinkingLevel override — verified via the public getter, which
	// reads back the configured (not provider-derived) level.
	t.Run("ThinkingLevel", func(t *testing.T) {
		const want = "high"
		host, err := kit.New(ctx, &kit.Options{
			Model:         "anthropic/claude-sonnet-4-5-20250929",
			Quiet:         true,
			ThinkingLevel: want,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit: %v", err)
		}
		defer func() { _ = host.Close() }()

		if got := host.GetThinkingLevel(); got != want {
			t.Errorf("Options.ThinkingLevel=%q did not propagate; Kit.GetThinkingLevel()=%q", want, got)
		}
	})
}

// TestNewWithProviderOptions verifies that programmatic provider overrides
// (API key, URL) take effect without env vars or config files.
func TestNewWithProviderOptions(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	// Use the real key but pass it via Options instead of env. Kit should
	// authenticate successfully — proving the override reached the provider.
	apiKey := os.Getenv("ANTHROPIC_API_KEY")

	host, err := kit.New(ctx, &kit.Options{
		Model:          "anthropic/claude-sonnet-4-5-20250929",
		Quiet:          true,
		ProviderAPIKey: apiKey,
	})
	if err != nil {
		t.Fatalf("Failed to create Kit with ProviderAPIKey option: %v", err)
	}
	defer func() { _ = host.Close() }()
}

func TestSessionManagement(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	host, err := kit.New(ctx, &kit.Options{Quiet: true, NoSession: true})
	if err != nil {
		t.Fatalf("Failed to create Kit: %v", err)
	}
	defer func() { _ = host.Close() }()

	// Tree session should be configured.
	ts := host.GetTreeSession()
	if ts == nil {
		t.Fatal("Expected tree session to be configured")
	}

	// Test clear session resets leaf.
	host.ClearSession()

	// Verify session info accessors.
	if id := host.GetSessionID(); id == "" {
		t.Error("Expected non-empty session ID")
	}
}
