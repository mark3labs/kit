package test

import (
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
)

// TestModelPricingCrossesYaegiBoundary verifies that ModelPricing and the
// SessionUsage.IsOAuth flag survive the interpreter boundary with their values
// intact. New extension-facing structs must be registered in
// internal/extensions/symbols.go; when they are not, Yaegi either fails to
// resolve the type or silently yields zero values, so this asserts on real
// numbers rather than merely on the extension loading.
func TestModelPricingCrossesYaegiBoundary(t *testing.T) {
	src := `package main

import (
	"fmt"

	"kit/ext"
)

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		caps, errStr := ctx.GetModelCapabilities("anthropic/claude-sonnet-4")
		if errStr != "" {
			ctx.Print("error: " + errStr)
			return
		}

		p := caps.Pricing
		ctx.Print(fmt.Sprintf("known=%v input=%.2f output=%.2f cacheRead=%.2f hasRead=%v hasWrite=%v",
			p.Known, p.Input, p.Output, p.CacheRead, p.HasCacheRead, p.HasCacheWrite))

		// Compute prompt-cache savings — the calculation that was impossible
		// before pricing was exposed.
		usage := ctx.GetSessionUsage()
		saved := float64(usage.TotalCacheReadTokens) * (p.Input - p.CacheRead) / 1000000
		ctx.Print(fmt.Sprintf("oauth=%v saved=%.4f", usage.IsOAuth, saved))
	})
}
`

	cacheRead := 0.30

	harness := New(t)
	harness.LoadString(src, "pricing.go")

	ctx := harness.Context().ToContext()
	ctx.GetModelCapabilities = func(model string) (extensions.ModelCapabilities, string) {
		return extensions.ModelCapabilities{
			Provider:     "anthropic",
			ModelID:      "claude-sonnet-4",
			ContextLimit: 200000,
			Pricing: extensions.ModelPricing{
				Input:        3.0,
				Output:       15.0,
				CacheRead:    cacheRead,
				HasCacheRead: true,
				Known:        true,
			},
		}, ""
	}
	ctx.GetSessionUsage = func() extensions.SessionUsage {
		return extensions.SessionUsage{
			TotalInputTokens:     1000,
			TotalCacheReadTokens: 1000000,
			IsOAuth:              true,
		}
	}
	harness.Runner().SetContext(ctx)

	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"}); err != nil {
		t.Fatalf("Emit(SessionStartEvent) error = %v", err)
	}

	prints := harness.Context().GetPrints()
	if len(prints) != 2 {
		t.Fatalf("got %d prints (%v); want 2", len(prints), prints)
	}

	wantPricing := "known=true input=3.00 output=15.00 cacheRead=0.30 hasRead=true hasWrite=false"
	if prints[0] != wantPricing {
		t.Errorf("pricing round-trip:\n got %q\nwant %q", prints[0], wantPricing)
	}

	// 1M cache-read tokens at $3.00 vs $0.30 per million = $2.70 saved.
	wantUsage := "oauth=true saved=2.7000"
	if prints[1] != wantUsage {
		t.Errorf("usage round-trip:\n got %q\nwant %q", prints[1], wantUsage)
	}
}

// TestModelPricingUnknownCrossesYaegiBoundary verifies that an unpriced model
// is distinguishable from a free one across the boundary. Extensions must be
// able to suppress cost UI rather than render a misleading "$0.00".
func TestModelPricingUnknownCrossesYaegiBoundary(t *testing.T) {
	src := `package main

import (
	"fmt"

	"kit/ext"
)

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		caps, _ := ctx.GetModelCapabilities("local/llama")
		ctx.Print(fmt.Sprintf("known=%v", caps.Pricing.Known))
	})
}
`

	harness := New(t)
	harness.LoadString(src, "unpriced.go")

	ctx := harness.Context().ToContext()
	ctx.GetModelCapabilities = func(model string) (extensions.ModelCapabilities, string) {
		return extensions.ModelCapabilities{Provider: "local", ModelID: "llama"}, ""
	}
	harness.Runner().SetContext(ctx)

	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"}); err != nil {
		t.Fatalf("Emit(SessionStartEvent) error = %v", err)
	}

	prints := harness.Context().GetPrints()
	if len(prints) != 1 || prints[0] != "known=false" {
		t.Errorf("got %v; want [known=false]", prints)
	}
}
