package app

import (
	"testing"

	"charm.land/fantasy"
	"github.com/mark3labs/mcphost/internal/session"
)

// makeTextMsg builds a minimal fantasy.Message with a single TextPart.
func makeTextMsg(role, text string) fantasy.Message {
	return fantasy.Message{
		Role:    fantasy.MessageRole(role),
		Content: []fantasy.MessagePart{fantasy.TextPart{Text: text}},
	}
}

// --------------------------------------------------------------------------
// NewMessageStore / NewMessageStoreWithMessages
// --------------------------------------------------------------------------

func TestNewMessageStore_empty(t *testing.T) {
	s := NewMessageStore(nil)
	if s == nil {
		t.Fatal("expected non-nil store")
	}
	if s.Len() != 0 {
		t.Fatalf("expected 0 messages, got %d", s.Len())
	}
}

func TestNewMessageStoreWithMessages_preloaded(t *testing.T) {
	msgs := []fantasy.Message{
		makeTextMsg("user", "hello"),
		makeTextMsg("assistant", "hi"),
	}
	s := NewMessageStoreWithMessages(msgs, nil)
	if s.Len() != 2 {
		t.Fatalf("expected 2 messages, got %d", s.Len())
	}
}

// NewMessageStoreWithMessages must deep-copy the slice so that external
// modifications don't affect the store.
func TestNewMessageStoreWithMessages_isolatesInput(t *testing.T) {
	msgs := []fantasy.Message{makeTextMsg("user", "hello")}
	s := NewMessageStoreWithMessages(msgs, nil)

	// Mutate the source slice.
	msgs[0] = makeTextMsg("user", "mutated")

	got := s.GetAll()
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	tp, ok := got[0].Content[0].(fantasy.TextPart)
	if !ok || tp.Text != "hello" {
		t.Fatalf("store was mutated by external slice change; got %q", tp.Text)
	}
}

// --------------------------------------------------------------------------
// Add
// --------------------------------------------------------------------------

func TestAdd_appendsMessage(t *testing.T) {
	s := NewMessageStore(nil)
	s.Add(makeTextMsg("user", "first"))
	s.Add(makeTextMsg("assistant", "second"))

	if s.Len() != 2 {
		t.Fatalf("expected 2 messages, got %d", s.Len())
	}
}

func TestAdd_preservesOrder(t *testing.T) {
	s := NewMessageStore(nil)
	texts := []string{"a", "b", "c"}
	for _, t2 := range texts {
		s.Add(makeTextMsg("user", t2))
	}
	got := s.GetAll()
	for i, expected := range texts {
		tp, ok := got[i].Content[0].(fantasy.TextPart)
		if !ok || tp.Text != expected {
			t.Fatalf("message[%d]: expected %q, got %q", i, expected, tp.Text)
		}
	}
}

// --------------------------------------------------------------------------
// Replace
// --------------------------------------------------------------------------

func TestReplace_swapsHistory(t *testing.T) {
	s := NewMessageStore(nil)
	s.Add(makeTextMsg("user", "old"))

	replacement := []fantasy.Message{
		makeTextMsg("user", "new1"),
		makeTextMsg("assistant", "new2"),
	}
	s.Replace(replacement)

	if s.Len() != 2 {
		t.Fatalf("expected 2 messages after replace, got %d", s.Len())
	}
	got := s.GetAll()
	tp0, _ := got[0].Content[0].(fantasy.TextPart)
	tp1, _ := got[1].Content[0].(fantasy.TextPart)
	if tp0.Text != "new1" || tp1.Text != "new2" {
		t.Fatalf("unexpected messages after replace: %q %q", tp0.Text, tp1.Text)
	}
}

// Replace must deep-copy the incoming slice.
func TestReplace_isolatesInput(t *testing.T) {
	s := NewMessageStore(nil)
	replacement := []fantasy.Message{makeTextMsg("user", "original")}
	s.Replace(replacement)

	replacement[0] = makeTextMsg("user", "mutated")

	got := s.GetAll()
	tp, _ := got[0].Content[0].(fantasy.TextPart)
	if tp.Text != "original" {
		t.Fatalf("store was mutated by external slice change after Replace; got %q", tp.Text)
	}
}

// --------------------------------------------------------------------------
// GetAll
// --------------------------------------------------------------------------

