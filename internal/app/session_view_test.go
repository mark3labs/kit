package app

import (
	"errors"
	"testing"

	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/session"
)

// newSessionApp creates an App backed by an in-memory tree session.
func newSessionApp(t *testing.T) (*App, *session.TreeManager) {
	t.Helper()
	tm := session.InMemoryTreeSession(t.TempDir())
	a := New(Options{TreeSession: tm}, nil)
	return a, tm
}

// appendMessage appends a single-text-part message with the given role.
func appendMessage(t *testing.T, tm *session.TreeManager, role message.MessageRole, text string) string {
	t.Helper()
	id, err := tm.AppendMessage(message.Message{
		Role:  role,
		Parts: []message.ContentPart{message.TextContent{Text: text}},
	})
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	return id
}

func appendUserMessage(t *testing.T, tm *session.TreeManager, text string) string {
	t.Helper()
	return appendMessage(t, tm, message.RoleUser, text)
}

// --------------------------------------------------------------------------
// SessionSnapshot
// --------------------------------------------------------------------------

func TestSessionSnapshotNoSession(t *testing.T) {
	a := New(Options{}, nil)

	snap, ok := a.SessionSnapshot()
	if ok {
		t.Fatal("expected ok=false when no tree session is configured")
	}
	if snap != (SessionSnapshot{}) {
		t.Fatalf("expected zero snapshot, got %+v", snap)
	}
}

func TestSessionSnapshotReflectsSession(t *testing.T) {
	a, tm := newSessionApp(t)

	appendUserMessage(t, tm, "hello")
	if _, err := tm.AppendSessionInfo("my session"); err != nil {
		t.Fatalf("AppendSessionInfo: %v", err)
	}

	snap, ok := a.SessionSnapshot()
	if !ok {
		t.Fatal("expected ok=true with an active tree session")
	}
	if snap.ID != tm.GetSessionID() {
		t.Errorf("ID = %q, want %q", snap.ID, tm.GetSessionID())
	}
	if snap.Name != "my session" {
		t.Errorf("Name = %q, want %q", snap.Name, "my session")
	}
	if snap.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", snap.MessageCount)
	}
	if snap.EntryCount != 2 {
		t.Errorf("EntryCount = %d, want 2", snap.EntryCount)
	}
	if snap.LeafID != tm.GetLeafID() {
		t.Errorf("LeafID = %q, want %q", snap.LeafID, tm.GetLeafID())
	}
	if snap.Persisted {
		t.Error("Persisted = true, want false for an in-memory session")
	}
	if snap.FilePath != "" {
		t.Errorf("FilePath = %q, want empty for an in-memory session", snap.FilePath)
	}
}

// A snapshot is a value: mutating the session afterwards must not change the
// copy the caller already holds.
func TestSessionSnapshotIsDetached(t *testing.T) {
	a, tm := newSessionApp(t)
	appendUserMessage(t, tm, "first")

	before, _ := a.SessionSnapshot()
	appendUserMessage(t, tm, "second")

	if before.MessageCount != 1 {
		t.Errorf("held snapshot mutated: MessageCount = %d, want 1", before.MessageCount)
	}
	after, _ := a.SessionSnapshot()
	if after.MessageCount != 2 {
		t.Errorf("fresh snapshot stale: MessageCount = %d, want 2", after.MessageCount)
	}
}

// --------------------------------------------------------------------------
// SessionTree
// --------------------------------------------------------------------------

func TestSessionTreeNoSession(t *testing.T) {
	a := New(Options{}, nil)
	if tree := a.SessionTree(); tree != nil {
		t.Fatalf("expected nil tree without a session, got %+v", tree)
	}
}

func TestSessionTreeProjectsEntryKinds(t *testing.T) {
	a, tm := newSessionApp(t)

	userID := appendUserMessage(t, tm, "what is 2+2")
	appendMessage(t, tm, message.RoleAssistant, "4")
	if _, err := tm.AppendModelChange("anthropic", "claude-sonnet-4-5"); err != nil {
		t.Fatalf("AppendModelChange: %v", err)
	}
	if _, err := tm.AppendLabel(userID, "the question"); err != nil {
		t.Fatalf("AppendLabel: %v", err)
	}

	// Flatten the projected tree so we can assert on it regardless of shape.
	var flat []TreeNodeView
	var walk func([]TreeNodeView)
	walk = func(nodes []TreeNodeView) {
		for _, n := range nodes {
			flat = append(flat, n)
			walk(n.Children)
		}
	}
	walk(a.SessionTree())

	byID := make(map[string]TreeNodeView, len(flat))
	for _, n := range flat {
		byID[n.ID] = n
	}

	user, ok := byID[userID]
	if !ok {
		t.Fatalf("user message %q missing from projected tree", userID)
	}
	if user.Kind != EntryKindMessage {
		t.Errorf("user Kind = %q, want %q", user.Kind, EntryKindMessage)
	}
	if user.Role != "user" {
		t.Errorf("user Role = %q, want %q", user.Role, "user")
	}
	if user.Text != "what is 2+2" {
		t.Errorf("user Text = %q, want %q", user.Text, "what is 2+2")
	}
	if !user.IsUserMessage() {
		t.Error("IsUserMessage() = false for a user message")
	}
	// Labels applied to an entry are surfaced on the node they target.
	if user.Label != "the question" {
		t.Errorf("user Label = %q, want %q", user.Label, "the question")
	}

	var sawModelChange bool
	for _, n := range flat {
		if n.Kind == EntryKindModelChange {
			sawModelChange = true
			if n.Text != "anthropic/claude-sonnet-4-5" {
				t.Errorf("model change Text = %q, want %q", n.Text, "anthropic/claude-sonnet-4-5")
			}
			if n.Role != "" {
				t.Errorf("model change Role = %q, want empty", n.Role)
			}
			if n.IsUserMessage() {
				t.Error("IsUserMessage() = true for a model change entry")
			}
		}
	}
	if !sawModelChange {
		t.Error("model change entry missing from projected tree")
	}
}

