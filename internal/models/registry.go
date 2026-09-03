package models

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/mark3labs/kit/internal/auth"
)

//go:embed embedded_models.json
var embeddedModelsJSON []byte

// ModelInfo represents information about a specific model.
type ModelInfo struct {
	ID           string
	Name         string
	Family       string // Model family (e.g., "claude", "gpt", "gemini")
	Attachment   bool
	Reasoning    bool
	Temperature  bool
	Cost         Cost
	Limit        Limit
	ProviderNPM  string // Model-specific provider npm override (e.g. "@ai-sdk/anthropic")
	BaseURL      string // Per-model base URL override (custom models only)
	APIKey       string // Per-model API key override (custom models only)
	APIModelName string // Per-model API model name override (custom models only)

	// Params holds per-model generation parameter defaults. These are applied
	// when the user hasn't explicitly set the corresponding CLI flag or global
	// config value. Nil pointer fields mean "no model-level default".
	Params *GenerationParams

	// Status is the model's lifecycle marker from the catalog: StatusDeprecated,
	// StatusBeta, or empty for normal general availability.
	Status string

	// ReasoningLevels lists the reasoning effort levels the model accepts, as
	// published by the catalog (e.g. "none", "low", "medium", "high", "max").
	//
	// Empty carries two distinct meanings, separated by ReasoningIsGraded:
	// either the catalog published no reasoning metadata at all, or the model
	// takes a reasoning budget that has no named levels (an on/off toggle or a
	// raw token budget). Callers should consult SupportsReasoningLevel rather
	// than testing this slice directly.
	ReasoningLevels []string

	// ReasoningIsGraded is true when the catalog says the model selects
	// reasoning by named effort level, meaning ReasoningLevels is an
	// exhaustive list. False means either no metadata, or a toggle/budget
	// model that accepts any level Kit maps onto a token budget.
	ReasoningIsGraded bool
}

// Model lifecycle markers published by the catalog.
const (
	// StatusDeprecated marks a model the provider has scheduled for removal.
	// It usually still answers requests, so this is advisory, not a block.
	StatusDeprecated = "deprecated"
	// StatusBeta marks a model the provider considers pre-release.
	StatusBeta = "beta"
)

// IsDeprecated reports whether the catalog marks this model as deprecated.
func (m *ModelInfo) IsDeprecated() bool { return m.Status == StatusDeprecated }

// SupportsReasoningLevel reports whether the model accepts the named reasoning
// effort level.
//
// Returns true when the catalog published no usable reasoning metadata, so an
// unknown model is never blocked on the strength of missing data. Callers that
// need to distinguish "known good" from "unknown" should check
// ReasoningIsGraded.
func (m *ModelInfo) SupportsReasoningLevel(level string) bool {
	if !m.ReasoningIsGraded {
		return true
	}
	return slices.Contains(m.ReasoningLevels, level)
}

// SupportsCaching returns true if this model family supports prompt caching.
// This enables automatic cost savings for supported models regardless of provider.
func (m *ModelInfo) SupportsCaching() bool {
	switch {
	case strings.HasPrefix(m.Family, "claude"):
		return true
	case strings.HasPrefix(m.Family, "gpt"),
		strings.HasPrefix(m.Family, "o1"),
		strings.HasPrefix(m.Family, "o3"),
		strings.HasPrefix(m.Family, "o4"),
		strings.HasPrefix(m.Family, "codex"):
		return true
	case strings.HasPrefix(m.Family, "gemini"):
		return true
	default:
		return false
	}
}

// CacheType returns the appropriate cache mechanism for this model family.
// Returns empty string if caching is not supported.
func (m *ModelInfo) CacheType() string {
	switch {
	case strings.HasPrefix(m.Family, "claude"):
		return "anthropic-ephemeral"
	case strings.HasPrefix(m.Family, "gpt"),
		strings.HasPrefix(m.Family, "o1"),
		strings.HasPrefix(m.Family, "o3"),
		strings.HasPrefix(m.Family, "o4"),
		strings.HasPrefix(m.Family, "codex"):
		return "openai-prompt-cache"
	case strings.HasPrefix(m.Family, "gemini"):
		return "google-cached-content"
	default:
		return ""
	}
}

