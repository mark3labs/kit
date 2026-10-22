package ui

import "testing"

func TestProviderCatalogEntriesAreHostOwned(t *testing.T) {
	entries := []ModelEntry{{Provider: "fake", ModelID: "echo", Name: "Echo", Available: true}}
	selector := NewModelSelectorWithEntries("fake/echo", 80, 24, entries)
	entries[0].ModelID = "changed"
	items := selector.popup.Items()
	if len(items) != 1 || items[0].Label != "echo" {
		t.Fatalf("items=%+v", items)
	}
	if item := items[0].Meta.(ModelEntry); item.Provider != "fake" || !item.Available {
		t.Fatalf("entry=%+v", item)
	}
	app := NewAppModel(nil, AppModelOptions{Width: 80, Height: 24, GetThinkingLevels: func() []string { return []string{"off", "custom"} }})
	levels := app.supportedThinkingLevels()
	if len(levels) != 2 || levels[1] != "custom" {
		t.Fatalf("levels=%v", levels)
	}
}
