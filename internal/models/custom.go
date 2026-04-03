package models

import (
	"log"

	"github.com/spf13/viper"
)

// loadCustomModelsFromConfig loads custom model definitions from the config file
// and returns them as a map of model ID -> ModelInfo. Returns nil if no custom
// models are configured.
func loadCustomModelsFromConfig() map[string]ModelInfo {
	if !viper.IsSet("customModels") {
		return nil
	}

	var customModels map[string]CustomModelConfig
	if err := viper.UnmarshalKey("customModels", &customModels); err != nil {
		log.Printf("Warning: Failed to parse customModels: %v", err)
		return nil
	}

	result := make(map[string]ModelInfo, len(customModels))
	for modelID, cfg := range customModels {
		info := modelConfigToModelInfo(modelID, cfg)
		result[modelID] = info
	}

	return result
}

// modelConfigToModelInfo converts a CustomModelConfig to a ModelInfo.
func modelConfigToModelInfo(modelID string, cfg CustomModelConfig) ModelInfo {
	return ModelInfo{
		ID:          modelID,
		Name:        cfg.Name,
		Attachment:  cfg.Attachment,
		Reasoning:   cfg.Reasoning,
		Temperature: cfg.Temperature,
		BaseURL:     cfg.BaseURL,
		APIKey:      cfg.APIKey,
		Cost: Cost{
			Input:  cfg.Cost.Input,
			Output: cfg.Cost.Output,
		},
		Limit: Limit{
			Context: cfg.Limit.Context,
			Output:  cfg.Limit.Output,
		},
	}
}

// CustomModelConfig defines a custom model configuration loaded from the config file.
// This is a duplicate here to avoid circular dependencies with internal/config.
type CustomModelConfig struct {
	Name        string      `json:"name" yaml:"name"`
	BaseURL     string      `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	APIKey      string      `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	Family      string      `json:"family,omitempty" yaml:"family,omitempty"`
	Attachment  bool        `json:"attachment,omitempty" yaml:"attachment,omitempty"`
	Reasoning   bool        `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`
	Temperature bool        `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	Knowledge   string      `json:"knowledge,omitempty" yaml:"knowledge,omitempty"`
	Cost        CostConfig  `json:"cost" yaml:"cost"`
	Limit       LimitConfig `json:"limit" yaml:"limit"`
}

// CostConfig defines the pricing for a custom model.
type CostConfig struct {
	Input  float64 `json:"input" yaml:"input"`
	Output float64 `json:"output" yaml:"output"`
}

// LimitConfig defines context and output limits for a custom model.
type LimitConfig struct {
	Context int `json:"context" yaml:"context"`
	Output  int `json:"output" yaml:"output"`
}
