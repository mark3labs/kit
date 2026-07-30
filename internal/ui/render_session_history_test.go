package ui

import (
	"testing"

	"github.com/mark3labs/kit/internal/message"
)

// textMessage builds a single-text-part message for history fixtures.
func textMessage(role message.MessageRole, text string) message.Message {
	return message.Message{
		Role:  role,
		Parts: []message.ContentPart{message.TextContent{Text: text}},
	}
}

// seedTranscript puts a stale message into the model's visible transcript so
// tests can assert whether renderSessionHistory clears it.
func seedTranscript(m *AppModel) {
	m.messages = []MessageItem{
		NewThemedMessageItem(generateMessageID(), "user", "stale message", func() string {
			return "stale message"
		}),
	}
}

// An empty branch must still clear the transcript. /retry and /undo pop the
// last user message and can leave the branch with no messages at all; forking
// or resuming can land on an empty session. Returning early there left the
// previous conversation on screen.
func TestRenderSessionHistoryClearsOnEmptyHistory(t *testing.T) {
	ctrl := &stubAppController{hasSession: true, sessionHistory: nil}
	m, _, _ := newTestAppModel(ctrl)
	seedTranscript(m)

	m.renderSessionHistory()

	if len(m.messages) != 0 {
		t.Fatalf("stale transcript survived an empty history: %d messages remain", len(m.messages))
	}
	if !m.layoutDirty {
		t.Error("layoutDirty = false, want true so the cleared list is re-laid out")
	}
	if !m.pendingGotoBottom {
		t.Error("pendingGotoBottom = false, want true")
	}
}

// Without a session there is nothing to render from, so the transcript must be
// left alone rather than blanked.
func TestRenderSessionHistoryLeavesTranscriptWhenNoSession(t *testing.T) {
	ctrl := &stubAppController{hasSession: false}
	m, _, _ := newTestAppModel(ctrl)
	seedTranscript(m)

	m.renderSessionHistory()

	if len(m.messages) != 1 {
		t.Fatalf("transcript was modified without an active session: %d messages", len(m.messages))
	}
}

func TestRenderSessionHistoryRendersMessages(t *testing.T) {
	ctrl := &stubAppController{
		hasSession: true,
		sessionHistory: []message.Message{
			textMessage(message.RoleUser, "what is the capital of France?"),
			textMessage(message.RoleAssistant, "Paris."),
		},
	}
	m, _, _ := newTestAppModel(ctrl)
	seedTranscript(m)

	m.renderSessionHistory()

	if len(m.messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(m.messages))
	}
	// The stale entry must be gone, replaced by the session's own history.
	first, ok := m.messages[0].(*TextMessageItem)
	if !ok {
		t.Fatalf("messages[0] is %T, want *TextMessageItem", m.messages[0])
	}
	if first.role != "user" || first.content != "what is the capital of France?" {
		t.Errorf("messages[0] = %s/%q, want user/the seeded question", first.role, first.content)
	}
	second, ok := m.messages[1].(*TextMessageItem)
	if !ok {
		t.Fatalf("messages[1] is %T, want *TextMessageItem", m.messages[1])
	}
	if second.role != "assistant" || second.content != "Paris." {
		t.Errorf("messages[1] = %s/%q, want assistant/Paris.", second.role, second.content)
	}
}

// Messages with no text content (pure tool interactions) contribute no
// transcript rows, but must still clear whatever was there before.
func TestRenderSessionHistoryClearsWhenHistoryHasNoRenderableText(t *testing.T) {
	ctrl := &stubAppController{
		hasSession:     true,
		sessionHistory: []message.Message{textMessage(message.RoleUser, "   ")},
	}
	m, _, _ := newTestAppModel(ctrl)
	seedTranscript(m)

	m.renderSessionHistory()

	if len(m.messages) != 0 {
		t.Fatalf("expected an empty transcript, got %d messages", len(m.messages))
	}
}
