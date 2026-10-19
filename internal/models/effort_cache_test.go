package models

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestEffortCatalogCachePreservesProviderIdentity(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var providers map[string]modelsDBProvider
	fixture := `{"fixture-a":{"id":"fixture-a","models":{"shared":{"id":"shared","reasoning":true,"reasoning_options":[{"type":"effort","values":["low",null,"","xhigh","Future"]},{"type":"effort","values":["xhigh","max"]},{"type":"toggle"}]}}},"fixture-b":{"id":"fixture-b","models":{"shared":{"id":"shared","reasoning":true,"reasoning_options":[{"type":"budget_tokens","min":1,"max":100},{"type":"effort","values":["none","medium"]}]}}}}`
	if err := json.Unmarshal([]byte(fixture), &providers); err != nil {
		t.Fatal(err)
	}
	if err := StoreCachedProviders(providers, "fixture-etag"); err != nil {
		t.Fatal(err)
	}
	cached, etag := LoadCachedProviders()
	if etag != "fixture-etag" {
		t.Fatal(etag)
	}
	registry := &ModelsRegistry{providers: buildFromModelsDB()}
	for provider, want := range map[string][]string{"fixture-a": {"low", "xhigh", "Future", "max"}, "fixture-b": {"none", "medium"}} {
		levels, graded := reasoningFrom(cached[provider].Models["shared"].ReasoningOptions)
		if !graded || !slices.Equal(levels, want) {
			t.Fatalf("cached %s: %v", provider, levels)
		}
		info := registry.LookupModel(provider, "shared")
		if info == nil || !info.ReasoningIsGraded || !slices.Equal(info.ReasoningLevels, want) {
			t.Fatalf("converted %s: %+v", provider, info)
		}
	}
}
