package kitsetup

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/mark3labs/kit/internal/auth"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/models"
)

type providerCloser struct{ close func(context.Context) error }

func (c providerCloser) Close() error                           { return c.close(context.Background()) }
func (c providerCloser) CloseContext(ctx context.Context) error { return c.close(ctx) }

var _ io.Closer = providerCloser{}

// PrepareProviderFactories adapts owned extension callbacks without modifying
// process registries. Sorted owner/ID checks make collision errors deterministic.
func PrepareProviderFactories(loaded []extensions.LoadedExtension, existing map[string]models.ProviderFactory) (map[string]models.ProviderFactory, error) {
	factories := make(map[string]models.ProviderFactory, len(existing))
	reserved := map[string]bool{}
	for _, id := range models.GetGlobalRegistry().GetSupportedProviders() {
		reserved[strings.ToLower(id)] = true
	}
	for _, id := range models.RegisteredProviderFactories() {
		reserved[id] = true
	}
	for _, id := range []string{"anthropic", "openai", "openai-compat", "google", "google-vertex", "google-vertex-anthropic", "azure", "bedrock", "ollama", "openrouter", "vercel", "copilot", "github-copilot"} {
		reserved[id] = true
	}
	for id, factory := range existing {
		factories[id] = factory
		reserved[id] = true
	}
	owned := slices.Clone(loaded)
	slices.SortFunc(owned, func(a, b extensions.LoadedExtension) int { return strings.Compare(a.Path, b.Path) })
	owners := map[string]string{}
	for _, owner := range owned {
		for _, definition := range owner.Providers {
			if reserved[definition.ID] {
				return nil, fmt.Errorf("extension %s provider %s collides with an existing provider", owner.Path, definition.ID)
			}
			if prior, exists := owners[definition.ID]; exists {
				return nil, fmt.Errorf("provider %s registered by both %s and %s", definition.ID, prior, owner.Path)
			}
			owners[definition.ID] = owner.Path
			factories[definition.ID] = func(ctx context.Context, cfg *models.ProviderConfig, model string) (*models.ProviderResult, error) {
				key := cfg.ProviderAPIKey
				if key == "" {
					if manager, err := auth.NewCredentialManager(); err == nil {
						key, _ = manager.GetProviderAPIKey(definition.ID)
					}
				}
				if key == "" {
					for _, variable := range definition.Auth.Environment {
						if key = os.Getenv(variable); key != "" {
							break
						}
					}
				}
				if definition.Auth.Mode == extensions.ProviderAuthAPIKey && key == "" {
					return nil, fmt.Errorf("extension provider %s requires an API key: %s", definition.ID, definition.Auth.SetupInstruction)
				}
				params := extensions.ProviderParameters{ReasoningLevel: string(cfg.ThinkingLevel), SettingsJSON: definition.SettingsJSON, StopSequences: slices.Clone(cfg.StopSequences)}
				maxTokens := int64(cfg.MaxTokens)
				params.MaxOutputTokens = &maxTokens
				if cfg.Temperature != nil {
					v := float64(*cfg.Temperature)
					params.Temperature = &v
				}
				if cfg.TopP != nil {
					v := float64(*cfg.TopP)
					params.TopP = &v
				}
				if cfg.TopK != nil {
					v := int64(*cfg.TopK)
					params.TopK = &v
				}
				if cfg.PresencePenalty != nil {
					v := float64(*cfg.PresencePenalty)
					params.PresencePenalty = &v
				}
				if cfg.FrequencyPenalty != nil {
					v := float64(*cfg.FrequencyPenalty)
					params.FrequencyPenalty = &v
				}
				adapter, close, err := extensions.NewProviderModel(ctx, definition, extensions.ProviderCreateRequest{Model: model, APIKey: key, Endpoint: cfg.ProviderURL, UserAgent: cfg.UserAgent, Parameters: params})
				if err != nil {
					return nil, err
				}
				return &models.ProviderResult{Model: adapter, Closer: providerCloser{close: close}}, nil
			}
		}
	}
	return factories, nil
}

// PrepareProviderResolver snapshots metadata without creating a model or MCP.
func PrepareProviderResolver(loaded []extensions.LoadedExtension) *models.Resolver {
	entries := map[string]map[string]models.ModelInfo{}
	unknown := map[string]bool{}
	for _, owner := range loaded {
		for _, def := range owner.Providers {
			entries[def.ID] = map[string]models.ModelInfo{}
			unknown[def.ID] = def.AllowUnknownModels
			for _, item := range def.Models {
				info := models.ModelInfo{CapabilitiesKnown: true, Tools: item.Capabilities.Tools, Streaming: item.Capabilities.Streaming, ID: item.ID, Name: item.Name, Limit: models.Limit{Context: int(item.ContextLimit), Output: int(item.OutputLimit)}, Reasoning: item.Capabilities.Reasoning, ReasoningIsGraded: true, ReasoningLevels: slices.Clone(item.Capabilities.ReasoningLevels), InputModalities: slices.Clone(item.Capabilities.InputModalities)}
				info.Cost = models.Cost{Input: item.InputPrice.Value, Output: item.OutputPrice.Value, Published: item.InputPrice.Known && item.OutputPrice.Known}
				if item.CacheReadPrice.Known {
					value := item.CacheReadPrice.Value
					info.Cost.CacheRead = &value
				}
				if item.CacheWritePrice.Known {
					value := item.CacheWritePrice.Value
					info.Cost.CacheWrite = &value
				}
				entries[def.ID][item.ID] = info
			}
		}
	}
	resolver := models.NewResolver(entries, unknown)
	for _, owner := range loaded {
		for _, def := range owner.Providers {
			resolver.SetOwner(def.ID, owner.Path)
		}
	}
	return resolver
}
