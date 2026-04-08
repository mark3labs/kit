package session

import (
	"slices"
	"testing"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/message"
)

// TestCompactionCreatesNewLeaf verifies that after compaction, the compaction
// entry has no parent (creating a new root), and BuildContext returns only
// the summary and kept messages, not the old compacted messages.
func TestCompactionCreatesNewLeaf(t *testing.T) {
	tm := InMemoryTreeSession("/test")

	// Add some messages: M1, M2 (old, will be compacted), M3, M4 (kept)
	msg1 := message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "Message 1 - old"}}}
	msg2 := message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "Message 2 - old"}}}
	msg3 := message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "Message 3 - kept"}}}
	msg4 := message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "Message 4 - kept"}}}

	_, _ = tm.AppendMessage(msg1)
	_, _ = tm.AppendMessage(msg2)
	id3, _ := tm.AppendMessage(msg3)
	id4, _ := tm.AppendMessage(msg4)

	// Verify initial state - all messages should be in context
	messages, _, _ := tm.BuildContext()
	if len(messages) != 4 {
		t.Fatalf("expected 4 messages before compaction, got %d", len(messages))
	}

	// Verify entry IDs
	entryIDs := tm.GetContextEntryIDs()
	if len(entryIDs) != 4 {
		t.Fatalf("expected 4 entry IDs before compaction, got %d", len(entryIDs))
	}

	// Now add a compaction entry, simulating that M3 is the first kept entry
	summary := "Summary of old messages"
	compactionID, err := tm.AppendCompaction(summary, id3, 1000, 500, 2, []string{}, []string{})
	if err != nil {
		t.Fatalf("failed to append compaction: %v", err)
	}

	// Verify the compaction entry has no parent (empty ParentID)
	compactionEntry := tm.GetEntry(compactionID).(*CompactionEntry)
	if compactionEntry.ParentID != "" {
		t.Errorf("compaction entry should have no parent, got %q", compactionEntry.ParentID)
	}

	// Verify the leaf is now the compaction entry
	if tm.GetLeafID() != compactionID {
		t.Errorf("leaf should be compaction entry %q, got %q", compactionID, tm.GetLeafID())
	}

	// Now BuildContext should return: [summary] + [M3, M4]
	messages, _, _ = tm.BuildContext()
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages after compaction (summary + 2 kept), got %d", len(messages))
	}

	// First message should be the summary
	if messages[0].Role != fantasy.MessageRoleSystem {
		t.Errorf("first message should be system summary, got %s", messages[0].Role)
	}
	summaryText := messages[0].Content[0].(fantasy.TextPart).Text
	if summaryText != "[Conversation summary — earlier messages were compacted]\n\n"+summary {
		t.Errorf("unexpected summary text: %s", summaryText)
	}

	// Second message should be M3 (kept)
	if messages[1].Role != fantasy.MessageRoleUser {
		t.Errorf("second message should be user (M3), got %s", messages[1].Role)
	}
	m3Text := messages[1].Content[0].(fantasy.TextPart).Text
	if m3Text != "Message 3 - kept" {
		t.Errorf("unexpected M3 text: %s", m3Text)
	}

	// Third message should be M4 (kept)
	if messages[2].Role != fantasy.MessageRoleAssistant {
		t.Errorf("third message should be assistant (M4), got %s", messages[2].Role)
	}
	m4Text := messages[2].Content[0].(fantasy.TextPart).Text
	if m4Text != "Message 4 - kept" {
		t.Errorf("unexpected M4 text: %s", m4Text)
	}

	// Verify GetContextEntryIDs returns correct IDs
	entryIDs = tm.GetContextEntryIDs()
	if len(entryIDs) != 3 {
		t.Fatalf("expected 3 entry IDs after compaction (empty for summary + 2 kept), got %d: %v", len(entryIDs), entryIDs)
	}

	// First entry ID should be empty (summary has no entry)
	if entryIDs[0] != "" {
		t.Errorf("first entry ID should be empty (summary), got %q", entryIDs[0])
	}

	// Second and third should be id3 and id4 (the kept messages)
	if entryIDs[1] != id3 {
		t.Errorf("second entry ID should be %q (M3), got %q", id3, entryIDs[1])
	}
	if entryIDs[2] != id4 {
		t.Errorf("third entry ID should be %q (M4), got %q", id4, entryIDs[2])
	}
}

