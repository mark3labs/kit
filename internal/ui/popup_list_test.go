package ui

import (
	"strings"
	"testing"
)

func TestPopupList_NewPositionsCursorOnActiveItem(t *testing.T) {
	items := []PopupItem{
		{Label: "alpha"},
		{Label: "beta"},
		{Label: "gamma", Active: true},
		{Label: "delta"},
	}
	p := NewPopupList("Test", items, 80, 40)

	if p.cursor != 2 {
		t.Errorf("expected cursor on active item (index 2), got %d", p.cursor)
	}
}

func TestPopupList_HandleKey_Navigation(t *testing.T) {
	items := []PopupItem{
		{Label: "alpha"},
		{Label: "beta"},
		{Label: "gamma"},
	}
	p := NewPopupList("Test", items, 80, 40)

	// Initial cursor at 0.
	if p.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", p.cursor)
	}

	// Down → 1.
	res := p.HandleKey("down", "")
	if !res.Changed || p.cursor != 1 {
		t.Errorf("down: changed=%v cursor=%d", res.Changed, p.cursor)
	}

	// Down → 2.
	p.HandleKey("down", "")
	if p.cursor != 2 {
		t.Errorf("expected cursor 2, got %d", p.cursor)
	}

	// Down at end → stays at 2.
	res = p.HandleKey("down", "")
	if p.cursor != 2 {
		t.Errorf("down at end: expected cursor 2, got %d", p.cursor)
	}

	// Up → 1.
	res = p.HandleKey("up", "")
	if !res.Changed || p.cursor != 1 {
		t.Errorf("up: changed=%v cursor=%d", res.Changed, p.cursor)
	}

	// Home → 0.
	p.HandleKey("home", "")
	if p.cursor != 0 {
		t.Errorf("home: expected cursor 0, got %d", p.cursor)
	}

	// End → 2.
	p.HandleKey("end", "")
	if p.cursor != 2 {
		t.Errorf("end: expected cursor 2, got %d", p.cursor)
	}
}

func TestPopupList_HandleKey_Search(t *testing.T) {
	items := []PopupItem{
		{Label: "apple"},
		{Label: "banana"},
		{Label: "cherry"},
	}
	p := NewPopupList("Test", items, 80, 40)

	// Type "an" → should filter to banana.
	p.HandleKey("a", "a")
	p.HandleKey("n", "n")

	if !p.IsSearching() {
		t.Error("expected IsSearching() to be true")
	}
	if len(p.filtered) == 0 {
		t.Fatal("expected at least one filtered result")
	}
	// banana should match (contains "an").
	found := false
	for _, item := range p.filtered {
		if item.Label == "banana" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'banana' in filtered results")
	}

	// Backspace removes last char.
	p.HandleKey("backspace", "")
	if p.search != "a" {
		t.Errorf("expected search 'a' after backspace, got %q", p.search)
	}

	// Esc clears search.
	res := p.HandleKey("esc", "")
	if res.Cancelled {
		t.Error("esc with search should clear search, not cancel")
	}
	if p.search != "" {
		t.Errorf("expected empty search after esc, got %q", p.search)
	}
}

func TestPopupList_HandleKey_SelectAndCancel(t *testing.T) {
	items := []PopupItem{
		{Label: "alpha", Meta: "first"},
		{Label: "beta", Meta: "second"},
	}
	p := NewPopupList("Test", items, 80, 40)

	// Select first item.
	res := p.HandleKey("enter", "")
	if res.Selected == nil {
		t.Fatal("expected a selection on enter")
	}
	if res.Selected.Label != "alpha" {
		t.Errorf("expected 'alpha', got %q", res.Selected.Label)
	}
	if res.Selected.Meta != "first" {
		t.Errorf("expected meta 'first', got %v", res.Selected.Meta)
	}

	// Cancel with esc (no search text).
	p2 := NewPopupList("Test", items, 80, 40)
	res = p2.HandleKey("esc", "")
	if !res.Cancelled {
		t.Error("expected Cancelled on esc with no search")
	}
}

