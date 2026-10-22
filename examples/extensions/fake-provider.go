//go:build ignore

package main

import "kit/ext"

// Helpers precede registration because Yaegi does not reliably resolve later
// named helpers referenced from function literals.
func fakeCreate(req ext.ProviderCreateRequest) ext.ProviderCreateResult {
	return ext.ProviderCreateResult{Runtime: ext.ProviderRuntime{
		Stream: func(req ext.ProviderRequest, emit func(ext.ProviderEvent) bool) ext.ProviderResult {
			select {
			case <-req.Done:
				return ext.ProviderResult{Error: &ext.ProviderError{Code: "canceled"}}
			default:
			}
			if !emit(ext.ProviderEvent{Type: ext.ProviderTextStart, ID: "greeting"}) {
				return ext.ProviderResult{}
			}
			if !emit(ext.ProviderEvent{Type: ext.ProviderTextDelta, ID: "greeting", Delta: "Hello from a fake provider."}) {
				return ext.ProviderResult{}
			}
			if !emit(ext.ProviderEvent{Type: ext.ProviderTextEnd, ID: "greeting"}) {
				return ext.ProviderResult{}
			}
			emit(ext.ProviderEvent{Type: ext.ProviderFinish, FinishReason: ext.ProviderFinishStop})
			return ext.ProviderResult{}
		},
		Close: func() error { return nil },
	}}
}

// Init declares a deterministic offline provider. No API key is required.
// kit --bare -e examples/extensions/fake-provider.go --model fake/echo --no-session --no-core-tools --quiet "hello"
func Init(api ext.API) {
	api.RegisterProvider(ext.ProviderDef{
		ID: "fake", Name: "Fake provider", Auth: ext.ProviderAuth{Mode: ext.ProviderAuthNone},
		Models: []ext.ProviderModel{{ID: "echo", Name: "Deterministic greeting", Capabilities: ext.ProviderCapabilities{Streaming: true, Tools: true, InputModalities: []string{"text"}}}},
		Create: fakeCreate,
	})
}
