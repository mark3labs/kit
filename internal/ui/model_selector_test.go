package ui

import "testing"

// TestFilterModels_UnavailableRankLast checks that models without
// credentials always sort below usable ones, even when they match the query
// better — the first hit must always be selectable.
func TestFilterModels_UnavailableRankLast(t *testing.T) {
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
	if got[0].Label != "gpt-4o-mini" {
		t.Errorf("expected the available model first, got %q", got[0].Label)
	}
	if !got[1].Disabled {
		t.Errorf("expected the unavailable model to remain in the list and stay disabled")
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
