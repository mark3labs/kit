//go:build ignore

package main

import "kit/ext"

func toolCreate(req ext.ProviderCreateRequest) ext.ProviderCreateResult {
	return ext.ProviderCreateResult{Runtime: ext.ProviderRuntime{Stream: func(req ext.ProviderRequest, emit func(ext.ProviderEvent) bool) ext.ProviderResult {
		// History, not a process-global counter, determines the next response.
		resultSeen := false
		for _, message := range req.Messages {
			for _, part := range message.Parts {
				if part.Type == ext.ProviderPartToolResult && part.CallID == "add-call" && part.Text == "5" && !part.IsError {
					resultSeen = true
				}
			}
		}
		if resultSeen {
			if !emit(ext.ProviderEvent{Type: ext.ProviderTextStart, ID: "answer"}) {
				return ext.ProviderResult{}
			}
			if !emit(ext.ProviderEvent{Type: ext.ProviderTextDelta, ID: "answer", Delta: "5"}) {
				return ext.ProviderResult{}
			}
			if !emit(ext.ProviderEvent{Type: ext.ProviderTextEnd, ID: "answer"}) {
				return ext.ProviderResult{}
			}
			emit(ext.ProviderEvent{Type: ext.ProviderFinish, FinishReason: ext.ProviderFinishStop})
		} else {
			if !emit(ext.ProviderEvent{Type: ext.ProviderToolStart, ID: "add-call", ToolName: "add"}) {
				return ext.ProviderResult{}
			}
			if !emit(ext.ProviderEvent{Type: ext.ProviderToolDelta, ID: "add-call", Delta: `{"a":2,`}) {
				return ext.ProviderResult{}
			}
			if !emit(ext.ProviderEvent{Type: ext.ProviderToolDelta, ID: "add-call", Delta: `"b":3}`}) {
				return ext.ProviderResult{}
			}
			if !emit(ext.ProviderEvent{Type: ext.ProviderToolEnd, ID: "add-call"}) {
				return ext.ProviderResult{}
			}
			emit(ext.ProviderEvent{Type: ext.ProviderFinish, FinishReason: ext.ProviderFinishToolCalls})
		}
		return ext.ProviderResult{}
	}}}
}

func Init(api ext.API) {
	api.RegisterProvider(ext.ProviderDef{ID: "fake-tool", Name: "History-driven tool fixture", Create: toolCreate,
		Models: []ext.ProviderModel{{ID: "add", Name: "Add", Capabilities: ext.ProviderCapabilities{Streaming: true, Tools: true}}},
	})
}
