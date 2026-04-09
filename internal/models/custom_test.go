package models

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestConvertGenerationParams(t *testing.T) {
	t.Run("empty config returns nil", func(t *testing.T) {
		cfg := GenerationParamsConfig{}
		p := convertGenerationParams(cfg)
		if p != nil {
			t.Errorf("expected nil, got %+v", p)
		}
	})

	t.Run("temperature only", func(t *testing.T) {
		temp := float32(0.7)
		cfg := GenerationParamsConfig{Temperature: &temp}
		p := convertGenerationParams(cfg)
		if p == nil {
			t.Fatal("expected non-nil")
		}
		if p.Temperature == nil || *p.Temperature != 0.7 {
			t.Errorf("expected temperature 0.7, got %v", p.Temperature)
		}
		if p.TopP != nil {
			t.Errorf("expected nil TopP, got %v", p.TopP)
		}
	})

	t.Run("all params set", func(t *testing.T) {
		maxTokens := 8192
		temp := float32(0.5)
		topP := float32(0.9)
		topK := int32(50)
		freqPenalty := float32(0.1)
		presPenalty := float32(0.2)
		cfg := GenerationParamsConfig{
			MaxTokens:        &maxTokens,
			Temperature:      &temp,
			TopP:             &topP,
			TopK:             &topK,
			FrequencyPenalty: &freqPenalty,
			PresencePenalty:  &presPenalty,
			StopSequences:    []string{"STOP"},
			ThinkingLevel:    "high",
		}
		p := convertGenerationParams(cfg)
		if p == nil {
			t.Fatal("expected non-nil")
		}
		if p.MaxTokens == nil || *p.MaxTokens != 8192 {
			t.Errorf("expected maxTokens 8192, got %v", p.MaxTokens)
		}
		if p.Temperature == nil || *p.Temperature != 0.5 {
			t.Errorf("expected temperature 0.5, got %v", p.Temperature)
		}
		if p.TopP == nil || *p.TopP != 0.9 {
			t.Errorf("expected topP 0.9, got %v", p.TopP)
		}
		if p.TopK == nil || *p.TopK != 50 {
			t.Errorf("expected topK 50, got %v", p.TopK)
		}
		if p.FrequencyPenalty == nil || *p.FrequencyPenalty != 0.1 {
			t.Errorf("expected frequencyPenalty 0.1, got %v", p.FrequencyPenalty)
		}
		if p.PresencePenalty == nil || *p.PresencePenalty != 0.2 {
			t.Errorf("expected presencePenalty 0.2, got %v", p.PresencePenalty)
		}
		if len(p.StopSequences) != 1 || p.StopSequences[0] != "STOP" {
			t.Errorf("expected stop sequences [STOP], got %v", p.StopSequences)
		}
		if p.ThinkingLevel != ThinkingHigh {
			t.Errorf("expected thinking level high, got %v", p.ThinkingLevel)
		}
	})

	t.Run("thinking level parsing", func(t *testing.T) {
		cfg := GenerationParamsConfig{ThinkingLevel: "medium"}
		p := convertGenerationParams(cfg)
		if p == nil {
			t.Fatal("expected non-nil")
		}
		if p.ThinkingLevel != ThinkingMedium {
			t.Errorf("expected thinking level medium, got %v", p.ThinkingLevel)
		}
	})
	t.Run("system prompt only", func(t *testing.T) {
		cfg := GenerationParamsConfig{SystemPrompt: "You are helpful."}
		p := convertGenerationParams(cfg)
		if p == nil {
			t.Fatal("expected non-nil")
		}
		if p.SystemPrompt != "You are helpful." {
			t.Errorf("expected system prompt, got %q", p.SystemPrompt)
		}
	})
}

func TestModelConfigToModelInfoWithParams(t *testing.T) {
	temp := float32(0.8)
	topP := float32(0.95)
	cfg := CustomModelConfig{
		Name:        "Test Model",
		BaseURL:     "http://localhost:8080/v1",
		Temperature: true,
		Params: GenerationParamsConfig{
			Temperature: &temp,
			TopP:        &topP,
		},
	}

	info := modelConfigToModelInfo("test-model", cfg)

	if info.Params == nil {
		t.Fatal("expected non-nil Params")
	}
	if info.Params.Temperature == nil || *info.Params.Temperature != 0.8 {
		t.Errorf("expected temperature 0.8, got %v", info.Params.Temperature)
	}
	if info.Params.TopP == nil || *info.Params.TopP != 0.95 {
		t.Errorf("expected topP 0.95, got %v", info.Params.TopP)
	}
}

