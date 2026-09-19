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

// TestPopupList_RenderBadge verifies the kind badge is drawn after the label
// on both normal and cursor rows, and that a badge never pushes a row past
// the popup's inner width.
func TestPopupList_RenderBadge(t *testing.T) {
	items := []PopupItem{
		{Label: "/pdf-processing", Badge: "skill", Description: "Extract PDF text"},
		{Label: "/review", Badge: "prompt", Description: "Code review template"},
		{Label: "/help", Description: "Built-in, no badge"},
	}
	p := NewPopupList("Commands", items, 80, 40)
	plain := stripAnsi(p.Render())

	for _, want := range []string{"/pdf-processing  skill  Extract PDF text", "/review  prompt  Code review template", "/help Built-in"} {
		if !strings.Contains(plain, want) {
			t.Errorf("expected %q in rendered popup:\n%s", want, plain)
		}
	}

	// Cursor row keeps the badge.
	p.HandleKey("down", "")
	plain = stripAnsi(p.Render())
	if !strings.Contains(plain, "> /review  prompt  Code review template") {
		t.Errorf("expected badge on cursor row:\n%s", plain)
	}

	// Narrow popup: badge is still present and lines are bounded.
	narrow := NewPopupList("Commands", items, 40, 40)
	_, _, innerW, _ := narrow.dimensions()
	for line := range strings.SplitSeq(stripAnsi(narrow.Render()), "\n") {
		if w := len([]rune(line)); w > innerW+6 { // border(2)+padding(4)
			t.Errorf("line too wide (%d > %d): %q", w, innerW+6, line)
		}
	}
}

// --- Disabled item behaviour ---

func TestPopupList_DisabledItemsSkippedOnInit(t *testing.T) {
	items := []PopupItem{
		{Label: "locked-a", Disabled: true},
		{Label: "locked-b", Disabled: true},
		{Label: "usable"},
	}
	p := NewPopupList("Test", items, 80, 40)

	if p.cursor != 2 {
		t.Errorf("expected the initial cursor to skip disabled items to index 2, got %d", p.cursor)
	}
}

func TestPopupList_DisabledActiveItemDoesNotHoldCursor(t *testing.T) {
	items := []PopupItem{
		{Label: "usable"},
		{Label: "locked", Active: true, Disabled: true},
	}
	p := NewPopupList("Test", items, 80, 40)

	if p.cursor != 0 {
		t.Errorf("expected cursor 0 (disabled active item skipped), got %d", p.cursor)
	}
}

// Disabled rows stay reachable: the user must be able to scroll through the
// full catalogue, they simply cannot pick a locked entry.
func TestPopupList_NavigationReachesDisabled(t *testing.T) {
	items := []PopupItem{
		{Label: "a"},
		{Label: "b", Disabled: true},
		{Label: "c", Disabled: true},
	}
	p := NewPopupList("Test", items, 80, 40)

	if res := p.HandleKey("down", ""); !res.Changed || p.cursor != 1 {
		t.Errorf("down: expected cursor 1, got %d (changed=%v)", p.cursor, res.Changed)
	}
	if res := p.HandleKey("down", ""); !res.Changed || p.cursor != 2 {
		t.Errorf("down: expected cursor 2, got %d (changed=%v)", p.cursor, res.Changed)
	}
	if res := p.HandleKey("up", ""); !res.Changed || p.cursor != 1 {
		t.Errorf("up: expected cursor 1, got %d (changed=%v)", p.cursor, res.Changed)
	}
}

func TestPopupList_EnterOnDisabledItemDoesNotSelect(t *testing.T) {
	items := []PopupItem{
		{Label: "usable"},
		{Label: "locked", Disabled: true},
	}
	p := NewPopupList("Test", items, 80, 40)
	p.HandleKey("down", "")

	res := p.HandleKey("enter", "")
	if res.Selected != nil {
		t.Errorf("expected no selection for a disabled item, got %q", res.Selected.Label)
	}
	if !res.Rejected {
		t.Error("expected Rejected=true so the caller can surface a hint")
	}
	if res.Cancelled {
		t.Error("expected the popup to stay open")
	}
}

func TestPopupList_FooterExplainsDisabledRow(t *testing.T) {
	items := []PopupItem{
		{Label: "usable"},
		{Label: "locked", Disabled: true},
	}
	p := NewPopupList("Test", items, 80, 40)
	p.DisabledHint = "no credentials — run /connect"
	p.HandleKey("down", "")

	out := p.Render()
	if !strings.Contains(out, "no credentials") {
		t.Error("expected the footer to explain why the row is disabled")
	}
}

