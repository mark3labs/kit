package session

import (
	"testing"

	"github.com/mark3labs/kit/internal/message"
)

// TestCompactionParentCycleRegression tests that after multiple compactions,
// newly appended messages always have a valid parent chain and BuildContext
// returns the correct messages.
func TestCompactionParentCycleRegression(t *testing.T) {
	tm := InMemoryTreeSession("/test")

	// Simulate a long conversation with multiple compactions.
	msg1, _ := tm.AppendMessage(message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "msg1"}}})
	msg2, _ := tm.AppendMessage(message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "msg2"}}})

	// First compaction
	comp1, _ := tm.AppendCompaction("Summary 1", msg1, 1000, 500, 1, []string{}, []string{})

	msg3, _ := tm.AppendMessage(message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "msg3"}}})
	msg4, _ := tm.AppendMessage(message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "msg4"}}})

	// Second compaction
	comp2, _ := tm.AppendCompaction("Summary 2", msg3, 1000, 500, 1, []string{}, []string{})

	msg5, _ := tm.AppendMessage(message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "msg5"}}})
	msg6, _ := tm.AppendMessage(message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "msg6"}}})

	// Verify parent chain integrity
	for _, id := range []string{msg1, msg2, comp1, msg3, msg4, comp2, msg5, msg6} {
		entry := tm.GetEntry(id)
		if entry == nil {
			t.Fatalf("entry %s not found in index", id)
		}
	}

	// Walk parent chain from msg6 — must reach root without cycles
	visited := make(map[string]bool)
	current := msg6
	for current != "" {
		if visited[current] {
			t.Fatalf("cycle detected at entry %s", current)
		}
		visited[current] = true
		entry := tm.GetEntry(current)
		if entry == nil {
			t.Fatalf("entry %s missing from index during parent walk", current)
		}
		parent := ""
		switch e := entry.(type) {
		case *MessageEntry:
			parent = e.ParentID
		case *CompactionEntry:
			parent = e.ParentID
		}
		current = parent
	}

	// BuildContext should return: Summary2 + msg6 + msg5 + msg3 + msg4 = 5 messages
	msgs, _, _ := tm.BuildContext()
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d: %+v", len(msgs), msgs)
	}
}
