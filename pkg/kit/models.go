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

// ThinkingLevels returns every reasoning level Kit understands, from least to
// most effort: "off", "none", "minimal", "low", "medium", "high".
//
// This is the full vocabulary, not the set any particular model accepts. Use
// [SupportedThinkingLevels] to narrow it to a model.
func ThinkingLevels() []string {
	levels := models.ThinkingLevels()
	out := make([]string, len(levels))
	for i, l := range levels {
		out[i] = string(l)
	}
	return out
}

// SupportedThinkingLevels returns the reasoning levels the given model
// accepts, drawn from the model catalog's published reasoning metadata.
//
// Providers differ: OpenAI's gpt-5.x line accepts "none" but not "minimal",
// the o-series accepts neither, and models with a token-budget or on/off
// reasoning control accept every level. When the catalog publishes no usable
// metadata — an unknown model, or one with no graded levels — every level is
// returned, since there is no evidence any of them would be rejected.
//
// "off" is always present: it means "do not request reasoning".
func SupportedThinkingLevels(provider, modelID string) []string {
	levels := models.SupportedThinkingLevels(provider, modelID)
	out := make([]string, len(levels))
	for i, l := range levels {
		out[i] = string(l)
	}
	return out
}

// IsThinkingLevelSupported reports whether a model accepts the named reasoning
// level. Unknown levels and unknown models report true, so callers are never
// blocked on missing catalog data.
func IsThinkingLevelSupported(provider, modelID, level string) bool {
	return models.IsValidThinkingLevelForModel(models.ParseThinkingLevel(level), provider, modelID)
}

// SuggestThinkingLevel returns the level to use in place of one the model does
// not accept, choosing the nearest supported level of similar cost. Returns
// the requested level unchanged when it is already supported, or "off" when
// the model accepts no reasoning level at all.
//
//	level := kit.SuggestThinkingLevel("openai", "o3", "minimal") // -> "low"
func SuggestThinkingLevel(provider, modelID, level string) string {
	return string(models.SuggestThinkingLevelFallback(models.ParseThinkingLevel(level), provider, modelID))
}