// Cost represents the pricing information for a model. All rates are in US
// dollars per one million tokens.
type Cost struct {
	Input      float64
	Output     float64
	CacheRead  *float64
	CacheWrite *float64

	// Tiers holds long-context pricing, ordered by ascending threshold. Many
	// long-context models bill roughly 2x the base rate once a request's
	// prompt passes a threshold (commonly 200k tokens), so charging the base
	// rate for every request under-reports the cost of large contexts. Empty
	// when the model prices every request at the base rate.
	Tiers []CostTier

	// Published is true when the source catalog supplied a pricing block for
	// the model. It distinguishes a model that is genuinely free (an explicit
	// zero rate, as with openrouter's ":free" variants) from one whose price
	// is simply unknown because the catalog omitted it — both otherwise look
	// identical, since the zero value of Input/Output is 0.
	//
	// Callers that display cost should check this before rendering a figure;
	// reporting "$0.00" for an unpriced model is worse than reporting nothing.
	Published bool
}

// CostTier is a set of rates that applies once a request's prompt exceeds
// Threshold tokens.
type CostTier struct {
	Threshold  int
	Input      float64
	Output     float64
	CacheRead  *float64
	CacheWrite *float64
}

// RatesFor returns the rates that apply to a request whose prompt is
// promptTokens long. The prompt is the right input for tier selection: it is
// the whole context sent to the model, so callers must include cached tokens
// as well as fresh ones.
//
// Returns the base rates when the model publishes no tiers or the prompt sits
// below every threshold. When several tiers match, the highest one wins.
func (c Cost) RatesFor(promptTokens int) Cost {
	out := Cost{
		Input:      c.Input,
		Output:     c.Output,
		CacheRead:  c.CacheRead,
		CacheWrite: c.CacheWrite,
		Published:  c.Published,
	}
	for _, t := range c.Tiers {
		if promptTokens > t.Threshold {
			out.Input = t.Input
			out.Output = t.Output
			// A tier that omits a cache rate keeps the base one rather than
			// silently dropping to "uncharged".
			if t.CacheRead != nil {
				out.CacheRead = t.CacheRead
			}
			if t.CacheWrite != nil {
				out.CacheWrite = t.CacheWrite
			}
		}
	}
	return out
}

// contextOver200KThreshold is the prompt size at which the catalog's
// `context_over_200k` rate takes over from the base rate.
const contextOver200KThreshold = 200_000

// costFrom converts a catalog pricing block into a Cost. A nil block means the
// catalog published no pricing for the model (~400 entries in the bundled
// catalog, including paid models proxied by aggregators), which is recorded as
// Published=false rather than as a zero rate.
func costFrom(c *modelsDBCost) Cost {
	if c == nil {
		return Cost{}
	}
	out := Cost{
		Input:      c.Input,
		Output:     c.Output,
		CacheRead:  c.CacheRead,
		CacheWrite: c.CacheWrite,
		Published:  true,
	}

	// `context_over_200k` is the common shorthand for a single 200k tier.
	if r := c.ContextOver200K; r != nil {
		out.Tiers = append(out.Tiers, CostTier{
			Threshold:  contextOver200KThreshold,
			Input:      r.Input,
			Output:     r.Output,
			CacheRead:  r.CacheRead,
			CacheWrite: r.CacheWrite,
		})
	}

	// `tiers` is the general form. Only context-sized tiers are understood;
	// any other tier type is ignored rather than guessed at.
	for _, t := range c.Tiers {
		if t.Tier.Type != "context" || t.Tier.Size <= 0 {
			continue
		}
		out.Tiers = append(out.Tiers, CostTier{
			Threshold:  t.Tier.Size,
			Input:      t.Input,
			Output:     t.Output,
			CacheRead:  t.CacheRead,
			CacheWrite: t.CacheWrite,
		})
	}

	// Ascending threshold order lets RatesFor apply the highest match last.
	slices.SortFunc(out.Tiers, func(a, b CostTier) int { return a.Threshold - b.Threshold })
	return out
}

