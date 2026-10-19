package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/spf13/viper"
)

func TestNativeEffortWire(t *testing.T) {
	for _, route := range []string{"openai", "forced", "anthropic"} {
		for _, level := range []ThinkingLevel{ThinkingOff, ThinkingNone, ThinkingXHigh, "max", "Future"} {
			t.Run(route+"/"+string(level), func(t *testing.T) {
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
					if route == "anthropic" {
						_, _ = fmt.Fprint(w, `{"id":"fixture","type":"message","role":"assistant","model":"fixture","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
					} else {
						_, _ = fmt.Fprint(w, `{"id":"fixture","object":"response","status":"completed","output":[{"id":"msg","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok","annotations":[]}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`)
					}
				}))
				defer server.Close()
				old := GetGlobalRegistry()
				provider, model := "openai", "gpt-5-fixture"
				if route == "forced" {
					provider = "fixture"
				}
				if route == "anthropic" {
					provider = "anthropic"
					model = "claude-fixture"
				}
				globalRegistry.Store(&ModelsRegistry{providers: map[string]ProviderInfo{provider: {ID: provider, Models: map[string]ModelInfo{model: {Reasoning: true, ReasoningIsGraded: true, ReasoningLevels: []string{"none", "xhigh", "max", "Future"}}}}}})
				defer globalRegistry.Store(old)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				cfg := &ProviderConfig{ModelString: provider + "/" + model, ProviderURL: server.URL, ProviderAPIKey: "fixture", ThinkingLevel: level, MaxTokens: 1024, ConfigStore: viper.New()}
				if route == "forced" {
					cfg.ProviderWire = "openai"
				}
				result, err := CreateProvider(ctx, cfg)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := result.Model.Generate(ctx, fantasy.Call{Prompt: []fantasy.Message{fantasy.NewUserMessage("hello")}, ProviderOptions: result.ProviderOptions, MaxOutputTokens: new(int64(1024))}); err != nil {
					t.Fatal(err)
				}
				select {
				case body := <-captured:
					field := "reasoning"
					if route == "anthropic" {
						field = "output_config"
					}
					block, _ := body[field].(map[string]any)
					effort, exists := block["effort"]
					if level == ThinkingOff {
						if exists {
							t.Fatal(body)
						}
					} else if effort != string(level) {
						t.Fatal(body)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			})
		}
	}
}

func TestForcedResponsesUnsupportedEffortIsExplicit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := CreateProvider(ctx, &ProviderConfig{ModelString: "fixture/vendor-model", ProviderURL: "http://127.0.0.1:1", ProviderWire: "openai", ProviderAPIKey: "fixture", ThinkingLevel: "Future", ConfigStore: viper.New()})
	if err == nil || !strings.Contains(err.Error(), "adapter cannot encode") {
		t.Fatalf("got %v", err)
	}
}

func TestInvalidGradedEffortFailsBeforeIO(t *testing.T) {
	_, err := CreateProvider(context.Background(), &ProviderConfig{ModelString: "openai/o3", ThinkingLevel: "typo", ConfigStore: viper.New()})
	if err == nil || !strings.Contains(err.Error(), "does not support thinking level") {
		t.Fatalf("got %v", err)
	}
}
