package kit

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/mark3labs/kit/internal/models"
)

// DecisionModel is a decision model that the application supplies. A
// [DecisionProviderFactory] returns one. Kit validates the request before
// it calls Decide, and it checks and normalizes the result after the call,
// so Decide gets only valid requests and its answers have the same shape as
// the answers of the built-in providers.
//
// req.Model is the full "provider/model" string. Decide must return one
// answer for each question, with the ID and Type of the question. Set
// Refused for a question that the model refuses. For a choice question set
// Choice; for a score question set Score (Kit sets Level); for a boolean
// question set Probability. Distribution is optional. A score entry is
// found by its level name, or by its index when the name is empty.
//
// Kit does not retry the call. An error that wraps
// [ErrInvalidDecisionRequest] or [ErrInvalidDecisionResponse] is returned
// as-is; other errors are classified like provider errors (see
// [ClassifyProviderError]).
type DecisionModel interface {
	Decide(ctx context.Context, req DecisionRequest) (*DecisionResult, error)
}

// DecisionModelFunc adapts a function to a [DecisionModel].
type DecisionModelFunc func(ctx context.Context, req DecisionRequest) (*DecisionResult, error)

// Decide calls f.
func (f DecisionModelFunc) Decide(ctx context.Context, req DecisionRequest) (*DecisionResult, error) {
	return f(ctx, req)
}

// DecisionProviderFactory creates a decision model for a provider name that
// the application supplies, for example an in-process local runtime, a
// test double or a proxy. Kit then sends decision requests for
// "name/<model>" to that model instead of to a built-in provider.
//
// modelName is the part of the model string after the first "/". cfg holds
// the Kit's HTTP transport, TLS setting and, when they belong to this
// provider, its ProviderAPIKey and ProviderURL overrides. The factory must
// not mutate cfg.
//
// Kit calls the factory for each decision request and does not close the
// model. A factory that loads expensive resources should cache them and
// release them from the application's shutdown path.
type DecisionProviderFactory func(ctx context.Context, cfg *ProviderConfig, modelName string) (DecisionModel, error)

var (
	decisionProvidersMu sync.RWMutex
	decisionProviders   = map[string]DecisionProviderFactory{}
)

// RegisterDecisionProvider registers f as the process-wide decision
// provider factory for name. After registration, [Decide] and every Kit in
// the process send requests for "name/<model>" to f. A registered factory
// takes precedence over the built-in decision providers, so it can also
// replace a built-in provider name. Factories set in
// [Options.DecisionProviders] take precedence over this registry.
//
// Registering a name again replaces the earlier factory, and a nil f
// removes it. The name is case-insensitive and must be non-empty and free
// of "/". Chat providers ([RegisterProvider]) and decision providers are
// separate registries, so one name can have both.
//
// Example:
//
//	kit.RegisterDecisionProvider("local", func(ctx context.Context, cfg *kit.ProviderConfig, model string) (kit.DecisionModel, error) {
//	    return backend.DecisionModel(ctx, model)
//	})
//	res, err := kit.Decide(ctx, kit.DecisionRequest{Model: "local/my-decider", ...})
func RegisterDecisionProvider(name string, f DecisionProviderFactory) error {
	key, err := models.ProviderNameKey(name)
	if err != nil {
		return err
	}
	decisionProvidersMu.Lock()
	defer decisionProvidersMu.Unlock()
	if f == nil {
		delete(decisionProviders, key)
		return nil
	}
	decisionProviders[key] = f
	return nil
}

// UnregisterDecisionProvider removes the process-wide decision provider
// factory registered for name. It reports whether a factory was registered.
func UnregisterDecisionProvider(name string) bool {
	key, _ := models.ProviderNameKey(name)
	decisionProvidersMu.Lock()
	defer decisionProvidersMu.Unlock()
	_, ok := decisionProviders[key]
	delete(decisionProviders, key)
	return ok
}

// RegisteredDecisionProviders returns the sorted names of all process-wide
// decision provider factories registered with [RegisterDecisionProvider].
func RegisteredDecisionProviders() []string {
	decisionProvidersMu.RLock()
	defer decisionProvidersMu.RUnlock()
	names := make([]string, 0, len(decisionProviders))
	for name := range decisionProviders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// WithDecisionProvider registers a decision provider factory for this Kit
// instance only. See [Options.DecisionProviders].
func WithDecisionProvider(name string, f DecisionProviderFactory) Option {
	return func(o *Options) {
		if o.DecisionProviders == nil {
			o.DecisionProviders = make(map[string]DecisionProviderFactory)
		}
		o.DecisionProviders[name] = f
	}
}

// normalizeDecisionProviders checks an instance factory map and returns a
// copy keyed by normalized name, with the same rules as
// [Options.Providers]. A nil or empty map gives a nil result.
func normalizeDecisionProviders(factories map[string]DecisionProviderFactory) (map[string]DecisionProviderFactory, error) {
	if len(factories) == 0 {
		return nil, nil
	}
	normalized := make(map[string]DecisionProviderFactory, len(factories))
	original := make(map[string]string, len(factories))
	for name, f := range factories {
		key, err := models.ProviderNameKey(name)
		if err != nil {
			return nil, err
		}
		if f == nil {
			return nil, fmt.Errorf("decision provider %q has a nil factory", name)
		}
		if prev, dup := original[key]; dup {
			a, b := prev, name
			if b < a {
				a, b = b, a
			}
			return nil, fmt.Errorf("decision provider names %q and %q are the same provider", a, b)
		}
		original[key] = name
		normalized[key] = f
	}
	return normalized, nil
}

// lookupDecisionProvider returns the factory for the provider of
// modelString: an instance factory first (instance is normalized), then a
// process-wide one.
func lookupDecisionProvider(instance map[string]DecisionProviderFactory, modelString string) (DecisionProviderFactory, string, string, bool) {
	provider, modelName, err := models.ParseModelString(modelString)
	if err != nil {
		return nil, "", "", false
	}
	key, err := models.ProviderNameKey(provider)
	if err != nil {
		return nil, "", "", false
	}
	if f := instance[key]; f != nil {
		return f, provider, modelName, true
	}
	decisionProvidersMu.RLock()
	defer decisionProvidersMu.RUnlock()
	f, ok := decisionProviders[key]
	return f, provider, modelName, ok
}

// hasDecisionProvider reports whether a decision provider factory handles
// modelString.
func hasDecisionProvider(instance map[string]DecisionProviderFactory, modelString string) bool {
	_, _, _, ok := lookupDecisionProvider(instance, modelString)
	return ok
}
