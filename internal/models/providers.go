package models

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"strings"
	"time"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/azure"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openaicompat"
	"charm.land/fantasy/providers/openrouter"
	"charm.land/fantasy/providers/vercel"
	openaisdk "github.com/charmbracelet/openai-go"

	"github.com/mark3labs/kit/internal/auth"
	"github.com/mark3labs/kit/internal/ui/progress"
)

const (
	// ClaudeCodePrompt is the required system prompt for OAuth authentication.
	ClaudeCodePrompt = "You are Claude Code, Anthropic's official CLI for Claude."
)

// resolveModelAlias resolves model aliases to their full names using the registry
func resolveModelAlias(provider, modelName string) string {
	registry := GetGlobalRegistry()

	aliasMap := map[string]string{
		// Anthropic aliases
		"claude-opus-latest":       "claude-opus-4-6",
		"claude-sonnet-latest":     "claude-sonnet-4-6",
		"claude-haiku-latest":      "claude-haiku-4-5",
		"claude-4-opus-latest":     "claude-opus-4-6",
		"claude-4-sonnet-latest":   "claude-sonnet-4-6",
		"claude-4-haiku-latest":    "claude-haiku-4-5",
		"claude-3-5-haiku-latest":  "claude-3-5-haiku-20241022",
		"claude-3-5-sonnet-latest": "claude-3-5-sonnet-20241022",
		"claude-3-7-sonnet-latest": "claude-3-7-sonnet-20250219",
		"claude-3-opus-latest":     "claude-3-opus-20240229",

		// OpenAI aliases
		"gpt-5-latest":      "gpt-5.4",
		"gpt-5-chat-latest": "gpt-5.4",
		"gpt-4-latest":      "gpt-4o",
		"gpt-4":             "gpt-4o",
		"gpt-3.5":           "gpt-3.5-turbo",
		"gpt-3.5-latest":    "gpt-3.5-turbo",
		"o1-latest":         "o1",
		"o3-latest":         "o3",
		"o4-latest":         "o4-mini",
		"codex-latest":      "codex-mini-latest",

		// Google Gemini aliases
		"gemini-pro-latest": "gemini-2.5-pro",
		"gemini-flash":      "gemini-2.5-flash",
		"gemini-pro":        "gemini-2.5-pro",
		"gemini-2-flash":    "gemini-2.0-flash",
		"gemini-2-pro":      "gemini-2.5-pro",
		"gemini-1.5-flash":  "gemini-1.5-flash",
		"gemini-1.5-pro":    "gemini-1.5-pro",
	}

	if resolved, exists := aliasMap[modelName]; exists {
		if registry.LookupModel(provider, resolved) != nil {
			return resolved
		}
	}

	return modelName
}

// ThinkingLevel controls extended thinking / reasoning budget for supported models.
type ThinkingLevel string

const (
	ThinkingOff     ThinkingLevel = "off"
	ThinkingMinimal ThinkingLevel = "minimal"
	ThinkingLow     ThinkingLevel = "low"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
)

// ThinkingLevels returns the ordered list of available thinking levels for cycling.
func ThinkingLevels() []ThinkingLevel {
	return []ThinkingLevel{ThinkingOff, ThinkingMinimal, ThinkingLow, ThinkingMedium, ThinkingHigh}
}

// thinkingBudgetTokens returns the token budget for a thinking level, or 0 for "off".
func thinkingBudgetTokens(level ThinkingLevel) int64 {
	switch level {
	case ThinkingMinimal:
		return 1024
	case ThinkingLow:
		return 4096
	case ThinkingMedium:
		return 10240
	case ThinkingHigh:
		return 20480
	default:
		return 0
	}
}

// ThinkingLevelDescription returns a human-readable description of a thinking level.
func ThinkingLevelDescription(level ThinkingLevel) string {
	switch level {
	case ThinkingOff:
		return "No reasoning"
	case ThinkingMinimal:
		return "Very brief reasoning (~1k tokens)"
	case ThinkingLow:
		return "Light reasoning (~4k tokens)"
	case ThinkingMedium:
		return "Moderate reasoning (~10k tokens)"
	case ThinkingHigh:
		return "Deep reasoning (~20k tokens)"
	default:
		return "No reasoning"
	}
}

// ParseThinkingLevel converts a string to a ThinkingLevel, defaulting to ThinkingOff.
func ParseThinkingLevel(s string) ThinkingLevel {
	switch ThinkingLevel(s) {
	case ThinkingMinimal, ThinkingLow, ThinkingMedium, ThinkingHigh:
		return ThinkingLevel(s)
	default:
		return ThinkingOff
	}
}

// ProviderConfig holds configuration for creating LLM providers.
type ProviderConfig struct {
	ModelString    string
	SystemPrompt   string
	ProviderAPIKey string
	ProviderURL    string
	MaxTokens      int
	Temperature    *float32
	TopP           *float32
	TopK           *int32
	StopSequences  []string
	NumGPU         *int32
	MainGPU        *int32
	TLSSkipVerify  bool
	ThinkingLevel  ThinkingLevel
	DisableCaching bool // Opt-out: set to true to disable automatic prompt caching
}

// ProviderResult contains the result of provider creation.
type ProviderResult struct {
	// Model is the created fantasy LanguageModel
	Model fantasy.LanguageModel
	// Message contains optional feedback for the user
	Message string
	// Closer is an optional cleanup function for providers that hold
	// resources (e.g. kronk's loaded models). May be nil.
	Closer io.Closer
	// ProviderOptions contains provider-specific options to be passed to the
	// fantasy agent (e.g. OpenAI Responses API reasoning options).
	ProviderOptions fantasy.ProviderOptions
	// SkipMaxOutputTokens indicates that this provider doesn't support the
	// max_output_tokens parameter (e.g., OpenAI Codex OAuth API).
	SkipMaxOutputTokens bool
}