// TestCompactionWithNewMessagesAfterCompaction verifies that messages appended
// after compaction are correctly included in the context.
func TestCompactionWithNewMessagesAfterCompaction(t *testing.T) {
	tm := InMemoryTreeSession("/test")

	// Add initial messages
	msg1 := message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "Message 1"}}}
	msg2 := message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "Message 2"}}}
	msg3 := message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "Message 3 - kept"}}}

	_, _ = tm.AppendMessage(msg1)
	_, _ = tm.AppendMessage(msg2)
	id3, _ := tm.AppendMessage(msg3)

	// Compact, keeping only M3
	_, _ = tm.AppendCompaction("Summary", id3, 1000, 500, 2, []string{}, []string{})

	// Add a new message after compaction
	msg4 := message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "Message 4 - after compaction"}}}
	_, _ = tm.AppendMessage(msg4)

	// BuildContext should return: [summary] + [M4 (new after compaction)] + [M3 (kept)]
	messages, _, _ := tm.BuildContext()
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages (summary + M4 + M3), got %d: %+v", len(messages), messages)
	}

	// Verify order: summary, M4 (new), M3 (kept)
	if messages[0].Role != fantasy.MessageRoleSystem {
		t.Errorf("first message should be summary, got %s", messages[0].Role)
	}
	if messages[1].Role != fantasy.MessageRoleAssistant {
		t.Errorf("second message should be assistant (M4), got %s", messages[1].Role)
	}
	m4Text := messages[1].Content[0].(fantasy.TextPart).Text
	if m4Text != "Message 4 - after compaction" {
		t.Errorf("unexpected M4 text: %s", m4Text)
	}
	if messages[2].Role != fantasy.MessageRoleUser {
		t.Errorf("third message should be user (M3), got %s", messages[2].Role)
	}

	// Verify that M1 is NOT in the context
	for i, msg := range messages {
		if msg.Role == fantasy.MessageRoleUser {
			text := msg.Content[0].(fantasy.TextPart).Text
			if text == "Message 1" {
				t.Errorf("Message 1 (compacted) should not be in context at index %d", i)
			}
		}
	}
}

// TestCompactionWithNoKeptMessages verifies compaction when all messages are compacted.
func TestCompactionWithNoKeptMessages(t *testing.T) {
	tm := InMemoryTreeSession("/test")

	// Add messages that will all be compacted
	msg1 := message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "Message 1"}}}
	msg2 := message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "Message 2"}}}

	if _, err := tm.AppendMessage(msg1); err != nil {
		t.Fatalf("failed to append message: %v", err)
	}
	if _, err := tm.AppendMessage(msg2); err != nil {
		t.Fatalf("failed to append message: %v", err)
	}

	// Compact with no kept messages (empty firstKeptEntryID)
	summary := "All messages summarized"
	compactionID, _ := tm.AppendCompaction(summary, "", 1000, 100, 2, []string{}, []string{})

	// Verify the compaction entry has no parent
	compactionEntry := tm.GetEntry(compactionID).(*CompactionEntry)
	if compactionEntry.ParentID != "" {
		t.Errorf("compaction entry should have no parent, got %q", compactionEntry.ParentID)
	}

	// BuildContext should return only the summary
	messages, _, _ := tm.BuildContext()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message (summary only), got %d: %+v", len(messages), messages)
	}
	if messages[0].Role != fantasy.MessageRoleSystem {
		t.Errorf("message should be system summary, got %s", messages[0].Role)
	}
}

