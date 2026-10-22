package message

import (
	"encoding/json"
	"reflect"
	"testing"

	"charm.land/fantasy"
)

func TestExtensionHistoryPreservesOrderAndIDs(t *testing.T) {
	options := fantasy.ProviderOptions{"kit-extension": &ExtensionMetadata{Provider: "fake", Signature: "signed", Replay: []ReplayData{{Version: 1, Provider: "fake", JSON: `{"opaque":true}`}}}}
	input := fantasy.Message{Role: fantasy.MessageRoleAssistant, ProviderOptions: options, Content: []fantasy.MessagePart{
		fantasy.TextPart{Text: "before", ProviderOptions: options},
		fantasy.ReasoningPart{Text: "", ProviderOptions: options},
		fantasy.TextPart{Text: "after", ProviderOptions: options},
		fantasy.ToolCallPart{ToolCallID: "exact/id.with:punctuation", ToolName: "add", Input: `{"a":2}`, ProviderOptions: options},
		fantasy.FilePart{Filename: "document.pdf", MediaType: "application/pdf", Data: []byte{1, 2}, ProviderOptions: options},
	}}
	stored := FromLLMMessage(input)
	data, err := MarshalParts(stored.Parts)
	if err != nil {
		t.Fatal(err)
	}
	stored.Parts, err = UnmarshalParts(data)
	if err != nil {
		t.Fatal(err)
	}
	output := stored.ToLLMMessages()
	if len(output) != 1 || len(output[0].Content) != 5 {
		t.Fatalf("output=%+v", output)
	}
	if output[0].Content[3].(fantasy.ToolCallPart).ToolCallID != "exact/id.with:punctuation" {
		t.Fatal("ID normalized")
	}
	reasoning := output[0].Content[1].(fantasy.ReasoningPart)
	if metadata := ExtensionOptions(reasoning.ProviderOptions); metadata.Signature != "signed" || len(metadata.Replay) != 1 {
		t.Fatalf("metadata=%+v", metadata)
	}
	file := output[0].Content[4].(fantasy.FilePart)
	if file.Filename != "document.pdf" || !reflect.DeepEqual(file.Data, []byte{1, 2}) {
		t.Fatal("file changed")
	}
	encoded, err := json.Marshal(options)
	if err != nil || len(encoded) == 0 {
		t.Fatalf("metadata serialization: %v", err)
	}
	clone := CloneOptions(options)
	clone["kit-extension"].(*ExtensionMetadata).Replay[0].JSON = "changed"
	if options["kit-extension"].(*ExtensionMetadata).Replay[0].JSON == "changed" {
		t.Fatal("clone aliases replay")
	}
}
