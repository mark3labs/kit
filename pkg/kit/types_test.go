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

// TestLLMRoleConstants verifies the LLM role constants have the correct values.
func TestLLMRoleConstants(t *testing.T) {
	if kit.LLMRoleUser != "user" {
		t.Errorf("LLMRoleUser = %q, want %q", kit.LLMRoleUser, "user")
	}
	if kit.LLMRoleAssistant != "assistant" {
		t.Errorf("LLMRoleAssistant = %q, want %q", kit.LLMRoleAssistant, "assistant")
	}
	if kit.LLMRoleSystem != "system" {
		t.Errorf("LLMRoleSystem = %q, want %q", kit.LLMRoleSystem, "system")
	}
	if kit.LLMRoleTool != "tool" {
		t.Errorf("LLMRoleTool = %q, want %q", kit.LLMRoleTool, "tool")
	}
}

// TestLLMMessageAlias verifies LLMMessage is a type alias for the underlying
// LLM provider message type and can be used interchangeably.
func TestLLMMessageAlias(t *testing.T) {
	// Construct an LLMMessage using alias types.
	msg := kit.LLMMessage{
		Role: kit.LLMRoleUser,
		Content: []kit.LLMMessagePart{
			kit.LLMTextPart{Text: "hello world"},
		},
	}
	if msg.Role != "user" {
		t.Errorf("LLMMessage.Role = %q, want %q", msg.Role, "user")
	}
	// Verify we can extract text via the part types.
	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(msg.Content))
	}
	tp, ok := msg.Content[0].(kit.LLMTextPart)
	if !ok {
		t.Fatal("content part is not LLMTextPart")
	}
	if tp.Text != "hello world" {
		t.Errorf("LLMTextPart.Text = %q, want %q", tp.Text, "hello world")
	}
}

// TestNewLLMUserMessage verifies the NewLLMUserMessage constructor works.
func TestNewLLMUserMessage(t *testing.T) {
	msg := kit.NewLLMUserMessage("hello from user")
	if msg.Role != kit.LLMRoleUser {
		t.Errorf("NewLLMUserMessage role = %q, want %q", msg.Role, kit.LLMRoleUser)
	}
	if len(msg.Content) == 0 {
		t.Fatal("NewLLMUserMessage content is empty")
	}
	tp, ok := msg.Content[0].(kit.LLMTextPart)
	if !ok {
		t.Fatal("content[0] is not LLMTextPart")
	}
	if tp.Text != "hello from user" {
		t.Errorf("NewLLMUserMessage text = %q, want %q", tp.Text, "hello from user")
	}
}

// TestNewLLMSystemMessage verifies the NewLLMSystemMessage constructor works.
func TestNewLLMSystemMessage(t *testing.T) {
	msg := kit.NewLLMSystemMessage("you are helpful")
	if msg.Role != kit.LLMRoleSystem {
		t.Errorf("NewLLMSystemMessage role = %q, want %q", msg.Role, kit.LLMRoleSystem)
	}
	if len(msg.Content) == 0 {
		t.Fatal("NewLLMSystemMessage content is empty")
	}
}

// TestLLMUsageAlias verifies LLMUsage is a type alias for the underlying
// LLM provider usage type and carries the correct fields.
func TestLLMUsageAlias(t *testing.T) {
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

	// Verify JSON marshaling uses snake_case (inherited from the provider's tags).
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("LLMUsage.MarshalJSON: %v", err)
	}
	jsonStr := string(data)
	if jsonStr == "" {
		t.Error("LLMUsage JSON is empty")
	}
	// Check that input_tokens key is present.
	if !containsStr(jsonStr, `"input_tokens":100`) {
		t.Errorf("LLMUsage JSON missing input_tokens: %s", jsonStr)
	}
}

// TestLLMFilePartAlias verifies LLMFilePart is a type alias for the underlying
// LLM provider file part type.
func TestLLMFilePartAlias(t *testing.T) {
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

	// Verify it can be used as a file part for constructing user messages.
	msg := kit.NewLLMUserMessage("see this image", fp)
	if msg.Role != kit.LLMRoleUser {
		t.Errorf("message role = %q, want user", msg.Role)
	}
}

// TestLLMPartTypesAlias verifies all the part type aliases compile and work.
func TestLLMPartTypesAlias(t *testing.T) {
	// LLMTextPart
	tp := kit.LLMTextPart{Text: "plain text"}
	if tp.Text != "plain text" {
		t.Errorf("LLMTextPart.Text = %q", tp.Text)
	}

	// LLMReasoningPart
	rp := kit.LLMReasoningPart{Text: "I think therefore"}
	if rp.Text != "I think therefore" {
		t.Errorf("LLMReasoningPart.Text = %q", rp.Text)
	}

	// LLMToolCallPart
	tc := kit.LLMToolCallPart{
		ToolCallID: "call-1",
		ToolName:   "bash",
		Input:      `{"cmd":"echo hi"}`,
	}
	if tc.ToolCallID != "call-1" {
		t.Errorf("LLMToolCallPart.ToolCallID = %q", tc.ToolCallID)
	}

	// LLMToolResultPart
	tro := kit.LLMToolResultOutputContentText{Text: "output text"}
	tr := kit.LLMToolResultPart{
		ToolCallID: "call-1",
		Output:     tro,
	}
	if tr.ToolCallID != "call-1" {
		t.Errorf("LLMToolResultPart.ToolCallID = %q", tr.ToolCallID)
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
	if llmMsgs[0].Role != kit.LLMRoleUser {
		t.Errorf("converted Role = %q, want %q", llmMsgs[0].Role, kit.LLMRoleUser)
	}
	// Check text is preserved in content parts.
	found := false
	for _, part := range llmMsgs[0].Content {
		if tp, ok := part.(kit.LLMTextPart); ok && tp.Text == "what is 2+2?" {
			found = true
		}
	}
	if !found {
		t.Errorf("text content not found in converted LLMMessage")
	}
}

// TestConvertFromLLMMessage verifies LLMMessage → Message conversion.
func TestConvertFromLLMMessage(t *testing.T) {
	llm := kit.NewLLMUserMessage("the answer is 4")
	llm.Role = kit.LLMRoleAssistant
	msg := kit.ConvertFromLLMMessage(llm)
	if msg.Role != kit.RoleAssistant {
		t.Errorf("converted Role = %q, want %q", msg.Role, kit.RoleAssistant)
	}
	if msg.Content() != "the answer is 4" {
		t.Errorf("converted Content() = %q, want %q", msg.Content(), "the answer is 4")
	}
}

// containsStr is a tiny helper to avoid importing strings in test.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && indexStr(s, substr) >= 0)
}

func indexStr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