func TestPopupList_AllDisabledKeepsCursorInRange(t *testing.T) {
	items := []PopupItem{
		{Label: "a", Disabled: true},
		{Label: "b", Disabled: true},
	}
	p := NewPopupList("Test", items, 80, 40)

	if p.cursor < 0 || p.cursor >= len(items) {
		t.Fatalf("cursor out of range: %d", p.cursor)
	}
	p.HandleKey("down", "")
	p.HandleKey("up", "")
	if p.cursor < 0 || p.cursor >= len(items) {
		t.Fatalf("cursor out of range after navigation: %d", p.cursor)
	}
	// Render must not panic with an all-disabled list.
	_ = p.Render()
}

func TestPopupList_DisabledItemRendersDimmed(t *testing.T) {
	items := []PopupItem{
		{Label: "usable", Description: "[ok]"},
		{Label: "locked", Description: "[provider] no credentials", Disabled: true},
	}
	p := NewPopupList("Test", items, 80, 40)

	out := p.Render()
	if !strings.Contains(out, "locked") {
		t.Error("expected the disabled item to still be visible in the list")
	}
	if !strings.Contains(out, "no credentials") {
		t.Error("expected the disabled item description to render")
	}
}

// Filtering must re-seat the cursor on the top hit instead of keeping a
// stale index, which on a long list lands on an arbitrary (often disabled)
// row of the new result set.
func TestPopupList_SearchResetsCursorToTopHit(t *testing.T) {
	items := []PopupItem{
		{Label: "alpha"},
		{Label: "beta"},
		{Label: "gamma", Active: true},
		{Label: "delta"},
	}
	p := NewPopupList("Test", items, 80, 40)
	if p.cursor != 2 {
		t.Fatalf("expected cursor on the active item, got %d", p.cursor)
	}

	p.HandleKey("a", "a")
	if p.cursor != 0 {
		t.Errorf("expected cursor 0 after filtering, got %d", p.cursor)
	}

	// Clearing the search returns to the active item.
	p.HandleKey("esc", "")
	if p.cursor != 2 {
		t.Errorf("expected cursor back on the active item, got %d", p.cursor)
	}
}

// Searching must leave the cursor on the top hit even when that row is
// disabled. The viewport centres on the cursor, so snapping forward to the
// first selectable row scrolls the best matches off-screen — on a long
// catalogue the user sees an arbitrary slice of weak matches and concludes
// the models they searched for are missing.
func TestPopupList_SearchKeepsCursorOnDisabledTopHit(t *testing.T) {
	// Labels are equal length so defaultFilter scores them identically and
	// falls back to alphabetical order, putting the locked rows on top.
	items := []PopupItem{
		{Label: "match-locked-a", Disabled: true},
		{Label: "match-locked-b", Disabled: true},
		{Label: "match-usable-x"},
	}
	p := NewPopupList("Test", items, 80, 40)

	p.HandleKey("m", "m")
	if p.cursor != 0 {
		t.Errorf("expected the cursor to stay on the top hit (index 0), got %d", p.cursor)
	}
	if !p.Items()[p.cursor].Disabled {
		t.Error("expected the top hit to be the disabled row, not a snapped-to selectable one")
	}
}

// The no-query list still opens on a pickable row: with nothing typed there
// is no relevance order to preserve, so skipping locked rows is free.
func TestPopupList_ClearedSearchSnapsToSelectable(t *testing.T) {
	items := []PopupItem{
		{Label: "locked", Disabled: true},
		{Label: "usable"},
	}
	p := NewPopupList("Test", items, 80, 40)

	p.HandleKey("l", "l")
	if p.cursor != 0 {
		t.Fatalf("expected cursor on the disabled top hit, got %d", p.cursor)
	}

	p.HandleKey("esc", "")
	if p.cursor != 1 {
		t.Errorf("expected the cleared list to snap to the selectable row, got %d", p.cursor)
	}
}

// Enter on the disabled top hit must still be refused rather than selecting
// it, now that the cursor is allowed to rest there.
func TestPopupList_EnterOnDisabledTopHitRejected(t *testing.T) {
	items := []PopupItem{
		{Label: "match-locked", Disabled: true},
		{Label: "match-usable"},
	}
	p := NewPopupList("Test", items, 80, 40)
	p.HandleKey("m", "m")

	res := p.HandleKey("enter", "")
	if res.Selected != nil {
		t.Errorf("expected no selection, got %q", res.Selected.Label)
	}
	if !res.Rejected {
		t.Error("expected Rejected=true so the caller can surface a hint")
	}
}