// ParseModelString parses a model string in "provider/model" format (e.g. "anthropic/claude-sonnet-4-5").
// It splits on the first "/" to extract the provider and model name.
// For backward compatibility, the legacy "provider:model" format is also accepted with a
// deprecation warning printed to stderr.
// Returns the provider name, model name, and any error.
func ParseModelString(modelString string) (provider, model string, err error) {
	if strings.Contains(modelString, "/") {
		parts := strings.SplitN(modelString, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], nil
		}
		return "", "", fmt.Errorf("invalid model format %q: expected provider/model (e.g. anthropic/claude-sonnet-4-5)", modelString)
	}

	return "", "", fmt.Errorf("invalid model format %q: expected provider/model (e.g. anthropic/claude-sonnet-4-5)", modelString)
}

// CreateProvider creates a fantasy LanguageModel based on the provider configuration.
// Model metadata is looked up from the models.dev database for cost tracking and
// capability detection, but unknown models are passed through to the provider
// API — the database is advisory, not a gatekeeper.
//
// Native providers: anthropic, openai, google, ollama, azure, google-vertex-anthropic,
// openrouter, bedrock, vercel.
// Any provider in models.dev with an api URL or openai-compatible npm package
// is auto-routed through fantasy's openaicompat provider.
func CreateProvider(ctx context.Context, config *ProviderConfig) (*ProviderResult, error) {
	provider, modelName, err := ParseModelString(config.ModelString)
	if err != nil {
		return nil, err
	}

	// Resolve model aliases to full model names
	if provider == "anthropic" || provider == "google-vertex-anthropic" || provider == "openai" || provider == "google" {
		modelName = resolveModelAlias(provider, modelName)
	}

	registry := GetGlobalRegistry()

	// Look up model metadata (advisory, not blocking).
	// When the model is known we validate config limits and print
	// suggestions on likely typos; when unknown we let the provider
	// API be the authority.
	modelInfo := registry.LookupModel(provider, modelName)
	if modelInfo == nil && provider != "ollama" && config.ProviderURL == "" {
		// Model not in database — warn with suggestions but don't block.
		if suggestions := registry.SuggestModels(provider, modelName); len(suggestions) > 0 {
			fmt.Fprintf(os.Stderr, "Warning: model %q not found in model database for provider %s. Similar models: %s\n",
				modelName, provider, strings.Join(suggestions, ", "))
		}
	}

	// NOTE: We intentionally skip registry.ValidateEnvironment() here.
	// Each create*Provider function handles its own auth resolution and
	// produces provider-specific error messages. The early env-var check
	// was too narrow — it didn't account for stored credentials (e.g.
	// OAuth tokens from 'kit auth login') and blocked valid auth paths.

	// Validate config against known model limits when metadata is available
	if modelInfo != nil {
		validateModelConfig(config, modelInfo)
	}

	// Create the base provider
	var result *ProviderResult
	var createErr error

	switch provider {
	case "anthropic":
		result, createErr = createAnthropicProvider(ctx, config, modelName)
	case "openai":
		result, createErr = createOpenAIProvider(ctx, config, modelName)
	case "google", "gemini":
		result, createErr = createGoogleProvider(ctx, config, modelName)
	case "ollama":
		result, createErr = createOllamaProvider(ctx, config, modelName)
	case "azure":
		result, createErr = createAzureProvider(ctx, config, modelName)
	case "google-vertex-anthropic":
		result, createErr = createVertexAnthropicProvider(ctx, config, modelName)
	case "openrouter":
		result, createErr = createOpenRouterProvider(ctx, config, modelName)
	case "bedrock":
		result, createErr = createBedrockProvider(ctx, config, modelName)
	case "vercel":
		result, createErr = createVercelProvider(ctx, config, modelName)
	case "custom":
		result, createErr = createCustomProvider(ctx, config, modelName)
	default:
		result, createErr = autoRouteProvider(ctx, config, provider, modelName, registry)
	}

	if createErr != nil {
		return nil, createErr
	}

	// AUTOMATICALLY ENABLE CACHING for supported models (unless disabled).
	// This works for BOTH native and auto-routed providers by detecting
	// the model family from the model metadata.
	if cacheOpts := buildCacheProviderOptions(modelInfo, config); cacheOpts != nil {
		if result.ProviderOptions == nil {
			result.ProviderOptions = cacheOpts
		} else {
			// Merge cache options with existing provider options.
			// Only add cache options for providers that don't already have
			// options set, to avoid type conflicts (e.g., Anthropic has
			// different types for regular options vs cache control options).
			for k, v := range cacheOpts {
				if _, exists := result.ProviderOptions[k]; !exists {
					result.ProviderOptions[k] = v
				}
			}
		}
	}

	return result, nil
}

