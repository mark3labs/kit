package session

import (
	"testing"

	"github.com/mark3labs/kit/internal/message"
)

// TestDetectCycleWithCorruptedParentChain tests that cycle detection works
// when a corrupted session has circular parent references.
func TestDetectCycleWithCorruptedParentChain(t *testing.T) {
	tm := InMemoryTreeSession("/test")

	// Create normal chain: msg1 -> msg2 -> msg3
	id1, _ := tm.AppendMessage(message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "msg1"}}})
	_, _ = tm.AppendMessage(message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "msg2"}}})
	id3, _ := tm.AppendMessage(message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "msg3"}}})

	// Simulate corruption: manually set msg1's parent to msg3, creating cycle
	// This simulates the condition seen in the user's session
	for _, entry := range tm.entries {
		if e, ok := entry.(*MessageEntry); ok && e.ID == id1 {
			e.ParentID = id3 // Create cycle: msg1 -> msg3 -> ... -> msg1
			break
		}
	}

	// DetectCycle should find the cycle
	// The cycle is: id1 -> id3 -> id2 -> id1
	// So detecting from id3 should find id1 as the repeat
	cycle, entry := tm.DetectCycle(id3)
	if !cycle {
		t.Fatal("expected to detect cycle, but none found")
	}
	// The cycle entry could be id1 or id3 depending on where we start
	if entry != id1 && entry != id3 {
		t.Fatalf("expected cycle at %s or %s, got %s", id1, id3, entry)
	}

	// BuildContext should still work (it has its own cycle detection)
	// but will truncate at the cycle point
	msgs, _, _ := tm.BuildContext()
	if len(msgs) == 0 {
		t.Fatal("BuildContext returned no messages")
	}
}

// TestAppendMessageRejectsInvalidParent tests that AppendMessage rejects
// appending when the current leaf has a broken parent chain.
func TestAppendMessageRejectsInvalidParent(t *testing.T) {
	tm := InMemoryTreeSession("/test")

	// Create normal message
	id1, err := tm.AppendMessage(message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "msg1"}}})
	if err != nil {
		t.Fatalf("failed to append msg1: %v", err)
	}

	// Simulate corruption: set leafID to a non-existent ID
	tm.leafID = "non-existent-id"

	// Next append should fail validation
	_, err = tm.AppendMessage(message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "msg2"}}})
	if err == nil {
		t.Fatal("expected error when appending with invalid leafID, got nil")
	}

	// Restore valid leafID
	tm.leafID = id1

	// Append should succeed now
	_, err = tm.AppendMessage(message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "msg3"}}})
	if err != nil {
		t.Fatalf("failed to append msg3 after restoring leafID: %v", err)
	}
}

// TestBuildContextHandlesCycleGracefully tests that BuildContext handles
// cycles gracefully by truncating the branch.
func TestBuildContextHandlesCycleGracefully(t *testing.T) {
	tm := InMemoryTreeSession("/test")

	// Create messages
	id1, _ := tm.AppendMessage(message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "msg1"}}})
	_, _ = tm.AppendMessage(message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "msg2"}}})
	id3, _ := tm.AppendMessage(message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "msg3"}}})

	// Verify normal case works
	msgs, _, _ := tm.BuildContext()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	// Simulate cycle: set msg1's parent to msg3
	for _, entry := range tm.entries {
		if e, ok := entry.(*MessageEntry); ok && e.ID == id1 {
			e.ParentID = id3
			break
		}
	}

	// BuildContext should handle cycle gracefully (getBranchLocked has cycle detection)
	msgs, _, _ = tm.BuildContext()
	// Should only include messages from the cycle: msg3, msg2, msg1
	// (msg3 is leaf, walks to msg2 -> msg1 -> msg3 (cycle detected, stops))
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages in cycle case, got %d: %+v", len(msgs), msgs)
	}
}
