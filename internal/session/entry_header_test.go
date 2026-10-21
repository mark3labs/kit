package session

import "testing"

// TestEntryIDAndParentIDCoverEveryTreeEntryType guards the treeEntry
// interface: every tree entry type must expose its ID and ParentID, or
// getBranchLocked silently stops walking at it.
func TestEntryIDAndParentIDCoverEveryTreeEntryType(t *testing.T) {
	tm := InMemoryTreeSession(t.TempDir())
	hdr := func(name string) Entry {
		return Entry{ID: name + "-id", ParentID: name + "-parent"}
	}

	entries := map[string]any{
		"message":           &MessageEntry{Entry: hdr("message")},
		"incomplete_output": &IncompleteOutputEntry{Entry: hdr("incomplete_output")},
		"model_change":      &ModelChangeEntry{Entry: hdr("model_change")},
		"branch_summary":    &BranchSummaryEntry{Entry: hdr("branch_summary")},
		"label":             &LabelEntry{Entry: hdr("label")},
		"session_info":      &SessionInfoEntry{Entry: hdr("session_info")},
		"extension_data":    &ExtensionDataEntry{Entry: hdr("extension_data")},
		"compaction":        &CompactionEntry{Entry: hdr("compaction")},
	}

	for name, entry := range entries {
		if got, want := tm.EntryID(entry), name+"-id"; got != want {
			t.Errorf("EntryID(%s) = %q, want %q", name, got, want)
		}
		if got, want := tm.entryParentID(entry), name+"-parent"; got != want {
			t.Errorf("entryParentID(%s) = %q, want %q", name, got, want)
		}
	}

	// Entries outside the tree have no ID/ParentID to report.
	for _, entry := range []any{&SystemPromptEntry{ID: "sp"}, &SessionHeader{ID: "hdr"}, nil} {
		if got := tm.EntryID(entry); got != "" {
			t.Errorf("EntryID(%T) = %q, want empty", entry, got)
		}
		if got := tm.entryParentID(entry); got != "" {
			t.Errorf("entryParentID(%T) = %q, want empty", entry, got)
		}
	}
}
