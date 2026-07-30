package ui

import (
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/app"
)

// msgNode builds a message node for filter/render tests.
func msgNode(id, role, text string) app.TreeNodeView {
	return app.TreeNodeView{ID: id, Kind: app.EntryKindMessage, Role: role, Text: text}
}

func TestEntryDisplayText(t *testing.T) {
	tests := []struct {
		name string
		node app.TreeNodeView
		want string
	}{
		{
			name: "user message",
			node: msgNode("1", "user", "hello there"),
			want: "user: hello there",
		},
		{
			name: "multi-line message collapses to one line",
			node: msgNode("1", "assistant", "line one\n\nline\ttwo"),
			want: "assistant: line one line two",
		},
		{
			name: "textless message is labelled as a tool interaction",
			node: msgNode("1", "assistant", ""),
			want: "assistant: (tool interaction)",
		},
		{
			name: "model change",
			node: app.TreeNodeView{Kind: app.EntryKindModelChange, Text: "anthropic/claude-sonnet-4-5"},
			want: "model: anthropic/claude-sonnet-4-5",
		},
		{
			name: "branch summary",
			node: app.TreeNodeView{Kind: app.EntryKindBranchSummary, Text: "explored the parser"},
			want: "branch summary: explored the parser",
		},
		{
			name: "compaction",
			node: app.TreeNodeView{Kind: app.EntryKindCompaction, Text: "earlier work"},
			want: "compaction: earlier work",
		},
		{
			name: "label",
			node: app.TreeNodeView{Kind: app.EntryKindLabel, Text: "checkpoint"},
			want: "label: checkpoint",
		},
		{
			name: "session info",
			node: app.TreeNodeView{Kind: app.EntryKindSessionInfo, Text: "my session"},
			want: "name: my session",
		},
		{
			name: "unknown kinds fall back rather than panic",
			node: app.TreeNodeView{Kind: app.EntryKindUnknown},
			want: "(unknown entry)",
		},
		{
			name: "extension data has no preview",
			node: app.TreeNodeView{Kind: app.EntryKindExtensionData},
			want: "(unknown entry)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := entryDisplayText(tt.node); got != tt.want {
				t.Errorf("entryDisplayText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEntryDisplayTextTruncatesLongMessages(t *testing.T) {
	long := strings.Repeat("a", 500)
	got := entryDisplayText(msgNode("1", "user", long))

	// "user: " + 200 runes (the last of which is the ellipsis).
	if want := len([]rune("user: ")) + 200; len([]rune(got)) != want {
		t.Errorf("len = %d runes, want %d", len([]rune(got)), want)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected a truncation ellipsis, got %q", got)
	}
}

func TestUserText(t *testing.T) {
	// The editor is repopulated from this, so it must not be truncated.
	long := strings.Repeat("b", 500)
	if got := userText(msgNode("1", "user", long)); got != long {
		t.Errorf("user text was altered: len %d, want %d", len(got), len(long))
	}
	if got := userText(msgNode("1", "assistant", "nope")); got != "" {
		t.Errorf("userText(assistant) = %q, want empty", got)
	}
	if got := userText(app.TreeNodeView{Kind: app.EntryKindCompaction, Text: "nope"}); got != "" {
		t.Errorf("userText(compaction) = %q, want empty", got)
	}
}

func TestPassesFilter(t *testing.T) {
	const leafID = "leaf"

	nodes := map[string]app.TreeNodeView{
		"user":        msgNode("user", "user", "question"),
		"assistant":   msgNode("assistant", "assistant", "answer"),
		"tool":        msgNode("tool", "tool", ""),
		"leafTool":    msgNode(leafID, "tool", ""),
		"modelChange": {ID: "modelChange", Kind: app.EntryKindModelChange},
		"label":       {ID: "label", Kind: app.EntryKindLabel},
		"sessionInfo": {ID: "sessionInfo", Kind: app.EntryKindSessionInfo},
		"compaction":  {ID: "compaction", Kind: app.EntryKindCompaction},
		"labelled":    {ID: "labelled", Kind: app.EntryKindCompaction, Label: "pinned"},
	}

	// want[filter][nodeKey] = should the node be visible
	want := map[TreeFilterMode]map[string]bool{
		TreeFilterAll: {
			"user": true, "assistant": true, "tool": true, "leafTool": true,
			"modelChange": true, "label": true, "sessionInfo": true,
			"compaction": true, "labelled": true,
		},
		TreeFilterDefault: {
			// Settings entries are noise; tool messages are hidden unless
			// they are the current leaf (so the active position stays visible).
			"user": true, "assistant": true, "tool": false, "leafTool": true,
			"modelChange": false, "label": false, "sessionInfo": false,
			"compaction": true, "labelled": true,
		},
		TreeFilterNoTools: {
			"user": true, "assistant": true, "tool": false, "leafTool": false,
			"modelChange": true, "label": true, "sessionInfo": true,
			"compaction": true, "labelled": true,
		},
		TreeFilterUserOnly: {
			"user": true, "assistant": false, "tool": false, "leafTool": false,
			"modelChange": false, "label": false, "sessionInfo": false,
			"compaction": false, "labelled": false,
		},
		TreeFilterLabelOnly: {
			"user": false, "assistant": false, "tool": false, "leafTool": false,
			"modelChange": false, "label": false, "sessionInfo": false,
			"compaction": false, "labelled": true,
		},
	}

	for filter, expectations := range want {
		ts := &TreeSelectorComponent{filter: filter, leafID: leafID}
		for key, visible := range expectations {
			if got := ts.passesFilter(nodes[key]); got != visible {
				t.Errorf("filter %s: passesFilter(%s) = %v, want %v", filter, key, got, visible)
			}
		}
	}
}

func TestTreeSelectorFlattensNestedTree(t *testing.T) {
	// user → assistant → tool(leaf), plus a sibling branch off the assistant.
	tree := []app.TreeNodeView{{
		ID: "u1", Kind: app.EntryKindMessage, Role: "user", Text: "hi",
		Children: []app.TreeNodeView{{
			ID: "a1", ParentID: "u1", Kind: app.EntryKindMessage, Role: "assistant", Text: "hello",
			Children: []app.TreeNodeView{
				{ID: "t1", ParentID: "a1", Kind: app.EntryKindMessage, Role: "tool"},
				{ID: "u2", ParentID: "a1", Kind: app.EntryKindMessage, Role: "user", Text: "more"},
			},
		}},
	}}

	ts := NewTreeSelector(tree, "u2", 80, 24)

	var ids []string
	for _, n := range ts.flatNodes {
		ids = append(ids, n.ID)
	}
	// Default filter hides the non-leaf tool message.
	if got := strings.Join(ids, ","); got != "u1,a1,u2" {
		t.Errorf("flattened ids = %q, want %q", got, "u1,a1,u2")
	}

	// Depth drives the indentation prefix; roots are never indented.
	if ts.flatNodes[0].Depth != 0 || ts.flatNodes[0].Prefix != "" {
		t.Errorf("root node depth/prefix = %d/%q, want 0/\"\"", ts.flatNodes[0].Depth, ts.flatNodes[0].Prefix)
	}
	if ts.flatNodes[1].Depth != 1 {
		t.Errorf("child depth = %d, want 1", ts.flatNodes[1].Depth)
	}
	if ts.flatNodes[2].Depth != 2 {
		t.Errorf("grandchild depth = %d, want 2", ts.flatNodes[2].Depth)
	}

	// The cursor starts on the active leaf.
	if got := ts.flatNodes[ts.popup.Cursor()].ID; got != "u2" {
		t.Errorf("cursor is on %q, want the leaf %q", got, "u2")
	}
}

func TestNewTreeSelectorForForkStartsOnLastUserMessage(t *testing.T) {
	tree := []app.TreeNodeView{{
		ID: "u1", Kind: app.EntryKindMessage, Role: "user", Text: "first",
		Children: []app.TreeNodeView{{
			ID: "a1", ParentID: "u1", Kind: app.EntryKindMessage, Role: "assistant", Text: "reply",
			Children: []app.TreeNodeView{{
				ID: "u2", ParentID: "a1", Kind: app.EntryKindMessage, Role: "user", Text: "second",
			}},
		}},
	}}

	ts := NewTreeSelectorForFork(tree, "u2", 80, 24)

	// Fork mode lists user messages only.
	if len(ts.flatNodes) != 2 {
		t.Fatalf("len(flatNodes) = %d, want 2: %+v", len(ts.flatNodes), ts.flatNodes)
	}
	selected := ts.flatNodes[ts.popup.Cursor()]
	if selected.ID != "u2" {
		t.Errorf("cursor is on %q, want the last user message %q", selected.ID, "u2")
	}
	// ParentID is what a fork branches from, so it must survive flattening.
	if selected.ParentID != "a1" {
		t.Errorf("ParentID = %q, want %q", selected.ParentID, "a1")
	}
}
