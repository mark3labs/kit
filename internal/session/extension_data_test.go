package session

import (
	"testing"
)

func TestTreeManager_ExtensionDataSurvivesCompaction(t *testing.T) {
	tm := InMemoryTreeSession(t.TempDir())

	// Append initial extension data before compaction
	extID, err := tm.AppendExtensionData("todo", `[{"id":"1","content":"task 1","status":"pending"}]`)
	if err != nil {
		t.Fatalf("AppendExtensionData failed: %v", err)
	}
	if extID == "" {
		t.Fatal("expected non-empty extension data entry ID")
	}

	// Verify it can be retrieved on current branch
	entries := tm.GetExtensionData("todo")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Data != `[{"id":"1","content":"task 1","status":"pending"}]` {
		t.Errorf("unexpected data: %s", entries[0].Data)
	}

	// Perform compaction
	_, err = tm.AppendCompaction("Conversation summary", "", 1000, 200, 5, nil, nil)
	if err != nil {
		t.Fatalf("AppendCompaction failed: %v", err)
	}

	// Verify extension data is STILL retrievable post-compaction!
	postCompactionEntries := tm.GetExtensionData("todo")
	if len(postCompactionEntries) != 1 {
		t.Fatalf("expected 1 entry post-compaction, got %d", len(postCompactionEntries))
	}
	if postCompactionEntries[0].Data != `[{"id":"1","content":"task 1","status":"pending"}]` {
		t.Errorf("unexpected data post-compaction: %s", postCompactionEntries[0].Data)
	}

	// Now append a new extension data entry on the post-compaction branch
	_, err = tm.AppendExtensionData("todo", `[{"id":"1","content":"task 1","status":"completed"}]`)
	if err != nil {
		t.Fatalf("AppendExtensionData failed: %v", err)
	}

	// Should now see the new entry on the active branch
	updatedEntries := tm.GetExtensionData("todo")
	if len(updatedEntries) != 1 {
		t.Fatalf("expected 1 entry on post-compaction branch, got %d", len(updatedEntries))
	}
	if updatedEntries[0].Data != `[{"id":"1","content":"task 1","status":"completed"}]` {
		t.Errorf("unexpected data: %s", updatedEntries[0].Data)
	}
}
