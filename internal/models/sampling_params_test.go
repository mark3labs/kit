package models

import (
	"context"
	"slices"
	"testing"

	"github.com/spf13/viper"
)

// useEmbeddedGlobalRegistry makes the global registry, which CreateProvider
// reads, use the embedded catalog only for the rest of the test, so a local
// `kit update-models` cache can't change the result.
func useEmbeddedGlobalRegistry(t *testing.T) {
	t.Helper()
	prev := globalRegistry.Load()
	globalRegistry.Store(NewEmbeddedModelsRegistry())
	t.Cleanup(func() { globalRegistry.Store(prev) })
}

func samplingParams() (*float32, *float32, *int32) {
	temp, topP, topK := float32(0.7), float32(0.9), int32(40)
	return &temp, &topP, &topK
}

// TestValidateModelConfigSamplingParams checks that temperature, top_p and
// top_k are all cleared for models whose metadata marks sampling unsupported,
// and kept for models that support temperature.
func TestValidateModelConfigSamplingParams(t *testing.T) {
	t.Run("unsupported", func(t *testing.T) {
		cfg := &ProviderConfig{}
		cfg.Temperature, cfg.TopP, cfg.TopK = samplingParams()
		validateModelConfig(cfg, &ModelInfo{ID: "m", Temperature: false, SamplingUnsupported: true})
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
	useEmbeddedGlobalRegistry(t)

	info := GetGlobalRegistry().LookupModel("anthropic", "claude-sonnet-5-5")
	if info == nil {
		t.Fatal("claude-sonnet-5-5 is not in the embedded catalog")
	}
	if info.Temperature || !info.SamplingUnsupported {
		t.Fatalf("embedded catalog does not mark sampling unsupported for claude-sonnet-5-5: temperature=%v sampling_unsupported=%v", info.Temperature, info.SamplingUnsupported)
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

// TestCreateProviderDropsModelSettingsSamplingParamsForSonnet55 checks that
// sampling parameters from a per-model modelSettings entry don't reach
// claude-sonnet-5-5 either. modelSettings is applied inside CreateProvider,
// so the clean-up has to run after it.
func TestCreateProviderDropsModelSettingsSamplingParamsForSonnet55(t *testing.T) {
	useEmbeddedGlobalRegistry(t)

	v := viper.New()
	v.Set("modelSettings", map[string]any{
		"anthropic/claude-sonnet-5-5": map[string]any{"temperature": 0.3, "topP": 0.9, "topK": 40},
	})
	cfg := &ProviderConfig{ModelString: "anthropic/claude-sonnet-5-5", ProviderAPIKey: "test-key", ConfigStore: v}
	if _, err := CreateProvider(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Temperature != nil || cfg.TopP != nil || cfg.TopK != nil {
		t.Errorf("modelSettings sampling params would be sent to claude-sonnet-5-5: temperature=%v top_p=%v top_k=%v", cfg.Temperature, cfg.TopP, cfg.TopK)
	}
}

// TestCustomModelKeepsTopPAndTopK covers a custom model without a
// "temperature" key: its params.topP and params.topK must reach the request,
// because the metadata does not mark them unsupported. Temperature is still
// cleared, as before.
func TestCustomModelKeepsTopPAndTopK(t *testing.T) {
	temp, topP, topK := samplingParams()
	info := &ModelInfo{ID: "my-model", Params: &GenerationParams{Temperature: temp, TopP: topP, TopK: topK}}
	cfg := &ProviderConfig{ModelString: "custom/my-model", ConfigStore: viper.New()}

	ApplyModelSettings(cfg, info)
	dropped := clearUnsupportedSampling(cfg, info)

	if cfg.TopP == nil || *cfg.TopP != *topP || cfg.TopK == nil || *cfg.TopK != *topK {
		t.Errorf("custom model lost top_p/top_k: top_p=%v top_k=%v", cfg.TopP, cfg.TopK)
	}
	if cfg.Temperature != nil {
		t.Errorf("temperature kept for a model without temperature support: %v", *cfg.Temperature)
	}
	if want := []string{"temperature"}; !slices.Equal(dropped, want) {
		t.Errorf("reported %v, want %v", dropped, want)
	}
}

// TestClearUnsupportedSamplingReportsUserValues checks that a value the user
// set is reported when it is cleared, and that a built-in default is not.
func TestClearUnsupportedSamplingReportsUserValues(t *testing.T) {
	info := &ModelInfo{ID: "m", Temperature: false, SamplingUnsupported: true}

	t.Run("defaults", func(t *testing.T) {
		cfg := &ProviderConfig{ModelString: "anthropic/m", ConfigStore: viper.New()}
		cfg.Temperature, cfg.TopP, cfg.TopK = samplingParams()
		if dropped := clearUnsupportedSampling(cfg, info); len(dropped) != 0 {
			t.Errorf("built-in defaults reported as user values: %v", dropped)
		}
	})
	t.Run("flags", func(t *testing.T) {
		v := viper.New()
		v.Set("top-p", 0.9)
		v.Set("top-k", 40)
		cfg := &ProviderConfig{ModelString: "anthropic/m", ConfigStore: v}
		cfg.Temperature, cfg.TopP, cfg.TopK = samplingParams()
		dropped := clearUnsupportedSampling(cfg, info)
		if want := []string{"top_p", "top_k"}; !slices.Equal(dropped, want) {
			t.Errorf("reported %v, want %v", dropped, want)
		}
		if cfg.Temperature != nil || cfg.TopP != nil || cfg.TopK != nil {
			t.Error("sampling params not cleared")
		}
	})
	t.Run("modelSettings", func(t *testing.T) {
		v := viper.New()
		v.Set("modelSettings", map[string]any{"anthropic/m": map[string]any{"temperature": 0.3, "topK": 20}})
		cfg := &ProviderConfig{ModelString: "anthropic/m", ConfigStore: v}
		cfg.Temperature, cfg.TopP, cfg.TopK = samplingParams()
		dropped := clearUnsupportedSampling(cfg, info)
		if want := []string{"temperature", "top_k"}; !slices.Equal(dropped, want) {
			t.Errorf("reported %v, want %v", dropped, want)
		}
	})
}

// TestCatalogOmittedTemperatureKeepsTopP checks that a catalog entry without
// a "temperature" key is not marked as rejecting sampling parameters: only an
// explicit "temperature": false does that.
func TestCatalogOmittedTemperatureKeepsTopP(t *testing.T) {
	reg := NewEmbeddedModelsRegistry()
	for providerID, p := range loadEmbeddedProviders() {
		for modelID, m := range p.Models {
			if m.Temperature != nil || m.Type != "" {
				continue
			}
			info := reg.LookupModel(providerID, modelID)
			if info == nil {
				continue
			}
			if info.Temperature || info.SamplingUnsupported {
				t.Fatalf("%s/%s omits temperature but got temperature=%v sampling_unsupported=%v",
					providerID, modelID, info.Temperature, info.SamplingUnsupported)
			}
			return
		}
	}
	t.Skip("embedded catalog has no chat model without a temperature key")
}
