package kit_test

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/viper"

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
		defer resetViper()

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
		if !viper.IsSet("max-tokens") {
			t.Error("viper.IsSet(\"max-tokens\") should be true after MaxTokens override")
		}
	})

	// ThinkingLevel override — verified via the public getter, which
	// reads back the configured (not provider-derived) level.
	t.Run("ThinkingLevel", func(t *testing.T) {
		defer resetViper()

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

	// Temperature override — pointer semantics let callers distinguish
	// "explicitly 0.0" from "unset", which we assert by pushing a distinct
	// value and reading it back off viper's merged state.
	t.Run("Temperature", func(t *testing.T) {
		defer resetViper()

		want := float32(0.12345)
		host, err := kit.New(ctx, &kit.Options{
			Model:       "anthropic/claude-sonnet-4-5-20250929",
			Quiet:       true,
			Temperature: &want,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit: %v", err)
		}
		defer func() { _ = host.Close() }()

		if !viper.IsSet("temperature") {
			t.Fatal("viper.IsSet(\"temperature\") should be true after Temperature override")
		}
		if got := float32(viper.GetFloat64("temperature")); got != want {
			t.Errorf("Options.Temperature=%v did not propagate; viper=%v", want, got)
		}
	})
}

// TestNewPreservesIsSetSemantics verifies that creating a Kit WITHOUT
// populating the generation-param Options fields does NOT mark those
// keys as explicitly set in viper. This is the precedence contract
// that per-model defaults (ApplyModelSettings) and right-sizing
// (rightSizeMaxTokens) rely on.
//
// Previously setSDKDefaults() used viper.SetDefault() for every param,
// which caused viper.IsSet() to return true for all of them — silently
// suppressing per-model defaults and pinning max-tokens at 4096 even
// on models with much larger output limits.
func TestNewPreservesIsSetSemantics(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	defer resetViper()

	ctx := context.Background()
	host, err := kit.New(ctx, &kit.Options{
		Model:      "anthropic/claude-sonnet-4-5-20250929",
		Quiet:      true,
		NoSession:  true,
		SkipConfig: true, // isolate from any ~/.kit.yml values
	})
	if err != nil {
		t.Fatalf("Failed to create Kit: %v", err)
	}
	defer func() { _ = host.Close() }()

	// These keys must remain "unset" from viper's perspective so the
	// downstream isExplicitlySet() checks allow per-model defaults to
	// take effect.
	checkKeys := []string{
		"max-tokens",
		"temperature",
		"top-p",
		"top-k",
		"frequency-penalty",
		"presence-penalty",
		"thinking-level",
	}

	// With SkipConfig: true, InitConfig() is not invoked, so viper has
	// no env-var bindings registered. Any IsSet() here would come purely
	// from SDK-side SetDefault/Set calls — which is exactly what this
	// test is guarding against.
	for _, k := range checkKeys {
		if viper.IsSet(k) {
			t.Errorf("viper.IsSet(%q) == true when no Options field set it "+
				"(SDK defaults must not corrupt IsSet semantics)", k)
		}
	}
}

// TestNewWithProviderOptions verifies that programmatic provider overrides
// (API key, URL) take effect without env vars or config files, and that
// Options.ProviderAPIKey *wins* over any pre-existing viper state.
func TestNewWithProviderOptions(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	t.Run("succeeds with API key from Options", func(t *testing.T) {
		defer resetViper()

		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		host, err := kit.New(ctx, &kit.Options{
			Model:          "anthropic/claude-sonnet-4-5-20250929",
			Quiet:          true,
			NoSession:      true,
			ProviderAPIKey: apiKey,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit with ProviderAPIKey option: %v", err)
		}
		defer func() { _ = host.Close() }()

		if got := viper.GetString("provider-api-key"); got != apiKey {
			t.Errorf("Options.ProviderAPIKey did not propagate to viper; got %q (len=%d)", got, len(got))
		}
	})

	// Override precedence: even when viper already holds a different
	// provider-api-key value (as it would if a config file or earlier
	// Set() call populated one), Options.ProviderAPIKey must win.
	t.Run("Options override beats pre-existing viper state", func(t *testing.T) {
		defer resetViper()

		viper.Set("provider-api-key", "sk-config-file-placeholder")

		want := "sk-from-options-override"
		// Use an OpenAI-flavored model so the validation path accepts
		// the placeholder without attempting a real Anthropic handshake.
		host, err := kit.New(ctx, &kit.Options{
			Model:            "openai/gpt-4o-mini",
			Quiet:            true,
			NoSession:        true,
			NoExtensions:     true,
			DisableCoreTools: true,
			ProviderAPIKey:   want,
		})
		// Creation may still fail if the model registry is strict, but
		// we only care that the override reached viper before any
		// provider handshake happened.
		if host != nil {
			defer func() { _ = host.Close() }()
		}
		_ = err

		if got := viper.GetString("provider-api-key"); got != want {
			t.Errorf("Options.ProviderAPIKey did not override pre-existing viper value; got %q, want %q", got, want)
		}
	})

	// ProviderURL override must also reach viper.
	t.Run("ProviderURL propagates", func(t *testing.T) {
		defer resetViper()

		const want = "https://custom.example.com/v1"
		host, err := kit.New(ctx, &kit.Options{
			Model:       "anthropic/claude-sonnet-4-5-20250929",
			Quiet:       true,
			NoSession:   true,
			ProviderURL: want,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit with ProviderURL option: %v", err)
		}
		defer func() { _ = host.Close() }()

		if got := viper.GetString("provider-url"); got != want {
			t.Errorf("Options.ProviderURL did not propagate; got %q, want %q", got, want)
		}
	})
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

// resetViper wipes viper's global state so a test case doesn't leak
// viper.Set() calls into the next one. Used via defer in subtests.
func resetViper() { viper.Reset() }
