package models

import (
	"encoding/json"
	"slices"
	"testing"
)

// ---------------------------------------------------------------------------
// Reasoning options
// ---------------------------------------------------------------------------

func TestReasoningFrom(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	_ = f

	tests := []struct {
		name       string
		opts       []modelsDBReasoningOption
		wantLevels []string
		wantGraded bool
	}{
		{
			name:       "absent metadata is not graded",
			opts:       nil,
			wantLevels: nil,
			wantGraded: false,
		},
		{
			name:       "empty list is not graded",
			opts:       []modelsDBReasoningOption{},
			wantLevels: nil,
			wantGraded: false,
		},
		{
			name:       "effort levels are graded",
			opts:       []modelsDBReasoningOption{{Type: "effort", Values: []string{"low", "medium", "high"}}},
			wantLevels: []string{"low", "medium", "high"},
			wantGraded: true,
		},
		{
			name:       "toggle has no levels and is not graded",
			opts:       []modelsDBReasoningOption{{Type: "toggle"}},
			wantLevels: nil,
			wantGraded: false,
		},
		{
			name:       "budget_tokens has no levels and is not graded",
			opts:       []modelsDBReasoningOption{{Type: "budget_tokens"}},
			wantLevels: nil,
			wantGraded: false,
		},
		{
			name:       "effort with empty values is not graded",
			opts:       []modelsDBReasoningOption{{Type: "effort", Values: nil}},
			wantLevels: nil,
			wantGraded: false,
		},
		{
			name: "several options merge without duplicates",
			opts: []modelsDBReasoningOption{
				{Type: "effort", Values: []string{"low", "high"}},
				{Type: "effort", Values: []string{"high", "max"}},
				{Type: "toggle"},
			},
			wantLevels: []string{"low", "high", "max"},
			wantGraded: true,
		},
		{
			name:       "unknown option type is ignored",
			opts:       []modelsDBReasoningOption{{Type: "something-new"}},
			wantLevels: nil,
			wantGraded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			levels, graded := reasoningFrom(tt.opts)
			if graded != tt.wantGraded {
				t.Errorf("graded = %v, want %v", graded, tt.wantGraded)
			}
			if !slices.Equal(levels, tt.wantLevels) {
				t.Errorf("levels = %v, want %v", levels, tt.wantLevels)
			}
		})
	}
}

func TestSupportsReasoningLevel(t *testing.T) {
	graded := ModelInfo{ReasoningLevels: []string{"none", "low", "high"}, ReasoningIsGraded: true}
	ungraded := ModelInfo{ReasoningIsGraded: false}

	if !graded.SupportsReasoningLevel("low") {
		t.Error("graded model should accept a listed level")
	}
	if graded.SupportsReasoningLevel("medium") {
		t.Error("graded model should reject an unlisted level")
	}
	// An ungraded model has no evidence against any level, so it must be
	// permissive rather than reject everything.
	for _, l := range []string{"none", "minimal", "low", "medium", "high"} {
		if !ungraded.SupportsReasoningLevel(l) {
			t.Errorf("ungraded model should accept %q", l)
		}
	}
}