// autoRouteProvider attempts to create a provider by looking up its npm package
// in the models.dev database and routing through the appropriate fantasy provider.
// For openai-compatible providers, it uses the api URL from models.dev.
// Models may have a provider override that specifies a different npm package than
// the provider's default (e.g., opencode's claude-opus-4-6 uses @ai-sdk/anthropic).
func autoRouteProvider(ctx context.Context, config *ProviderConfig, provider, modelName string, registry *ModelsRegistry) (*ProviderResult, error) {
	providerInfo := registry.GetProviderInfo(provider)
	if providerInfo == nil {
		return nil, fmt.Errorf("unsupported provider: %s (not found in model database)", provider)
	}

	// Check for model-specific provider override
	npmPackage := providerInfo.NPM
	if modelInfo := registry.LookupModel(provider, modelName); modelInfo != nil && modelInfo.ProviderNPM != "" {
		npmPackage = modelInfo.ProviderNPM
	}

	// Determine the LLM provider for this npm package
	llmProvider := npmToLLMProvider[npmPackage]
	if llmProvider == "" && providerInfo.API != "" {
		// Unknown npm but has API URL → route through openaicompat
		llmProvider = "openaicompat"
	}

	switch llmProvider {
	case "openaicompat":
		return createAutoRoutedOpenAICompatProvider(ctx, config, modelName, providerInfo)
	case "anthropic":
		if config.ProviderURL == "" && providerInfo.API != "" {
			config.ProviderURL = providerInfo.API
		}
		return createAutoRoutedAnthropicProvider(ctx, config, modelName, providerInfo)
	case "openai":
		if config.ProviderURL == "" && providerInfo.API != "" {
			config.ProviderURL = providerInfo.API
		}
		return createAutoRoutedOpenAIProvider(ctx, config, modelName, providerInfo)
	default:
		return nil, fmt.Errorf("unsupported provider: %s (npm: %s has no LLM provider mapping)", provider, npmPackage)
	}
}

// createAutoRoutedOpenAICompatProvider creates an openaicompat provider using
// the api URL and env vars from models.dev.
func createAutoRoutedOpenAICompatProvider(ctx context.Context, config *ProviderConfig, modelName string, info *ProviderInfo) (*ProviderResult, error) {
	apiURL := config.ProviderURL
	if apiURL == "" {
		apiURL = info.API
	}
	if apiURL == "" {
		return nil, fmt.Errorf("provider %s requires --provider-url (no API URL in database)", info.ID)
	}

	apiKey := resolveAPIKey(config.ProviderAPIKey, info.Env)
	if apiKey == "" {
		return nil, fmt.Errorf("%s API key not provided. Use --provider-api-key or set %s",
			info.Name, strings.Join(info.Env, " / "))
	}

	var opts []openaicompat.Option
	opts = append(opts, openaicompat.WithBaseURL(apiURL))
	opts = append(opts, openaicompat.WithAPIKey(apiKey))
	opts = append(opts, openaicompat.WithName(info.ID))

	if config.TLSSkipVerify {
		opts = append(opts, openaicompat.WithHTTPClient(createHTTPClientWithTLSConfig(true)))
	}

	p, err := openaicompat.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s provider: %w", info.Name, err)
	}

	model, err := p.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s model: %w", info.Name, err)
	}

	return &ProviderResult{Model: model}, nil
}

// createAutoRoutedAnthropicProvider creates an anthropic provider for
// third-party providers with anthropic-compatible APIs (e.g. minimax).
func createAutoRoutedAnthropicProvider(ctx context.Context, config *ProviderConfig, modelName string, info *ProviderInfo) (*ProviderResult, error) {
	clearConflictingAnthropicSamplingParams(config)

	apiKey := resolveAPIKey(config.ProviderAPIKey, info.Env)
	if apiKey == "" {
		return nil, fmt.Errorf("%s API key not provided. Use --provider-api-key or set %s",
			info.Name, strings.Join(info.Env, " / "))
	}

	var opts []anthropic.Option
	opts = append(opts, anthropic.WithAPIKey(apiKey))

	if config.ProviderURL != "" {
		// The anthropic client appends "/v1/messages" to the base URL.
		// If the provider URL ends with "/v1", strip it to avoid double "/v1/v1" paths.
		baseURL := strings.TrimSuffix(config.ProviderURL, "/v1")
		opts = append(opts, anthropic.WithBaseURL(baseURL))
	}

	if config.TLSSkipVerify {
		opts = append(opts, anthropic.WithHTTPClient(createHTTPClientWithTLSConfig(true)))
	}

	p, err := anthropic.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s provider: %w", info.Name, err)
	}

	model, err := p.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s model: %w", info.Name, err)
	}

	return &ProviderResult{Model: model}, nil
}

// createAutoRoutedOpenAIProvider creates an openai provider for
// third-party providers with openai-compatible APIs.
func createAutoRoutedOpenAIProvider(ctx context.Context, config *ProviderConfig, modelName string, info *ProviderInfo) (*ProviderResult, error) {
	apiKey := resolveAPIKey(config.ProviderAPIKey, info.Env)
	if apiKey == "" {
		return nil, fmt.Errorf("%s API key not provided. Use --provider-api-key or set %s",
			info.Name, strings.Join(info.Env, " / "))
	}

	var opts []openai.Option
	opts = append(opts, openai.WithAPIKey(apiKey))
	opts = append(opts, openai.WithUseResponsesAPI())

	if config.ProviderURL != "" {
		opts = append(opts, openai.WithBaseURL(config.ProviderURL))
	}

	if config.TLSSkipVerify {
		opts = append(opts, openai.WithHTTPClient(createHTTPClientWithTLSConfig(true)))
	}

	p, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s provider: %w", info.Name, err)
	}

	model, err := p.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s model: %w", info.Name, err)
	}

	providerOpts := buildOpenAIProviderOptions(config, modelName)

	return &ProviderResult{Model: model, ProviderOptions: providerOpts}, nil
}

