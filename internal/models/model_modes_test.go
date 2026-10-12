package models

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charm.land/fantasy"
)

func TestModelsDBModeParsingAndCost(t *testing.T) {
	var db ModelsDBProviders
	data := []byte(`{"p":{"id":"p","models":{"m":{"id":"m","name":"M","cost":{"input":2,"output":4,"cache_read":0.2,"cache_write":0.3,"context_over_200k":{"input":4,"output":8}},"experimental":{"modes":{"fast":{"provider":{"body":{"service_tier":"priority"},"headers":{"x-mode":"fast"}},"cost":{"input":1.5}}}}}}}}`)
	if err := json.Unmarshal(data, &db); err != nil {
		t.Fatal(err)
	}
	mode := db["p"].Models["m"].Experimental.Modes["fast"]
	if mode.Provider.Headers["x-mode"] != "fast" || mode.Provider.Body["service_tier"] != "priority" {
		t.Fatalf("mode was not parsed: %#v", mode)
	}
	base := costFrom(db["p"].Models["m"].Cost)
	cost := modeCostFrom(mode.Cost, db["p"].Models["m"].Cost)
	info := ModelInfo{Cost: base, Modes: []ModelMode{{Name: "fast", Cost: cost}}}
	if got := info.EffectiveCost("fast"); !got.Published || got.Input != 1.5 || got.Output != 4 || *got.CacheRead != 0.2 || *got.CacheWrite != 0.3 || len(got.Tiers) != 0 {
		t.Fatalf("unexpected effective mode cost: %#v", got)
	}
}

func TestModeCostContextPricing(t *testing.T) {
	base := &modelsDBCost{Input: 2, Output: 10, ContextOver200K: &modelsDBRates{Input: 4, Output: 15}}
	input, output := 12.0, 60.0
	mode := &modelsDBModeCost{Input: &input, Output: &output}
	cost := modeCostFrom(mode, base)
	if rates := cost.RatesFor(300000); rates.Input != 12 || rates.Output != 60 {
		t.Fatalf("large prompt uses base tiers: %#v", rates)
	}
	mode.ContextOver200K = &modelsDBRates{Input: 24, Output: 90}
	if rates := modeCostFrom(mode, base).RatesFor(300000); rates.Input != 24 || rates.Output != 90 {
		t.Fatalf("explicit mode tier was lost: %#v", rates)
	}
	if rates := modeCostFrom(nil, base).RatesFor(300000); rates.Input != 4 || rates.Output != 15 {
		t.Fatalf("default mode must preserve base tiers: %#v", rates)
	}
}

func TestModelInfoModeLookupAndEffectiveCost(t *testing.T) {
	base := Cost{Input: 2, Published: true}
	override := Cost{Input: 5, Published: true}
	info := ModelInfo{Cost: base, Modes: []ModelMode{{Name: "fast", Cost: override}, {Name: "cheap"}}}
	if !info.SupportsMode("FAST") {
		t.Fatal("mode lookup must ignore case")
	}
	if got := info.EffectiveCost("fast"); got.Input != 5 {
		t.Fatalf("effective cost = %v, want 5", got.Input)
	}
	if got := info.EffectiveCost("cheap"); got.Input != 2 {
		t.Fatalf("missing override must use base cost; got %v", got.Input)
	}
	if got := info.EffectiveCost(""); got.Input != 2 {
		t.Fatalf("empty mode must use base cost; got %v", got.Input)
	}
}

func TestModeUnsupportedReasonOpenAIUltrafastRestrictions(t *testing.T) {
	mode := ModelInfo{Modes: []ModelMode{{Name: "fast", Options: map[string]any{"service_tier": "ultrafast"}}}}
	for _, model := range []string{"openai/gpt-6", "openai/gpt-6.1-sol"} {
		t.Run(model, func(t *testing.T) {
			if reason := ModeUnsupportedReason(model, "fast", &mode); reason != "" {
				t.Fatalf("ModeUnsupportedReason(%q) = %q, want supported", model, reason)
			}
		})
	}
	for _, model := range []string{"openai/gpt-6-priority", "openai/gpt-6-pro"} {
		t.Run(model+"-priority", func(t *testing.T) {
			info := ModelInfo{Modes: []ModelMode{{Name: "fast", Options: map[string]any{"service_tier": "priority"}}}}
			if reason := ModeUnsupportedReason(model, "fast", &info); reason == "" {
				t.Fatalf("ModeUnsupportedReason(%q) accepted unsupported priority tier", model)
			}
		})
	}
	withHeaders := ModelInfo{Modes: []ModelMode{{Name: "fast", Options: map[string]any{"service_tier": "ultrafast"}, Headers: map[string]string{"x-mode": "fast"}}}}
	if reason := ModeUnsupportedReason("openai/gpt-6", "fast", &withHeaders); !strings.Contains(reason, "headers") {
		t.Fatalf("header mode reason = %q, want header rejection", reason)
	}
}

func TestOpenAIUltrafastModeSerializedOnGenerateAndStream(t *testing.T) {
	requests := make(chan map[string]any, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := r.Body.Close(); err != nil {
				t.Errorf("close request body: %v", err)
			}
		}()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- body
		if body["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"r1\",\"status\":\"completed\",\"output\":[]}}\n\ndata: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"r1","status":"completed","output":[]}`)
	}))
	defer server.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	registry := GetGlobalRegistry()
	original := registry.providers["openai"].Models["gpt-6"]
	updated := original
	updated.Modes = []ModelMode{{Name: "ultrafast", Options: map[string]any{"service_tier": "ultrafast"}}}
	registry.providers["openai"].Models["gpt-6"] = updated
	t.Cleanup(func() { registry.providers["openai"].Models["gpt-6"] = original })
	result, err := CreateProvider(context.Background(), &ProviderConfig{ModelString: "openai/gpt-6", ModelMode: "ultrafast", ProviderURL: server.URL, ProviderAPIKey: "test-key"})
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	defer func() {
		if result.Closer != nil {
			_ = result.Closer.Close()
		}
	}()

	if _, err := result.Model.Generate(context.Background(), fantasy.Call{ProviderOptions: result.ProviderOptions}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	assertUltrafastRequest := func(operation string) {
		t.Helper()
		body := <-requests
		if got := body["service_tier"]; got != "ultrafast" {
			t.Errorf("%s service_tier = %#v, want \"ultrafast\" (body: %#v)", operation, got, body)
		}
	}
	assertUltrafastRequest("Generate")

	stream, err := result.Model.Stream(context.Background(), fantasy.Call{ProviderOptions: result.ProviderOptions})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for range stream {
	}
	assertUltrafastRequest("Stream")
}