func TestPopupList_DefaultFilter(t *testing.T) {
	items := []PopupItem{
		{Label: "foo-bar"},
		{Label: "baz-qux"},
		{Label: "foobar"},
	}

	// Exact prefix.
	result := defaultFilter("foo", items)
	if len(result) < 2 {
		t.Fatalf("expected at least 2 matches for 'foo', got %d", len(result))
	}
	// "foobar" should rank higher (shorter match) or equal to "foo-bar".
	if result[0].Label != "foobar" && result[1].Label != "foobar" {
		t.Error("expected 'foobar' in top results")
	}

	// No match.
	result = defaultFilter("zzz", items)
	if len(result) != 0 {
		t.Errorf("expected 0 matches for 'zzz', got %d", len(result))
	}
}

func TestPopupList_CustomFilterFunc(t *testing.T) {
	items := []PopupItem{
		{Label: "alpha"},
		{Label: "beta"},
		{Label: "gamma"},
	}
	p := NewPopupList("Test", items, 80, 40)
	p.FilterFunc = func(query string, allItems []PopupItem) []PopupItem {
		// Custom: only return items whose label starts with query.
		var result []PopupItem
		for _, item := range allItems {
			if strings.HasPrefix(item.Label, query) {
				result = append(result, item)
			}
		}
		return result
	}

	p.HandleKey("b", "b")
	if len(p.filtered) != 1 || p.filtered[0].Label != "beta" {
		t.Errorf("expected ['beta'], got %v", p.filtered)
	}
}

func TestPopupList_Render(t *testing.T) {
	items := []PopupItem{
		{Label: "alpha", Description: "[test]"},
		{Label: "beta", Description: "[test]", Active: true},
	}
	p := NewPopupList("My List", items, 80, 40)
	p.Subtitle = "Some subtitle"

	rendered := p.Render()
	if rendered == "" {
		t.Fatal("expected non-empty rendered output")
	}

	// Strip ANSI escape sequences for content checking.
	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "My List") {
		t.Error("expected title 'My List' in rendered output")
	}
	if !strings.Contains(plain, "alpha") {
		t.Error("expected 'alpha' in rendered output")
	}
	if !strings.Contains(plain, "beta") {
		t.Error("expected 'beta' in rendered output")
	}
	if !strings.Contains(plain, "✓") {
		t.Error("expected checkmark for active item")
	}
}

func TestPopupList_RenderCentered(t *testing.T) {
	items := []PopupItem{
		{Label: "item1"},
	}
	p := NewPopupList("Test", items, 80, 40)

	centered := p.RenderCentered(80, 40)
	if centered == "" {
		t.Fatal("expected non-empty centered output")
	}
	// Should contain newlines for vertical centering.
	lines := strings.Split(centered, "\n")
	if len(lines) < 10 {
		t.Errorf("expected centered output to have many lines, got %d", len(lines))
	}
}

func TestPopupList_EmptyItems(t *testing.T) {
	p := NewPopupList("Empty", nil, 80, 40)

	rendered := p.Render()
	if !strings.Contains(rendered, "No items") {
		t.Error("expected 'No items' for empty list")
	}

	// Navigate on empty list shouldn't panic.
	p.HandleKey("down", "")
	p.HandleKey("up", "")
	res := p.HandleKey("enter", "")
	if res.Selected != nil {
		t.Error("enter on empty list should not select")
	}
}

func TestPopupList_SearchNoResults(t *testing.T) {
	items := []PopupItem{
		{Label: "alpha"},
		{Label: "beta"},
	}
	p := NewPopupList("Test", items, 80, 40)

	// Type something that doesn't match.
	p.HandleKey("z", "z")
	p.HandleKey("z", "z")
	p.HandleKey("z", "z")

	rendered := p.Render()
	if !strings.Contains(rendered, "No matches") {
		t.Error("expected 'No matches' message for empty search results")
	}
}

func TestPopupList_CursorClamping(t *testing.T) {
	items := []PopupItem{
		{Label: "alpha"},
		{Label: "beta"},
		{Label: "gamma"},
	}
	p := NewPopupList("Test", items, 80, 40)

	// Move to last item.
	p.HandleKey("end", "")
	if p.cursor != 2 {
		t.Fatalf("expected cursor 2, got %d", p.cursor)
	}

	// Search that reduces list to 1 item → cursor should clamp.
	p.HandleKey("a", "a")
	p.HandleKey("l", "l")
	// Only "alpha" should match.
	if p.cursor >= len(p.filtered) {
		t.Errorf("cursor %d should be < filtered count %d", p.cursor, len(p.filtered))
	}
}

// stripAnsi is defined in usage_tracker_render_test.go
