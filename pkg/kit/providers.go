package kit

import (
	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/models"
)

// LLMProvider is a source of language models. Backends that implement it
// can be adapted into a [ProviderFactory].
type LLMProvider = fantasy.Provider

// LLMLanguageModel is a language model that Kit drives. A [ProviderFactory]
// returns one in [ProviderResult.Model].
type LLMLanguageModel = fantasy.LanguageModel

// ProviderFactory creates a language model for a provider name that the
// application supplies. It lets an application bundle its own inference
// backend (for example an in-process local runtime), a test double, or a
// proxy, and use it through normal "provider/model" strings.
//
// modelName is the part of the model string after the first "/". cfg is the
// resolved provider configuration: generation parameters, max tokens, system
// prompt and per-model settings. For [Kit.ExecuteCompletion], MaxTokens is the
// request's value. cfg also carries the Kit's ProviderAPIKey,
// ProviderURL and ProviderWire overrides when they belong to this provider;
// overrides configured for a different provider are left empty. The factory
// must not mutate cfg.
//
// Kit calls the factory each time it needs a model for the provider: at
// construction, on [Kit.SetModel], for completions that name a model, and in
// subagents. A factory that loads expensive resources should cache them.
// Set [ProviderResult.Closer] to release per-model resources; Kit closes it
// when the model is replaced or the Kit is closed. Resources shared by all
// models (such as a loaded runtime) are best released by the application
// after all Kit instances are closed.
//
// Kit does not add automatic prompt-cache options to models from a factory.
// Put any provider-specific options in [ProviderResult.ProviderOptions].
type ProviderFactory = models.ProviderFactory

// RegisterProvider registers f as the process-wide factory for the provider
// name. After registration, every Kit instance in the process resolves model
// strings "name/<model>" through f. A registered factory takes precedence over
// the built-in providers, so it can also replace a built-in provider name.
// Factories set in [Options.Providers] take precedence over this registry.
//
// Registering a name again replaces the earlier factory, and a nil f removes
// it. The name is case-insensitive and must be non-empty and free of "/".
//
// Example:
//
//	kit.RegisterProvider("local", func(ctx context.Context, cfg *kit.ProviderConfig, model string) (*kit.ProviderResult, error) {
//	    m, err := backend.LanguageModel(ctx, model)
//	    if err != nil {
//	        return nil, err
//	    }
//	    return &kit.ProviderResult{Model: m}, nil
//	})
//	host, _ := kit.New(ctx, &kit.Options{Model: "local/qwen3-8b"})
func RegisterProvider(name string, f ProviderFactory) error {
	return models.RegisterProviderFactory(name, f)
}

// UnregisterProvider removes the process-wide factory registered for name.
// It reports whether a factory was registered. Kit instances that already
// hold a model from the factory keep using it.
func UnregisterProvider(name string) bool {
	return models.UnregisterProviderFactory(name)
}

// RegisteredProviders returns the sorted names of all process-wide provider
// factories registered with [RegisterProvider].
func RegisteredProviders() []string {
	return models.RegisteredProviderFactories()
}

// WithProvider registers a provider factory for this Kit instance only. See
// [Options.Providers].
func WithProvider(name string, f ProviderFactory) Option {
	return func(o *Options) {
		if o.Providers == nil {
			o.Providers = make(map[string]ProviderFactory)
		}
		o.Providers[name] = f
	}
}

// applyEffectiveProviderSettings copies this Kit's effective endpoint
// overrides and generation settings into cfg for modelString.
//
// The endpoint overrides (provider-api-key, provider-url, provider-wire)
// belong to one provider. They apply only when modelString uses that
// provider; otherwise `--provider-url http://localhost:1234/v1 --model local`
// and then `/model openai/gpt-x` would send openai requests, with the local
// key, to the local server.
//
// Generation parameter pointers are set only when the user explicitly
// provided a value, so nil pointers leave room for per-model defaults
// (modelSettings / customModels params).
func (m *Kit) applyEffectiveProviderSettings(cfg *models.ProviderConfig, modelString string) {
	if m.endpointOverridesApply(modelString) {
		cfg.ProviderAPIKey = m.v.GetString("provider-api-key")
		cfg.ProviderURL = m.v.GetString("provider-url")
		cfg.ProviderWire = m.v.GetString("provider-wire")
	} else {
		cfg.ProviderAPIKey = ""
		cfg.ProviderURL = ""
		cfg.ProviderWire = ""
	}

	if m.v.IsSet("temperature") {
		v := float32(m.v.GetFloat64("temperature"))
		cfg.Temperature = &v
	}
	if m.v.IsSet("top-p") {
		v := float32(m.v.GetFloat64("top-p"))
		cfg.TopP = &v
	}
	if m.v.IsSet("top-k") {
		v := int32(m.v.GetInt("top-k"))
		cfg.TopK = &v
	}
	if m.v.IsSet("frequency-penalty") {
		v := float32(m.v.GetFloat64("frequency-penalty"))
		cfg.FrequencyPenalty = &v
	}
	if m.v.IsSet("presence-penalty") {
		v := float32(m.v.GetFloat64("presence-penalty"))
		cfg.PresencePenalty = &v
	}
}

// hasInstanceProvider reports whether modelString names a provider that has
// an instance factory.
func (m *Kit) hasInstanceProvider(modelString string) bool {
	if len(m.providers) == 0 {
		return false
	}
	provider, _, err := models.ParseModelString(modelString)
	if err != nil {
		return false
	}
	return models.HasProviderFactory(&models.ProviderConfig{ProviderFactories: m.providers}, provider)
}
