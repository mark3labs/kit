package kit_test

import (
	"testing"

	kit "github.com/mark3labs/kit/pkg/kit"
)

func TestTypeExports(t *testing.T) {
	// Verify role constants match expected string values.
	if kit.RoleUser != "user" {
		t.Errorf("RoleUser = %q, want %q", kit.RoleUser, "user")
	}
	if kit.RoleAssistant != "assistant" {
		t.Errorf("RoleAssistant = %q, want %q", kit.RoleAssistant, "assistant")
	}
	if kit.RoleTool != "tool" {
		t.Errorf("RoleTool = %q, want %q", kit.RoleTool, "tool")
	}
	if kit.RoleSystem != "system" {
		t.Errorf("RoleSystem = %q, want %q", kit.RoleSystem, "system")
	}

	// Verify Message construction and Content() accessor.
	msg := kit.Message{
		Role: kit.RoleUser,
		Parts: []kit.ContentPart{
			kit.TextContent{Text: "hello"},
		},
	}
	if msg.Content() != "hello" {
		t.Errorf("Message.Content() = %q, want %q", msg.Content(), "hello")
	}

	// Verify Finish content part compiles.
	finish := kit.Finish{Reason: "end_turn"}
	if finish.Reason != "end_turn" {
		t.Errorf("Finish.Reason = %q, want %q", finish.Reason, "end_turn")
	}

	// Verify session constructors are callable.
	s := kit.NewSession()
	if s == nil {
		t.Error("NewSession() returned nil")
	}

	mgr := kit.NewSessionManager("")
	if mgr == nil {
		t.Error("NewSessionManager() returned nil")
	}

	// Verify registry is accessible.
	reg := kit.GetGlobalRegistry()
	if reg == nil {
		t.Error("GetGlobalRegistry() returned nil")
	}

	// Verify conversion helpers compile and work.
	userMsg := kit.Message{
		Role:  kit.RoleUser,
		Parts: []kit.ContentPart{kit.TextContent{Text: "test"}},
	}
	fantasyMsgs := kit.ConvertToFantasyMessages(&userMsg)
	if len(fantasyMsgs) == 0 {
		t.Error("ConvertToFantasyMessages returned empty slice")
	}

	roundTrip := kit.ConvertFromFantasyMessage(fantasyMsgs[0])
	if roundTrip.Content() != "test" {
		t.Errorf("round-trip Content() = %q, want %q", roundTrip.Content(), "test")
	}
}