// resolveAPIKey returns the first non-empty API key from the explicit key
// or the environment variables.
func resolveAPIKey(explicitKey string, envVars []string) string {
	if explicitKey != "" {
		return explicitKey
	}
	for _, envVar := range envVars {
		if v := os.Getenv(envVar); v != "" {
			return v
		}
	}
	return ""
}

// validateModelConfig adjusts configuration parameters against known model capabilities.
// It silently clears temperature for models that don't support it, and warns (but
// does not block) when max_tokens exceeds the model's known output limit.
func validateModelConfig(config *ProviderConfig, modelInfo *ModelInfo) {
	if config.Temperature != nil && !modelInfo.Temperature {
		config.Temperature = nil
	}

	if modelInfo.Limit.Output > 0 && config.MaxTokens > modelInfo.Limit.Output {
		fmt.Fprintf(os.Stderr, "Warning: max_tokens (%d) exceeds model's known output limit (%d) for %s\n",
			config.MaxTokens, modelInfo.Limit.Output, modelInfo.ID)
	}
}

// clearConflictingAnthropicSamplingParams ensures that temperature and top_p are
// not both sent to the Anthropic API, which rejects requests containing both.
// When both are set (typically from defaults), top_p is cleared so that
// temperature takes precedence.
func clearConflictingAnthropicSamplingParams(config *ProviderConfig) {
	if config.Temperature != nil && config.TopP != nil {
		config.TopP = nil
	}
}

// buildOpenAIProviderOptions returns fantasy.ProviderOptions configured for
// OpenAI Responses API models. For reasoning models it sets reasoning_summary
// to "auto", includes encrypted reasoning content, and maps the ThinkingLevel
// to an OpenAI ReasoningEffort. For non-responses or non-reasoning models the
// returned map is nil (no extra options needed).
func buildOpenAIProviderOptions(config *ProviderConfig, modelName string) fantasy.ProviderOptions {
	if !openai.IsResponsesModel(modelName) {
		return nil
	}

	if openai.IsResponsesReasoningModel(modelName) {
		reasoningSummary := "auto"
		opts := &openai.ResponsesProviderOptions{
			ReasoningSummary: &reasoningSummary,
			Include: []openai.IncludeType{
				openai.IncludeReasoningEncryptedContent,
			},
		}

		// Map ThinkingLevel to OpenAI ReasoningEffort.
		if effort := thinkingLevelToReasoningEffort(config.ThinkingLevel); effort != nil {
			opts.ReasoningEffort = effort
		}

		return fantasy.ProviderOptions{
			openai.Name: opts,
		}
	}

	return nil
}

// thinkingLevelToReasoningEffort maps a ThinkingLevel to an OpenAI ReasoningEffort.
// Returns nil for ThinkingOff (use the model's default).
func thinkingLevelToReasoningEffort(level ThinkingLevel) *openai.ReasoningEffort {
	switch level {
	case ThinkingMinimal:
		return openai.ReasoningEffortOption(openai.ReasoningEffortMinimal)
	case ThinkingLow:
		return openai.ReasoningEffortOption(openai.ReasoningEffortLow)
	case ThinkingMedium:
		return openai.ReasoningEffortOption(openai.ReasoningEffortMedium)
	case ThinkingHigh:
		return openai.ReasoningEffortOption(openai.ReasoningEffortHigh)
	default:
		return nil
	}
}

// buildAnthropicProviderOptions returns fantasy.ProviderOptions configured for
// Anthropic models with extended thinking. When thinking is enabled, it sets
// SendReasoning to true and configures the thinking budget. For thinking-off
// or non-reasoning models the returned map is nil.
//
// NOTE: With message-level caching, thinking and caching can work together.
// Message-level cache control (ProviderCacheControlOptions) doesn't conflict
// with provider-level thinking options (ProviderOptions).
//
// Anthropic requires max_tokens > thinking.budget_tokens. If the configured
// MaxTokens is too low, it is bumped to budget + 4096 to leave room for the
// actual response.
func buildAnthropicProviderOptions(config *ProviderConfig, modelName string) fantasy.ProviderOptions {
	// Thinking is OFF by default. If user hasn't explicitly enabled it, return nil.
	if config.ThinkingLevel == "" || config.ThinkingLevel == ThinkingOff {
		return nil
	}

	budget := thinkingBudgetTokens(config.ThinkingLevel)
	if budget == 0 {
		return nil
	}

	// Ensure MaxTokens exceeds the thinking budget (Anthropic requirement).
	minRequired := int(budget) + 4096
	if config.MaxTokens < minRequired {
		config.MaxTokens = minRequired
	}

	sendReasoning := true
	opts := &anthropic.ProviderOptions{
		SendReasoning: &sendReasoning,
		Thinking: &anthropic.ThinkingProviderOption{
			BudgetTokens: budget,
		},
	}
	return anthropic.NewProviderOptions(opts)
}

func createAnthropicProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	clearConflictingAnthropicSamplingParams(config)

	apiKey, source, err := auth.GetAnthropicAPIKey(config.ProviderAPIKey)
	if err != nil {
		return nil, err
	}

	if os.Getenv("DEBUG") != "" || os.Getenv("KIT_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "Using Anthropic API key from: %s\n", source)
	}

	var opts []anthropic.Option
	opts = append(opts, anthropic.WithAPIKey(apiKey))

	if config.ProviderURL != "" {
		opts = append(opts, anthropic.WithBaseURL(config.ProviderURL))
	}

	// Handle OAuth vs API key authentication
	if strings.HasPrefix(source, "stored OAuth") {
		httpClient := createOAuthHTTPClient(apiKey, config.TLSSkipVerify)
		opts = append(opts, anthropic.WithHTTPClient(httpClient))
		// Note: For OAuth, the API key is set as a placeholder; the transport handles auth
	} else if config.TLSSkipVerify {
		opts = append(opts, anthropic.WithHTTPClient(createHTTPClientWithTLSConfig(true)))
	}

	provider, err := anthropic.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Anthropic provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Anthropic model: %w", err)
	}

	// Build provider options for extended thinking (reasoning budget).
	providerOpts := buildAnthropicProviderOptions(config, modelName)

	return &ProviderResult{Model: model, ProviderOptions: providerOpts}, nil
}

func createVertexAnthropicProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	clearConflictingAnthropicSamplingParams(config)

	projectID := firstNonEmpty(
		os.Getenv("GOOGLE_VERTEX_PROJECT"),
		os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID"),
		os.Getenv("GOOGLE_CLOUD_PROJECT"),
		os.Getenv("GCLOUD_PROJECT"),
		os.Getenv("CLOUDSDK_CORE_PROJECT"),
	)
	if projectID == "" {
		return nil, fmt.Errorf("google Vertex project ID not provided, set ANTHROPIC_VERTEX_PROJECT_ID, GOOGLE_CLOUD_PROJECT, or GCLOUD_PROJECT environment variable")
	}

	region := firstNonEmpty(
		os.Getenv("GOOGLE_VERTEX_LOCATION"),
		os.Getenv("ANTHROPIC_VERTEX_REGION"),
		os.Getenv("CLOUD_ML_REGION"),
	)
	if region == "" {
		region = "global"
	}

	var opts []anthropic.Option
	opts = append(opts, anthropic.WithVertex(projectID, region))

	provider, err := anthropic.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex Anthropic provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex Anthropic model: %w", err)
	}

	return &ProviderResult{Model: model}, nil
}

func createOpenAIProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	apiKey := config.ProviderAPIKey
	source := "command-line flag"
	var accountID string
	var isCodexOAuth bool

	if apiKey == "" {
		// Check stored credentials first
		cm, err := auth.NewCredentialManager()
		if err == nil {
			if creds, err := cm.GetOpenAICredentials(); err == nil && creds != nil {
				if creds.Type == "oauth" && creds.AccessToken != "" {
					// For OAuth, get a valid access token (may refresh if needed)
					token, err := cm.GetValidOpenAIAccessToken()
					if err == nil && token != "" {
						apiKey = token
						accountID = creds.AccountID
						isCodexOAuth = true
						source = "stored Codex OAuth credentials"
					}
				} else if creds.Type == "api_key" && creds.APIKey != "" {
					apiKey = creds.APIKey
					source = "stored API key"
				}
			}
		}
	}

	// Fall back to environment variable
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		source = "OPENAI_API_KEY environment variable"
	}

	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not provided. Use 'kit auth login openai', --provider-api-key flag, or OPENAI_API_KEY environment variable")
	}

	if os.Getenv("DEBUG") != "" || os.Getenv("KIT_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "Using OpenAI API key from: %s\n", source)
	}

	// For Codex OAuth, use the ChatGPT backend API with custom headers
	if isCodexOAuth {
		return createOpenAICodexProvider(ctx, config, modelName, apiKey, accountID)
	}

	// Regular OpenAI API key flow
	var opts []openai.Option
	opts = append(opts, openai.WithAPIKey(apiKey))
	opts = append(opts, openai.WithUseResponsesAPI())

	if config.ProviderURL != "" {
		opts = append(opts, openai.WithBaseURL(config.ProviderURL))
	}

	if config.TLSSkipVerify {
		opts = append(opts, openai.WithHTTPClient(createHTTPClientWithTLSConfig(true)))
	}

	provider, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI model: %w", err)
	}

	// Build provider options for OpenAI Responses API reasoning models.
	providerOpts := buildOpenAIProviderOptions(config, modelName)

	return &ProviderResult{Model: model, ProviderOptions: providerOpts}, nil
}

// createOpenAICodexProvider creates a provider for ChatGPT/Codex OAuth tokens.
// Uses the chatgpt.com/backend-api/codex endpoint with special headers.
func createOpenAICodexProvider(ctx context.Context, config *ProviderConfig, modelName, token, accountID string) (*ProviderResult, error) {
	// Check for spark models which are not accessible via OAuth
	if detectCodexModelFamily(modelName) == "gpt-codex-spark" {
		return nil, fmt.Errorf("gpt-codex-spark models are not accessible via ChatGPT OAuth. " +
			"These models require special access or a different authentication method. " +
			"Please use regular Codex models like 'openai/gpt-5.3-codex' instead")
	}

	// Use the ChatGPT backend API with /codex path
	baseURL := "https://chatgpt.com/backend-api/codex"
	if config.ProviderURL != "" {
		baseURL = config.ProviderURL
	}

	// Build custom HTTP client with required headers
	httpClient := createCodexHTTPClient(token, accountID, config.TLSSkipVerify)

	var opts []openai.Option
	opts = append(opts, openai.WithAPIKey(token))
	opts = append(opts, openai.WithBaseURL(baseURL))
	opts = append(opts, openai.WithUseResponsesAPI())
	opts = append(opts, openai.WithHTTPClient(httpClient))

	provider, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI Codex provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI Codex model: %w", err)
	}

	providerOpts := buildCodexProviderOptions(config, modelName)

	return &ProviderResult{
		Model:               model,
		ProviderOptions:     providerOpts,
		SkipMaxOutputTokens: true,
	}, nil
}

