package message

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"charm.land/fantasy"
)

// ReplayData is opaque JSON scoped to the provider that produced it.
type ReplayData struct {
	Version  int    `json:"version"`
	Provider string `json:"provider"`
	JSON     string `json:"json"`
}

// ExtensionMetadata is a compiled carrier. Its fields contain no credentials.
// It marks exact extension-origin IDs and ordered history without affecting
// legacy built-in normalization.
type ExtensionMetadata struct {
	Provider  string       `json:"provider"`
	Signature string       `json:"signature,omitempty"`
	Replay    []ReplayData `json:"replay,omitempty"`
}

func (*ExtensionMetadata) Options() {}
func (m ExtensionMetadata) MarshalJSON() ([]byte, error) {
	type plain ExtensionMetadata
	return fantasy.MarshalProviderType("kit.extension.replay.v1", plain(m))
}
func (m *ExtensionMetadata) UnmarshalJSON(data []byte) error {
	type plain ExtensionMetadata
	var value plain
	if err := fantasy.UnmarshalProviderType(data, &value); err != nil {
		return err
	}
	*m = ExtensionMetadata(value)
	return nil
}
func init() {
	fantasy.RegisterProviderType("kit.extension.replay.v1", func(data []byte) (fantasy.ProviderOptionsData, error) {
		var value ExtensionMetadata
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		return &value, nil
	})
}

// ExtensionOptions returns only the compiled extension replay carrier.
func ExtensionOptions(options fantasy.ProviderOptions) *ExtensionMetadata {
	if value, ok := options["kit-extension"].(*ExtensionMetadata); ok {
		copy := *value
		copy.Replay = slices.Clone(value.Replay)
		return &copy
	}
	return nil
}

// CloneOptions isolates extension metadata while keeping other providers' option
// behavior unchanged.
func CloneOptions(options fantasy.ProviderOptions) fantasy.ProviderOptions {
	if options == nil {
		return nil
	}
	copy := maps.Clone(options)
	if ext := ExtensionOptions(options); ext != nil {
		copy["kit-extension"] = ext
	}
	return copy
}

func orderedExtensionMessages(m *Message) []fantasy.Message {
	var content []fantasy.MessagePart
	for _, part := range m.Parts {
		options := fantasy.ProviderOptions{"kit-extension": &ExtensionMetadata{Provider: m.ExtensionProvider}}
		switch value := part.(type) {
		case TextContent:
			options["kit-extension"] = &ExtensionMetadata{Provider: m.ExtensionProvider, Replay: slices.Clone(value.Replay)}
			content = append(content, fantasy.TextPart{Text: value.Text, ProviderOptions: options})
		case ReasoningContent:
			options["kit-extension"] = &ExtensionMetadata{Provider: m.ExtensionProvider, Signature: value.Signature, Replay: slices.Clone(value.Replay)}
			content = append(content, fantasy.ReasoningPart{Text: value.Thinking, ProviderOptions: options})
		case ImageContent:
			options["kit-extension"] = &ExtensionMetadata{Provider: m.ExtensionProvider, Replay: slices.Clone(value.Replay)}
			content = append(content, fantasy.FilePart{Data: slices.Clone(value.Data), MediaType: value.MediaType, Filename: value.Filename, ProviderOptions: options})
		case ToolCall:
			options["kit-extension"] = &ExtensionMetadata{Provider: m.ExtensionProvider, Replay: slices.Clone(value.Replay)}
			content = append(content, fantasy.ToolCallPart{ToolCallID: value.ID, ToolName: value.Name, Input: value.Input, ProviderOptions: options})
		case ToolResult:
			options["kit-extension"] = &ExtensionMetadata{Provider: m.ExtensionProvider, Replay: slices.Clone(value.Replay)}
			var output fantasy.ToolResultOutputContent = fantasy.ToolResultOutputContentText{Text: value.Content}
			if value.IsError {
				output = fantasy.ToolResultOutputContentError{Error: fmt.Errorf("%s", value.Content)}
			}
			if value.MediaData != "" {
				output = fantasy.ToolResultOutputContentMedia{Data: value.MediaData, MediaType: value.MediaType, Text: value.Content}
			}
			content = append(content, fantasy.ToolResultPart{ToolCallID: value.ToolCallID, Output: output, ProviderOptions: options})
		}
	}
	return []fantasy.Message{{Role: fantasy.MessageRole(m.Role), Content: content, ProviderOptions: fantasy.ProviderOptions{"kit-extension": &ExtensionMetadata{Provider: m.ExtensionProvider, Replay: slices.Clone(m.Replay)}}}}
}
