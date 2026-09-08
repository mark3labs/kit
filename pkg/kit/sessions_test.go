package kit

import (
	"errors"
	"testing"
)

// TestErrNoSessionSentinel verifies that every session-dependent entry point
// returns the shared ErrNoSession sentinel when the Kit has no session
// manager, so callers can classify the failure with errors.Is instead of
// matching the message text.
func TestErrNoSessionSentinel(t *testing.T) {
	k := &Kit{}

	if err := k.Branch("x"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Branch: got %v, want ErrNoSession", err)
	}
	if err := k.NavigateTo("x"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("NavigateTo: got %v, want ErrNoSession", err)
	}
	if _, err := k.SummarizeBranch("a", "b"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("SummarizeBranch: got %v, want ErrNoSession", err)
	}
	if err := k.CollapseBranch("a", "b", "s"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("CollapseBranch: got %v, want ErrNoSession", err)
	}
	if err := k.SetSessionName("n"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("SetSessionName: got %v, want ErrNoSession", err)
	}

	api := &extensionAPI{kit: k}
	if _, err := api.AppendEntry("ext", "{}"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("extensionAPI.AppendEntry: got %v, want ErrNoSession", err)
	}
}
