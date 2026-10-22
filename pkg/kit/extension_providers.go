package kit

import (
	"os"
	"slices"
	"strings"

	"github.com/mark3labs/kit/internal/auth"
)

// LookupModel returns metadata in this Kit's catalog, including its extensions.
// The package-level LookupModel continues to query only the process catalog.
func (m *Kit) LookupModel(provider, modelID string) *ModelInfo {
	return m.resolver.Lookup(provider, modelID)
}

// GetModelsForProvider returns this Kit's declared models. Unknown-model lookup
// never inserts an undeclared name in this list.
func (m *Kit) GetModelsForProvider(provider string) (map[string]ModelInfo, error) {
	if m.resolver.Owns(provider) {
		return m.resolver.List()[strings.ToLower(provider)], nil
	}
	return GetModelsForProvider(provider)
}

// GetLLMProviders lists providers in this Kit's catalog.
func (m *Kit) GetLLMProviders() []string {
	result := GetLLMProviders()
	for provider := range m.resolver.List() {
		result = append(result, provider)
	}
	slices.Sort(result)
	return slices.Compact(result)
}

// GetModelCapabilities resolves capabilities in the owning Kit catalog.
func (m *Kit) GetModelCapabilities(model string) (ModelCapabilities, string) {
	if model == "" {
		model = m.GetCurrentModel()
	}
	provider, id, err := ParseModelString(model)
	if err != nil {
		return ModelCapabilities{}, err.Error()
	}
	info := m.LookupModel(provider, id)
	if info == nil {
		return ModelCapabilities{}, "model not found in instance catalog"
	}
	return ModelCapabilities{Provider: provider, ModelID: id, ContextLimit: info.Limit.Context, OutputLimit: info.Limit.Output, Reasoning: info.Reasoning, Streaming: true, Pricing: modelPricingFrom(info)}, ""
}

// CheckModelAvailable checks identity and metadata in this instance.
func (m *Kit) CheckModelAvailable(model string) bool {
	provider, _, err := ParseModelString(model)
	if err != nil {
		return false
	}
	if m.resolver.Owns(provider) {
		if m.resolver.Validate(model) != nil {
			return false
		}
		if m.extRunner != nil {
			for _, owner := range m.extRunner.Extensions() {
				for _, def := range owner.Providers {
					if def.ID == provider {
						configured := false
						if m.endpointOverridesApply(model) && m.v.GetString("provider-api-key") != "" {
							configured = true
						}
						if !configured {
							if manager, err := auth.NewCredentialManager(); err == nil {
								key, _ := manager.GetProviderAPIKey(provider)
								configured = key != ""
							}
						}
						if !configured {
							for _, variable := range def.Auth.Environment {
								if os.Getenv(variable) != "" {
									configured = true
									break
								}
							}
						}
						return def.AuthStatus(configured).Status == "configured"
					}
				}
			}
		}
		return false
	}
	return CheckModelAvailable(model)
}

// ResolveModelChain selects the first valid model in the instance catalog.
func (m *Kit) ResolveModelChain(preferences []string) ModelResolutionResult {
	result := ModelResolutionResult{}
	for _, model := range preferences {
		result.Attempted = append(result.Attempted, model)
		if !m.CheckModelAvailable(model) {
			continue
		}
		caps, err := m.GetModelCapabilities(model)
		if err != "" {
			continue
		}
		result.Model = model
		result.Capabilities = caps
		return result
	}
	result.Error = "no models in chain are available"
	return result
}
