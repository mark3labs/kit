package kit

import (
	"slices"
	"testing"
)

func TestThinkingLevelsVocabulary(t *testing.T) {
	levels := ThinkingLevels()
	want := []string{"off", "none", "minimal", "low", "medium", "high"}
	if !slices.Equal(levels, want) {
		t.Errorf("ThinkingLevels() = %v, want %v", levels, want)
	}
}

func TestSupportedThinkingLevelsSDK(t *testing.T) {
	t.Run("always includes off", func(t *testing.T) {
		for _, m := range []struct{ provider, model string }{
			{"openai", "gpt-5.4"},
			{"anthropic", "claude-sonnet-4-6"},
			{"openai", "o3"},
			{"nope", "nope"},
		} {
			got := SupportedThinkingLevels(m.provider, m.model)
			if !slices.Contains(got, "off") {
				t.Errorf("%s/%s: 'off' missing from %v", m.provider, m.model, got)
			}
		}
	})

	t.Run("unknown model returns the full vocabulary", func(t *testing.T) {
		got := SupportedThinkingLevels("openai", "definitely-not-real")
		if !slices.Equal(got, ThinkingLevels()) {
			t.Errorf("unknown model should allow every level, got %v", got)
		}
	})

	t.Run("agrees with the predicate form", func(t *testing.T) {
		for _, m := range []struct{ provider, model string }{
			{"openai", "gpt-5.4"},
			{"openai", "o3"},
			{"anthropic", "claude-sonnet-4-6"},
		} {
			supported := SupportedThinkingLevels(m.provider, m.model)
			for _, l := range ThinkingLevels() {
				inList := slices.Contains(supported, l)
				pred := IsThinkingLevelSupported(m.provider, m.model, l)
				if inList != pred {
					t.Errorf("%s/%s level %q: list=%v predicate=%v",
						m.provider, m.model, l, inList, pred)
				}
			}
		}
	})

	t.Run("graded model reports a real subset", func(t *testing.T) {
		// o3 publishes low/medium/high only, so the SDK must not offer
		// "minimal" or "none" for it.
		if info := LookupModel("openai", "o3"); info == nil || !info.ReasoningIsGraded {
			t.Skip("openai/o3 not graded in this catalog snapshot")
		}
		got := SupportedThinkingLevels("openai", "o3")
		for _, bad := range []string{"minimal", "none"} {
			if slices.Contains(got, bad) {
				t.Errorf("o3 should not offer %q, got %v", bad, got)
			}
		}
		for _, good := range []string{"low", "medium", "high"} {
			if !slices.Contains(got, good) {
				t.Errorf("o3 should offer %q, got %v", good, got)
			}
		}
	})
}

func TestSuggestThinkingLevelSDK(t *testing.T) {
	t.Run("supported level passes through", func(t *testing.T) {
		if got := SuggestThinkingLevel("anthropic", "claude-sonnet-4-6", "high"); got != "high" {
			t.Errorf("got %q, want high", got)
		}
	})

	t.Run("unsupported level maps to a supported one", func(t *testing.T) {
		if info := LookupModel("openai", "o3"); info == nil || !info.ReasoningIsGraded {
			t.Skip("openai/o3 not graded in this catalog snapshot")
		}
		got := SuggestThinkingLevel("openai", "o3", "minimal")
		if got == "minimal" {
			t.Fatal("minimal is unsupported on o3 and should have been substituted")
		}
		if !IsThinkingLevelSupported("openai", "o3", got) {
			t.Errorf("suggested level %q is itself unsupported", got)
		}
	})

	t.Run("suggestion is always usable", func(t *testing.T) {
		for _, p := range []string{"openai", "anthropic", "google"} {
			ms, err := GetModelsForProvider(p)
			if err != nil {
				continue
			}
			for id := range ms {
				for _, l := range ThinkingLevels() {
					s := SuggestThinkingLevel(p, id, l)
					if !IsThinkingLevelSupported(p, id, s) {
						t.Errorf("%s/%s: suggestion %q for %q is unsupported", p, id, s, l)
					}
				}
			}
		}
	})
}

// TestModelCostTiersExposed confirms tiered pricing reaches SDK consumers
// through the ModelInfo alias, and that RatesFor is usable from outside.
func TestModelCostTiersExposed(t *testing.T) {
	found := false
	for _, p := range GetSupportedProviders() {
		ms, err := GetModelsForProvider(p)
		if err != nil {
			continue
		}
		for _, info := range ms {
			if len(info.Cost.Tiers) == 0 {
				continue
			}
			found = true
			base := info.Cost.RatesFor(1000)
			long := info.Cost.RatesFor(1 << 30)
			if long.Input < base.Input {
				t.Errorf("long-context rate %v below base %v", long.Input, base.Input)
			}
		}
	}
	if !found {
		t.Fatal("no tiered pricing visible through the SDK")
	}
}

// TestModelStatusExposed confirms the deprecation marker reaches the SDK.
func TestModelStatusExposed(t *testing.T) {
	deprecated := 0
	for _, p := range GetSupportedProviders() {
		ms, err := GetModelsForProvider(p)
		if err != nil {
			continue
		}
		for _, info := range ms {
			if info.IsDeprecated() {
				deprecated++
			}
		}
	}
	if deprecated == 0 {
		t.Fatal("no deprecated models visible through the SDK")
	}
	t.Logf("SDK sees %d deprecated models", deprecated)
}
