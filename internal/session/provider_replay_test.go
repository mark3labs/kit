package session

import (
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/message"
)

func TestExtensionReplaySurvivesEntryAndContextClone(t *testing.T) {
	options := fantasy.ProviderOptions{"kit-extension": &message.ExtensionMetadata{Provider: "fake", Signature: "signature", Replay: []message.ReplayData{{Version: 1, Provider: "fake", JSON: `{"value":1}`}}}}
	original := fantasy.Message{Role: fantasy.MessageRoleAssistant, ProviderOptions: options, Content: []fantasy.MessagePart{
		fantasy.TextPart{Text: "before", ProviderOptions: options},
		fantasy.ReasoningPart{Text: "", ProviderOptions: options},
		fantasy.ToolCallPart{ToolCallID: "call/with:punctuation", ToolName: "add", Input: `{}`, ProviderOptions: options},
	}}
	entry, err := NewMessageEntry("", message.FromLLMMessage(original))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	var reopened MessageEntry
	if err := json.Unmarshal(data, &reopened); err != nil {
		t.Fatal(err)
	}
	decoded, err := reopened.ToMessage()
	if err != nil {
		t.Fatal(err)
	}
	tm := InMemoryTreeSession("/test")
	if _, err := tm.AppendMessage(decoded); err != nil {
		t.Fatal(err)
	}
	first, _, _ := tm.BuildContext()
	if len(first) != 1 || len(first[0].Content) != 3 {
		t.Fatalf("context=%+v", first)
	}
	if first[0].Content[2].(fantasy.ToolCallPart).ToolCallID != "call/with:punctuation" {
		t.Fatal("ID changed")
	}
	metadata := first[0].Content[1].Options()["kit-extension"].(*message.ExtensionMetadata)
	metadata.Replay[0].JSON = "mutated"
	second, _, _ := tm.BuildContext()
	if message.ExtensionOptions(second[0].Content[1].Options()).Replay[0].JSON != "{\"value\":1}" {
		t.Fatal("context replay aliases cache")
	}
}
