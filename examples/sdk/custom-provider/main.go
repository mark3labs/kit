// Command custom-provider shows how to plug an in-process language model into
// Kit with a ProviderFactory. The model uses only kit.LLM* types, so the
// application does not import the underlying LLM library.
//
// The model echoes the last user message. Replace its body with calls to your
// own inference backend.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// echoModel implements kit.LLMLanguageModel.
type echoModel struct {
	model string
}

// reply builds the answer for a call from the last user text and the tool
// definitions that Kit sent.
func (m *echoModel) reply(call kit.LLMCall) string {
	var last string
	for _, msg := range call.Prompt {
		if msg.Role != kit.LLMRoleUser {
			continue
		}
		for _, part := range msg.Content {
			if text, ok := part.(kit.LLMTextPart); ok {
				last = text.Text
			}
		}
	}
	var tools []string
	for _, t := range call.Tools {
		if fn, ok := t.(kit.LLMFunctionTool); ok {
			tools = append(tools, fn.Name)
		}
	}
	return fmt.Sprintf("[%s] you said %q (tools offered: %d)", m.model, last, len(tools))
}

// Generate returns the whole reply in one response. Kit uses it when
// streaming is off.
func (m *echoModel) Generate(_ context.Context, call kit.LLMCall) (*kit.LLMResponse, error) {
	return &kit.LLMResponse{
		Content:      kit.LLMResponseContent{kit.LLMTextContent{Text: m.reply(call)}},
		FinishReason: kit.FinishReasonStop,
		Usage:        kit.LLMUsage{InputTokens: 1, OutputTokens: 1},
	}, nil
}

// Stream returns the reply as a text block (start, delta, end) and then a
// finish part with the finish reason and usage. Kit uses it when streaming
// is on.
func (m *echoModel) Stream(_ context.Context, call kit.LLMCall) (kit.LLMStreamResponse, error) {
	text := m.reply(call)
	return func(yield func(kit.LLMStreamPart) bool) {
		parts := []kit.LLMStreamPart{
			{Type: kit.LLMStreamPartTypeTextStart, ID: "0"},
			{Type: kit.LLMStreamPartTypeTextDelta, ID: "0", Delta: text},
			{Type: kit.LLMStreamPartTypeTextEnd, ID: "0"},
			{
				Type:         kit.LLMStreamPartTypeFinish,
				FinishReason: kit.FinishReasonStop,
				Usage:        kit.LLMUsage{InputTokens: 1, OutputTokens: 1},
			},
		}
		for _, p := range parts {
			if !yield(p) {
				return
			}
		}
	}, nil
}

// GenerateObject and StreamObject serve structured output. A backend without
// structured output can return an error.
func (m *echoModel) GenerateObject(context.Context, kit.LLMObjectCall) (*kit.LLMObjectResponse, error) {
	return nil, errors.New("structured output is not supported")
}

func (m *echoModel) StreamObject(context.Context, kit.LLMObjectCall) (kit.LLMObjectStreamResponse, error) {
	return nil, errors.New("structured output is not supported")
}

// Provider returns the provider name.
func (m *echoModel) Provider() string { return "echo" }

// Model returns the model name after the provider prefix.
func (m *echoModel) Model() string { return m.model }

// main registers the echo provider, sends one prompt and prints the reply.
func main() {
	ctx := context.Background()

	factory := func(_ context.Context, _ *kit.ProviderConfig, model string) (*kit.ProviderResult, error) {
		return &kit.ProviderResult{Model: &echoModel{model: model}}, nil
	}

	host, err := kit.NewIsolatedAgent(ctx,
		kit.WithProvider("echo", factory),
		kit.WithModel("echo/v1"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = host.Close() }()

	prompt := "Hello from a custom provider"
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}
	response, err := host.Prompt(ctx, prompt)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response)
}