// buildCodexProviderOptions returns fantasy.ProviderOptions configured for
// OpenAI Codex API. The Codex API requires the system prompt to be passed
// as 'instructions' rather than as a system message.
func buildCodexProviderOptions(config *ProviderConfig, modelName string) fantasy.ProviderOptions {
	store := false
	opts := &openai.ResponsesProviderOptions{
		Store: &store,
	}

	if config.SystemPrompt != "" {
		opts.Instructions = &config.SystemPrompt
	}

	if openai.IsResponsesReasoningModel(modelName) {
		opts.ReasoningEffort = thinkingLevelToReasoningEffort(config.ThinkingLevel)
	}

	return fantasy.ProviderOptions{openai.Name: opts}
}

// detectCodexModelFamily determines the model family from the model name
func detectCodexModelFamily(modelName string) string {
	modelName = strings.ToLower(modelName)
	if strings.Contains(modelName, "spark") {
		return "gpt-codex-spark"
	}
	if strings.Contains(modelName, "codex-mini") || strings.Contains(modelName, "mini-latest") {
		return "gpt-codex-mini"
	}
	if strings.Contains(modelName, "codex") {
		return "gpt-codex"
	}
	return ""
}

// createCodexHTTPClient creates an HTTP client with headers required for ChatGPT/Codex API
func createCodexHTTPClient(token, accountID string, skipVerify bool) *http.Client {
	var base http.RoundTripper
	if skipVerify {
		base = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	} else {
		base = http.DefaultTransport
	}

	return &http.Client{
		Transport: &codexTransport{
			base:      base,
			token:     token,
			accountID: accountID,
		},
		Timeout: 120 * time.Second,
	}
}

// codexTransport is a custom RoundTripper that adds ChatGPT/Codex specific headers
type codexTransport struct {
	base      http.RoundTripper
	token     string
	accountID string
}

func (t *codexTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())

	// Add required headers for ChatGPT/Codex API
	// These headers mimic the official pi client to avoid Cloudflare blocking
	newReq.Header.Set("Authorization", "Bearer "+t.token)
	if t.accountID != "" {
		newReq.Header.Set("chatgpt-account-id", t.accountID)
	}
	newReq.Header.Set("originator", "kit")
	newReq.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	newReq.Header.Set("OpenAI-Beta", "responses=experimental")
	newReq.Header.Set("Accept", "text/event-stream")
	newReq.Header.Set("Accept-Language", "en-US,en;q=0.9")
	newReq.Header.Set("Cache-Control", "no-cache")
	newReq.Header.Set("Pragma", "no-cache")

	return t.base.RoundTrip(newReq)
}

func createGoogleProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	apiKey := firstNonEmpty(
		config.ProviderAPIKey,
		os.Getenv("GOOGLE_API_KEY"),
		os.Getenv("GEMINI_API_KEY"),
		os.Getenv("GOOGLE_GENERATIVE_AI_API_KEY"),
	)
	if apiKey == "" {
		return nil, fmt.Errorf("google API key not provided, use --provider-api-key flag or GOOGLE_API_KEY/GEMINI_API_KEY/GOOGLE_GENERATIVE_AI_API_KEY environment variable")
	}

	var opts []google.Option
	opts = append(opts, google.WithGeminiAPIKey(apiKey))

	provider, err := google.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google model: %w", err)
	}

	return &ProviderResult{Model: model}, nil
}

func createAzureProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	apiKey := config.ProviderAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("AZURE_OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("azure OpenAI API key not provided, use --provider-api-key flag or AZURE_OPENAI_API_KEY environment variable")
	}

	baseURL := config.ProviderURL
	if baseURL == "" {
		baseURL = os.Getenv("AZURE_OPENAI_BASE_URL")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("azure OpenAI base URL not provided, use --provider-url flag or AZURE_OPENAI_BASE_URL environment variable")
	}

	var opts []azure.Option
	opts = append(opts, azure.WithAPIKey(apiKey))
	opts = append(opts, azure.WithBaseURL(baseURL))

	if config.TLSSkipVerify {
		opts = append(opts, azure.WithHTTPClient(createHTTPClientWithTLSConfig(true)))
	}

	provider, err := azure.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure OpenAI provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure OpenAI model: %w", err)
	}

	return &ProviderResult{Model: model}, nil
}

func createOpenRouterProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	apiKey := config.ProviderAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenRouter API key not provided. Use --provider-api-key flag or OPENROUTER_API_KEY environment variable")
	}

	var opts []openrouter.Option
	opts = append(opts, openrouter.WithAPIKey(apiKey))

	provider, err := openrouter.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenRouter provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenRouter model: %w", err)
	}

	return &ProviderResult{Model: model}, nil
}

func createBedrockProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	var opts []bedrock.Option

	// Bedrock uses AWS SDK default credential chain (env vars, shared config, etc.)
	provider, err := bedrock.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Bedrock provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Bedrock model: %w", err)
	}

	return &ProviderResult{Model: model}, nil
}

func createVercelProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	apiKey := config.ProviderAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("VERCEL_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("vercel API key not provided, use --provider-api-key flag or VERCEL_API_KEY environment variable")
	}

	var opts []vercel.Option
	opts = append(opts, vercel.WithAPIKey(apiKey))

	if config.ProviderURL != "" {
		opts = append(opts, vercel.WithBaseURL(config.ProviderURL))
	}

	provider, err := vercel.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vercel provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vercel model: %w", err)
	}

	return &ProviderResult{Model: model}, nil
}

