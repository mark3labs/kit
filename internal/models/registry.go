package models

import (
	"fmt"
	"os"
	"strings"

	"charm.land/catwalk/pkg/embedded"
)

// ModelInfo represents information about a specific model.
type ModelInfo struct {
	ID          string
	Name        string
	Attachment  bool
	Reasoning   bool
	Temperature bool
	Cost        Cost
	Limit       Limit
}

// Cost represents the pricing information for a model.
type Cost struct {
	Input      float64
	Output     float64
	CacheRead  *float64
	CacheWrite *float64
}

// Limit represents the context and output limits for a model.
type Limit struct {
	Context int
	Output  int
}

// ProviderInfo represents information about a model provider.
type ProviderInfo struct {
	ID     string
	Env    []string
	Name   string
	Models map[string]ModelInfo
}

// providerEnvVars maps provider IDs to their required environment variables.
// Catwalk provides APIKey field names but we need the actual env var names.
var providerEnvVars = map[string][]string{
	"anthropic":               {"ANTHROPIC_API_KEY"},
	"openai":                  {"OPENAI_API_KEY"},
	"google":                  {"GOOGLE_API_KEY", "GEMINI_API_KEY", "GOOGLE_GENERATIVE_AI_API_KEY"},
	"azure":                   {"AZURE_OPENAI_API_KEY"},
	"openrouter":              {"OPENROUTER_API_KEY"},
	"bedrock":                 {"AWS_ACCESS_KEY_ID"},
	"google-vertex-anthropic": {"GOOGLE_APPLICATION_CREDENTIALS"},
	"ollama":                  {},
	"mistral":                 {"MISTRAL_API_KEY"},
	"groq":                    {"GROQ_API_KEY"},
	"deepseek":                {"DEEPSEEK_API_KEY"},
	"xai":                     {"XAI_API_KEY"},
	"fireworks":               {"FIREWORKS_API_KEY"},
	"together":                {"TOGETHER_API_KEY"},
	"perplexity":              {"PERPLEXITY_API_KEY"},
	"alibaba":                 {"DASHSCOPE_API_KEY"},
	"cohere":                  {"COHERE_API_KEY"},
}

// ModelsRegistry provides validation and information about models.
// It maintains a registry of all supported LLM providers and their models,
// including capabilities, pricing, and configuration requirements.
// The registry data comes from the catwalk embedded database.
type ModelsRegistry struct {
	providers map[string]ProviderInfo
}

// NewModelsRegistry creates a new models registry populated from the catwalk embedded database.
func NewModelsRegistry() *ModelsRegistry {
	return &ModelsRegistry{
		providers: buildFromCatwalk(),
	}
}

// buildFromCatwalk converts catwalk embedded data into our internal format.
func buildFromCatwalk() map[string]ProviderInfo {
	providers := make(map[string]ProviderInfo)

	for _, cp := range embedded.GetAll() {
		providerID := string(cp.ID)

		modelsMap := make(map[string]ModelInfo, len(cp.Models))
		for _, cm := range cp.Models {
			var cacheRead, cacheWrite *float64
			if cm.CostPer1MInCached > 0 {
				v := cm.CostPer1MInCached
				cacheRead = &v
			}
			if cm.CostPer1MOutCached > 0 {
				v := cm.CostPer1MOutCached
				cacheWrite = &v
			}

			hasTemperature := true // most models support temperature
			if cm.Options.Temperature != nil && *cm.Options.Temperature == 0 {
				hasTemperature = false
			}

			modelsMap[cm.ID] = ModelInfo{
				ID:          cm.ID,
				Name:        cm.Name,
				Attachment:  cm.SupportsImages,
				Reasoning:   cm.CanReason,
				Temperature: hasTemperature,
				Cost: Cost{
					Input:      cm.CostPer1MIn,
					Output:     cm.CostPer1MOut,
					CacheRead:  cacheRead,
					CacheWrite: cacheWrite,
				},
				Limit: Limit{
					Context: int(cm.ContextWindow),
					Output:  int(cm.DefaultMaxTokens),
				},
			}
		}

		envVars := providerEnvVars[providerID]
		if envVars == nil {
			// Derive from the catwalk APIKey field if available
			if cp.APIKey != "" {
				envVars = []string{cp.APIKey}
			}
		}

		providers[providerID] = ProviderInfo{
			ID:     providerID,
			Env:    envVars,
			Name:   cp.Name,
			Models: modelsMap,
		}
	}

	// Ensure providers that mcphost explicitly supports are always present
	// even if catwalk doesn't list them (e.g. ollama, google-vertex-anthropic)
	ensureProvider(providers, "ollama", "Ollama", nil)
	ensureProvider(providers, "google-vertex-anthropic", "Google Vertex (Anthropic)",
		providerEnvVars["google-vertex-anthropic"])

	return providers
}