func TestModelConfigToModelInfoWithoutParams(t *testing.T) {
	cfg := CustomModelConfig{
		Name:    "Test Model",
		BaseURL: "http://localhost:8080/v1",
	}

	info := modelConfigToModelInfo("test-model", cfg)

	if info.Params != nil {
		t.Errorf("expected nil Params, got %+v", info.Params)
	}
}

func TestApplyModelSettings(t *testing.T) {
	// Save and restore viper state.
	originalViper := viper.AllSettings()
	defer func() {
		viper.Reset()
		for k, v := range originalViper {
			viper.Set(k, v)
		}
	}()

	t.Run("applies model params when not explicitly set", func(t *testing.T) {
		viper.Reset()

		temp := float32(0.8)
		topK := int32(50)
		maxTokens := 4096
		modelInfo := &ModelInfo{
			ID: "test-model",
			Params: &GenerationParams{
				Temperature: &temp,
				TopK:        &topK,
				MaxTokens:   &maxTokens,
			},
		}

		config := &ProviderConfig{
			ModelString: "custom/test-model",
		}

		ApplyModelSettings(config, modelInfo)

		if config.Temperature == nil || *config.Temperature != 0.8 {
			t.Errorf("expected temperature 0.8, got %v", config.Temperature)
		}
		if config.TopK == nil || *config.TopK != 50 {
			t.Errorf("expected topK 50, got %v", config.TopK)
		}
		if config.MaxTokens != 4096 {
			t.Errorf("expected maxTokens 4096, got %d", config.MaxTokens)
		}
	})

	t.Run("explicit viper values take precedence", func(t *testing.T) {
		viper.Reset()
		viper.Set("temperature", 0.3)

		temp := float32(0.8)
		modelInfo := &ModelInfo{
			ID: "test-model",
			Params: &GenerationParams{
				Temperature: &temp,
			},
		}

		explicitTemp := float32(0.3)
		config := &ProviderConfig{
			ModelString: "custom/test-model",
			Temperature: &explicitTemp,
		}

		ApplyModelSettings(config, modelInfo)

		// Temperature should NOT be overridden because it's explicitly set in viper
		if config.Temperature == nil || *config.Temperature != 0.3 {
			t.Errorf("expected temperature 0.3 (explicit), got %v", config.Temperature)
		}
	})

	t.Run("nil model info is safe", func(t *testing.T) {
		viper.Reset()

		config := &ProviderConfig{
			ModelString: "custom/test-model",
		}

		// Should not panic
		ApplyModelSettings(config, nil)

		if config.Temperature != nil {
			t.Errorf("expected nil temperature, got %v", config.Temperature)
		}
	})

	t.Run("model info without params is safe", func(t *testing.T) {
		viper.Reset()

		modelInfo := &ModelInfo{ID: "test-model"}
		config := &ProviderConfig{
			ModelString: "custom/test-model",
		}

		ApplyModelSettings(config, modelInfo)

		if config.Temperature != nil {
			t.Errorf("expected nil temperature, got %v", config.Temperature)
		}
	})

	t.Run("modelSettings from viper takes priority over ModelInfo.Params", func(t *testing.T) {
		viper.Reset()

		// Set up modelSettings in viper (simulating config file)
		viper.Set("modelSettings", map[string]any{
			"custom/test-model": map[string]any{
				"temperature": 0.5,
				"topK":        30,
			},
		})

		// ModelInfo has different params
		temp := float32(0.8)
		topK := int32(50)
		modelInfo := &ModelInfo{
			ID: "test-model",
			Params: &GenerationParams{
				Temperature: &temp,
				TopK:        &topK,
			},
		}

		config := &ProviderConfig{
			ModelString: "custom/test-model",
		}

		ApplyModelSettings(config, modelInfo)

		// modelSettings should win over ModelInfo.Params
		if config.Temperature == nil || *config.Temperature != 0.5 {
			t.Errorf("expected temperature 0.5 (from modelSettings), got %v", config.Temperature)
		}
		if config.TopK == nil || *config.TopK != 30 {
			t.Errorf("expected topK 30 (from modelSettings), got %v", config.TopK)
		}
	})

	t.Run("stop sequences applied from model params", func(t *testing.T) {
		viper.Reset()

		modelInfo := &ModelInfo{
			ID: "test-model",
			Params: &GenerationParams{
				StopSequences: []string{"STOP", "END"},
			},
		}

		config := &ProviderConfig{
			ModelString: "custom/test-model",
		}

		ApplyModelSettings(config, modelInfo)

		if len(config.StopSequences) != 2 || config.StopSequences[0] != "STOP" {
			t.Errorf("expected stop sequences [STOP END], got %v", config.StopSequences)
		}
	})

	t.Run("thinking level applied from model params", func(t *testing.T) {
		viper.Reset()

		modelInfo := &ModelInfo{
			ID: "test-model",
			Params: &GenerationParams{
				ThinkingLevel: ThinkingHigh,
			},
		}

		config := &ProviderConfig{
			ModelString: "custom/test-model",
		}

		ApplyModelSettings(config, modelInfo)

		if config.ThinkingLevel != ThinkingHigh {
			t.Errorf("expected thinking level high, got %v", config.ThinkingLevel)
		}
	})

	t.Run("system prompt applied from model params", func(t *testing.T) {
		viper.Reset()

		modelInfo := &ModelInfo{
			ID: "test-model",
			Params: &GenerationParams{
				SystemPrompt: "You are a coding assistant.",
			},
		}

		config := &ProviderConfig{
			ModelString: "custom/test-model",
		}

		ApplyModelSettings(config, modelInfo)

		if config.SystemPrompt != "You are a coding assistant." {
			t.Errorf("expected system prompt to be set, got %q", config.SystemPrompt)
		}
	})

	t.Run("explicit system prompt takes precedence", func(t *testing.T) {
		viper.Reset()

		modelInfo := &ModelInfo{
			ID: "test-model",
			Params: &GenerationParams{
				SystemPrompt: "Model-specific prompt",
			},
		}

		config := &ProviderConfig{
			ModelString:  "custom/test-model",
			SystemPrompt: "Global prompt",
		}

		ApplyModelSettings(config, modelInfo)

		// Global system prompt should NOT be overridden because config
		// already has a non-empty SystemPrompt.
		if config.SystemPrompt != "Global prompt" {
			t.Errorf("expected global prompt preserved, got %q", config.SystemPrompt)
		}
	})

	t.Run("system prompt from file path", func(t *testing.T) {
		viper.Reset()

		// Create a temp file with a system prompt
		tmpFile, err := os.CreateTemp("", "kit-test-prompt-*.txt")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()
		if _, err := tmpFile.WriteString("  Prompt from file  "); err != nil {
			t.Fatal(err)
		}
		_ = tmpFile.Close()

		modelInfo := &ModelInfo{
			ID: "test-model",
			Params: &GenerationParams{
				SystemPrompt: tmpFile.Name(),
			},
		}

		config := &ProviderConfig{
			ModelString: "custom/test-model",
		}

		ApplyModelSettings(config, modelInfo)

		if config.SystemPrompt != "Prompt from file" {
			t.Errorf("expected trimmed file content, got %q", config.SystemPrompt)
		}
	})

	t.Run("modelSettings system prompt overrides custom model params", func(t *testing.T) {
		viper.Reset()

		viper.Set("modelSettings", map[string]any{
			"custom/test-model": map[string]any{
				"systemPrompt": "From modelSettings",
			},
		})

		modelInfo := &ModelInfo{
			ID: "test-model",
			Params: &GenerationParams{
				SystemPrompt: "From custom model",
			},
		}

		config := &ProviderConfig{
			ModelString: "custom/test-model",
		}

		ApplyModelSettings(config, modelInfo)

		if config.SystemPrompt != "From modelSettings" {
			t.Errorf("expected modelSettings prompt, got %q", config.SystemPrompt)
		}
	})
}