// customToPromptFunc converts prompts to OpenAI format using the default conversion.
func customToPromptFunc(prompt fantasy.Prompt, systemPrompt, user string) ([]openaisdk.ChatCompletionMessageParamUnion, []fantasy.CallWarning) {
	return openai.DefaultToPrompt(prompt, systemPrompt, user)
}

func createCustomProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	// Resolve base URL: per-model override > global provider-url flag/config
	registry := GetGlobalRegistry()
	modelInfo := registry.LookupModel("custom", modelName)

	baseURL := config.ProviderURL
	if modelInfo != nil && modelInfo.BaseURL != "" {
		baseURL = modelInfo.BaseURL
	}

	if baseURL == "" {
		return nil, fmt.Errorf("custom provider requires --provider-url or a baseUrl in the model config")
	}

	apiKey := config.ProviderAPIKey
	if modelInfo != nil && modelInfo.APIKey != "" {
		apiKey = modelInfo.APIKey
	}
	if apiKey == "" {
		apiKey = os.Getenv("CUSTOM_API_KEY")
	}
	if apiKey == "" {
		// Many local/custom endpoints don't require a key; use a placeholder.
		apiKey = "custom"
	}

	// <think> tag extraction is handled transparently at the agent layer,
	// so no provider-level hooks are needed here.
	var opts []openai.Option
	opts = append(opts, openai.WithBaseURL(baseURL))
	opts = append(opts, openai.WithAPIKey(apiKey))
	opts = append(opts, openai.WithName("custom"))
	opts = append(opts, openai.WithLanguageModelOptions(
		openai.WithLanguageModelToPromptFunc(customToPromptFunc),
	))

	if config.TLSSkipVerify {
		opts = append(opts, openai.WithHTTPClient(createHTTPClientWithTLSConfig(true)))
	}

	p, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom provider: %w", err)
	}

	model, err := p.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom model: %w", err)
	}

	return &ProviderResult{Model: model}, nil
}

func createOllamaProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	baseURL := "http://localhost:11434"
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		baseURL = host
	}
	if config.ProviderURL != "" {
		baseURL = config.ProviderURL
	}

	// Pre-load model with GPU fallback
	loadingResult, err := loadOllamaModelWithFallback(ctx, baseURL, modelName, config)
	var loadingMessage string
	if err != nil {
		loadingMessage = ""
	} else {
		loadingMessage = loadingResult.Message
	}

	// Use openaicompat provider pointed at Ollama's OpenAI-compatible endpoint
	ollamaAPIBase := strings.TrimRight(baseURL, "/") + "/v1"

	var opts []openaicompat.Option
	opts = append(opts, openaicompat.WithBaseURL(ollamaAPIBase))
	opts = append(opts, openaicompat.WithName("ollama"))

	if config.ProviderAPIKey != "" {
		opts = append(opts, openaicompat.WithAPIKey(config.ProviderAPIKey))
	} else {
		// Ollama doesn't require an API key, but the openaicompat provider might need one
		opts = append(opts, openaicompat.WithAPIKey("ollama"))
	}

	if config.TLSSkipVerify {
		opts = append(opts, openaicompat.WithHTTPClient(createHTTPClientWithTLSConfig(true)))
	}

	provider, err := openaicompat.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama model: %w", err)
	}

	return &ProviderResult{
		Model:   model,
		Message: loadingMessage,
	}, nil
}

// OllamaLoadingResult contains the result of model loading with actual settings used.
type OllamaLoadingResult struct {
	Message string
}

// loadOllamaModelWithFallback loads an Ollama model with GPU settings and automatic CPU fallback
func loadOllamaModelWithFallback(ctx context.Context, baseURL, modelName string, config *ProviderConfig) (*OllamaLoadingResult, error) {
	client := createHTTPClientWithTLSConfig(config.TLSSkipVerify)

	// Phase 1: Check if model exists locally
	if err := checkOllamaModelExists(client, baseURL, modelName); err != nil {
		// Phase 2: Pull model if not found
		if err := pullOllamaModel(ctx, client, baseURL, modelName); err != nil {
			return nil, fmt.Errorf("failed to pull model %s: %v", modelName, err)
		}
	}

	// Phase 3: Warmup the model
	options := buildOllamaOptions(config)
	_, err := loadOllamaModelWithOptions(ctx, client, baseURL, modelName, options)
	if err != nil {
		// Phase 4: Fallback to CPU if GPU memory insufficient
		if isGPUMemoryError(err) {
			cpuOptions := make(map[string]any)
			maps.Copy(cpuOptions, options)
			cpuOptions["num_gpu"] = 0

			_, cpuErr := loadOllamaModelWithOptions(ctx, client, baseURL, modelName, cpuOptions)
			if cpuErr != nil {
				return nil, fmt.Errorf("failed to load model on GPU (%v) and CPU fallback failed (%v)", err, cpuErr)
			}

			return &OllamaLoadingResult{
				Message: "Insufficient GPU memory, falling back to CPU inference",
			}, nil
		}
		return nil, err
	}

	return &OllamaLoadingResult{
		Message: "Model loaded successfully on GPU",
	}, nil
}

