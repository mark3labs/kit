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
	if len(updatedEntries) != 2 {
		t.Fatalf("expected 2 entries across history, got %d", len(updatedEntries))
	}
	if updatedEntries[len(updatedEntries)-1].Data != `[{"id":"1","content":"task 1","status":"completed"}]` {
		t.Errorf("unexpected latest data: %s", updatedEntries[len(updatedEntries)-1].Data)
	}
}

func TestTreeManager_ExtensionDataBranchIsolationAcrossCompaction(t *testing.T) {
	tm := InMemoryTreeSession(t.TempDir())

	// Common root extension data
	rootID, err := tm.AppendExtensionData("todo", `root`)
	if err != nil {
		t.Fatalf("root append failed: %v", err)
	}

	// Branch A: adds Branch A data, then undergoes compaction
	_, err = tm.AppendExtensionData("todo", `branch-a`)
	if err != nil {
		t.Fatalf("branch A append failed: %v", err)
	}
	compA, err := tm.AppendCompaction("Branch A summary", "", 500, 100, 2, nil, nil)
	if err != nil {
		t.Fatalf("branch A compaction failed: %v", err)
	}
	leafA := compA

	// Branch B: branches from root, adds Branch B data
	if err := tm.Branch(rootID); err != nil {
		t.Fatalf("branch back to root failed: %v", err)
	}
	_, err = tm.AppendExtensionData("todo", `branch-b`)
	if err != nil {
		t.Fatalf("branch B append failed: %v", err)
	}

	// Verify Branch B sees only root and branch-b
	entriesB := tm.GetExtensionData("todo")
	if len(entriesB) != 2 {
		t.Fatalf("expected 2 entries on branch B, got %d", len(entriesB))
	}
	if entriesB[0].Data != "root" || entriesB[1].Data != "branch-b" {
		t.Errorf("unexpected branch B entries: %+v", entriesB)
	}

	// Switch to Branch A (post-compaction)
	if err := tm.Branch(leafA); err != nil {
		t.Fatalf("branch to leafA failed: %v", err)
	}

	// Verify Branch A sees ONLY root and branch-a across compaction, NEVER branch-b!
	entriesA := tm.GetExtensionData("todo")
	if len(entriesA) != 2 {
		t.Fatalf("expected 2 entries on branch A across compaction, got %d", len(entriesA))
	}
	if entriesA[0].Data != "root" || entriesA[1].Data != "branch-a" {
		t.Errorf("unexpected branch A entries (branch B leaked!): %+v", entriesA)
	}
}
