package models

import (
	"log"
	"os"
	"strings"

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
	info := ModelInfo{
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

	// Convert custom model generation params if any are set.
	if p := convertGenerationParams(cfg.Params); p != nil {
		info.Params = p
	}

	return info
}

// LoadModelSettingsFromConfig loads per-model generation parameter overrides
// from the config file. Keys are "provider/model" strings. Returns nil if
// no model settings are configured.
func LoadModelSettingsFromConfig() map[string]*GenerationParams {
	if !viper.IsSet("modelSettings") {
		return nil
	}

	var settings map[string]GenerationParamsConfig
	if err := viper.UnmarshalKey("modelSettings", &settings); err != nil {
		log.Printf("Warning: Failed to parse modelSettings: %v", err)
		return nil
	}

	result := make(map[string]*GenerationParams, len(settings))
	for modelKey, cfg := range settings {
		if p := convertGenerationParams(cfg); p != nil {
			result[modelKey] = p
		}
	}

	return result
}

// convertGenerationParams converts a GenerationParamsConfig to a GenerationParams.
// Returns nil if no parameters are set.
func convertGenerationParams(cfg GenerationParamsConfig) *GenerationParams {
	p := &GenerationParams{}
	any := false

	if cfg.MaxTokens != nil {
		p.MaxTokens = cfg.MaxTokens
		any = true
	}
	if cfg.Temperature != nil {
		p.Temperature = cfg.Temperature
		any = true
	}
	if cfg.TopP != nil {
		p.TopP = cfg.TopP
		any = true
	}
	if cfg.TopK != nil {
		p.TopK = cfg.TopK
		any = true
	}
	if cfg.FrequencyPenalty != nil {
		p.FrequencyPenalty = cfg.FrequencyPenalty
		any = true
	}
	if cfg.PresencePenalty != nil {
		p.PresencePenalty = cfg.PresencePenalty
		any = true
	}
	if len(cfg.StopSequences) > 0 {
		p.StopSequences = cfg.StopSequences
		any = true
	}
	if cfg.ThinkingLevel != "" {
		p.ThinkingLevel = ParseThinkingLevel(cfg.ThinkingLevel)
		any = true
	}
	if cfg.SystemPrompt != "" {
		p.SystemPrompt = cfg.SystemPrompt
		any = true
	}

	if !any {
		return nil
	}
	return p
}

// ApplyModelSettings merges per-model generation parameter defaults from the
// registry into a ProviderConfig. Model-level params are only applied for
// fields where the user has not explicitly set a value (i.e., the
// corresponding viper key is not set via CLI flag or global config).
//
// The lookup order is:
//  1. modelSettings["provider/model"] from config (highest model-level priority)
//  2. ModelInfo.Params from custom model definitions
//
// Both are overridden by explicit CLI flags / global config values.
func ApplyModelSettings(config *ProviderConfig, modelInfo *ModelInfo) {
	provider, modelName, err := ParseModelString(config.ModelString)
	if err != nil {
		return
	}

	// Collect model-level params: modelSettings override > custom model params.
	// modelSettings takes priority because it's the more specific/intentional config.
	var params *GenerationParams

	// First check modelSettings from config.
	if settings := LoadModelSettingsFromConfig(); settings != nil {
		modelKey := provider + "/" + modelName
		if p, ok := settings[modelKey]; ok {
			params = p
		}
	}

	// Fall back to ModelInfo.Params (from custom model definitions).
	if params == nil && modelInfo != nil && modelInfo.Params != nil {
		params = modelInfo.Params
	}

	if params == nil {
		return
	}

	// Apply each parameter only when the user hasn't explicitly set it.
	// We check viper.IsSet() which returns true only when the key was
	// set via CLI flag, environment variable, or config file global section.

	if params.MaxTokens != nil && !isExplicitlySet("max-tokens") {
		config.MaxTokens = *params.MaxTokens
	}
	if params.Temperature != nil && !isExplicitlySet("temperature") {
		config.Temperature = params.Temperature
	}
	if params.TopP != nil && !isExplicitlySet("top-p") {
		config.TopP = params.TopP
	}
	if params.TopK != nil && !isExplicitlySet("top-k") {
		config.TopK = params.TopK
	}
	if params.FrequencyPenalty != nil && !isExplicitlySet("frequency-penalty") {
		config.FrequencyPenalty = params.FrequencyPenalty
	}
	if params.PresencePenalty != nil && !isExplicitlySet("presence-penalty") {
		config.PresencePenalty = params.PresencePenalty
	}
	if len(params.StopSequences) > 0 && !isExplicitlySet("stop-sequences") {
		config.StopSequences = params.StopSequences
	}
	if params.ThinkingLevel != "" && !isExplicitlySet("thinking-level") {
		config.ThinkingLevel = params.ThinkingLevel
	}
	if params.SystemPrompt != "" && config.SystemPrompt == "" {
		// Resolve file paths: if the value points to an existing file, read it.
		// We check config.SystemPrompt == "" rather than isExplicitlySet because
		// viper.BindPFlag causes IsSet to return true even for unset flags.
		config.SystemPrompt = LoadSystemPromptValue(params.SystemPrompt)
	}
}

// LoadSystemPromptValue resolves a system prompt value that may be either
// inline text or a file path. If the value is a path to an existing file,
// its contents are read and returned. Otherwise the string is returned as-is.
// This mirrors config.LoadSystemPrompt but lives in the models package to
// avoid circular dependencies.
func LoadSystemPromptValue(input string) string {
	if input == "" {
		return ""
	}
	if info, err := os.Stat(input); err == nil && !info.IsDir() {
		content, err := os.ReadFile(input)
		if err != nil {
			log.Printf("Warning: failed to read system prompt file %q: %v", input, err)
			return input
		}
		return strings.TrimSpace(string(content))
	}
	return input
}

// isExplicitlySet returns true when the user has explicitly set a config key
// via CLI flag, environment variable, or the global section of the config file.
// Model-level defaults should not override explicitly set values.
func isExplicitlySet(key string) bool {
	// viper.IsSet returns true if the key has been set in any of the
	// data stores (flag, env, config file, default). We need to check
	// whether the value was set at the global config level (not just
	// as a default). For generation params, the global config keys use
	// hyphenated names (e.g. "max-tokens", "top-p").
	//
	// Since viper merges all sources, IsSet returns true even for config
	// file values. This means global config file values (e.g.
	// temperature: 0.7 at the top level) will correctly take precedence
	// over model-level defaults, which is the desired behavior.
	return viper.IsSet(key)
}

// GenerationParams holds per-model generation parameter defaults.
// These are stored on ModelInfo and applied during provider creation.
// Nil pointer fields mean "no model-level default" — the global config
// or CLI flag value (if any) will be used instead.
type GenerationParams struct {
	MaxTokens        *int
	Temperature      *float32
	TopP             *float32
	TopK             *int32
	FrequencyPenalty *float32
	PresencePenalty  *float32
	StopSequences    []string
	ThinkingLevel    ThinkingLevel
	SystemPrompt     string // Per-model system prompt (inline text or file path)
}

// CustomModelConfig defines a custom model configuration loaded from the config file.
// This is a duplicate here to avoid circular dependencies with internal/config.
type CustomModelConfig struct {
	Name        string                 `json:"name" yaml:"name"`
	BaseURL     string                 `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	APIKey      string                 `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	Family      string                 `json:"family,omitempty" yaml:"family,omitempty"`
	Attachment  bool                   `json:"attachment,omitempty" yaml:"attachment,omitempty"`
	Reasoning   bool                   `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`
	Temperature bool                   `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	Knowledge   string                 `json:"knowledge,omitempty" yaml:"knowledge,omitempty"`
	Cost        CostConfig             `json:"cost" yaml:"cost"`
	Limit       LimitConfig            `json:"limit" yaml:"limit"`
	Params      GenerationParamsConfig `json:"params,omitzero" yaml:"params,omitempty"`
}

// GenerationParamsConfig is the JSON/YAML-serializable form of generation
// parameter defaults. Used in both customModels[].params and modelSettings[].
type GenerationParamsConfig struct {
	MaxTokens        *int     `json:"maxTokens,omitempty" yaml:"maxTokens,omitempty"`
	Temperature      *float32 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	TopP             *float32 `json:"topP,omitempty" yaml:"topP,omitempty"`
	TopK             *int32   `json:"topK,omitempty" yaml:"topK,omitempty"`
	FrequencyPenalty *float32 `json:"frequencyPenalty,omitempty" yaml:"frequencyPenalty,omitempty"`
	PresencePenalty  *float32 `json:"presencePenalty,omitempty" yaml:"presencePenalty,omitempty"`
	StopSequences    []string `json:"stopSequences,omitempty" yaml:"stopSequences,omitempty"`
	ThinkingLevel    string   `json:"thinkingLevel,omitempty" yaml:"thinkingLevel,omitempty"`
	SystemPrompt     string   `json:"systemPrompt,omitempty" yaml:"systemPrompt,omitempty"`
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
