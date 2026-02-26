package extensions

import "testing"

func TestAllEventTypes_Count(t *testing.T) {
	all := AllEventTypes()
	if len(all) != 13 {
		t.Fatalf("expected 13 event types, got %d", len(all))
	}
}

func TestAllEventTypes_NoDuplicates(t *testing.T) {
	seen := make(map[EventType]bool)
	for _, et := range AllEventTypes() {
		if seen[et] {
			t.Fatalf("duplicate event type: %s", et)
		}
		seen[et] = true
	}
}

func TestEventType_IsValid(t *testing.T) {
	for _, et := range AllEventTypes() {
		if !et.IsValid() {
			t.Errorf("expected %s to be valid", et)
		}
	}

	invalid := EventType("nonexistent_event")
	if invalid.IsValid() {
		t.Error("expected 'nonexistent_event' to be invalid")
	}
}

func TestEventType_TypeMethod(t *testing.T) {
	tests := []struct {
		event Event
		want  EventType
	}{
		{ToolCallEvent{ToolName: "test"}, ToolCall},
		{ToolExecutionStartEvent{ToolName: "test"}, ToolExecutionStart},
		{ToolExecutionEndEvent{ToolName: "test"}, ToolExecutionEnd},
		{ToolResultEvent{ToolName: "test"}, ToolResult},
		{InputEvent{Text: "hello"}, Input},
		{BeforeAgentStartEvent{Prompt: "test"}, BeforeAgentStart},
		{AgentStartEvent{Prompt: "test"}, AgentStart},
		{AgentEndEvent{Response: "done"}, AgentEnd},
		{MessageStartEvent{}, MessageStart},
		{MessageUpdateEvent{Chunk: "hi"}, MessageUpdate},
		{MessageEndEvent{Content: "done"}, MessageEnd},
		{SessionStartEvent{SessionID: "abc"}, SessionStart},
		{SessionShutdownEvent{}, SessionShutdown},
	}

	for _, tt := range tests {
		if got := tt.event.Type(); got != tt.want {
			t.Errorf("event %T.Type() = %s, want %s", tt.event, got, tt.want)
		}
	}
}
