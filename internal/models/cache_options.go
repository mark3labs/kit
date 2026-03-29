package models

import (
	"crypto/sha256"
	"encoding/hex"
	"os"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/openai"
)

// buildCacheProviderOptions returns caching options for supported models.
// Caching is enabled by default for all supported models to reduce costs.
// Set KIT_DISABLE_CACHE=1 or ProviderConfig.DisableCaching=true to opt out.
func buildCacheProviderOptions(modelInfo *ModelInfo, config *ProviderConfig) fantasy.ProviderOptions {
	// Check explicit opt-out via config
	if config.DisableCaching {
		return nil
	}

	// Check global opt-out via environment
	if os.Getenv("KIT_DISABLE_CACHE") != "" {
		return nil
	}

	// Check if model supports caching
	if modelInfo == nil || !modelInfo.SupportsCaching() {
		return nil
	}

	switch modelInfo.CacheType() {
	case "anthropic-ephemeral":
		// NOTE: Anthropic caching is DISABLED due to Fantasy library limitation.
		// The library's prepareParams() strictly validates that ProviderOptions["anthropic"]
		// must be *anthropic.ProviderOptions, but cache control uses 
		// *anthropic.ProviderCacheControlOptions which fails this check.
		// TODO: Re-enable when Fantasy supports cache control options at the provider level.
		return nil
	case "openai-prompt-cache":
		return buildOpenAICacheOptions(config, modelInfo.ID)
	case "google-cached-content":
		// NOTE: Google caching disabled for now - requires pre-caching workflow
		// and may have similar type conflicts as Anthropic.
		return nil
	default:
		return nil
	}
}

// buildAnthropicCacheOptions enables ephemeral caching for Anthropic models.
// This reduces costs by 60-90% for repeated prompts with the same system context.
// NOTE: This only works when thinking is NOT enabled, due to type conflicts
// in the Fantasy library (both use different types under the same provider key).
func buildAnthropicCacheOptions() fantasy.ProviderOptions {
	return anthropic.NewProviderCacheControlOptions(&anthropic.ProviderCacheControlOptions{
		CacheControl: anthropic.CacheControl{
			Type: "ephemeral",
		},
	})
}

// buildOpenAICacheOptions enables prompt caching for OpenAI models.
// Uses a deterministic cache key based on system prompt and model ID.
func buildOpenAICacheOptions(config *ProviderConfig, modelID string) fantasy.ProviderOptions {
	cacheKey := generateCacheKey(config.SystemPrompt, modelID)

	return fantasy.ProviderOptions{
		openai.Name: &openai.ProviderOptions{
			PromptCacheKey: &cacheKey,
		},
	}
}

// generateCacheKey creates a deterministic cache key from system prompt and model.
// This ensures the same system prompt + model combination gets cache hits.
func generateCacheKey(systemPrompt, modelID string) string {
	if systemPrompt == "" {
		systemPrompt = "default"
	}

	h := sha256.New()
	h.Write([]byte(systemPrompt))
	h.Write([]byte(modelID))

	// Prefix with "kit-" to identify KIT-generated cache keys
	return "kit-" + hex.EncodeToString(h.Sum(nil))[:24]
}

// mergeProviderOptions merges multiple ProviderOptions maps.
// Later maps take precedence over earlier ones.
func mergeProviderOptions(opts ...fantasy.ProviderOptions) fantasy.ProviderOptions {
	result := make(fantasy.ProviderOptions)

	for _, opt := range opts {
		for k, v := range opt {
			result[k] = v
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}
