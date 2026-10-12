package models

import (
	"fmt"
	"strings"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/openai"
)

// ModeUnsupportedReason returns an explanation when a model mode cannot be applied.
// Pass model metadata when available. An empty result means the mode is supported.
func ModeUnsupportedReason(modelString, modeName string, config ...*ModelInfo) string {
	var info *ModelInfo
	if len(config) > 0 {
		info = config[0]
	}
	if info == nil {
		return fmt.Sprintf("model mode %q is unsupported: model metadata is unavailable", modeName)
	}
	mode, ok := info.Mode(modeName)
	if !ok {
		return fmt.Sprintf("model %q does not support mode %q", modelString, modeName)
	}
	if len(mode.Headers) > 0 {
		return fmt.Sprintf("model mode %q requires request headers that this provider adapter cannot apply", modeName)
	}
	if len(mode.Options) == 0 {
		return ""
	}
	provider, model, _ := ParseModelString(modelString)
	if provider == "openai" && len(mode.Options) == 1 && openai.IsResponsesModel(model) {
		tier, _ := mode.Options["service_tier"].(string)
		if tier == "ultrafast" || tier == "auto" {
			return ""
		}
		lower := strings.ToLower(model)
		// Match the pinned provider's priority guard. Otherwise it clears the setting.
		priority := strings.Contains(lower, "gpt-4") || strings.Contains(lower, "gpt-5-mini") ||
			(strings.Contains(lower, "gpt-5") && !strings.Contains(lower, "gpt-5-nano") && !strings.Contains(lower, "gpt-5-chat")) ||
			strings.HasPrefix(model, "o3") || strings.Contains(model, "-o3") || strings.Contains(model, "o4-mini")
		if tier == "priority" && priority {
			return ""
		}
	}
	if provider == "anthropic" {
		return ""
	}
	return fmt.Sprintf("model mode %q requires request options that this provider adapter cannot safely apply for provider %q", modeName, provider)
}

func applyOpenAIModeOptions(result *ProviderResult, model ModelMode) error {
	value, ok := model.Options["service_tier"].(string)
	if len(model.Options) != 1 || !ok {
		return fmt.Errorf("unsupported OpenAI mode request options")
	}
	tier := openai.ServiceTier(value)
	var opts *openai.ResponsesProviderOptions
	if existing, ok := result.ProviderOptions[openai.Name].(*openai.ResponsesProviderOptions); ok {
		copy := *existing
		opts = &copy
	} else {
		opts = &openai.ResponsesProviderOptions{}
	}
	opts.ServiceTier = &tier
	if result.ProviderOptions == nil {
		result.ProviderOptions = fantasy.ProviderOptions{}
	}
	result.ProviderOptions[openai.Name] = opts
	return nil
}
