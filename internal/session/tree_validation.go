package session

import (
	"fmt"

	"github.com/charmbracelet/log"
)

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

	// A cycle is a real problem, so it is always reported.
	if tm.leafID != "" {
		if cycle, entry := tm.detectCycleLocked(tm.leafID); cycle {
			log.Warn("session tree: cycle detected", "entry", entry)
		}
	}

	// The rest is debug output. It used to go to stderr on every session
	// open, which was noise in the TUI and in ACP mode.
	if log.GetLevel() > log.DebugLevel {
		return
	}
	log.Debug("session tree", "entries", len(tm.entries), "leaf", tm.leafID)

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
	log.Debug("session tree entry types", "counts", counts)
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