// TestThinkingLevelMatchesCatalog is the regression oracle for the hardcoded
// heuristic this replaced. Kit used substring matching on model names to guess
// which reasoning levels a model accepted, which disagreed with the catalog on
// hundreds of models. Every graded model must now agree with its own metadata.
func TestThinkingLevelMatchesCatalog(t *testing.T) {
	registry := GetGlobalRegistry()

	checked := 0
	for _, provider := range registry.GetSupportedProviders() {
		models, err := registry.GetModelsForProvider(provider)
		if err != nil {
			continue
		}
		for modelID, info := range models {
			if !info.ReasoningIsGraded {
				continue
			}
			for _, level := range ThinkingLevels() {
				got := IsValidThinkingLevelForModel(level, provider, modelID)

				want := true
				if catalogLevel, ok := catalogReasoningLevel(level); ok {
					want = slices.Contains(info.ReasoningLevels, catalogLevel)
				}
				if got != want {
					t.Errorf("%s/%s level %q: got %v, want %v (catalog levels %v)",
						provider, modelID, level, got, want, info.ReasoningLevels)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no graded models found in catalog; the metadata is not being parsed")
	}
	t.Logf("verified %d level/model combinations against the catalog", checked)
}

// TestThinkingLevelKnownModels pins behaviour for specific models whose
// reasoning support differs, so a catalog refresh that changes them is
// visible rather than silent.
func TestThinkingLevelKnownModels(t *testing.T) {
	registry := GetGlobalRegistry()

	tests := []struct {
		provider string
		model    string
		level    ThinkingLevel
		want     bool
	}{
		// "off" means "do not request reasoning" and always applies.
		{"openai", "gpt-5.4", ThinkingOff, true},
		{"anthropic", "claude-sonnet-4-6", ThinkingOff, true},
		// Unknown models are permitted every level.
		{"openai", "definitely-not-a-real-model", ThinkingHigh, true},
		{"no-such-provider", "no-such-model", ThinkingMinimal, true},
	}

	for _, tt := range tests {
		if registry.LookupModel(tt.provider, tt.model) == nil && tt.want {
			// Unknown-model cases are the point of the test; keep going.
			_ = tt
		}
		got := IsValidThinkingLevelForModel(tt.level, tt.provider, tt.model)
		if got != tt.want {
			t.Errorf("IsValidThinkingLevelForModel(%q, %q, %q) = %v, want %v",
				tt.level, tt.provider, tt.model, got, tt.want)
		}
	}
}

// TestSupportedThinkingLevelsIsConsistent checks the list form agrees with the
// predicate form, and that "off" is never dropped.
func TestSupportedThinkingLevelsIsConsistent(t *testing.T) {
	registry := GetGlobalRegistry()

	for _, provider := range []string{"openai", "anthropic", "google", "opencode"} {
		models, err := registry.GetModelsForProvider(provider)
		if err != nil {
			continue
		}
		for modelID := range models {
			levels := SupportedThinkingLevels(provider, modelID)
			if !slices.Contains(levels, ThinkingOff) {
				t.Errorf("%s/%s: 'off' missing from supported levels %v", provider, modelID, levels)
			}
			for _, l := range ThinkingLevels() {
				inList := slices.Contains(levels, l)
				valid := IsValidThinkingLevelForModel(l, provider, modelID)
				if inList != valid {
					t.Errorf("%s/%s level %q: in list = %v but IsValid = %v",
						provider, modelID, l, inList, valid)
				}
			}
		}
	}
}

func TestSuggestThinkingLevelFallback(t *testing.T) {
	// A supported level is returned unchanged.
	if got := SuggestThinkingLevelFallback(ThinkingHigh, "anthropic", "claude-sonnet-4-6"); got != ThinkingHigh {
		t.Errorf("supported level should pass through, got %q", got)
	}

	// An unknown model supports everything, so nothing is substituted.
	if got := SuggestThinkingLevelFallback(ThinkingMinimal, "openai", "not-a-model"); got != ThinkingMinimal {
		t.Errorf("unknown model should pass through, got %q", got)
	}

	// Every fallback must itself be supported, across the whole catalog.
	registry := GetGlobalRegistry()
	for _, provider := range []string{"openai", "anthropic", "google", "opencode", "azure"} {
		models, err := registry.GetModelsForProvider(provider)
		if err != nil {
			continue
		}
		for modelID := range models {
			for _, level := range ThinkingLevels() {
				fallback := SuggestThinkingLevelFallback(level, provider, modelID)
				if !IsValidThinkingLevelForModel(fallback, provider, modelID) {
					t.Errorf("%s/%s: fallback for %q is %q, which is itself unsupported",
						provider, modelID, level, fallback)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Tiered cost
// ---------------------------------------------------------------------------

func TestCostRatesFor(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	base := Cost{
		Input: 3, Output: 15,
		CacheRead: f(0.3), CacheWrite: f(3.75),
		Published: true,
		Tiers: []CostTier{
			{Threshold: 200_000, Input: 6, Output: 22.5, CacheRead: f(0.6), CacheWrite: f(7.5)},
		},
	}

	t.Run("below threshold uses base rates", func(t *testing.T) {
		r := base.RatesFor(199_999)
		if r.Input != 3 || r.Output != 15 {
			t.Errorf("got in/out %v/%v, want 3/15", r.Input, r.Output)
		}
	})

	t.Run("at threshold still uses base rates", func(t *testing.T) {
		// The tier applies *above* the threshold, matching "context over 200k".
		r := base.RatesFor(200_000)
		if r.Input != 3 || r.Output != 15 {
			t.Errorf("got in/out %v/%v, want 3/15", r.Input, r.Output)
		}
	})

	t.Run("above threshold uses tier rates", func(t *testing.T) {
		r := base.RatesFor(200_001)
		if r.Input != 6 || r.Output != 22.5 {
			t.Errorf("got in/out %v/%v, want 6/22.5", r.Input, r.Output)
		}
		if r.CacheRead == nil || *r.CacheRead != 0.6 {
			t.Errorf("cache read not switched to tier rate: %v", r.CacheRead)
		}
	})

	t.Run("no tiers always uses base rates", func(t *testing.T) {
		flat := Cost{Input: 1, Output: 2, Published: true}
		r := flat.RatesFor(10_000_000)
		if r.Input != 1 || r.Output != 2 {
			t.Errorf("flat pricing must not change, got %v/%v", r.Input, r.Output)
		}
	})

	t.Run("highest matching tier wins", func(t *testing.T) {
		multi := Cost{
			Input: 1, Output: 2, Published: true,
			Tiers: []CostTier{
				{Threshold: 100_000, Input: 2, Output: 4},
				{Threshold: 500_000, Input: 4, Output: 8},
			},
		}
		if r := multi.RatesFor(150_000); r.Input != 2 {
			t.Errorf("mid tier: got %v, want 2", r.Input)
		}
		if r := multi.RatesFor(600_000); r.Input != 4 {
			t.Errorf("top tier: got %v, want 4", r.Input)
		}
	})

	t.Run("tier without cache rate keeps the base one", func(t *testing.T) {
		c := Cost{
			Input: 1, Output: 2, CacheRead: f(0.1), Published: true,
			Tiers: []CostTier{{Threshold: 100, Input: 2, Output: 4}},
		}
		r := c.RatesFor(200)
		if r.CacheRead == nil || *r.CacheRead != 0.1 {
			t.Errorf("cache read should fall back to base rate, got %v", r.CacheRead)
		}
	})

	t.Run("Published survives tier selection", func(t *testing.T) {
		if !base.RatesFor(300_000).Published {
			t.Error("Published must be preserved")
		}
		unpriced := Cost{}
		if unpriced.RatesFor(300_000).Published {
			t.Error("unpriced model must stay unpublished")
		}
	})
}

func TestCostFromParsesTiers(t *testing.T) {
	t.Run("context_over_200k becomes a 200k tier", func(t *testing.T) {
		var c modelsDBCost
		if err := json.Unmarshal([]byte(`{
			"input": 3, "output": 15, "cache_read": 0.3,
			"context_over_200k": {"input": 6, "output": 22.5, "cache_read": 0.6}
		}`), &c); err != nil {
			t.Fatal(err)
		}
		got := costFrom(&c)
		if len(got.Tiers) != 1 {
			t.Fatalf("want 1 tier, got %d", len(got.Tiers))
		}
		if got.Tiers[0].Threshold != 200_000 {
			t.Errorf("threshold = %d, want 200000", got.Tiers[0].Threshold)
		}
		if got.Tiers[0].Input != 6 || got.Tiers[0].Output != 22.5 {
			t.Errorf("tier rates = %v/%v, want 6/22.5", got.Tiers[0].Input, got.Tiers[0].Output)
		}
	})

	t.Run("tiers array is parsed", func(t *testing.T) {
		var c modelsDBCost
		if err := json.Unmarshal([]byte(`{
			"input": 10, "output": 45,
			"tiers": [{"input": 20, "output": 90, "tier": {"type": "context", "size": 272000}}]
		}`), &c); err != nil {
			t.Fatal(err)
		}
		got := costFrom(&c)
		if len(got.Tiers) != 1 || got.Tiers[0].Threshold != 272_000 {
			t.Fatalf("unexpected tiers: %+v", got.Tiers)
		}
	})

	t.Run("non-context tiers are ignored", func(t *testing.T) {
		var c modelsDBCost
		if err := json.Unmarshal([]byte(`{
			"input": 1, "output": 2,
			"tiers": [{"input": 9, "output": 9, "tier": {"type": "something-else", "size": 100}}]
		}`), &c); err != nil {
			t.Fatal(err)
		}
		if got := costFrom(&c); len(got.Tiers) != 0 {
			t.Errorf("unknown tier type must be ignored, got %+v", got.Tiers)
		}
	})

	t.Run("tiers are sorted ascending", func(t *testing.T) {
		var c modelsDBCost
		if err := json.Unmarshal([]byte(`{
			"input": 1, "output": 2,
			"context_over_200k": {"input": 2, "output": 4},
			"tiers": [{"input": 4, "output": 8, "tier": {"type": "context", "size": 100000}}]
		}`), &c); err != nil {
			t.Fatal(err)
		}
		got := costFrom(&c)
		if len(got.Tiers) != 2 {
			t.Fatalf("want 2 tiers, got %d", len(got.Tiers))
		}
		if got.Tiers[0].Threshold >= got.Tiers[1].Threshold {
			t.Errorf("tiers not sorted ascending: %v", got.Tiers)
		}
	})

	t.Run("nil cost stays unpublished", func(t *testing.T) {
		if got := costFrom(nil); got.Published || len(got.Tiers) != 0 {
			t.Errorf("nil cost must be empty and unpublished, got %+v", got)
		}
	})
}

// TestCatalogTierPricingIsLoaded confirms tiered pricing survives the real
// catalog parse, not just synthetic JSON, and that the long-context rate is
// genuinely higher than the base rate.
func TestCatalogTierPricingIsLoaded(t *testing.T) {
	registry := GetGlobalRegistry()

	tiered := 0
	for _, provider := range registry.GetSupportedProviders() {
		models, err := registry.GetModelsForProvider(provider)
		if err != nil {
			continue
		}
		for modelID, info := range models {
			if len(info.Cost.Tiers) == 0 {
				continue
			}
			tiered++
			for _, tier := range info.Cost.Tiers {
				if tier.Threshold <= 0 {
					t.Errorf("%s/%s: tier threshold must be positive, got %d",
						provider, modelID, tier.Threshold)
				}
			}
			// Long-context pricing is a surcharge in every case published so
			// far; a cheaper tier would mean the rates were mismapped.
			hi := info.Cost.RatesFor(1 << 30)
			if hi.Input < info.Cost.Input {
				t.Errorf("%s/%s: long-context input rate %v is below base %v",
					provider, modelID, hi.Input, info.Cost.Input)
			}
		}
	}
	if tiered == 0 {
		t.Fatal("no tiered pricing found in catalog; the fields are not being parsed")
	}
	t.Logf("loaded tiered pricing for %d models", tiered)
}

// ---------------------------------------------------------------------------
// Status
// ---------------------------------------------------------------------------

func TestModelStatusIsLoaded(t *testing.T) {
	registry := GetGlobalRegistry()

	var deprecated, beta int
	for _, provider := range registry.GetSupportedProviders() {
		models, err := registry.GetModelsForProvider(provider)
		if err != nil {
			continue
		}
		for _, info := range models {
			switch info.Status {
			case StatusDeprecated:
				deprecated++
				if !info.IsDeprecated() {
					t.Errorf("IsDeprecated disagrees with Status %q", info.Status)
				}
			case StatusBeta:
				beta++
				if info.IsDeprecated() {
					t.Error("beta model must not report as deprecated")
				}
			case "":
				if info.IsDeprecated() {
					t.Error("model with no status must not report as deprecated")
				}
			}
		}
	}
	if deprecated == 0 {
		t.Fatal("no deprecated models found; status is not being parsed")
	}
	t.Logf("catalog reports %d deprecated and %d beta models", deprecated, beta)
}

// TestSuggestModelsPrefersLiveModels checks that deprecated models sort behind
// current ones, and that the result is stable across runs (the underlying scan
// walks a map).
func TestSuggestModelsPrefersLiveModels(t *testing.T) {
	registry := GetGlobalRegistry()

	first := registry.SuggestModels("openai", "gpt-4")
	for range 8 {
		if got := registry.SuggestModels("openai", "gpt-4"); !slices.Equal(got, first) {
			t.Fatalf("suggestions are unstable: %v vs %v", got, first)
		}
	}

	// Within the returned list, no live model may appear after a deprecated one.
	seenDeprecated := false
	for _, id := range first {
		info := registry.LookupModel("openai", id)
		if info == nil {
			continue
		}
		if info.IsDeprecated() {
			seenDeprecated = true
			continue
		}
		if seenDeprecated {
			t.Errorf("live model %q ranked after a deprecated one in %v", id, first)
		}
	}
}
