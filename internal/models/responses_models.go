package models

import (
	_ "unsafe" // Required for go:linkname.
)

// responsesModelIDs and responsesReasoningModelIDs are the unexported slices
// in charm.land/fantasy/providers/openai that gate whether a model uses the
// Responses API code path vs Chat Completions. When a brand-new model is
// released (e.g. gpt-5.5) and models.dev is updated via `kit update-models`,
// Kit recognises the model but fantasy's hardcoded list does not. That causes
// a type-mismatch crash: Kit builds *ResponsesProviderOptions (correct for
// the Responses endpoint) but fantasy routes through Chat Completions and
// rejects the type.
//
// RegisterResponsesModels appends model IDs that are missing from fantasy's
// lists so the provider routes through the correct code path. It is called
// once during provider creation after loading the model database.

//go:linkname fantasyResponsesModelIDs charm.land/fantasy/providers/openai.responsesModelIDs
var fantasyResponsesModelIDs []string

//go:linkname fantasyResponsesReasoningModelIDs charm.land/fantasy/providers/openai.responsesReasoningModelIDs
var fantasyResponsesReasoningModelIDs []string

// RegisterResponsesModels ensures every OpenAI model known to our model
// database that should use the Responses API is present in fantasy's
// internal lists. This is a no-op for models already registered.
func RegisterResponsesModels() {
	registry := GetGlobalRegistry()
	providerInfo := registry.GetProviderInfo("openai")
	if providerInfo == nil {
		return
	}

	existing := make(map[string]bool, len(fantasyResponsesModelIDs))
	for _, id := range fantasyResponsesModelIDs {
		existing[id] = true
	}
	existingReasoning := make(map[string]bool, len(fantasyResponsesReasoningModelIDs))
	for _, id := range fantasyResponsesReasoningModelIDs {
		existingReasoning[id] = true
	}

	for modelID, modelInfo := range providerInfo.Models {
		if !isResponsesAPIModel(modelID) {
			continue
		}
		if !existing[modelID] {
			fantasyResponsesModelIDs = append(fantasyResponsesModelIDs, modelID)
			existing[modelID] = true
		}
		if modelInfo.Reasoning && !existingReasoning[modelID] {
			fantasyResponsesReasoningModelIDs = append(fantasyResponsesReasoningModelIDs, modelID)
			existingReasoning[modelID] = true
		}
	}
}
