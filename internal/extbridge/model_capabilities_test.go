package extbridge

import (
	"context"
	"testing"

	kitpkg "github.com/mark3labs/kit/pkg/kit"
)

// TestBaseContext_GetModelCapabilitiesResolvesActiveModel covers the empty-model
// contract at the bridge level.
//
// The package-level kit.GetModelCapabilities("") intentionally still returns
// "no model specified" — it has no Kit instance to resolve against. The
// resolution lives in the BaseContext closure, so it needs its own coverage:
// this asserts the closure substitutes the configured model and returns its
// capabilities and pricing.
func TestBaseContext_GetModelCapabilitiesResolvesActiveModel(t *testing.T) {
	const model = "anthropic/claude-sonnet-4-5-20250929"

	ctx := context.Background()
	host, err := kitpkg.New(ctx, &kitpkg.Options{
		Quiet:        true,
		NoSession:    true,
		NoExtensions: true,
		Model:        model,
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	defer func() { _ = host.Close() }()

	extCtx := BaseContext(ctx, host)
	if extCtx.GetModelCapabilities == nil {
		t.Fatal("BaseContext did not wire GetModelCapabilities")
	}

	caps, errStr := extCtx.GetModelCapabilities("")
	if errStr != "" {
		t.Fatalf("GetModelCapabilities(\"\") error = %q; want the active model to resolve", errStr)
	}

	if caps.Provider != "anthropic" {
		t.Errorf("Provider = %q; want %q", caps.Provider, "anthropic")
	}
	if caps.ModelID != "claude-sonnet-4-5-20250929" {
		t.Errorf("ModelID = %q; want %q", caps.ModelID, "claude-sonnet-4-5-20250929")
	}
	if caps.ContextLimit <= 0 {
		t.Errorf("ContextLimit = %d; want a positive limit from the registry", caps.ContextLimit)
	}

	// Pricing must survive the bridge, not just the package-level lookup.
	if !caps.Pricing.Known {
		t.Error("Pricing.Known = false; want registry pricing for a first-party model")
	}
	if caps.Pricing.Input <= 0 || caps.Pricing.Output <= 0 {
		t.Errorf("Pricing input/output = %v/%v; want positive rates",
			caps.Pricing.Input, caps.Pricing.Output)
	}

	// An explicit model must still work, and agree with the resolved one.
	explicit, errStr := extCtx.GetModelCapabilities(model)
	if errStr != "" {
		t.Fatalf("GetModelCapabilities(%q) error = %q", model, errStr)
	}
	if explicit != caps {
		t.Errorf("explicit lookup = %+v; want same as empty-string resolution %+v", explicit, caps)
	}
}

// TestBaseContext_GetModelCapabilitiesUnknownModel verifies the empty-model
// fallback does not mask genuine lookup failures.
func TestBaseContext_GetModelCapabilitiesUnknownModel(t *testing.T) {
	ctx := context.Background()
	host, err := kitpkg.New(ctx, &kitpkg.Options{
		Quiet: true, NoSession: true, NoExtensions: true,
		Model: "anthropic/claude-sonnet-4-5-20250929",
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	defer func() { _ = host.Close() }()

	extCtx := BaseContext(ctx, host)

	caps, errStr := extCtx.GetModelCapabilities("nosuchprovider/nosuchmodel")
	if errStr == "" {
		t.Error("error = \"\"; want a message for an unknown model")
	}
	if caps.Pricing.Known {
		t.Error("Pricing.Known = true for an unknown model; want false")
	}
}

// TestBaseContext_GetThinkingLevelDefaultsOff verifies the bridge never reports
// an empty level, which would render as a blank in any UI that displays it.
func TestBaseContext_GetThinkingLevelDefaultsOff(t *testing.T) {
	ctx := context.Background()
	host, err := kitpkg.New(ctx, &kitpkg.Options{
		Quiet: true, NoSession: true, NoExtensions: true,
		Model: "anthropic/claude-sonnet-4-5-20250929",
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	defer func() { _ = host.Close() }()

	extCtx := BaseContext(ctx, host)
	if extCtx.GetThinkingLevel == nil {
		t.Fatal("BaseContext did not wire GetThinkingLevel")
	}
	if got := extCtx.GetThinkingLevel(); got == "" {
		t.Error("GetThinkingLevel() = \"\"; want a concrete level such as \"off\"")
	}
}