// ensureProvider ensures a provider entry exists in the map.
func ensureProvider(providers map[string]ProviderInfo, id, name string, env []string) {
	if _, exists := providers[id]; !exists {
		providers[id] = ProviderInfo{
			ID:     id,
			Env:    env,
			Name:   name,
			Models: make(map[string]ModelInfo),
		}
	}
}

// ValidateModel validates if a model exists and returns detailed information.
func (r *ModelsRegistry) ValidateModel(provider, modelID string) (*ModelInfo, error) {
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	modelInfo, exists := providerInfo.Models[modelID]
	if !exists {
		return nil, fmt.Errorf("model %s not found for provider %s", modelID, provider)
	}

	return &modelInfo, nil
}

// GetRequiredEnvVars returns the required environment variables for a provider.
func (r *ModelsRegistry) GetRequiredEnvVars(provider string) ([]string, error) {
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return providerInfo.Env, nil
}

// ValidateEnvironment checks if required environment variables are set.
func (r *ModelsRegistry) ValidateEnvironment(provider string, apiKey string) error {
	envVars, err := r.GetRequiredEnvVars(provider)
	if err != nil {
		return err
	}

	if apiKey != "" {
		return nil
	}

	// Add alternative environment variable names for google-vertex-anthropic
	if provider == "google-vertex-anthropic" {
		envVars = append(envVars,
			"ANTHROPIC_VERTEX_PROJECT_ID",
			"GOOGLE_CLOUD_PROJECT",
			"GCLOUD_PROJECT",
			"CLOUDSDK_CORE_PROJECT",
			"ANTHROPIC_VERTEX_REGION",
			"CLOUD_ML_REGION",
		)
	}

	var foundVar bool
	for _, envVar := range envVars {
		if os.Getenv(envVar) != "" {
			foundVar = true
			break
		}
	}

	if !foundVar {
		return fmt.Errorf("missing required environment variables for %s: %s (at least one required)",
			provider, strings.Join(envVars, ", "))
	}

	return nil
}

// SuggestModels returns similar model names when an invalid model is provided.
func (r *ModelsRegistry) SuggestModels(provider, invalidModel string) []string {
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil
	}

	var suggestions []string
	invalidLower := strings.ToLower(invalidModel)

	for modelID, modelInfo := range providerInfo.Models {
		modelIDLower := strings.ToLower(modelID)
		modelNameLower := strings.ToLower(modelInfo.Name)

		if strings.Contains(modelIDLower, invalidLower) ||
			strings.Contains(modelNameLower, invalidLower) ||
			strings.Contains(invalidLower, strings.ToLower(strings.Split(modelID, "-")[0])) {
			suggestions = append(suggestions, modelID)
		}
	}

	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}

	return suggestions
}

// GetSupportedProviders returns a list of all supported providers.
func (r *ModelsRegistry) GetSupportedProviders() []string {
	var providers []string
	for providerID := range r.providers {
		providers = append(providers, providerID)
	}
	return providers
}

// GetModelsForProvider returns all models for a specific provider.
func (r *ModelsRegistry) GetModelsForProvider(provider string) (map[string]ModelInfo, error) {
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return providerInfo.Models, nil
}

// Global registry instance
var globalRegistry = NewModelsRegistry()

// GetGlobalRegistry returns the global models registry instance.
func GetGlobalRegistry() *ModelsRegistry {
	return globalRegistry
}
