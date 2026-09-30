package session

import (
	"sync"
	"testing"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/message"
)

// textOf returns the text of the first part of msg, or "" if it has none.
func textOf(t *testing.T, msg fantasy.Message) string {
	t.Helper()
	if len(msg.Content) == 0 {
		return ""
	}
	tp, ok := msg.Content[0].(fantasy.TextPart)
	if !ok {
		t.Fatalf("first part is %T, want fantasy.TextPart", msg.Content[0])
	}
	return tp.Text
}

// TestBuildContext_ResultIsIsolatedFromCache checks that decoded messages are
// cached but callers get their own copies: changing a returned message (as an
// SDK ContextPrepare hook may do) must not leak into the next BuildContext.
func TestBuildContext_ResultIsIsolatedFromCache(t *testing.T) {
	tm := InMemoryTreeSession("/test")
	if _, err := tm.AppendMessage(newTestMessage("original")); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	first, _, _ := tm.BuildContext()
	if len(first) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(first))
	}
	first[0].Content[0] = fantasy.TextPart{Text: "mutated"}
	first[0].Content = append(first[0].Content, fantasy.TextPart{Text: "extra"})
	first[0].Role = fantasy.MessageRoleSystem

	second, _, _ := tm.BuildContext()
	if got := textOf(t, second[0]); got != "original" {
		t.Errorf("text after caller mutation = %q, want %q", got, "original")
	}
	if len(second[0].Content) != 1 {
		t.Errorf("len(Content) after caller append = %d, want 1", len(second[0].Content))
	}
	if second[0].Role != fantasy.MessageRoleUser {
		t.Errorf("role after caller mutation = %q, want %q", second[0].Role, fantasy.MessageRoleUser)
	}
}

// TestBuildContext_ImageBytesIsolatedFromCache checks that image data is
// copied too: a ContextPrepare hook that edits image bytes in place (e.g. to
// redact or downscale) must not change what the next turn sends.
func TestBuildContext_ImageBytesIsolatedFromCache(t *testing.T) {
	tm := InMemoryTreeSession("/test")
	msg := newTestMessage("look at this")
	msg.Parts = append(msg.Parts, message.ImageContent{Data: []byte{1, 2, 3}, MediaType: "image/png"})
	if _, err := tm.AppendMessage(msg); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	first, _, _ := tm.BuildContext()
	file := findFilePart(t, first[0])
	file.Data[0] = 99

	second, _, _ := tm.BuildContext()
	if got := findFilePart(t, second[0]).Data; got[0] != 1 {
		t.Errorf("image bytes after caller edit = %v, want [1 2 3]", got)
	}
}

// TestCloneLLMMessage_CopiesProviderOptions checks that every ProviderOptions
// map is copied, on the message and on each part type that carries one.
func TestCloneLLMMessage_CopiesProviderOptions(t *testing.T) {
	opts := func() fantasy.ProviderOptions { return fantasy.ProviderOptions{"a": nil} }
	orig := fantasy.Message{
		Role:            fantasy.MessageRoleAssistant,
		ProviderOptions: opts(),
		Content: []fantasy.MessagePart{
			fantasy.TextPart{Text: "t", ProviderOptions: opts()},
			fantasy.ReasoningPart{Text: "r", ProviderOptions: opts()},
			fantasy.FilePart{Data: []byte{1}, ProviderOptions: opts()},
			fantasy.ToolCallPart{ToolCallID: "c", ProviderOptions: opts()},
			fantasy.ToolResultPart{ToolCallID: "c", ProviderOptions: opts()},
		},
	}

	c := cloneLLMMessage(orig)
	c.ProviderOptions["x"] = nil
	for _, part := range c.Content {
		part.Options()["x"] = nil
	}

	if _, leaked := orig.ProviderOptions["x"]; leaked {
		t.Error("message ProviderOptions is shared with the clone")
	}
	for i, part := range orig.Content {
		if _, leaked := part.Options()["x"]; leaked {
			t.Errorf("part %d (%T) ProviderOptions is shared with the clone", i, part)
		}
	}
}

// findFilePart returns the first FilePart in msg.
func findFilePart(t *testing.T, msg fantasy.Message) fantasy.FilePart {
	t.Helper()
	for _, part := range msg.Content {
		if fp, ok := part.(fantasy.FilePart); ok {
			return fp
		}
	}
	t.Fatalf("no FilePart in message %+v", msg)
	return fantasy.FilePart{}
}

// TestBuildContext_SeesEntriesAppendedAfterCaching checks that the cache
// never hides new entries: it is keyed per entry, not per context.
func TestBuildContext_SeesEntriesAppendedAfterCaching(t *testing.T) {
	tm := InMemoryTreeSession("/test")
	_, _ = tm.AppendMessage(newTestMessage("one"))
	if msgs, _, _ := tm.BuildContext(); len(msgs) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(msgs))
	}

	_, _ = tm.AppendMessage(newTestMessage("two"))
	msgs, _, _ := tm.BuildContext()
	if len(msgs) != 2 || textOf(t, msgs[1]) != "two" {
		t.Fatalf("messages after append = %d (last %q), want 2 ending in %q", len(msgs), textOf(t, msgs[len(msgs)-1]), "two")
	}
	if ids := tm.GetContextEntryIDs(); len(ids) != len(msgs) {
		t.Fatalf("len(ids) = %d, want %d", len(ids), len(msgs))
	}
}

// TestBuildContext_ConcurrentReaders exercises the lazily filled cache from
// many readers at once. It only asserts consistency; its real value is under
// `go test -race`.
func TestBuildContext_ConcurrentReaders(t *testing.T) {
	tm := InMemoryTreeSession("/test")
	for _, text := range []string{"a", "b", "c", "d"} {
		_, _ = tm.AppendMessage(newTestMessage(text))
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 20 {
				msgs, _, _ := tm.BuildContext()
				ids := tm.GetContextEntryIDs()
				if len(msgs) != 4 || len(ids) != 4 {
					t.Errorf("got %d messages / %d ids, want 4 / 4", len(msgs), len(ids))
					return
				}
			}
		})
	}
	wg.Wait()
}
