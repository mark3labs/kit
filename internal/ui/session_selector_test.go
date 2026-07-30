package ui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/app"
)

// --------------------------------------------------------------------------
// Stub SessionStore
// --------------------------------------------------------------------------

// stubSessionStore is an in-memory SessionStore for the picker tests.
type stubSessionStore struct {
	cwdSessions []app.SessionSummary
	allSessions []app.SessionSummary

	listErr   error
	deleteErr error

	cwdArg  string
	deleted []string
}

func (s *stubSessionStore) ListSessions(cwd string) ([]app.SessionSummary, error) {
	s.cwdArg = cwd
	return s.cwdSessions, s.listErr
}

func (s *stubSessionStore) ListAllSessions() ([]app.SessionSummary, error) {
	return s.allSessions, s.listErr
}

func (s *stubSessionStore) DeleteSession(path string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, path)
	return nil
}

// summary builds a SessionSummary with a distinct modification time so
// ordering is observable.
func summary(path, name, first string) app.SessionSummary {
	return app.SessionSummary{
		Path:         path,
		ID:           "id-" + path,
		Name:         name,
		Cwd:          "/work",
		Created:      time.Now().Add(-time.Hour),
		Modified:     time.Now().Add(-time.Minute),
		MessageCount: 3,
		FirstMessage: first,
	}
}

func keyPress(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// --------------------------------------------------------------------------
// Loading
// --------------------------------------------------------------------------

func TestNewSessionSelectorLoadsFromStore(t *testing.T) {
	store := &stubSessionStore{
		cwdSessions: []app.SessionSummary{summary("/a.jsonl", "", "hello")},
		allSessions: []app.SessionSummary{
			summary("/a.jsonl", "", "hello"),
			summary("/b.jsonl", "other", "hi"),
		},
	}

	ss := NewSessionSelector(store, "/work", 80, 24)

	if store.cwdArg != "/work" {
		t.Errorf("ListSessions called with %q, want %q", store.cwdArg, "/work")
	}
	if ss.scope != SessionScopeCwd {
		t.Errorf("scope = %v, want SessionScopeCwd when the cwd has sessions", ss.scope)
	}
	if len(ss.filtered) != 1 {
		t.Errorf("got %d visible sessions, want 1 (cwd scope)", len(ss.filtered))
	}
}

func TestNewSessionSelectorEmptyCwdUsesAllScope(t *testing.T) {
	store := &stubSessionStore{
		allSessions: []app.SessionSummary{summary("/a.jsonl", "", "hello")},
	}

	ss := NewSessionSelector(store, "", 80, 24)

	if ss.scope != SessionScopeAll {
		t.Errorf("scope = %v, want SessionScopeAll when cwd is empty", ss.scope)
	}
	if len(ss.filtered) != 1 {
		t.Errorf("got %d visible sessions, want 1", len(ss.filtered))
	}
}

func TestNewSessionSelectorFallsBackToAllWhenCwdEmptyResult(t *testing.T) {
	store := &stubSessionStore{
		allSessions: []app.SessionSummary{summary("/a.jsonl", "", "hello")},
	}

	ss := NewSessionSelector(store, "/work", 80, 24)

	if ss.scope != SessionScopeAll {
		t.Errorf("scope = %v, want SessionScopeAll when the cwd has no sessions", ss.scope)
	}
}

func TestNewSessionSelectorToleratesListErrors(t *testing.T) {
	store := &stubSessionStore{listErr: errors.New("boom")}

	ss := NewSessionSelector(store, "/work", 80, 24)

	if len(ss.filtered) != 0 {
		t.Errorf("got %d sessions, want 0 when listing fails", len(ss.filtered))
	}
	if !ss.IsActive() {
		t.Error("selector should still be active after a listing error")
	}
}

// --------------------------------------------------------------------------
// Scope and filter
// --------------------------------------------------------------------------

func TestSessionSelectorScopeToggle(t *testing.T) {
	store := &stubSessionStore{
		cwdSessions: []app.SessionSummary{summary("/a.jsonl", "", "hello")},
		allSessions: []app.SessionSummary{
			summary("/a.jsonl", "", "hello"),
			summary("/b.jsonl", "other", "hi"),
		},
	}
	ss := NewSessionSelector(store, "/work", 80, 24)

	if len(ss.filtered) != 1 {
		t.Fatalf("got %d sessions in cwd scope, want 1", len(ss.filtered))
	}

	ss.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if ss.scope != SessionScopeAll {
		t.Fatalf("scope = %v, want SessionScopeAll after tab", ss.scope)
	}
	if len(ss.filtered) != 2 {
		t.Errorf("got %d sessions in all scope, want 2", len(ss.filtered))
	}

	ss.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if ss.scope != SessionScopeCwd {
		t.Errorf("scope = %v, want SessionScopeCwd after a second tab", ss.scope)
	}
}

func TestSessionSelectorNamedFilter(t *testing.T) {
	store := &stubSessionStore{
		allSessions: []app.SessionSummary{
			summary("/a.jsonl", "", "unnamed one"),
			summary("/b.jsonl", "named", "hi"),
		},
	}
	ss := NewSessionSelector(store, "", 80, 24)

	if len(ss.filtered) != 2 {
		t.Fatalf("got %d sessions, want 2 unfiltered", len(ss.filtered))
	}

	ss.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	if ss.filter != SessionFilterNamed {
		t.Fatalf("filter = %v, want SessionFilterNamed", ss.filter)
	}
	if len(ss.filtered) != 1 {
		t.Fatalf("got %d sessions, want only the named one", len(ss.filtered))
	}
	if ss.filtered[0].Name != "named" {
		t.Errorf("kept %q, want the named session", ss.filtered[0].Name)
	}
}

// --------------------------------------------------------------------------
// Selection and deletion
// --------------------------------------------------------------------------

func TestSessionSelectorEmitsSelection(t *testing.T) {
	store := &stubSessionStore{
		allSessions: []app.SessionSummary{summary("/a.jsonl", "", "hello")},
	}
	ss := NewSessionSelector(store, "", 80, 24)

	_, cmd := ss.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command on selection")
	}
	msg, ok := cmd().(SessionSelectedMsg)
	if !ok {
		t.Fatalf("got %T, want SessionSelectedMsg", cmd())
	}
	if msg.Path != "/a.jsonl" {
		t.Errorf("Path = %q, want %q", msg.Path, "/a.jsonl")
	}
	if ss.IsActive() {
		t.Error("selector should be inactive after selecting")
	}
}