// Limit represents the context and output limits for a model.
type Limit struct {
	Context int
	Output  int
}

// ProviderInfo represents information about a model provider.
type ProviderInfo struct {
	ID   string
	Env  []string
	NPM  string // npm package identifier from models.dev (e.g. "@ai-sdk/openai-compatible")
	API  string // base API URL for openai-compatible providers
	Name string
	// Wire is an explicit wire protocol declaration ("openai",
	// "openai-compat", "anthropic", "google"). When set it takes precedence
	// over the npm-package heuristic in auto-routing. Empty means "infer".
	// Populated from the `providers` config section; not present in the
	// models.dev data itself.
	Wire string
	// Headers are default HTTP headers added to every request to this
	// provider (auto-routed wires only). Populated from the `providers`
	// config section.
	Headers map[string]string
	Models  map[string]ModelInfo
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

// reasoningFrom flattens the catalog's reasoning_options into the levels a
// model accepts, and whether that list is exhaustive.
//
// graded is true only when the catalog publishes an "effort" option carrying
// named values. A "toggle" or "budget_tokens" model takes a reasoning budget
// with no named levels, so every level Kit offers maps onto it; that is
// reported as not-graded with no levels, which SupportsReasoningLevel treats
// as permissive.
func reasoningFrom(opts []modelsDBReasoningOption) (levels []string, graded bool) {
	for _, o := range opts {
		switch o.Type {
		case reasoningTypeEffort:
			for _, v := range o.Values {
				if v != "" && !slices.Contains(levels, v) {
					levels = append(levels, v)
				}
			}
		case reasoningTypeToggle, reasoningTypeBudgetTokens:
			// No named levels to collect.
		}
	}
	// An effort option with an empty value list says nothing useful, so it is
	// not treated as an exhaustive list.
	return levels, len(levels) > 0
}

// buildFromModelsDB converts models.dev provider data into our internal format.
// It starts from the compile-time embedded database and merges on-disk cached
// data from `kit update-models` on top. Cached provider metadata replaces
// embedded metadata, and model entries are merged with cached models taking
// precedence. This means newly synced models are available while embedded
// models that haven't been synced yet are still reachable.
func buildFromModelsDB() map[string]ProviderInfo {
	// Start with compile-time embedded data as the base.
	dbProviders := loadEmbeddedProviders()
	if dbProviders == nil {
		dbProviders = make(ModelsDBProviders)
	}

	// Merge on-disk cached data on top (cached takes precedence).
	if cached, _ := LoadCachedProviders(); len(cached) > 0 {
		for providerID, cp := range cached {
			if existing, ok := dbProviders[providerID]; ok {
				// Merge models: embedded base + cached overrides.
				mergedModels := make(map[string]modelsDBModel, len(existing.Models)+len(cp.Models))
				maps.Copy(mergedModels, existing.Models)
				maps.Copy(mergedModels, cp.Models)
				cp.Models = mergedModels
			}
			dbProviders[providerID] = cp
		}
	}

	providers := make(map[string]ProviderInfo, len(dbProviders))

	for providerID, dp := range dbProviders {
		modelsMap := make(map[string]ModelInfo, len(dp.Models))
		for modelID, dm := range dp.Models {
			providerNPM := ""
			if dm.Provider != nil {
				providerNPM = dm.Provider.NPM
			}
			reasoningLevels, reasoningGraded := reasoningFrom(dm.ReasoningOptions)
			modelsMap[modelID] = ModelInfo{
				ID:          dm.ID,
				Name:        dm.Name,
				Family:      dm.Family,
				Attachment:  dm.Attachment,
				Reasoning:   dm.Reasoning,
				Temperature: dm.Temperature,
				Cost:        costFrom(dm.Cost),
				Limit: Limit{
					Context: dm.Limit.Context,
					Output:  dm.Limit.Output,
				},
				ProviderNPM:       providerNPM,
				Status:            dm.Status,
				ReasoningLevels:   reasoningLevels,
				ReasoningIsGraded: reasoningGraded,
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

	// Register the "custom" provider stub for --provider-url without --model.
	// This allows users to point kit at any OpenAI-compatible endpoint without
	// needing to specify a model from the database.
	providers["custom"] = ProviderInfo{
		ID:   "custom",
		Name: "Custom",
		Models: map[string]ModelInfo{
			"custom": {
				ID:          "custom",
				Name:        "Custom",
				Attachment:  false,
				Reasoning:   true,
				Temperature: true,
				// A placeholder for user-configured endpoints; its real
				// price is whatever the user's provider charges, so it is
				// unpriced rather than free.
				Cost: Cost{},
				Limit: Limit{
					Context: 262_144,
					Output:  65_536,
				},
			},
		},
	}

	// Load custom models from config file and merge into custom provider.
	// Config file models take precedence - if a model ID exists in both
	// models.dev and config, the config version wins.
	if customModels := loadCustomModelsFromConfig(); customModels != nil {
		for modelID, info := range customModels {
			// Validate custom model config
			if info.Limit.Context <= 0 {
				fmt.Fprintf(os.Stderr, "Warning: custom model %q has invalid context limit: %d\n", modelID, info.Limit.Context)
			}
			if info.Limit.Output <= 0 {
				fmt.Fprintf(os.Stderr, "Warning: custom model %q has invalid output limit: %d\n", modelID, info.Limit.Output)
			}
			providers["custom"].Models[modelID] = info
		}
	}

	// Apply provider overrides from the config `providers` section as the
	// final layer: explicit wire/URL/env/header declarations win over
	// everything derived from the models.dev data, and unknown provider IDs
	// are registered fresh (advisory model lookups tolerate empty model maps).
	applyProviderOverrides(providers, loadProviderOverridesFromConfig())

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
	provider = catalogProviderID(provider)
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

// LookupModelForSettings is a convenience function that parses a
// "provider/model" string and looks up the ModelInfo in the global registry.
// Returns nil when the model string is invalid or the model is unknown.
// Used by Kit.SetModel to pre-apply per-model settings before CreateProvider.
func LookupModelForSettings(modelString string) *ModelInfo {
	provider, modelName, err := ParseModelString(modelString)
	if err != nil {
		return nil
	}
	return GetGlobalRegistry().LookupModel(provider, modelName)
}

// getRequiredEnvVars returns the required environment variables for a provider.
func (r *ModelsRegistry) getRequiredEnvVars(provider string) ([]string, error) {
	provider = catalogProviderID(provider)
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return providerInfo.Env, nil
}

// ValidateEnvironment checks if required credentials are available for a
// provider. It checks the explicit API key, stored credentials (for
// providers that support them, such as Anthropic OAuth), and environment
// variables. Returns nil for providers not in the registry (unknown
// providers are assumed to handle auth themselves or via --provider-api-key).
func (r *ModelsRegistry) ValidateEnvironment(provider string, apiKey string) error {
	provider = catalogProviderID(provider)
	if apiKey != "" {
		return nil
	}

	// For anthropic, also check stored credentials (OAuth / API key)
	// since auth resolution goes through the credential manager, not
	// just environment variables.
	if provider == "anthropic" {
		if cm, err := auth.NewCredentialManager(); err == nil {
			if has, _ := cm.HasAnthropicCredentials(); has {
				return nil
			}
		}
	}

	// For openai, check stored credentials (OAuth / API key)
	if provider == "openai" {
		if cm, err := auth.NewCredentialManager(); err == nil {
			if has, _ := cm.HasOpenAICredentials(); has {
				return nil
			}
		}
	}

	// For GitHub Copilot, check stored GitHub OAuth credentials.
	if provider == copilotProviderID {
		if cm, err := auth.NewCredentialManager(); err == nil {
			if has, _ := cm.HasCopilotCredentials(); has {
				return nil
			}
		}
	}

	envVars, err := r.getRequiredEnvVars(provider)
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
	provider = catalogProviderID(provider)
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

	// Order the matches before truncating. Models the catalog marks
	// deprecated sort last, so a short list is not spent recommending models
	// the provider is retiring. Ties break by name to keep the output stable:
	// the scan above walks a map, so without this the five surfaced
	// suggestions would vary between runs.
	slices.SortFunc(suggestions, func(a, b string) int {
		infoA, infoB := providerInfo.Models[a], providerInfo.Models[b]
		depA, depB := infoA.IsDeprecated(), infoB.IsDeprecated()
		if depA != depB {
			if depA {
				return 1
			}
			return -1
		}
		return strings.Compare(a, b)
	})

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

// GetLLMProviders returns provider IDs that have LLM support,
// either through a native provider or via openaicompat auto-routing.
func (r *ModelsRegistry) GetLLMProviders() []string {
	var providers []string
	for providerID, info := range r.providers {
		if isProviderLLMSupported(providerID, &info) {
			providers = append(providers, providerID)
		}
	}
	return providers
}

// isProviderLLMSupported checks if a provider can be used with the LLM layer.
func isProviderLLMSupported(providerID string, info *ProviderInfo) bool {
	// Ollama and custom are always supported (model names are user-defined).
	if providerID == "ollama" || providerID == "custom" {
		return true
	}

	// Explicit wire declaration (from a provider override) is authoritative.
	if _, ok := parseWire(info.Wire); ok {
		return true
	}

	// Check if npm maps to a known wire protocol
	if _, ok := npmToWireProtocol[info.NPM]; ok {
		return true
	}

	// Any provider with an API URL can be auto-routed through openaicompat
	return info.API != ""
}

// GetModelsForProvider returns all models for a specific provider.
func (r *ModelsRegistry) GetModelsForProvider(provider string) (map[string]ModelInfo, error) {
	provider = catalogProviderID(provider)
	providerInfo, exists := r.providers[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return providerInfo.Models, nil
}

// GetProviderInfo returns the full provider info, or nil if not found.
func (r *ModelsRegistry) GetProviderInfo(provider string) *ProviderInfo {
	provider = catalogProviderID(provider)
	info, exists := r.providers[provider]
	if !exists {
		return nil
	}
	return &info
}

// ValidateModelString checks whether a model string is well-formed and refers
// to a known provider. It returns a user-friendly error with suggestions when
// the model or provider is unrecognised. Passing validation does not guarantee
// that API authentication will succeed — it only catches obvious mistakes
// (typos, missing provider prefix, non-existent provider names) early so that
// callers such as subagent spawning can return fast feedback.
//
// Unknown models under a known provider are allowed (the provider API is the
// authority), but a completely unknown provider is rejected.
func (r *ModelsRegistry) ValidateModelString(modelString string) error {
	provider, modelName, err := ParseModelString(modelString)
	if err != nil {
		return err
	}

	// Ollama and custom are always valid — model names are user-defined.
	if provider == "ollama" || provider == "custom" {
		return nil
	}

	// Check if the provider exists in the registry.
	providerInfo := r.GetProviderInfo(provider)
	if providerInfo == nil {
		known := r.GetSupportedProviders()
		return fmt.Errorf(
			"unknown provider %q in model string %q. Known providers: %s",
			provider, modelString, strings.Join(known, ", "),
		)
	}

	// Provider exists — check if the model is known. An unknown model is
	// only a warning (the provider API decides), but we surface suggestions
	// so the caller can self-correct.
	if r.LookupModel(provider, modelName) == nil {
		if suggestions := r.SuggestModels(provider, modelName); len(suggestions) > 0 {
			return fmt.Errorf(
				"model %q not found for provider %s. Did you mean one of: %s",
				modelName, provider, strings.Join(suggestions, ", "),
			)
		}
		// No suggestions — let it through; the provider API is the authority.
	}

	return nil
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
