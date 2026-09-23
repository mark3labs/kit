package models

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ProviderFactory creates a language model for a provider that Kit does not
// build in. It lets embedders plug in their own inference backend (for
// example an in-process local runtime, a test double, or a proxy) under a
// provider name of their choice.
//
// modelName is the part of the model string after the first "/" (for
// "local/qwen3-8b" it is "qwen3-8b"). cfg is the fully resolved provider
// configuration, with per-model settings and max-token right-sizing already
// applied. The factory must not mutate cfg.
//
// The factory is called every time Kit needs a model for this provider: at
// agent construction, on every model switch, for standalone completions that
// name a model, and for subagents. Factories that load expensive resources
// should cache them and release them from ProviderResult.Closer or from the
// application's own shutdown path.
type ProviderFactory func(ctx context.Context, cfg *ProviderConfig, modelName string) (*ProviderResult, error)

var (
	providerFactoriesMu sync.RWMutex
	providerFactories   = map[string]ProviderFactory{}
)

// normalizeProviderName returns the lookup key for a provider name.
func normalizeProviderName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// validateProviderName reports whether name can be used as a provider name.
// A "/" is not allowed because model strings split on the first "/".
func validateProviderName(name string) error {
	if name == "" {
		return fmt.Errorf("provider name must not be empty")
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("provider name %q must not contain \"/\"", name)
	}
	return nil
}

// RegisterProviderFactory registers f as the process-wide factory for the
// provider name. A registered factory takes precedence over the built-in
// providers and the model database auto-routing, so it can also override a
// built-in provider name. Registering a name again replaces the earlier
// factory. A nil factory removes the registration.
func RegisterProviderFactory(name string, f ProviderFactory) error {
	key := normalizeProviderName(name)
	if err := validateProviderName(key); err != nil {
		return err
	}
	providerFactoriesMu.Lock()
	defer providerFactoriesMu.Unlock()
	if f == nil {
		delete(providerFactories, key)
		return nil
	}
	providerFactories[key] = f
	return nil
}

// UnregisterProviderFactory removes the process-wide factory for name. It
// reports whether a factory was registered.
func UnregisterProviderFactory(name string) bool {
	key := normalizeProviderName(name)
	providerFactoriesMu.Lock()
	defer providerFactoriesMu.Unlock()
	_, ok := providerFactories[key]
	delete(providerFactories, key)
	return ok
}

// RegisteredProviderFactories returns the sorted names of all process-wide
// provider factories.
func RegisteredProviderFactories() []string {
	providerFactoriesMu.RLock()
	defer providerFactoriesMu.RUnlock()
	names := make([]string, 0, len(providerFactories))
	for name := range providerFactories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ValidateProviderFactories checks the keys of a per-instance factory map.
func ValidateProviderFactories(factories map[string]ProviderFactory) error {
	for name, f := range factories {
		if err := validateProviderName(normalizeProviderName(name)); err != nil {
			return err
		}
		if f == nil {
			return fmt.Errorf("provider %q has a nil factory", name)
		}
	}
	return nil
}

// lookupProviderFactory returns the factory for provider. Per-instance
// factories on cfg take precedence over process-wide ones.
func lookupProviderFactory(cfg *ProviderConfig, provider string) (ProviderFactory, bool) {
	key := normalizeProviderName(provider)
	if cfg != nil {
		for name, f := range cfg.ProviderFactories {
			if f != nil && normalizeProviderName(name) == key {
				return f, true
			}
		}
	}
	providerFactoriesMu.RLock()
	defer providerFactoriesMu.RUnlock()
	f, ok := providerFactories[key]
	return f, ok
}

// HasProviderFactory reports whether a factory is registered for provider,
// either on cfg or process-wide.
func HasProviderFactory(cfg *ProviderConfig, provider string) bool {
	_, ok := lookupProviderFactory(cfg, provider)
	return ok
}

// createFromFactory calls f and checks its result.
func createFromFactory(ctx context.Context, f ProviderFactory, config *ProviderConfig, provider, modelName string) (*ProviderResult, error) {
	result, err := f(ctx, config, modelName)
	if err != nil {
		if result != nil && result.Closer != nil {
			_ = result.Closer.Close()
		}
		return nil, fmt.Errorf("provider %s: %w", provider, err)
	}
	if result == nil || result.Model == nil {
		if result != nil && result.Closer != nil {
			_ = result.Closer.Close()
		}
		return nil, fmt.Errorf("provider %s: factory returned no model", provider)
	}
	return result, nil
}
