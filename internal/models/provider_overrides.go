package models

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// ProviderOverrideConfig declares or patches a provider in the model registry
// via the `providers` config section. All fields are optional; non-empty
// fields override the corresponding registry values (which come from the
// models.dev database). A provider ID that is not in the database is
// registered fresh, so users can declare entirely new providers (e.g.
// internal LLM gateways) without touching the database:
//
//	providers:
//	  # Patch the wire protocol of a known provider
//	  minimax:
//	    wire: anthropic
//
//	  # Declare a brand-new provider
//	  corp-llm:
//	    name: "Corp LLM Gateway"
//	    wire: anthropic
//	    baseUrl: https://llm.internal.corp/api
//	    apiKeyEnv: [CORP_LLM_KEY, LLM_GATEWAY_KEY]
//	    headers:
//	      X-Team: platform
type ProviderOverrideConfig struct {
	// Name is the human-readable provider name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Wire is the wire protocol this provider speaks: "openai" (Responses
	// API), "openai-compat" (chat completions), "anthropic", or "google".
	Wire string `json:"wire,omitempty" yaml:"wire,omitempty"`
	// BaseURL is the provider's API base URL.
	BaseURL string `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	// APIKeyEnv lists environment variable names to try (in order) when
	// resolving the API key.
	APIKeyEnv []string `json:"apiKeyEnv,omitempty" yaml:"apiKeyEnv,omitempty"`
	// Headers are default HTTP headers added to every request.
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// loadProviderOverridesFromConfig loads the `providers` config section from
// the process-global viper store (the model registry is a process-global
// singleton, mirroring loadCustomModelsFromConfig).
func loadProviderOverridesFromConfig() map[string]ProviderOverrideConfig {
	return loadProviderOverridesFrom(viper.GetViper())
}

// loadProviderOverridesFrom loads provider overrides from the supplied store.
// When v is nil the process-global store is used. Returns nil when the
// `providers` key is absent or malformed.
func loadProviderOverridesFrom(v *viper.Viper) map[string]ProviderOverrideConfig {
	if v == nil {
		v = viper.GetViper()
	}
	if !v.IsSet("providers") {
		return nil
	}

	var overrides map[string]ProviderOverrideConfig
	if err := v.UnmarshalKey("providers", &overrides); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to parse providers config: %v\n", err)
		return nil
	}
	return overrides
}

// applyProviderOverrides merges provider overrides into the registry provider
// map. Non-empty override fields replace the registry values; unset fields
// inherit whatever the database (or special-case injection) provided. IDs not
// already present are registered as new providers with an empty model map,
// like ollama — model lookups are advisory for auto-routed providers.
func applyProviderOverrides(providers map[string]ProviderInfo, overrides map[string]ProviderOverrideConfig) {
	for id, o := range overrides {
		if o.Wire != "" {
			if _, ok := parseWire(o.Wire); !ok {
				fmt.Fprintf(os.Stderr, "Warning: provider %q has unknown wire %q (expected one of: %s); ignoring override\n",
					id, o.Wire, wireNames())
				continue
			}
		}

		info, exists := providers[id]
		if !exists {
			info = ProviderInfo{
				ID:     id,
				Name:   id,
				Models: make(map[string]ModelInfo),
			}
		}

		if o.Name != "" {
			info.Name = o.Name
		}
		if o.Wire != "" {
			info.Wire = o.Wire
		}
		if o.BaseURL != "" {
			info.API = o.BaseURL
		}
		if len(o.APIKeyEnv) > 0 {
			info.Env = o.APIKeyEnv
		}
		if len(o.Headers) > 0 {
			info.Headers = o.Headers
		}

		providers[id] = info
	}
}