// TestMultipleCompactions verifies that multiple compactions work correctly.
func TestMultipleCompactions(t *testing.T) {
	tm := InMemoryTreeSession("/test")

	// First batch of messages
	msg1 := message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "Batch 1 - User"}}}
	msg2 := message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "Batch 1 - Assistant"}}}
	id1, _ := tm.AppendMessage(msg1)
	id2, _ := tm.AppendMessage(msg2)

	// First compaction
	_, _ = tm.AppendCompaction("Summary 1", id1, 1000, 500, 1, []string{}, []string{})

	// Second batch
	msg3 := message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "Batch 2 - User"}}}
	msg4 := message.Message{Role: message.RoleAssistant, Parts: []message.ContentPart{message.TextContent{Text: "Batch 2 - Assistant"}}}
	id3, _ := tm.AppendMessage(msg3)
	id4, _ := tm.AppendMessage(msg4)

	// Second compaction (compacting the first compaction + batch 2)
	// Note: id3 is the first kept entry, so id3 and id4 should be preserved
	compactionID2, _ := tm.AppendCompaction("Summary 2", id3, 1000, 500, 3, []string{}, []string{})

	// Verify second compaction has no parent
	compactionEntry2 := tm.GetEntry(compactionID2).(*CompactionEntry)
	if compactionEntry2.ParentID != "" {
		t.Errorf("second compaction entry should have no parent, got %q", compactionEntry2.ParentID)
	}

	// Add final message
	msg5 := message.Message{Role: message.RoleUser, Parts: []message.ContentPart{message.TextContent{Text: "Final message"}}}
	id5, _ := tm.AppendMessage(msg5)

	// BuildContext should include:
	// - Summary 2 (from second compaction)
	// - msg5 (final message)
	// - msg3, msg4 (kept from second compaction)
	// But NOT Summary 1 or msg1, msg2 (they're before the first kept entry of compaction 2)
	messages, _, _ := tm.BuildContext()

	// Should have: Summary 2 + msg5 + msg3 + msg4 = 4 messages
	if len(messages) != 4 {
		t.Fatalf("expected 4 messages (Summary 2 + msg5 + msg3 + msg4), got %d: %+v", len(messages), messages)
	}

	// First should be Summary 2
	if messages[0].Role != fantasy.MessageRoleSystem {
		t.Errorf("first message should be system (Summary 2), got %s", messages[0].Role)
	}
	summaryText := messages[0].Content[0].(fantasy.TextPart).Text
	if summaryText != "[Conversation summary — earlier messages were compacted]\n\nSummary 2" {
		t.Errorf("unexpected summary: %s", summaryText)
	}

	// Verify msg5 is included
	foundFinal := false
	for _, msg := range messages {
		if msg.Role == fantasy.MessageRoleUser {
			text := msg.Content[0].(fantasy.TextPart).Text
			if text == "Final message" {
				foundFinal = true
				break
			}
		}
	}
	if !foundFinal {
		t.Error("Final message (msg5) should be in context")
	}

	// Verify msg1, msg2 are NOT included (compacted by first compaction, then second)
	for _, msg := range messages {
		if msg.Role == fantasy.MessageRoleUser || msg.Role == fantasy.MessageRoleAssistant {
			text := msg.Content[0].(fantasy.TextPart).Text
			if text == "Batch 1 - User" || text == "Batch 1 - Assistant" {
				t.Errorf("Batch 1 messages should not be in context, found: %s", text)
			}
		}
	}

	// Verify entry IDs
	entryIDs := tm.GetContextEntryIDs()
	if len(entryIDs) != 4 {
		t.Fatalf("expected 4 entry IDs, got %d: %v", len(entryIDs), entryIDs)
	}

	// First should be empty (summary)
	if entryIDs[0] != "" {
		t.Errorf("first entry ID should be empty (summary), got %q", entryIDs[0])
	}

	// Check that id5 is in the list
	if !slices.Contains(entryIDs, id5) {
		t.Errorf("id5 (final message) should be in entry IDs, got %v", entryIDs)
	}

	// Verify id3 and id4 ARE in the list (they were kept)
	foundID3, foundID4 := false, false
	for _, id := range entryIDs {
		if id == id3 {
			foundID3 = true
		}
		if id == id4 {
			foundID4 = true
		}
	}
	if !foundID3 {
		t.Errorf("id3 (kept message) should be in entry IDs, got %v", entryIDs)
	}
	if !foundID4 {
		t.Errorf("id4 (kept message) should be in entry IDs, got %v", entryIDs)
	}

	// Verify id1 and id2 are NOT in the list (they were compacted away)
	for _, id := range entryIDs {
		if id == id1 || id == id2 {
			t.Errorf("id1 or id2 (compacted) should not be in entry IDs, found %q in %v", id, entryIDs)
		}
	}
}
