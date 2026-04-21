package session

import (
	"fmt"
	"log"
)

// ValidateParentChain checks that the parent ID points to an existing entry
// and that appending this entry would not create a cycle. This should be called
// before appending any entry to the tree.
// Returns an error if the parent is invalid or would create a cycle.
func (tm *TreeManager) ValidateParentChain(parentID string, newEntryID string) error {
	if parentID == "" {
		// Empty parent is valid (root entry)
		return nil
	}

	// Check that parent exists
	if _, ok := tm.index[parentID]; !ok {
		return fmt.Errorf("parent entry %q does not exist in index", parentID)
	}

	// Check that we're not creating a cycle by walking up the parent chain
	// from parentID and ensuring we don't hit newEntryID (or any node that
	// has newEntryID as an ancestor, but since newEntryID is new, just check
	// that parentID isn't newEntryID, which it can't be since we check existence)
	visited := make(map[string]bool)
	current := parentID
	for current != "" {
		if visited[current] {
			return fmt.Errorf("existing cycle detected at entry %q", current)
		}
		visited[current] = true

		// Safety check: if somehow we reach the new entry ID, that's a cycle
		if current == newEntryID {
			return fmt.Errorf("would create cycle: entry %q cannot be its own ancestor", newEntryID)
		}

		entry, ok := tm.index[current]
		if !ok {
			return fmt.Errorf("broken parent chain: entry %q not found", current)
		}
		current = tm.entryParentID(entry)
	}

	return nil
}

// DetectCycle walks the parent chain from the given entry ID and returns true
// if a cycle is detected. This is used for diagnostics.
func (tm *TreeManager) DetectCycle(fromID string) (cycleDetected bool, cycleEntry string) {
	visited := make(map[string]bool)
	current := fromID
	for current != "" {
		if visited[current] {
			return true, current
		}
		visited[current] = true
		entry, ok := tm.index[current]
		if !ok {
			return false, ""
		}
		current = tm.entryParentID(entry)
	}
	return false, ""
}

// LogTreeDiagnostics logs information about the tree structure for debugging.
// Call this after OpenTreeSession or when anomalies are detected.
func (tm *TreeManager) LogTreeDiagnostics() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Printf("[TreeManager] Entry count: %d, Leaf ID: %s", len(tm.entries), tm.leafID)

	// Check for cycles from leaf
	if tm.leafID != "" {
		if cycle, entry := tm.detectCycleLocked(tm.leafID); cycle {
			log.Printf("[TreeManager] WARNING: Cycle detected in tree at entry %s", entry)
		}
	}

	// Count entries by type
	counts := make(map[EntryType]int)
	for _, entry := range tm.entries {
		var et EntryType
		switch e := entry.(type) {
		case *MessageEntry:
			et = e.Type
		case *ModelChangeEntry:
			et = e.Type
		case *BranchSummaryEntry:
			et = e.Type
		case *LabelEntry:
			et = e.Type
		case *SessionInfoEntry:
			et = e.Type
		case *ExtensionDataEntry:
			et = e.Type
		case *CompactionEntry:
			et = e.Type
		default:
			et = "unknown"
		}
		counts[et]++
	}
	log.Printf("[TreeManager] Entry types: %+v", counts)
}

// detectCycleLocked is the internal version of DetectCycle (must hold read lock)
func (tm *TreeManager) detectCycleLocked(fromID string) (bool, string) {
	visited := make(map[string]bool)
	current := fromID
	for current != "" {
		if visited[current] {
			return true, current
		}
		visited[current] = true
		entry, ok := tm.index[current]
		if !ok {
			return false, ""
		}
		current = tm.entryParentID(entry)
	}
	return false, ""
}

// validateParentChainLocked is the internal version used by append methods.
// Must be called with the write lock held.
func (tm *TreeManager) validateParentChainLocked(parentID string, newEntryID string) error {
	if parentID == "" {
		return nil
	}
	if _, ok := tm.index[parentID]; !ok {
		return fmt.Errorf("parent entry %q does not exist", parentID)
	}
	// Check for existing cycles in the parent chain
	if cycle, entry := tm.detectCycleLocked(parentID); cycle {
		return fmt.Errorf("existing cycle detected at entry %q in parent chain", entry)
	}
	return nil
}