func buildOllamaOptions(config *ProviderConfig) map[string]any {
	options := make(map[string]any)
	if config.Temperature != nil {
		options["temperature"] = *config.Temperature
	}
	if config.TopP != nil {
		options["top_p"] = *config.TopP
	}
	if config.TopK != nil {
		options["top_k"] = int(*config.TopK)
	}
	if len(config.StopSequences) > 0 {
		options["stop"] = config.StopSequences
	}
	if config.MaxTokens > 0 {
		options["num_predict"] = config.MaxTokens
	}
	if config.NumGPU != nil {
		options["num_gpu"] = int(*config.NumGPU)
	}
	if config.MainGPU != nil {
		options["main_gpu"] = int(*config.MainGPU)
	}
	return options
}

func checkOllamaModelExists(client *http.Client, baseURL, modelName string) error {
	reqBody := map[string]string{"model": modelName}
	jsonBody, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/show", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("model not found locally")
	}
	return nil
}

func pullOllamaModel(ctx context.Context, client *http.Client, baseURL, modelName string) error {
	return pullOllamaModelWithProgress(ctx, client, baseURL, modelName, true)
}

func pullOllamaModelWithProgress(ctx context.Context, client *http.Client, baseURL, modelName string, showProgress bool) error {
	reqBody := map[string]string{"name": modelName}
	jsonBody, _ := json.Marshal(reqBody)

	pullCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(pullCtx, "POST", baseURL+"/api/pull", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to pull model (status %d): %s", resp.StatusCode, string(body))
	}

	if showProgress {
		progressReader := progress.NewProgressReader(resp.Body)
		defer func() { _ = progressReader.Close() }()
		_, err = io.ReadAll(progressReader)
	} else {
		_, err = io.ReadAll(resp.Body)
	}
	return err
}

func loadOllamaModelWithOptions(ctx context.Context, client *http.Client, baseURL, modelName string, options map[string]any) (map[string]any, error) {
	warmupOptions := make(map[string]any)
	maps.Copy(warmupOptions, options)
	warmupOptions["num_predict"] = 1

	reqBody := map[string]any{
		"model":   modelName,
		"prompt":  "Hello",
		"stream":  false,
		"options": warmupOptions,
	}

	jsonBody, _ := json.Marshal(reqBody)

	warmupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(warmupCtx, "POST", baseURL+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("warmup request failed (status %d): %s", resp.StatusCode, string(body))
	}

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return options, nil
}

func isGPUMemoryError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "out of memory") ||
		strings.Contains(errStr, "insufficient memory") ||
		strings.Contains(errStr, "cuda out of memory") ||
		strings.Contains(errStr, "gpu memory")
}

// createHTTPClientWithTLSConfig creates an HTTP client with optional TLS skip verify
func createHTTPClientWithTLSConfig(skipVerify bool) *http.Client {
	if !skipVerify {
		return &http.Client{}
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	return &http.Client{
		Transport: transport,
	}
}

// createOAuthHTTPClient creates an HTTP client that adds OAuth headers for Anthropic API
func createOAuthHTTPClient(accessToken string, skipVerify bool) *http.Client {
	var base = http.DefaultTransport
	if skipVerify {
		base = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	return &http.Client{
		Transport: &oauthTransport{
			accessToken: accessToken,
			base:        base,
		},
	}
}

type oauthTransport struct {
	accessToken string
	base        http.RoundTripper
}

func (t *oauthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Resolve the freshest available token. The credential manager
	// automatically refreshes tokens nearing expiry (5-minute buffer).
	// This keeps long-lived sessions (e.g. ACP) working across token
	// renewals. Falls back to the originally-provided token if the
	// credential manager is unavailable.
	token := t.accessToken
	if cm, err := auth.NewCredentialManager(); err == nil {
		if fresh, err := cm.GetValidAccessToken(); err == nil && fresh != "" {
			token = fresh
		}
	}

	newReq := req.Clone(req.Context())
	newReq.Header.Del("x-api-key")
	newReq.Header.Set("Authorization", "Bearer "+token)
	newReq.Header.Set("anthropic-beta", "oauth-2025-04-20")
	newReq.Header.Set("anthropic-version", "2023-06-01")

	if req.Method == "POST" && strings.Contains(req.URL.Path, "/v1/messages") && req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err == nil {
			modifiedBody, err := t.injectClaudeCodePrompt(body)
			if err == nil {
				newReq.Body = io.NopCloser(bytes.NewReader(modifiedBody))
				newReq.ContentLength = int64(len(modifiedBody))
			}
		}
	}

	return t.base.RoundTrip(newReq)
}

func (t *oauthTransport) injectClaudeCodePrompt(body []byte) ([]byte, error) {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return body, nil
	}

	systemRaw, hasSystem := data["system"]
	if !hasSystem {
		data["system"] = ClaudeCodePrompt
		return json.Marshal(data)
	}

	switch system := systemRaw.(type) {
	case string:
		if system == ClaudeCodePrompt {
			return body, nil
		}
		data["system"] = []any{
			map[string]any{"type": "text", "text": ClaudeCodePrompt},
			map[string]any{"type": "text", "text": system},
		}
	case []any:
		if len(system) > 0 {
			if first, ok := system[0].(map[string]any); ok {
				if text, ok := first["text"].(string); ok && text == ClaudeCodePrompt {
					return body, nil
				}
			}
		}
		newSystem := []any{
			map[string]any{"type": "text", "text": ClaudeCodePrompt},
		}
		data["system"] = append(newSystem, system...)
	}

	return json.Marshal(data)
}

// firstNonEmpty returns the first non-empty string from the arguments.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