// The projection must be a deep copy: mutating it must not be observable, and
// later session writes must not be visible in an already-taken projection.
func TestSessionTreeIsSnapshot(t *testing.T) {
	a, tm := newSessionApp(t)
	appendUserMessage(t, tm, "first")

	before := a.SessionTree()
	if len(before) != 1 {
		t.Fatalf("expected 1 root, got %d", len(before))
	}
	if len(before[0].Children) != 0 {
		t.Fatalf("expected no children, got %d", len(before[0].Children))
	}

	appendUserMessage(t, tm, "second")

	if len(before[0].Children) != 0 {
		t.Error("previously returned projection mutated by a later session write")
	}
	after := a.SessionTree()
	if len(after) != 1 || len(after[0].Children) != 1 {
		t.Errorf("fresh projection missing the new entry: %+v", after)
	}
}

func TestDescribeEntryUnknown(t *testing.T) {
	kind, role, text := describeEntry(struct{ Nonsense int }{})
	if kind != EntryKindUnknown {
		t.Errorf("Kind = %q, want %q", kind, EntryKindUnknown)
	}
	if role != "" || text != "" {
		t.Errorf("role/text = %q/%q, want empty", role, text)
	}
}

// --------------------------------------------------------------------------
// SessionHistory
// --------------------------------------------------------------------------

func TestSessionHistoryNoSession(t *testing.T) {
	a := New(Options{}, nil)
	if h := a.SessionHistory(); h != nil {
		t.Fatalf("expected nil history without a session, got %+v", h)
	}
}

func TestSessionHistoryReturnsBranchMessages(t *testing.T) {
	a, tm := newSessionApp(t)

	appendUserMessage(t, tm, "hello")
	appendMessage(t, tm, message.RoleAssistant, "hi there")
	// Non-message entries on the branch must be skipped, not returned.
	if _, err := tm.AppendModelChange("anthropic", "claude-sonnet-4-5"); err != nil {
		t.Fatalf("AppendModelChange: %v", err)
	}

	history := a.SessionHistory()
	if len(history) != 2 {
		t.Fatalf("len(history) = %d, want 2: %+v", len(history), history)
	}
	if history[0].Role != message.RoleUser || history[0].Content() != "hello" {
		t.Errorf("history[0] = %v/%q, want user/hello", history[0].Role, history[0].Content())
	}
	if history[1].Role != message.RoleAssistant || history[1].Content() != "hi there" {
		t.Errorf("history[1] = %v/%q, want assistant/hi there", history[1].Role, history[1].Content())
	}
}

// --------------------------------------------------------------------------
// Mutations
// --------------------------------------------------------------------------

func TestSessionMutationsWithoutSession(t *testing.T) {
	a := New(Options{}, nil)

	if err := a.SetSessionName("nope"); !errors.Is(err, ErrNoSession) {
		t.Errorf("SetSessionName err = %v, want ErrNoSession", err)
	}
	if err := a.NewSession(t.TempDir()); !errors.Is(err, ErrNoSession) {
		t.Errorf("NewSession err = %v, want ErrNoSession", err)
	}
	if err := a.ForkSession(t.TempDir(), "abc"); !errors.Is(err, ErrNoSession) {
		t.Errorf("ForkSession err = %v, want ErrNoSession", err)
	}
}

func TestSetSessionName(t *testing.T) {
	a, _ := newSessionApp(t)

	if err := a.SetSessionName("renamed"); err != nil {
		t.Fatalf("SetSessionName: %v", err)
	}
	snap, ok := a.SessionSnapshot()
	if !ok {
		t.Fatal("expected an active session")
	}
	if snap.Name != "renamed" {
		t.Errorf("Name = %q, want %q", snap.Name, "renamed")
	}
}
