package kit

import (
	"fmt"

	"github.com/mark3labs/kit/internal/models"
)

// LookupModel returns information about a model, or nil if unknown.
func LookupModel(provider, modelID string) *ModelInfo {
	return models.GetGlobalRegistry().LookupModel(provider, modelID)
}

// GetSupportedProviders returns all known provider names in the registry.
func GetSupportedProviders() []string {
	return models.GetGlobalRegistry().GetSupportedProviders()
}

// GetFantasyProviders returns provider IDs that can be used with fantasy,
// either through a native provider or via openaicompat auto-routing.
func GetFantasyProviders() []string {
	return models.GetGlobalRegistry().GetFantasyProviders()
}

// GetModelsForProvider returns all known models for a provider.
func GetModelsForProvider(provider string) (map[string]ModelInfo, error) {
	return models.GetGlobalRegistry().GetModelsForProvider(provider)
}

// GetProviderInfo returns information about a provider (env vars, API URL, etc.).
// Returns nil if the provider is not in the registry.
func GetProviderInfo(provider string) *ProviderInfo {
	return models.GetGlobalRegistry().GetProviderInfo(provider)
}

// ValidateEnvironment checks if required API keys are set for a provider.
// Returns nil for providers not in the registry (unknown providers are
// assumed to handle auth themselves or via --provider-api-key).
func ValidateEnvironment(provider string, apiKey string) error {
	return models.GetGlobalRegistry().ValidateEnvironment(provider, apiKey)
}

// SuggestModels returns model names similar to an invalid model string.
func SuggestModels(provider, invalidModel string) []string {
	return models.GetGlobalRegistry().SuggestModels(provider, invalidModel)
}

// RefreshModelRegistry reloads the global model database from the current
// data sources (cache -> embedded). Call after updating the cache.
func RefreshModelRegistry() {
	models.ReloadGlobalRegistry()
}

// CheckProviderReady validates that a provider is properly configured
// by checking that it exists in the registry and has required environment
// variables set.
func CheckProviderReady(provider string) error {
	info := models.GetGlobalRegistry().GetProviderInfo(provider)
	if info == nil {
		return fmt.Errorf("unknown provider: %s", provider)
	}
	return models.GetGlobalRegistry().ValidateEnvironment(provider, "")
}
