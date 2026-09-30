package ui

import (
	"fmt"
	"strings"
	"testing"
	"unsafe"
)

// stableItem returns the same string on every Render, like the real message
// items do once their render is cached.
type stableItem struct {
	id      string
	content string
}

func (i *stableItem) ID() string          { return i.id }
func (i *stableItem) Render(_ int) string { return i.content }
func (i *stableItem) Height() int         { return strings.Count(i.content, "\n") + 1 }

func sameBacking(a, b []string) bool {
	return len(a) > 0 && len(b) > 0 && &a[0] == &b[0]
}

func TestScrollList_LineCacheReusedUntilContentChanges(t *testing.T) {
	item := &stableItem{id: "a", content: "one\ntwo\nthree"}
	s := NewScrollList(40, 10)
	s.SetItems([]MessageItem{item})

	first := s.renderedLines(0)
	if got := strings.Join(first, "|"); got != "one|two|three" {
		t.Fatalf("lines = %q", got)
	}
	if again := s.renderedLines(0); !sameBacking(first, again) {
		t.Error("unchanged content was split again instead of served from cache")
	}

	item.content = "one\ntwo\nthree\nfour"
	changed := s.renderedLines(0)
	if len(changed) != 4 || changed[3] != "four" {
		t.Fatalf("after content change lines = %q, want 4 lines ending in four", changed)
	}
	if got := s.renderedHeight(0); got != 4 {
		t.Errorf("renderedHeight = %d, want 4", got)
	}

	item.content = ""
	if got := s.renderedLines(0); got != nil {
		t.Errorf("empty render: lines = %q, want nil", got)
	}
}

func TestScrollList_LineCachePruned(t *testing.T) {
	s := NewScrollList(40, 10)
	for round := range 20 {
		items := make([]MessageItem, 5)
		for i := range items {
			items[i] = &stableItem{id: fmt.Sprintf("r%d-%d", round, i), content: "x\ny"}
		}
		s.SetItems(items)
		_ = s.View()
	}
	if n := len(s.lineCache); n > 2*5+64 {
		t.Errorf("lineCache holds %d entries for 5 live items; pruning is not running", n)
	}
}

func TestScrollList_SelectionFrameCache(t *testing.T) {
	item := &stableItem{id: "a", content: "hello\nworld"}
	s := NewScrollList(40, 10)
	s.SetItems([]MessageItem{item})
	s.SetSelectedIndex(0)

	f1 := s.renderItem(0)
	f2 := s.renderItem(0)
	if unsafe.StringData(f1) != unsafe.StringData(f2) {
		t.Error("unchanged selected item was framed again instead of served from cache")
	}

	s.SetSelectionFrame("LABEL", "")
	if f3 := s.renderItem(0); !strings.Contains(f3, "LABEL") {
		t.Error("label change did not reach the frame")
	}

	item.content = "changed"
	if f4 := s.renderItem(0); !strings.Contains(f4, "changed") {
		t.Error("content change did not reach the frame")
	}

	before := s.renderItem(0)
	s.SetWidth(30)
	if after := s.renderItem(0); after == before {
		t.Error("width change did not re-frame the item")
	}

	before = s.renderItem(0)
	withTheme(t, "kitt")
	if after := s.renderItem(0); unsafe.StringData(after) == unsafe.StringData(before) {
		t.Error("theme change did not re-frame the item")
	}
}
