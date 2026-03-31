package kit_test

import (
	"encoding/json"
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
	llmMsgs := kit.ConvertToLLMMessages(&userMsg)
	if len(llmMsgs) == 0 {
		t.Error("ConvertToLLMMessages returned empty slice")
	}

	roundTrip := kit.ConvertFromLLMMessage(llmMsgs[0])
	if roundTrip.Content() != "test" {
		t.Errorf("round-trip Content() = %q, want %q", roundTrip.Content(), "test")
	}
}

// TestLLMMessageConcrete verifies LLMMessage is a concrete Kit-owned type
// with no dependency on charm.land/fantasy in its definition.
func TestLLMMessageConcrete(t *testing.T) {
	msg := kit.LLMMessage{
		Role:    kit.LLMMessageRoleUser,
		Content: "hello world",
	}
	if msg.Role != "user" {
		t.Errorf("LLMMessage.Role = %q, want %q", msg.Role, "user")
	}
	if msg.Content != "hello world" {
		t.Errorf("LLMMessage.Content = %q, want %q", msg.Content, "hello world")
	}

	// All role constants should match their string values.
	if kit.LLMMessageRoleUser != "user" {
		t.Errorf("LLMMessageRoleUser = %q, want %q", kit.LLMMessageRoleUser, "user")
	}
	if kit.LLMMessageRoleAssistant != "assistant" {
		t.Errorf("LLMMessageRoleAssistant = %q, want %q", kit.LLMMessageRoleAssistant, "assistant")
	}
	if kit.LLMMessageRoleSystem != "system" {
		t.Errorf("LLMMessageRoleSystem = %q, want %q", kit.LLMMessageRoleSystem, "system")
	}
	if kit.LLMMessageRoleTool != "tool" {
		t.Errorf("LLMMessageRoleTool = %q, want %q", kit.LLMMessageRoleTool, "tool")
	}
}

// TestLLMUsageConcrete verifies LLMUsage is a concrete Kit-owned type.
func TestLLMUsageConcrete(t *testing.T) {
	u := kit.LLMUsage{
		InputTokens:         100,
		OutputTokens:        50,
		TotalTokens:         150,
		ReasoningTokens:     10,
		CacheCreationTokens: 5,
		CacheReadTokens:     20,
	}
	if u.InputTokens != 100 {
		t.Errorf("LLMUsage.InputTokens = %d, want 100", u.InputTokens)
	}
	if u.TotalTokens != 150 {
		t.Errorf("LLMUsage.TotalTokens = %d, want 150", u.TotalTokens)
	}

	// Verify JSON marshaling uses snake_case.
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("LLMUsage.MarshalJSON: %v", err)
	}
	if string(data) != `{"input_tokens":100,"output_tokens":50,"total_tokens":150,"reasoning_tokens":10,"cache_creation_tokens":5,"cache_read_tokens":20}` {
		t.Errorf("LLMUsage JSON = %s", data)
	}
}

// TestLLMResponseConcrete verifies LLMResponse is a concrete Kit-owned type.
func TestLLMResponseConcrete(t *testing.T) {
	r := kit.LLMResponse{
		Content:      "here is my answer",
		FinishReason: "stop",
		Usage: kit.LLMUsage{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}
	if r.Content != "here is my answer" {
		t.Errorf("LLMResponse.Content = %q, want %q", r.Content, "here is my answer")
	}
	if r.FinishReason != "stop" {
		t.Errorf("LLMResponse.FinishReason = %q, want %q", r.FinishReason, "stop")
	}
}

// TestLLMFilePartConcrete verifies LLMFilePart is a concrete Kit-owned type.
func TestLLMFilePartConcrete(t *testing.T) {
	fp := kit.LLMFilePart{
		Filename:  "screenshot.png",
		Data:      []byte{0x89, 0x50, 0x4E, 0x47},
		MediaType: "image/png",
	}
	if fp.Filename != "screenshot.png" {
		t.Errorf("LLMFilePart.Filename = %q, want %q", fp.Filename, "screenshot.png")
	}
	if fp.MediaType != "image/png" {
		t.Errorf("LLMFilePart.MediaType = %q, want %q", fp.MediaType, "image/png")
	}
	if len(fp.Data) != 4 {
		t.Errorf("LLMFilePart.Data len = %d, want 4", len(fp.Data))
	}
}

// TestConvertToLLMMessages verifies round-trip conversion preserves content.
func TestConvertToLLMMessages(t *testing.T) {
	msg := kit.Message{
		Role:  kit.RoleUser,
		Parts: []kit.ContentPart{kit.TextContent{Text: "what is 2+2?"}},
	}
	llmMsgs := kit.ConvertToLLMMessages(&msg)
	if len(llmMsgs) == 0 {
		t.Fatal("ConvertToLLMMessages returned empty slice")
	}
	if llmMsgs[0].Role != kit.LLMMessageRoleUser {
		t.Errorf("converted Role = %q, want %q", llmMsgs[0].Role, kit.LLMMessageRoleUser)
	}
	if llmMsgs[0].Content != "what is 2+2?" {
		t.Errorf("converted Content = %q, want %q", llmMsgs[0].Content, "what is 2+2?")
	}
}

// TestConvertFromLLMMessage verifies LLMMessage → Message conversion.
func TestConvertFromLLMMessage(t *testing.T) {
	llm := kit.LLMMessage{
		Role:    kit.LLMMessageRoleAssistant,
		Content: "the answer is 4",
	}
	msg := kit.ConvertFromLLMMessage(llm)
	if msg.Role != kit.RoleAssistant {
		t.Errorf("converted Role = %q, want %q", msg.Role, kit.RoleAssistant)
	}
	if msg.Content() != "the answer is 4" {
		t.Errorf("converted Content() = %q, want %q", msg.Content(), "the answer is 4")
	}
}

// TestNoFantasyInLLMTypes verifies that none of the LLM* types require a
// fantasy import to construct — they are plain Go structs.
func TestNoFantasyInLLMTypes(t *testing.T) {
	// If this file compiles without importing charm.land/fantasy,
	// the types are properly encapsulated. This test just documents intent.
	_ = kit.LLMMessage{Role: kit.LLMMessageRoleUser, Content: "hi"}
	_ = kit.LLMUsage{InputTokens: 1}
	_ = kit.LLMResponse{Content: "ok", FinishReason: "stop"}
	_ = kit.LLMFilePart{Filename: "f.png", MediaType: "image/png"}
}