func TestSessionSelectorDeleteFlow(t *testing.T) {
	store := &stubSessionStore{
		allSessions: []app.SessionSummary{
			summary("/a.jsonl", "first", "hello"),
			summary("/b.jsonl", "second", "hi"),
		},
	}
	ss := NewSessionSelector(store, "", 80, 24)

	// 'd' arms the confirmation but must not delete yet.
	ss.Update(keyPress("d"))
	if ss.confirmDelete != 0 {
		t.Fatalf("confirmDelete = %d, want 0 (cursor row armed)", ss.confirmDelete)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("deleted %v before confirmation", store.deleted)
	}

	// 'y' confirms.
	_, cmd := ss.Update(keyPress("y"))
	if len(store.deleted) != 1 || store.deleted[0] != "/a.jsonl" {
		t.Fatalf("deleted = %v, want [/a.jsonl]", store.deleted)
	}
	if cmd == nil {
		t.Fatal("expected a command after deleting")
	}
	msg, ok := cmd().(SessionDeletedMsg)
	if !ok {
		t.Fatalf("got %T, want SessionDeletedMsg", cmd())
	}
	if msg.Name != "first" {
		t.Errorf("Name = %q, want %q", msg.Name, "first")
	}
	if len(ss.filtered) != 1 || ss.filtered[0].Path != "/b.jsonl" {
		t.Errorf("remaining = %v, want only /b.jsonl", ss.filtered)
	}
}

func TestSessionSelectorDeleteCancelled(t *testing.T) {
	store := &stubSessionStore{
		allSessions: []app.SessionSummary{summary("/a.jsonl", "first", "hello")},
	}
	ss := NewSessionSelector(store, "", 80, 24)

	ss.Update(keyPress("d"))
	ss.Update(keyPress("n"))

	if ss.confirmDelete != -1 {
		t.Errorf("confirmDelete = %d, want -1 after declining", ss.confirmDelete)
	}
	if len(store.deleted) != 0 {
		t.Errorf("deleted %v, want nothing after declining", store.deleted)
	}
	if len(ss.filtered) != 1 {
		t.Errorf("got %d sessions, want the list untouched", len(ss.filtered))
	}
}

func TestSessionSelectorDeleteErrorKeepsSession(t *testing.T) {
	store := &stubSessionStore{
		allSessions: []app.SessionSummary{summary("/a.jsonl", "first", "hello")},
		deleteErr:   errors.New("permission denied"),
	}
	ss := NewSessionSelector(store, "", 80, 24)

	ss.Update(keyPress("d"))
	_, cmd := ss.Update(keyPress("y"))

	if cmd != nil {
		t.Error("expected no SessionDeletedMsg when deletion fails")
	}
	if len(ss.filtered) != 1 {
		t.Errorf("got %d sessions, want the entry kept when deletion fails", len(ss.filtered))
	}
}

// --------------------------------------------------------------------------
// Display
// --------------------------------------------------------------------------

func TestSessionDisplayName(t *testing.T) {
	tests := []struct {
		name string
		in   app.SessionSummary
		want string
	}{
		{"name wins", summary("/a", "my name", "first message"), "my name"},
		{"first message fallback", summary("/a", "", "first message"), "first message"},
		{"empty session", summary("/a", "", ""), "(empty session)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sessionDisplayName(tt.in); got != tt.want {
				t.Errorf("sessionDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