func TestGetAll_returnsCopy(t *testing.T) {
	s := NewMessageStore(nil)
	s.Add(makeTextMsg("user", "hello"))

	got := s.GetAll()
	// Mutate the returned copy — store must not be affected.
	got[0] = makeTextMsg("user", "mutated")

	internal := s.GetAll()
	tp, _ := internal[0].Content[0].(fantasy.TextPart)
	if tp.Text != "hello" {
		t.Fatalf("GetAll returned non-copy; store was mutated to %q", tp.Text)
	}
}

func TestGetAll_emptyStore(t *testing.T) {
	s := NewMessageStore(nil)
	got := s.GetAll()
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d elements", len(got))
	}
}

// --------------------------------------------------------------------------
// Clear
// --------------------------------------------------------------------------

func TestClear_removesAllMessages(t *testing.T) {
	s := NewMessageStore(nil)
	s.Add(makeTextMsg("user", "a"))
	s.Add(makeTextMsg("user", "b"))
	s.Clear()

	if s.Len() != 0 {
		t.Fatalf("expected 0 messages after Clear, got %d", s.Len())
	}
}

func TestClear_allowsSubsequentAdds(t *testing.T) {
	s := NewMessageStore(nil)
	s.Add(makeTextMsg("user", "before"))
	s.Clear()
	s.Add(makeTextMsg("user", "after"))

	if s.Len() != 1 {
		t.Fatalf("expected 1 message after Clear+Add, got %d", s.Len())
	}
	got := s.GetAll()
	tp, _ := got[0].Content[0].(fantasy.TextPart)
	if tp.Text != "after" {
		t.Fatalf("expected %q, got %q", "after", tp.Text)
	}
}

// --------------------------------------------------------------------------
// Session.Manager bridge
// --------------------------------------------------------------------------

// newInMemoryManager creates a session.Manager that never writes to disk
// (empty filePath) so we can use it in tests without temp files.
func newInMemoryManager() *session.Manager {
	return session.NewManager("")
}

func TestSessionBridge_AddPersists(t *testing.T) {
	mgr := newInMemoryManager()
	s := NewMessageStore(mgr)

	s.Add(makeTextMsg("user", "hello"))

	// Manager.MessageCount() reflects in-memory state.
	if got := mgr.MessageCount(); got != 1 {
		t.Fatalf("expected manager to have 1 message after Add, got %d", got)
	}
}

func TestSessionBridge_ReplacePersists(t *testing.T) {
	mgr := newInMemoryManager()
	s := NewMessageStore(mgr)

	s.Add(makeTextMsg("user", "old"))
	s.Replace([]fantasy.Message{
		makeTextMsg("user", "new1"),
		makeTextMsg("assistant", "new2"),
	})

	if got := mgr.MessageCount(); got != 2 {
		t.Fatalf("expected manager to have 2 messages after Replace, got %d", got)
	}
}

func TestSessionBridge_ClearPersists(t *testing.T) {
	mgr := newInMemoryManager()
	s := NewMessageStore(mgr)

	s.Add(makeTextMsg("user", "a"))
	s.Add(makeTextMsg("user", "b"))
	s.Clear()

	if got := mgr.MessageCount(); got != 0 {
		t.Fatalf("expected manager to have 0 messages after Clear, got %d", got)
	}
}

func TestSessionBridge_NilManager_nocrash(t *testing.T) {
	// Ensure all operations work without a session manager.
	s := NewMessageStore(nil)
	s.Add(makeTextMsg("user", "a"))
	s.Replace([]fantasy.Message{makeTextMsg("user", "b")})
	s.Clear()
}

// NewMessageStoreWithMessages must NOT write to the session manager on
// construction (messages are assumed to already be persisted).
func TestSessionBridge_WithMessages_doesNotPersistOnConstruction(t *testing.T) {
	mgr := newInMemoryManager()
	_ = NewMessageStoreWithMessages([]fantasy.Message{makeTextMsg("user", "pre")}, mgr)

	// Manager should have 0 messages — construction is read-only from manager's
	// perspective; the pre-loaded messages are already on disk.
	if got := mgr.MessageCount(); got != 0 {
		t.Fatalf("expected 0 (no write on construction), got %d", got)
	}
}

// --------------------------------------------------------------------------
// Concurrency smoke test
// --------------------------------------------------------------------------

func TestConcurrentAccess(t *testing.T) {
	s := NewMessageStore(nil)
	done := make(chan struct{})

	// Writer goroutine.
	go func() {
		for i := 0; i < 100; i++ {
			s.Add(makeTextMsg("user", "concurrent"))
		}
		close(done)
	}()

	// Reader goroutine — must not race with writer.
	for i := 0; i < 50; i++ {
		_ = s.GetAll()
		_ = s.Len()
	}

	<-done
}
