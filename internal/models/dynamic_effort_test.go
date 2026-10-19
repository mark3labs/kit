package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/spf13/viper"
)

func TestDynamicEffortMetadata(t *testing.T) {
	var opts []modelsDBReasoningOption
	if err := json.Unmarshal([]byte(`[{"type":"effort","values":["low",null,"","xhigh","Future"]},{"type":"toggle"},{"type":"budget_tokens","min":1,"max":100},{"type":"effort","values":["xhigh","max","null"]}]`), &opts); err != nil {
		t.Fatal(err)
	}
	levels, graded := reasoningFrom(opts)
	want := []string{"low", "xhigh", "Future", "max", "null"}
	if !graded || !slices.Equal(levels, want) {
		t.Fatalf("got %v, %v", levels, graded)
	}
	info := ModelInfo{Reasoning: true, ReasoningLevels: levels, ReasoningIsGraded: graded}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var restored ModelInfo
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(restored.ReasoningLevels, want) {
		t.Fatal(restored.ReasoningLevels)
	}
	old := GetGlobalRegistry()
	globalRegistry.Store(&ModelsRegistry{providers: map[string]ProviderInfo{"fixture": {ID: "fixture", Models: map[string]ModelInfo{"model": restored}}}})
	t.Cleanup(func() { globalRegistry.Store(old) })
	if got := SupportedThinkingLevels("fixture", "model"); !slices.Equal(got, []ThinkingLevel{"off", "low", "xhigh", "Future", "max", "null"}) {
		t.Fatal(got)
	}
	for _, level := range []string{"Future", "xhigh", "max"} {
		if ParseThinkingLevel(level) != ThinkingLevel(level) || !IsValidThinkingLevelForModel(ParseThinkingLevel(level), "fixture", "model") {
			t.Fatal(level)
		}
	}
	for _, level := range []ThinkingLevel{"future", "typo"} {
		if IsValidThinkingLevelForModel(level, "fixture", "model") || SuggestThinkingLevelFallback(level, "fixture", "model") != ThinkingOff {
			t.Fatal(level)
		}
	}
}

func TestCustomReasoningDeclarations(t *testing.T) {
	for _, format := range []string{"yaml", "json"} {
		t.Run(format, func(t *testing.T) {
			v := viper.New()
			v.SetConfigType(format)
			input := `customModels:
  fixture:
    reasoning: true
    apiModelName: wire-alias
    reasoningLevels: [low, medium, xhigh, Future, low]
    params:
      thinkingLevel: Future
`
			if format == "json" {
				input = `{"customModels":{"fixture":{"reasoning":true,"apiModelName":"wire-alias","reasoningLevels":["low","medium","xhigh","Future","low"],"params":{"thinkingLevel":"Future"}}}}`
			}
			if err := v.ReadConfig(strings.NewReader(input)); err != nil {
				t.Fatal(err)
			}
			info, ok := loadCustomModelsFrom(v)["fixture"]
			if !ok || !info.ReasoningIsGraded || !slices.Equal(info.ReasoningLevels, []string{"low", "medium", "xhigh", "Future"}) || info.APIModelName != "wire-alias" || info.Params.ThinkingLevel != "Future" {
				t.Fatalf("%+v", info)
			}
		})
	}
	for _, raw := range []string{`["off"]`, `[""]`, `["  "]`, `[3]`, `[null]`, `"low"`} {
		v := viper.New()
		v.SetConfigType("json")
		if err := v.ReadConfig(strings.NewReader(`{"customModels":{"bad":{"reasoning":true,"reasoningLevels":` + raw + `}}}`)); err != nil {
			t.Fatal(err)
		}
		if _, ok := loadCustomModelsFrom(v)["bad"]; ok {
			t.Fatalf("accepted %s", raw)
		}
	}
	v := viper.New()
	v.Set("customModels", map[string]any{
		"qwen3.8": map[string]any{"reasoning": true, "reasoningLevels": []string{"Future"}},
		"invalid": map[string]any{"reasoning": true, "reasoningLevels": []any{map[string]any{"bad": true}}},
	})
	loaded := loadCustomModelsFrom(v)
	if len(loaded) != 1 || !slices.Equal(loaded["qwen3.8"].ReasoningLevels, []string{"Future"}) {
		t.Fatalf("invalid neighbor or dotted ID changed valid model: %+v", loaded)
	}
	cfg := CustomModelConfig{Reasoning: true, ReasoningLevels: []string{"low", "Future", "low"}}
	info := modelConfigToModelInfo("copy", cfg)
	cfg.ReasoningLevels[0] = "changed"
	if info.ReasoningLevels[0] != "low" {
		t.Fatal("list aliases configuration")
	}
	if err := validateCustomReasoningLevels(nil, CustomModelConfig{ReasoningLevels: []string{"low"}}); err == nil {
		t.Fatal("accepted reasoning false")
	}
	for _, list := range [][]string{nil, {}} {
		if err := validateCustomReasoningLevels(nil, CustomModelConfig{ReasoningLevels: list}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestChatEffortWire(t *testing.T) {
	for _, route := range []string{"custom", "auto"} {
		for _, level := range []ThinkingLevel{ThinkingOff, ThinkingNone, ThinkingXHigh, "max", "Future"} {
			for _, streaming := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/stream=%v", route, level, streaming), func(t *testing.T) {
					captured := make(chan map[string]any, 1)
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
							w.WriteHeader(400)
							return
						}
						captured <- body
						if streaming {
							w.Header().Set("Content-Type", "text/event-stream")
							_, _ = fmt.Fprint(w, "data: {\"id\":\"fixture\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"fixture\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
						} else {
							w.Header().Set("Content-Type", "application/json")
							_, _ = fmt.Fprint(w, `{"id":"fixture","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
						}
					}))
					defer server.Close()
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					cfg := &ProviderConfig{ModelString: "custom/unlisted-fixture", ProviderURL: server.URL + "/v1", ProviderAPIKey: "fixture", ThinkingLevel: level, ConfigStore: viper.New()}
					if route == "auto" {
						cfg.ModelString = "fixture/unlisted-fixture"
						cfg.ProviderWire = "openai-compat"
					}
					result, err := CreateProvider(ctx, cfg)
					if err != nil {
						t.Fatal(err)
					}
					call := fantasy.Call{Prompt: []fantasy.Message{fantasy.NewUserMessage("hello")}, ProviderOptions: result.ProviderOptions}
					if streaming {
						stream, err := result.Model.Stream(ctx, call)
						if err != nil {
							t.Fatal(err)
						}
						for part := range stream {
							if part.Error != nil {
								t.Fatal(part.Error)
							}
						}
					} else {
						if _, err := result.Model.Generate(ctx, call); err != nil {
							t.Fatal(err)
						}
					}
					select {
					case body := <-captured:
						effort, exists := body["reasoning_effort"]
						if level == ThinkingOff {
							if exists {
								t.Fatal(body)
							}
						} else if effort != string(level) {
							t.Fatal(body)
						}
						for _, key := range []string{"reasoning", "include", "reasoning_summary"} {
							if _, ok := body[key]; ok {
								t.Fatalf("chat contains %s", key)
							}
						}
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
				})
			}
		}
	}
}
