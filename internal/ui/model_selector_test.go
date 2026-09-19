package ui

import "testing"

// TestFilterModels_RanksByRelevance checks that fuzzy search ranks models by
// match quality alone. A model whose provider has no credentials must still
// surface at the top when it is the best match — dimming tells the user to run
// /connect, but hiding it behind thousands of better-credentialed rows makes
// whole providers look absent from the catalogue.
func TestFilterModels_RanksByRelevance(t *testing.T) {
	items := []PopupItem{
		{
			Label:    "gpt-4o",
			Disabled: true,
			Meta:     ModelEntry{Provider: "openai", ModelID: "gpt-4o", Available: false},
		},
		{
			Label: "gpt-4o-mini",
			Meta:  ModelEntry{Provider: "local", ModelID: "gpt-4o-mini", Available: true},
		},
	}

	got := filterModels("gpt-4o", items)
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	// Exact match on the model ID beats a prefix match, credentials aside.
	if got[0].Label != "gpt-4o" {
		t.Errorf("expected the exact match first, got %q", got[0].Label)
	}
	if !got[0].Disabled {
		t.Errorf("expected the uncredentialed model to stay disabled")
	}
}

// TestFilterModels_AvailableWinsTies checks that credentials break a tie when
// two models match the query equally well.
func TestFilterModels_AvailableWinsTies(t *testing.T) {
	items := []PopupItem{
		{
			Label:    "claude-sonnet-4-6",
			Disabled: true,
			Meta:     ModelEntry{Provider: "openrouter", ModelID: "claude-sonnet-4-6", Available: false},
		},
		{
			Label: "claude-sonnet-4-6",
			Meta:  ModelEntry{Provider: "anthropic", ModelID: "claude-sonnet-4-6", Available: true},
		},
	}

	got := filterModels("claude-sonnet-4-6", items)
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	if entry := got[0].Meta.(ModelEntry); entry.Provider != "anthropic" {
		t.Errorf("equal scores: expected the credentialed provider first, got %q", entry.Provider)
	}
}

// TestFilterModels_UncredentialedProviderIsReachable checks that searching by
// provider prefix surfaces that provider's models above better-credentialed
// but weaker matches. This guards the "OpenRouter models are missing" report:
// they were present but ranked below every credentialed model in the list.
func TestFilterModels_UncredentialedProviderIsReachable(t *testing.T) {
	items := []PopupItem{
		{
			Label:    "anthropic/claude-sonnet-4",
			Disabled: true,
			Meta:     ModelEntry{Provider: "openrouter", ModelID: "anthropic/claude-sonnet-4", Available: false},
		},
	}
	// Pad the list with credentialed but weaker matches.
	for _, id := range []string{"openrouter-ish-alpha", "openrouter-ish-beta"} {
		items = append(items, PopupItem{
			Label: id,
			Meta:  ModelEntry{Provider: "local", ModelID: id, Available: true},
		})
	}

	got := filterModels("openrouter/anthropic", items)
	if len(got) == 0 {
		t.Fatal("expected the openrouter model to match a provider-prefixed query")
	}
	if entry := got[0].Meta.(ModelEntry); entry.Provider != "openrouter" {
		t.Errorf("expected the openrouter model first, got %q", entry.Provider)
	}
}

// TestModelSelector_ListsUnavailableModels checks that the selector keeps
// models from providers without credentials and marks them disabled.
func TestModelSelector_ListsUnavailableModels(t *testing.T) {
	ms := NewModelSelector("", 100, 40)
	items := ms.popup.Items()
	if len(items) == 0 {
		t.Skip("no models in registry")
	}

	var disabled, enabled int
	for _, it := range items {
		entry, ok := it.Meta.(ModelEntry)
		if !ok {
			t.Fatalf("item %q has no ModelEntry meta", it.Label)
		}
		if it.Disabled != !entry.Available {
			t.Errorf("item %q: Disabled=%v but Available=%v", it.Label, it.Disabled, entry.Available)
		}
		if it.Disabled {
			disabled++
		} else {
			enabled++
		}
	}

	// Available models must come first in the list.
	seenDisabled := false
	for _, it := range items {
		if it.Disabled {
			seenDisabled = true
			continue
		}
		if seenDisabled {
			t.Fatal("an available model appears after a disabled one")
		}
	}

	// The cursor must never rest on a disabled row when a usable row exists.
	if enabled > 0 && items[ms.popup.Cursor()].Disabled {
		t.Error("cursor landed on a disabled model")
	}
}
