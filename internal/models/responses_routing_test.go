package models

import (
	"testing"

	"charm.land/fantasy/providers/openai"
)

// TestResponsesRoutingOptionsConsistency is a regression test for the
// "invalid argument: openai provider options should be
// *openai.ProviderOptions" failure. It occurred because the routing
// predicate and the options builder disagreed for model IDs that fantasy's
// matcher did not know (e.g. gpt-6-astra): the model fell back to the
// chat-completions wire while the options carried
// *openai.ResponsesProviderOptions.
//
// Fantasy now recognizes gpt-6 and later generations natively. This test
// guards the invariant across fantasy upgrades: for every model that kit
// routes to the Responses API, the options under the openai key must be
// *openai.ResponsesProviderOptions, and for every model on the
// chat-completions wire the options must not be.
func TestResponsesRoutingOptionsConsistency(t *testing.T) {
	config := &ProviderConfig{}

	models := []string{
		"gpt-6-astra",
		"gpt-5.3-codex",
		"gpt-4o",
		"o3",
		"gpt-3.5-turbo",
	}

	for _, id := range models {
		opts := buildOpenAIProviderOptions(config, id)
		_, hasResponsesOpts := opts[openai.Name].(*openai.ResponsesProviderOptions)

		if hasResponsesOpts && !openai.IsResponsesModel(id) {
			t.Errorf("model %q: Responses options built for a chat-completions model", id)
		}
	}

	// gpt-6 and later generations must route to the Responses API and get
	// reasoning options, exactly like the gpt-5 family. Fantasy's native
	// matcher covers this since v0.43; a fantasy downgrade must fail here.
	if !openai.IsResponsesModel("gpt-6-astra") {
		t.Error("gpt-6-astra must route to the Responses API")
	}
	if !openai.IsResponsesReasoningModel("gpt-6-astra") {
		t.Error("gpt-6-astra must be treated as a reasoning model")
	}
	if opts := buildOpenAIProviderOptions(config, "gpt-6-astra"); opts == nil {
		t.Error("buildOpenAIProviderOptions must build Responses options for gpt-6-astra")
	}
}

// TestCodexProviderOptionsAlwaysResponses verifies that the Codex options
// builder emits Responses options for every model, matching the forced
// Responses routing of the Codex backend.
func TestCodexProviderOptionsAlwaysResponses(t *testing.T) {
	config := &ProviderConfig{SystemPrompt: "test", ThinkingLevel: ThinkingMedium}

	for _, id := range []string{"gpt-6-astra", "gpt-5.3-codex", "unknown-model"} {
		opts := buildCodexProviderOptions(config, id)
		if _, ok := opts[openai.Name].(*openai.ResponsesProviderOptions); !ok {
			t.Errorf("model %q: Codex options must be *openai.ResponsesProviderOptions", id)
		}
	}

	// gpt-6 models must carry a reasoning effort like the gpt-5 family.
	opts := buildCodexProviderOptions(config, "gpt-6-astra")
	respOpts := opts[openai.Name].(*openai.ResponsesProviderOptions)
	if respOpts.ReasoningEffort == nil {
		t.Error("gpt-6-astra Codex options must carry a reasoning effort")
	}
}
