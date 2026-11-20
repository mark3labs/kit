package models

import (
	"context"
	"testing"
)

func samplingParams() (*float32, *float32, *int32) {
	temp, topP, topK := float32(0.7), float32(0.9), int32(40)
	return &temp, &topP, &topK
}

// TestValidateModelConfigSamplingParams checks that temperature, top_p and
// top_k are all cleared for models that don't support temperature, and kept
// for models that do.
func TestValidateModelConfigSamplingParams(t *testing.T) {
	t.Run("unsupported", func(t *testing.T) {
		cfg := &ProviderConfig{}
		cfg.Temperature, cfg.TopP, cfg.TopK = samplingParams()
		validateModelConfig(cfg, &ModelInfo{ID: "m", Temperature: false})
		if cfg.Temperature != nil || cfg.TopP != nil || cfg.TopK != nil {
			t.Errorf("sampling params not cleared: temperature=%v top_p=%v top_k=%v", cfg.Temperature, cfg.TopP, cfg.TopK)
		}
	})
	t.Run("supported", func(t *testing.T) {
		cfg := &ProviderConfig{}
		cfg.Temperature, cfg.TopP, cfg.TopK = samplingParams()
		validateModelConfig(cfg, &ModelInfo{ID: "m", Temperature: true})
		if cfg.Temperature == nil || cfg.TopP == nil || cfg.TopK == nil {
			t.Errorf("sampling params cleared for a model that supports them: temperature=%v top_p=%v top_k=%v", cfg.Temperature, cfg.TopP, cfg.TopK)
		}
	})
}

// TestCreateProviderDropsSamplingParamsForSonnet55 covers the default model:
// claude-sonnet-5-5 rejects temperature, top_p and top_k, so none of them may
// reach the request, including when --temperature and --top-p are both set.
func TestCreateProviderDropsSamplingParamsForSonnet55(t *testing.T) {
	info := NewEmbeddedModelsRegistry().LookupModel("anthropic", "claude-sonnet-5-5")
	if info == nil {
		t.Fatal("claude-sonnet-5-5 is not in the embedded catalog")
	}
	if info.Temperature {
		t.Fatal("embedded catalog lists claude-sonnet-5-5 as supporting temperature")
	}

	cfg := &ProviderConfig{ModelString: "anthropic/claude-sonnet-5-5", ProviderAPIKey: "test-key"}
	cfg.Temperature, cfg.TopP, cfg.TopK = samplingParams()
	if _, err := CreateProvider(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Temperature != nil || cfg.TopP != nil || cfg.TopK != nil {
		t.Errorf("sampling params would be sent to claude-sonnet-5-5: temperature=%v top_p=%v top_k=%v", cfg.Temperature, cfg.TopP, cfg.TopK)
	}
}
