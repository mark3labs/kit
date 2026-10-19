package models

import (
	"context"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/spf13/viper"
)

// bedrockTestAuth gives the AWS SDK a fake bearer token and region, and keeps
// it away from the developer's own AWS setup.
func bedrockTestAuth(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "test-token")
	t.Setenv("AWS_REGION", "us-east-1")
}

// vertexTestAuth points Application Default Credentials at a fake
// authorized-user file and sets a project.
func vertexTestAuth(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "google.json")
	if err := os.WriteFile(path, []byte(`{"type":"authorized_user","client_id":"fake","client_secret":"fake","refresh_token":"fake","token_uri":"http://credentials.test/token"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
	t.Setenv("GOOGLE_VERTEX_PROJECT", "test-project")
	t.Setenv("GOOGLE_VERTEX_LOCATION", "us-east5")
}

// TestBuildAnthropicProviderOptionsFor_CloudProviders checks that the
// Bedrock and Vertex IDs of Anthropic's models get the options of the native
// rule from their own catalog entry: effort for a model with an effort
// scale, else a token budget.
func TestBuildAnthropicProviderOptionsFor_CloudProviders(t *testing.T) {
	tests := []struct {
		provider, model string
		effort          bool
	}{
		{"amazon-bedrock", "us.anthropic.claude-opus-5-5", true},
		{"amazon-bedrock", "global.anthropic.claude-opus-5-5", true},
		// Opus 4.6 lists a token budget too, and gets the effort, as on the
		// native provider.
		{"amazon-bedrock", "us.anthropic.claude-opus-4-6-v1", true},
		{"amazon-bedrock", "us.anthropic.claude-sonnet-4-5-20250929-v1:0", false},
		{"google-vertex-anthropic", "claude-opus-5-5@default", true},
		{"google-vertex-anthropic", "claude-opus-4-6@default", true},
		{"google-vertex-anthropic", "claude-sonnet-4-5@20250929", false},
	}
	for _, tt := range tests {
		t.Run(tt.provider+"/"+tt.model, func(t *testing.T) {
			if GetGlobalRegistry().LookupModel(tt.provider, tt.model) == nil {
				t.Skip("model not in the catalog")
			}
			cfg := &ProviderConfig{ThinkingLevel: ThinkingHigh, MaxTokens: 8192}
			opts := buildAnthropicProviderOptionsFor(cfg, tt.provider, tt.model)
			po, ok := opts[anthropic.Name].(*anthropic.ProviderOptions)
			if !ok || po == nil {
				t.Fatalf("no anthropic options: %+v", opts)
			}
			if tt.effort {
				if po.Thinking != nil || po.Effort == nil || *po.Effort != anthropic.EffortHigh {
					t.Errorf("want effort high and no budget, got effort=%v thinking=%+v", po.Effort, po.Thinking)
				}
				return
			}
			if po.Effort != nil || po.Thinking == nil || po.Thinking.BudgetTokens == 0 {
				t.Errorf("want a token budget and no effort, got effort=%v thinking=%+v", po.Effort, po.Thinking)
			}
		})
	}
}

// TestBedrockRequestCarriesThinkingAndCaching checks the provider result and
// that the effort reaches the request Bedrock receives.
func TestBedrockRequestCarriesThinkingAndCaching(t *testing.T) {
	bedrockTestAuth(t)
	var body string
	mockDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
		if r.Body != nil {
			data, _ := io.ReadAll(r.Body)
			body = string(data)
		}
		return jsonTestResponse(r, `{}`), nil
	})
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	result, err := CreateProvider(ctx, &ProviderConfig{ModelString: "amazon-bedrock/us.anthropic.claude-opus-5-5", ThinkingLevel: ThinkingHigh})
	if err != nil {
		t.Fatal(err)
	}
	if !result.MessageCacheControl {
		t.Error("prompt caching is off for a Claude model in the catalog")
	}
	_, _ = result.Model.Generate(ctx, fantasy.Call{
		Prompt:          []fantasy.Message{fantasy.NewUserMessage("hi")},
		ProviderOptions: result.ProviderOptions,
	})
	if !strings.Contains(body, `"output_config"`) || !strings.Contains(body, `"effort":"high"`) {
		t.Errorf("request body has no effort: %s", body)
	}
}

// TestCloudThinkingNeedsReasoningModel checks that a model the catalog lists
// without reasoning gets no thinking options on Bedrock and Vertex.
func TestCloudThinkingNeedsReasoningModel(t *testing.T) {
	old := GetGlobalRegistry()
	reg := &ModelsRegistry{providers: maps.Clone(old.providers)}
	for _, provider := range []string{"amazon-bedrock", "google-vertex-anthropic"} {
		info := reg.providers[provider]
		info.Models = maps.Clone(info.Models)
		info.Models["test.claude-no-reasoning"] = ModelInfo{ID: "test.claude-no-reasoning", Family: "claude-sonnet"}
		reg.providers[provider] = info
	}
	globalRegistry.Store(reg)
	t.Cleanup(func() { globalRegistry.Store(old) })

	for _, provider := range []string{"amazon-bedrock", "google-vertex-anthropic"} {
		cfg := &ProviderConfig{ThinkingLevel: ThinkingHigh, MaxTokens: 8192}
		if opts := buildAnthropicProviderOptionsFor(cfg, provider, "test.claude-no-reasoning"); opts != nil {
			t.Errorf("%s: options = %+v, want none for a model without reasoning", provider, opts)
		}
		if cfg.MaxTokens != 8192 {
			t.Errorf("%s: MaxTokens = %d, want it unchanged", provider, cfg.MaxTokens)
		}
	}
}

// TestBedrockUnknownModelKeepsPlainRequest checks that a model that is not
// in the catalog gets no caching and no thinking options. With a thinking
// level set, Kit refuses the model, because there are no thinking options
// to send, and leaves max_tokens unchanged.
func TestBedrockUnknownModelKeepsPlainRequest(t *testing.T) {
	bedrockTestAuth(t)
	for _, model := range []string{
		"anthropic.claude-v2",
		"arn:aws:bedrock:us-east-1:123456789012:application-inference-profile/kit-test",
	} {
		t.Run(model, func(t *testing.T) {
			result, err := CreateProvider(t.Context(), &ProviderConfig{ModelString: "amazon-bedrock/" + model, ConfigStore: viper.New()})
			if err != nil {
				t.Fatal(err)
			}
			if result.MessageCacheControl {
				t.Error("prompt caching is on for a model that is not in the catalog")
			}
			if result.ProviderOptions != nil {
				t.Errorf("ProviderOptions = %+v, want none", result.ProviderOptions)
			}

			cfg := &ProviderConfig{ModelString: "amazon-bedrock/" + model, ThinkingLevel: ThinkingHigh, MaxTokens: 8192, ConfigStore: viper.New()}
			if _, err := CreateProvider(t.Context(), cfg); err == nil || !strings.Contains(err.Error(), "cannot encode reasoning effort") {
				t.Errorf("thinking level high: err = %v, want the level refused", err)
			}
			if cfg.MaxTokens != 8192 {
				t.Errorf("MaxTokens = %d, want it unchanged", cfg.MaxTokens)
			}
		})
	}
}

// TestBedrockSamplingParams checks that temperature and top_p are not sent
// together, as for the native provider.
func TestBedrockSamplingParams(t *testing.T) {
	bedrockTestAuth(t)
	temperature, topP := float32(0.5), float32(0.9)
	cfg := &ProviderConfig{
		ModelString: "amazon-bedrock/us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		Temperature: &temperature,
		TopP:        &topP,
	}
	if _, err := CreateProvider(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Temperature == nil || cfg.TopP != nil {
		t.Errorf("temperature=%v top_p=%v, want temperature only", cfg.Temperature, cfg.TopP)
	}
}

// TestBedrockThinkingLevels checks that Bedrock follows the thinking-level
// rules of the native provider: a model with an effort scale takes the
// efforts its catalog entry lists, and a model that takes a token budget
// refuses a named effort such as xhigh instead of sending no thinking.
func TestBedrockThinkingLevels(t *testing.T) {
	bedrockTestAuth(t)
	const graded = "us.anthropic.claude-opus-5-5"
	const budget = "us.anthropic.claude-sonnet-4-5-20250929-v1:0"
	for _, model := range []string{graded, budget} {
		if GetGlobalRegistry().LookupModel("amazon-bedrock", model) == nil {
			t.Skipf("%s not in the catalog", model)
		}
	}

	if levels := SupportedThinkingLevels("amazon-bedrock", budget); slices.Contains(levels, ThinkingXHigh) {
		t.Errorf("levels for %s = %v, want no xhigh", budget, levels)
	}
	if IsValidThinkingLevelForModel(ThinkingXHigh, "amazon-bedrock", budget) {
		t.Errorf("xhigh is valid for %s, want it refused", budget)
	}
	if !IsValidThinkingLevelForModel(ThinkingHigh, "amazon-bedrock", budget) {
		t.Errorf("high is refused for %s", budget)
	}
	_, err := CreateProvider(t.Context(), &ProviderConfig{ModelString: "amazon-bedrock/" + budget, ThinkingLevel: ThinkingXHigh, ConfigStore: viper.New()})
	if err == nil || !strings.Contains(err.Error(), "does not support thinking level") {
		t.Errorf("xhigh on %s: err = %v, want the level refused", budget, err)
	}

	cfg := &ProviderConfig{ThinkingLevel: ThinkingXHigh, MaxTokens: 8192}
	po, ok := buildAnthropicProviderOptionsFor(cfg, "amazon-bedrock", graded)[anthropic.Name].(*anthropic.ProviderOptions)
	if !ok || po.Effort == nil || *po.Effort != anthropic.EffortXHigh {
		t.Errorf("options for %s with xhigh = %+v, want effort xhigh", graded, po)
	}
}

// TestVertexAnthropicThinkingOptions checks that the thinking level reaches
// the Vertex provider.
func TestVertexAnthropicThinkingOptions(t *testing.T) {
	vertexTestAuth(t)
	result, err := CreateProvider(t.Context(), &ProviderConfig{ModelString: "google-vertex-anthropic/claude-opus-5-5@default", ThinkingLevel: ThinkingHigh})
	if err != nil {
		t.Fatal(err)
	}
	po, ok := result.ProviderOptions[anthropic.Name].(*anthropic.ProviderOptions)
	if !ok || po.Effort == nil || *po.Effort != anthropic.EffortHigh {
		t.Errorf("ProviderOptions = %+v, want effort high", result.ProviderOptions)
	}
	if !result.MessageCacheControl {
		t.Error("prompt caching is off on Vertex")
	}
}
