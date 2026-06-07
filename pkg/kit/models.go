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

// GetLLMProviders returns provider IDs that have LLM support,
// either through a native provider or via openaicompat auto-routing.
func GetLLMProviders() []string {
	return models.GetGlobalRegistry().GetLLMProviders()
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

// ResolveProviderBaseURL returns the base API URL kit will use when talking to
// the given provider, applying the same resolution order that CreateProvider
// uses internally:
//
//  1. The provider's `api` field from the models.dev registry.
//  2. The hard-coded default base URL of its npm SDK package (e.g.
//     @ai-sdk/groq → https://api.groq.com/openai/v1).
//  3. Template substitution against the current process environment when the
//     URL contains "${VAR}" placeholders.
//
// Returns a non-nil error when the provider is unknown, when no URL can be
// derived, or when a templated URL has unset placeholders.
//
// Use this from your SDK integration to surface the effective endpoint before
// instantiating a Kit, or to validate that a provider is reachable without
// running an actual request.
func ResolveProviderBaseURL(providerID string) (string, error) {
	return models.ResolveProviderBaseURL(providerID)
}
