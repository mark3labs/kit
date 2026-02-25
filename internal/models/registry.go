package models

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//go:embed embedded_models.json
var embeddedModelsJSON []byte

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
	NPM    string // npm package identifier from models.dev (e.g. "@ai-sdk/openai-compatible")
	API    string // base API URL for openai-compatible providers
	Name   string
	Models map[string]ModelInfo
}

// ModelsRegistry provides validation and information about models.
// It maintains a registry of all supported LLM providers and their models,
// including capabilities, pricing, and configuration requirements.
// The registry data comes from models.dev.
type ModelsRegistry struct {
	providers map[string]ProviderInfo
}

// NewModelsRegistry creates a new models registry populated from models.dev data.
func NewModelsRegistry() *ModelsRegistry {
	return &ModelsRegistry{
		providers: buildFromModelsDB(),
	}
}

// buildFromModelsDB converts models.dev provider data into our internal format.
// It tries the on-disk cache first and falls back to the embedded database.
func buildFromModelsDB() map[string]ProviderInfo {
	// Try cached data first (from `mcphost update-models`)
	dbProviders, _ := LoadCachedProviders()
	if len(dbProviders) == 0 {
		// Fall back to compile-time embedded data
		dbProviders = loadEmbeddedProviders()
	}

	providers := make(map[string]ProviderInfo, len(dbProviders))

	for providerID, dp := range dbProviders {
		modelsMap := make(map[string]ModelInfo, len(dp.Models))
		for modelID, dm := range dp.Models {
			modelsMap[modelID] = ModelInfo{
				ID:          dm.ID,
				Name:        dm.Name,
				Attachment:  dm.Attachment,
				Reasoning:   dm.Reasoning,
				Temperature: dm.Temperature,
				Cost: Cost{
					Input:      dm.Cost.Input,
					Output:     dm.Cost.Output,
					CacheRead:  dm.Cost.CacheRead,
					CacheWrite: dm.Cost.CacheWrite,
				},
				Limit: Limit{
					Context: dm.Limit.Context,
					Output:  dm.Limit.Output,
				},
			}
		}

		providers[providerID] = ProviderInfo{
			ID:     providerID,
			Env:    dp.Env,
			NPM:    dp.NPM,
			API:    dp.API,
			Name:   dp.Name,
			Models: modelsMap,
		}
	}

	// Ensure ollama is always present (not in models.dev — it's a local server)
	if _, exists := providers["ollama"]; !exists {
		providers["ollama"] = ProviderInfo{
			ID:     "ollama",
			Name:   "Ollama",
			Models: make(map[string]ModelInfo),
		}
	}

	return providers
}

// loadEmbeddedProviders parses the compile-time embedded models.dev snapshot.
func loadEmbeddedProviders() map[string]modelsDBProvider {
	var providers map[string]modelsDBProvider
	if err := json.Unmarshal(embeddedModelsJSON, &providers); err != nil {
		return nil
	}
	return providers
}

// LookupModel returns model metadata from the database if available.
// Returns nil when the model or provider is not in the database — this is
// expected for new models, custom fine-tunes, or providers the database
// doesn't track yet. Callers should treat a nil return as "unknown model"
// and continue with sensible defaults.
func (r *ModelsRegistry) LookupModel(provider, modelID string) *ModelInfo {
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil
	}

	modelInfo, exists := providerInfo.Models[modelID]
	if !exists {
		return nil
	}

	return &modelInfo
}

// ValidateModel validates if a model exists and returns detailed information.
// Deprecated: Use LookupModel instead — it returns nil for unknown models
// rather than an error, letting the provider API be the authority.
func (r *ModelsRegistry) ValidateModel(provider, modelID string) (*ModelInfo, error) {
	if info := r.LookupModel(provider, modelID); info != nil {
		return info, nil
	}

	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return nil, fmt.Errorf("model %s not found for provider %s", modelID, providerInfo.ID)
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
// Returns nil for providers not in the registry (unknown providers are
// assumed to handle auth themselves or via --provider-api-key).
func (r *ModelsRegistry) ValidateEnvironment(provider string, apiKey string) error {
	if apiKey != "" {
		return nil
	}

	envVars, err := r.GetRequiredEnvVars(provider)
	if err != nil {
		// Unknown provider — nothing to validate
		return nil
	}

	if len(envVars) == 0 {
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

	// Add GOOGLE_API_KEY as an alternative for google
	if provider == "google" || provider == "gemini" {
		envVars = append(envVars, "GOOGLE_API_KEY")
	}

	for _, envVar := range envVars {
		if os.Getenv(envVar) != "" {
			return nil
		}
	}

	return fmt.Errorf("missing required environment variables for %s: %s (at least one required)",
		provider, strings.Join(envVars, ", "))
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

// GetSupportedProviders returns a list of all provider IDs in the registry.
func (r *ModelsRegistry) GetSupportedProviders() []string {
	providers := make([]string, 0, len(r.providers))
	for providerID := range r.providers {
		providers = append(providers, providerID)
	}
	return providers
}

// GetFantasyProviders returns provider IDs that can be used with fantasy,
// either through a native provider or via openaicompat auto-routing.
func (r *ModelsRegistry) GetFantasyProviders() []string {
	var providers []string
	for providerID, info := range r.providers {
		if isProviderFantasySupported(providerID, &info) {
			providers = append(providers, providerID)
		}
	}
	return providers
}

// isProviderFantasySupported checks if a provider can be used with fantasy.
func isProviderFantasySupported(providerID string, info *ProviderInfo) bool {
	// Ollama is always supported (via openaicompat pointed at localhost)
	if providerID == "ollama" {
		return true
	}

	// Check if npm maps to a fantasy provider
	if _, ok := npmToFantasyProvider[info.NPM]; ok {
		return true
	}

	// Any provider with an API URL can be auto-routed through openaicompat
	return info.API != ""
}

// GetModelsForProvider returns all models for a specific provider.
func (r *ModelsRegistry) GetModelsForProvider(provider string) (map[string]ModelInfo, error) {
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return providerInfo.Models, nil
}

// GetProviderInfo returns the full provider info, or nil if not found.
func (r *ModelsRegistry) GetProviderInfo(provider string) *ProviderInfo {
	info, exists := r.providers[provider]
	if !exists {
		return nil
	}
	return &info
}

// Global registry instance
var globalRegistry = NewModelsRegistry()

// GetGlobalRegistry returns the global models registry instance.
func GetGlobalRegistry() *ModelsRegistry {
	return globalRegistry
}

// ReloadGlobalRegistry rebuilds the global registry from the current
// data sources (cache → embedded). Call after updating the cache.
func ReloadGlobalRegistry() {
	globalRegistry = NewModelsRegistry()
}
