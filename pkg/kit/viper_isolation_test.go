package kit_test

import (
	"context"
	"os"
	"testing"

	"github.com/mark3labs/kit/pkg/kit"
)

// TestOptionFunctionsPlumbing verifies that the functional options apply their
// values to the underlying Options struct. This does not create a provider, so
// it runs without API keys.
func TestOptionFunctionsPlumbing(t *testing.T) {
	o := &kit.Options{}
	opts := []kit.Option{
		kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),
		kit.WithSystemPrompt("be terse"),
		kit.WithMaxTokens(4321),
		kit.WithThinkingLevel("high"),
		kit.WithProviderAPIKey("sk-test"),
		kit.WithProviderURL("https://example.test/v1"),
		kit.WithConfigFile("/tmp/.kit.yml"),
		kit.WithStreaming(false),
		kit.WithDebug(),
		kit.Ephemeral(),
	}
	for _, fn := range opts {
		fn(o)
	}

	if o.Model != "anthropic/claude-sonnet-4-5-20250929" {
		t.Errorf("WithModel: got %q", o.Model)
	}
	if o.SystemPrompt != "be terse" {
		t.Errorf("WithSystemPrompt: got %q", o.SystemPrompt)
	}
	if o.MaxTokens != 4321 {
		t.Errorf("WithMaxTokens: got %d", o.MaxTokens)
	}
	if o.ThinkingLevel != "high" {
		t.Errorf("WithThinkingLevel: got %q", o.ThinkingLevel)
	}
	if o.ProviderAPIKey != "sk-test" {
		t.Errorf("WithProviderAPIKey: got %q", o.ProviderAPIKey)
	}
	if o.ProviderURL != "https://example.test/v1" {
		t.Errorf("WithProviderURL: got %q", o.ProviderURL)
	}
	if o.ConfigFile != "/tmp/.kit.yml" {
		t.Errorf("WithConfigFile: got %q", o.ConfigFile)
	}
	if o.Streaming == nil {
		t.Error("WithStreaming: expected Streaming to be set (non-nil)")
	} else if *o.Streaming {
		t.Error("WithStreaming(false): expected *Streaming=false")
	}
	if !o.Debug {
		t.Error("WithDebug: expected Debug=true")
	}
	if !o.NoSession {
		t.Error("Ephemeral: expected NoSession=true")
	}
}

// TestOptionOrderingOverrides verifies later options override earlier ones.
func TestOptionOrderingOverrides(t *testing.T) {
	o := &kit.Options{}
	kit.WithModel("a/b")(o)
	kit.WithModel("c/d")(o)
	if o.Model != "c/d" {
		t.Errorf("later WithModel should win; got %q", o.Model)
	}
}

// TestKitConfigIsolation is the regression test for issue #40: two Kit
// instances constructed in the same process must own independent configuration
// stores. Setting the thinking level (or model) on one must not affect the
// other. Against the previous global-viper implementation this test fails
// because both Kits read and write the same process-global store.
func TestKitConfigIsolation(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	a, err := kit.New(ctx, &kit.Options{
		Model:         "anthropic/claude-sonnet-4-5-20250929",
		ThinkingLevel: "low",
		Quiet:         true,
		NoSession:     true,
		NoExtensions:  true,
	})
	if err != nil {
		t.Fatalf("failed to create Kit A: %v", err)
	}
	defer func() { _ = a.Close() }()

	b, err := kit.New(ctx, &kit.Options{
		Model:         "anthropic/claude-sonnet-4-5-20250929",
		ThinkingLevel: "high",
		Quiet:         true,
		NoSession:     true,
		NoExtensions:  true,
	})
	if err != nil {
		t.Fatalf("failed to create Kit B: %v", err)
	}
	defer func() { _ = b.Close() }()

	// Each instance must retain its own configured thinking level. Under the
	// old global-viper implementation, B's construction overwrote A's value.
	if got := a.GetThinkingLevel(); got != "low" {
		t.Errorf("Kit A thinking level = %q; want %q (config leaked from B)", got, "low")
	}
	if got := b.GetThinkingLevel(); got != "high" {
		t.Errorf("Kit B thinking level = %q; want %q", got, "high")
	}

	// Mutating one at runtime must not bleed into the other.
	if err := a.SetThinkingLevel(ctx, "medium"); err != nil {
		t.Fatalf("SetThinkingLevel on A: %v", err)
	}
	if got := a.GetThinkingLevel(); got != "medium" {
		t.Errorf("after SetThinkingLevel, Kit A = %q; want %q", got, "medium")
	}
	if got := b.GetThinkingLevel(); got != "high" {
		t.Errorf("after mutating A, Kit B leaked to %q; want %q", got, "high")
	}
}

// TestNewAgentDefaultsStreamingOn verifies that the ergonomic constructor
// enables streaming by default and applies functional options.
func TestNewAgentDefaultsStreamingOn(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()
	k, err := kit.NewAgent(ctx,
		kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),
		kit.WithMaxTokens(2048),
		kit.Ephemeral(),
	)
	if err != nil {
		t.Fatalf("NewAgent failed: %v", err)
	}
	defer func() { _ = k.Close() }()

	if !k.ConfigValueIsSetForTest("max-tokens") {
		t.Error("NewAgent did not propagate WithMaxTokens to the instance store")
	}
	if !k.ConfigBoolForTest("stream") {
		t.Error("NewAgent should enable streaming by default")
	}
}

// TestNewAgentStreamingOptOut verifies WithStreaming(false) disables the
// default-on streaming behaviour of NewAgent.
func TestNewAgentStreamingOptOut(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()
	k, err := kit.NewAgent(ctx,
		kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),
		kit.WithStreaming(false),
		kit.Ephemeral(),
	)
	if err != nil {
		t.Fatalf("NewAgent failed: %v", err)
	}
	defer func() { _ = k.Close() }()

	if k.ConfigBoolForTest("stream") {
		t.Error("WithStreaming(false) should disable streaming")
	}
}

// TestNewZeroOptionsKeepsStreamingDefault is the regression test for the
// unconditional `v.Set("stream", opts.Streaming)` bug: a zero-valued Options
// (Streaming == nil) must NOT force stream=false. With Streaming unset,
// streaming resolves through the precedence chain, whose SDK default is true.
func TestNewZeroOptionsKeepsStreamingDefault(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()
	k, err := kit.New(ctx, &kit.Options{
		Model:      "anthropic/claude-sonnet-4-5-20250929",
		Quiet:      true,
		NoSession:  true,
		SkipConfig: true, // isolate from any ~/.kit.yml / env stream setting
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = k.Close() }()

	if !k.ConfigBoolForTest("stream") {
		t.Error("zero-valued Options must not force stream=false; expected the default (true)")
	}
}

// TestNewStreamingExplicitOptOut verifies that a raw Options can still disable
// streaming by setting Streaming to a pointer to false.
func TestNewStreamingExplicitOptOut(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	streamOff := false
	ctx := context.Background()
	k, err := kit.New(ctx, &kit.Options{
		Model:      "anthropic/claude-sonnet-4-5-20250929",
		Quiet:      true,
		NoSession:  true,
		SkipConfig: true,
		Streaming:  &streamOff,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = k.Close() }()

	if k.ConfigBoolForTest("stream") {
		t.Error("Streaming=&false should disable streaming")
	}
}
