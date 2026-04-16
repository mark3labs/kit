package models

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// bindMaxTokensFlag wires a fresh pflag-backed "max-tokens" key into viper so
// isExplicitlySet behaves the same way it does in production. Returns a
// cleanup function that removes the binding so sibling tests see a clean
// state.
func bindMaxTokensFlag(t *testing.T, args []string) func() {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Int("max-tokens", 8192, "")
	if err := viper.BindPFlag("max-tokens", fs.Lookup("max-tokens")); err != nil {
		t.Fatalf("BindPFlag: %v", err)
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("fs.Parse: %v", err)
	}
	return func() {
		viper.Reset()
	}
}

func TestRightSizeMaxTokens_RaisesWhenBelowCeiling(t *testing.T) {
	cleanup := bindMaxTokensFlag(t, nil) // no args → flag.Changed = false
	defer cleanup()

	config := &ProviderConfig{MaxTokens: 8192}
	modelInfo := &ModelInfo{
		ID:    "claude-sonnet-4-5",
		Limit: Limit{Context: 200000, Output: 64000},
	}

	rightSizeMaxTokens(config, modelInfo)

	if config.MaxTokens != 32768 {
		t.Errorf("expected MaxTokens raised to defaultRightSizeCap (32768), got %d", config.MaxTokens)
	}
}

func TestRightSizeMaxTokens_CapsAtDefaultRightSizeCap(t *testing.T) {
	cleanup := bindMaxTokensFlag(t, nil)
	defer cleanup()

	config := &ProviderConfig{MaxTokens: 8192}
	// Mistral Devstral has 262144 output — we should still cap at 32768.
	modelInfo := &ModelInfo{
		ID:    "devstral-medium-latest",
		Limit: Limit{Context: 262144, Output: 262144},
	}

	rightSizeMaxTokens(config, modelInfo)

	if config.MaxTokens != defaultRightSizeCap {
		t.Errorf("expected MaxTokens capped at %d, got %d", defaultRightSizeCap, config.MaxTokens)
	}
}

func TestRightSizeMaxTokens_UsesExactOutputWhenBelowCap(t *testing.T) {
	cleanup := bindMaxTokensFlag(t, nil)
	defer cleanup()

	config := &ProviderConfig{MaxTokens: 4096}
	// Model with output limit smaller than the cap.
	modelInfo := &ModelInfo{
		ID:    "gpt-4",
		Limit: Limit{Context: 8192, Output: 8192},
	}

	rightSizeMaxTokens(config, modelInfo)

	if config.MaxTokens != 8192 {
		t.Errorf("expected MaxTokens raised to model output ceiling (8192), got %d", config.MaxTokens)
	}
}

func TestRightSizeMaxTokens_DoesNotLowerCurrentValue(t *testing.T) {
	cleanup := bindMaxTokensFlag(t, nil)
	defer cleanup()

	// User (via per-model settings, applied earlier) already bumped MaxTokens
	// above the cap — we must not clobber their choice.
	config := &ProviderConfig{MaxTokens: 100000}
	modelInfo := &ModelInfo{
		ID:    "devstral-medium-latest",
		Limit: Limit{Context: 262144, Output: 262144},
	}

	rightSizeMaxTokens(config, modelInfo)

	if config.MaxTokens != 100000 {
		t.Errorf("expected MaxTokens preserved at 100000, got %d", config.MaxTokens)
	}
}

func TestRightSizeMaxTokens_RespectsExplicitFlag(t *testing.T) {
	// Simulate `--max-tokens 4096` on the command line.
	cleanup := bindMaxTokensFlag(t, []string{"--max-tokens", "4096"})
	defer cleanup()

	config := &ProviderConfig{MaxTokens: 4096}
	modelInfo := &ModelInfo{
		ID:    "claude-sonnet-4-5",
		Limit: Limit{Context: 200000, Output: 64000},
	}

	rightSizeMaxTokens(config, modelInfo)

	if config.MaxTokens != 4096 {
		t.Errorf("expected explicit --max-tokens to be preserved (4096), got %d", config.MaxTokens)
	}
}

func TestRightSizeMaxTokens_NilModelInfo(t *testing.T) {
	cleanup := bindMaxTokensFlag(t, nil)
	defer cleanup()

	config := &ProviderConfig{MaxTokens: 8192}
	// Custom model / Ollama / unknown provider → no model info.
	rightSizeMaxTokens(config, nil)

	if config.MaxTokens != 8192 {
		t.Errorf("expected MaxTokens unchanged with nil modelInfo, got %d", config.MaxTokens)
	}
}

func TestRightSizeMaxTokens_ZeroOutputLimit(t *testing.T) {
	cleanup := bindMaxTokensFlag(t, nil)
	defer cleanup()

	config := &ProviderConfig{MaxTokens: 8192}
	// Model present in catalog but with no known output limit.
	modelInfo := &ModelInfo{
		ID:    "unknown-model",
		Limit: Limit{Context: 0, Output: 0},
	}

	rightSizeMaxTokens(config, modelInfo)

	if config.MaxTokens != 8192 {
		t.Errorf("expected MaxTokens unchanged with zero output limit, got %d", config.MaxTokens)
	}
}
