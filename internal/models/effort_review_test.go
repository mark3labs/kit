package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/spf13/viper"
)

func TestAutoGoogleEffortUsesNativeNamespace(t *testing.T) {
	captured := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		captured <- body
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`)
	}))
	defer server.Close()
	old := GetGlobalRegistry()
	defer globalRegistry.Store(old)
	globalRegistry.Store(&ModelsRegistry{providers: map[string]ProviderInfo{"fixture-google": {ID: "fixture-google", Wire: "google", Models: map[string]ModelInfo{"vendor-model": {Reasoning: true, ReasoningIsGraded: true, ReasoningLevels: []string{"Future"}}}}}})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := CreateProvider(ctx, &ProviderConfig{ModelString: "fixture-google/vendor-model", ProviderURL: server.URL, ProviderAPIKey: "fixture", ThinkingLevel: "Future", ConfigStore: viper.New()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := result.Model.Generate(ctx, fantasy.Call{Prompt: []fantasy.Message{fantasy.NewUserMessage("hello")}, ProviderOptions: result.ProviderOptions}); err != nil {
		t.Fatal(err)
	}
	select {
	case body := <-captured:
		generation, _ := body["generationConfig"].(map[string]any)
		thinking, _ := generation["thinkingConfig"].(map[string]any)
		if thinking["thinkingLevel"] != "Future" {
			t.Fatal(body)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestInheritedUnsupportedEffortFallsBack(t *testing.T) {
	for _, model := range []string{"ungraded", "non-reasoning", "graded"} {
		t.Run(model, func(t *testing.T) {
			old := GetGlobalRegistry()
			defer globalRegistry.Store(old)
			info := ModelInfo{}
			if model == "graded" {
				info.Reasoning = true
				info.ReasoningIsGraded = true
				info.ReasoningLevels = []string{"low"}
			}
			globalRegistry.Store(&ModelsRegistry{providers: map[string]ProviderInfo{"fixture": {ID: "fixture", Wire: "google", Models: map[string]ModelInfo{model: info}}}})
			for _, inherited := range []bool{false, true} {
				store := viper.New()
				store.Set("thinking-level", "Future")
				cfg := &ProviderConfig{ModelString: "fixture/" + model, ProviderURL: "http://127.0.0.1:1", ProviderAPIKey: "fixture", ThinkingLevel: "Future", ThinkingLevelInherited: inherited, ConfigStore: store}
				_, err := CreateProvider(context.Background(), cfg)
				if inherited {
					if err != nil || cfg.ThinkingLevel != ThinkingOff || store.GetString("thinking-level") != "off" {
						t.Fatalf("fallback: %+v, %v", cfg, err)
					}
				} else if err == nil {
					t.Fatal("explicit unsupported selection accepted")
				}
			}
		})
	}
}
