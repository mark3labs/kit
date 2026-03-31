package app

import (
	"testing"

	"charm.land/fantasy"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// makeTextMsg builds a minimal kit.LLMMessage using fantasy.NewUserMessage
// or constructing with the given role.
func makeTextMsg(role, text string) kit.LLMMessage {
	return kit.LLMMessage{
		Role:    kit.LLMMessageRole(role),
		Content: []fantasy.MessagePart{fantasy.TextPart{Text: text}},
	}
}

// textOf extracts the plain text from an LLMMessage for assertions.
func textOf(msg kit.LLMMessage) string {
	for _, part := range msg.Content {
		if tp, ok := part.(fantasy.TextPart); ok {
			return tp.Text
		}
	}
	return ""
}

// --------------------------------------------------------------------------
// NewMessageStore / NewMessageStoreWithMessages
// --------------------------------------------------------------------------

func TestNewMessageStore_empty(t *testing.T) {
	s := NewMessageStore()
	if s == nil {
		t.Fatal("expected non-nil store")
	}
	if s.Len() != 0 {
		t.Fatalf("expected 0 messages, got %d", s.Len())
	}
}

func TestNewMessageStoreWithMessages_preloaded(t *testing.T) {
	msgs := []kit.LLMMessage{
		makeTextMsg("user", "hello"),
		makeTextMsg("assistant", "hi"),
	}
	s := NewMessageStoreWithMessages(msgs)
	if s.Len() != 2 {
		t.Fatalf("expected 2 messages, got %d", s.Len())
	}
}

// NewMessageStoreWithMessages must deep-copy the slice so that external
// modifications don't affect the store.
func TestNewMessageStoreWithMessages_isolatesInput(t *testing.T) {
	msgs := []kit.LLMMessage{makeTextMsg("user", "hello")}
	s := NewMessageStoreWithMessages(msgs)

	// Mutate the source slice.
	msgs[0] = makeTextMsg("user", "mutated")

	got := s.GetAll()
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	if textOf(got[0]) != "hello" {
		t.Fatalf("store was mutated by external slice change; got %q", textOf(got[0]))
	}
}

// --------------------------------------------------------------------------
// Add
// --------------------------------------------------------------------------

func TestAdd_appendsMessage(t *testing.T) {
	s := NewMessageStore()
	s.Add(makeTextMsg("user", "first"))
	s.Add(makeTextMsg("assistant", "second"))

	if s.Len() != 2 {
		t.Fatalf("expected 2 messages, got %d", s.Len())
	}
}

func TestAdd_preservesOrder(t *testing.T) {
	s := NewMessageStore()
	texts := []string{"a", "b", "c"}
	for _, t2 := range texts {
		s.Add(makeTextMsg("user", t2))
	}
	got := s.GetAll()
	for i, expected := range texts {
		if textOf(got[i]) != expected {
			t.Fatalf("message[%d]: expected %q, got %q", i, expected, textOf(got[i]))
		}
	}
}

// --------------------------------------------------------------------------
// Replace
// --------------------------------------------------------------------------

func TestReplace_swapsHistory(t *testing.T) {
	s := NewMessageStore()
	s.Add(makeTextMsg("user", "old"))

	replacement := []kit.LLMMessage{
		makeTextMsg("user", "new1"),
		makeTextMsg("assistant", "new2"),
	}
	s.Replace(replacement)

	if s.Len() != 2 {
		t.Fatalf("expected 2 messages after replace, got %d", s.Len())
	}
	got := s.GetAll()
	if textOf(got[0]) != "new1" || textOf(got[1]) != "new2" {
		t.Fatalf("unexpected messages after replace: %q %q", textOf(got[0]), textOf(got[1]))
	}
}

// Replace must deep-copy the incoming slice.
func TestReplace_isolatesInput(t *testing.T) {
	s := NewMessageStore()
	replacement := []kit.LLMMessage{makeTextMsg("user", "original")}
	s.Replace(replacement)

	replacement[0] = makeTextMsg("user", "mutated")

	got := s.GetAll()
	if textOf(got[0]) != "original" {
		t.Fatalf("store was mutated by external slice change after Replace; got %q", textOf(got[0]))
	}
}

// --------------------------------------------------------------------------
// GetAll
// --------------------------------------------------------------------------

func TestGetAll_returnsCopy(t *testing.T) {
	s := NewMessageStore()
	s.Add(makeTextMsg("user", "hello"))

	got := s.GetAll()
	// Mutate the returned copy — store must not be affected.
	got[0] = makeTextMsg("user", "mutated")

	internal := s.GetAll()
	if textOf(internal[0]) != "hello" {
		t.Fatalf("GetAll returned non-copy; store was mutated to %q", textOf(internal[0]))
	}
}

func TestGetAll_emptyStore(t *testing.T) {
	s := NewMessageStore()
	got := s.GetAll()
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d elements", len(got))
	}
}

// --------------------------------------------------------------------------
// Clear
// --------------------------------------------------------------------------

func TestClear_removesAllMessages(t *testing.T) {
	s := NewMessageStore()
	s.Add(makeTextMsg("user", "a"))
	s.Add(makeTextMsg("user", "b"))
	s.Clear()

	if s.Len() != 0 {
		t.Fatalf("expected 0 messages after Clear, got %d", s.Len())
	}
}

func TestClear_allowsSubsequentAdds(t *testing.T) {
	s := NewMessageStore()
	s.Add(makeTextMsg("user", "before"))
	s.Clear()
	s.Add(makeTextMsg("user", "after"))

	if s.Len() != 1 {
		t.Fatalf("expected 1 message after Clear+Add, got %d", s.Len())
	}
	got := s.GetAll()
	if textOf(got[0]) != "after" {
		t.Fatalf("expected %q, got %q", "after", textOf(got[0]))
	}
}

// --------------------------------------------------------------------------
// Concurrency smoke test
// --------------------------------------------------------------------------

func TestConcurrentAccess(t *testing.T) {
	s := NewMessageStore()
	done := make(chan struct{})

	// Writer goroutine.
	go func() {
		for range 100 {
			s.Add(makeTextMsg("user", "concurrent"))
		}
		close(done)
	}()

	// Reader goroutine — must not race with writer.
	for range 50 {
		_ = s.GetAll()
		_ = s.Len()
	}

	<-done
}
