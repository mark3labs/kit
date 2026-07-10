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
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
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
	"github.com/spf13/viper"
)

const (
	// ClaudeCodePrompt is the required system prompt for OAuth authentication.
	ClaudeCodePrompt = "You are Claude Code, Anthropic's official CLI for Claude."

	// copilotProviderID is the canonical models.dev provider key. The CLI also
	// accepts the shorter "copilot" alias for user-facing model strings.
	copilotProviderID = "github-copilot"
	// copilotAliasProviderID is the short provider prefix accepted by kit.
	copilotAliasProviderID = "copilot"
	// copilotBaseURL is the fallback API URL if the model catalog has no API URL.
	copilotBaseURL = "https://api.githubcopilot.com"

	// GitHub Copilot currently expects VS Code Copilot Chat client identifiers.
	// Keep these centralized so they are easy to audit and update when GitHub
	// changes accepted client metadata.
	copilotIntegrationID       = "vscode-chat"
	copilotEditorVersion       = "vscode/1.104.1"
	copilotEditorPluginVersion = "copilot-chat/0.31.0"
	copilotUserAgent           = "GitHubCopilotChat/0.31.0"
	copilotOpenAIIntent        = "conversation-agent"
	copilotGitHubAPIVersion    = "2026-01-09"
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
	ThinkingNone    ThinkingLevel = "none"
	ThinkingMinimal ThinkingLevel = "minimal"
	ThinkingLow     ThinkingLevel = "low"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
)

// ThinkingLevels returns the ordered list of available thinking levels for cycling.
func ThinkingLevels() []ThinkingLevel {
	return []ThinkingLevel{ThinkingOff, ThinkingNone, ThinkingMinimal, ThinkingLow, ThinkingMedium, ThinkingHigh}
}

// thinkingBudgetTokens returns the token budget for a thinking level, or 0 for "off" or "none".
func thinkingBudgetTokens(level ThinkingLevel) int64 {
	switch level {
	case ThinkingNone:
		return 1024
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
	case ThinkingNone:
		return "Minimal reasoning (OpenAI 'none')"
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
	case ThinkingNone, ThinkingMinimal, ThinkingLow, ThinkingMedium, ThinkingHigh:
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
	// ProviderWire is an explicit wire protocol override ("openai",
	// "openai-compat", "anthropic", "google") for auto-routed providers.
	// Takes precedence over the registry's Wire declaration and the
	// npm-package heuristic. Set from the --provider-wire flag.
	ProviderWire     string
	MaxTokens        int
	Temperature      *float32
	TopP             *float32
	TopK             *int32
	FrequencyPenalty *float32
	PresencePenalty  *float32
	StopSequences    []string
	NumGPU           *int32
	MainGPU          *int32
	TLSSkipVerify    bool
	ThinkingLevel    ThinkingLevel
	DisableCaching   bool // Opt-out: set to true to disable automatic prompt caching

	// ConfigStore is the per-instance configuration store used to resolve
	// "explicitly set" precedence checks (isExplicitlySet), per-model
	// settings, and right-sizing. When nil, the process-global viper store is
	// used. Threading a per-Kit store here keeps generation-parameter
	// precedence isolated between Kit instances in the same process.
	ConfigStore *viper.Viper

	// ProgressReaderFunc, when set, wraps an io.Reader with progress display
	// for long operations like Ollama model pulls. The returned io.ReadCloser
	// must be closed when done. When nil, the raw reader is consumed directly
	// with no progress UI.
	ProgressReaderFunc func(io.Reader) io.ReadCloser
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

// isCopilotProvider reports whether provider is the canonical catalog key or
// the user-facing shorthand alias.
func isCopilotProvider(provider string) bool {
	return provider == copilotAliasProviderID || provider == copilotProviderID
}

// catalogProviderID maps supported provider aliases to their models.dev keys.
func catalogProviderID(provider string) string {
	if isCopilotProvider(provider) {
		return copilotProviderID
	}
	return provider
}

// CreateProvider creates a fantasy LanguageModel based on the provider configuration.
// Model metadata is looked up from the models.dev database for cost tracking and
// capability detection, but unknown models are passed through to the provider
// API — the database is advisory, not a gatekeeper.
//
// Native providers: anthropic, openai, google, ollama, azure, google-vertex-anthropic,
// openrouter, bedrock, vercel.
// Any other provider in models.dev is auto-routed by wire protocol: its npm
// package (or per-model override) selects the OpenAI, Anthropic, or Google
// transport, using the provider's api URL as the base. Providers with an api
// URL but an unrecognized npm package fall back to the OpenAI-compatible wire.
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
	lookupProvider := catalogProviderID(provider)

	// Look up model metadata (advisory for most providers, strict for Copilot).
	// When the model is known we validate config limits and print
	// suggestions on likely typos; when unknown we let the provider
	// API be the authority except for Copilot, whose non-GPT catalog entries
	// require unsupported wire protocols.
	modelInfo := registry.LookupModel(lookupProvider, modelName)
	if isCopilotProvider(provider) {
		providerInfo := registry.GetProviderInfo(copilotProviderID)
		if providerInfo == nil {
			return nil, fmt.Errorf("unsupported provider: %s (not found in model database)", copilotProviderID)
		}
		if modelInfo == nil {
			if suggestions := registry.SuggestModels(copilotProviderID, modelName); len(suggestions) > 0 {
				return nil, fmt.Errorf("model %q not found for provider %s. Did you mean one of: %s", modelName, copilotProviderID, strings.Join(suggestions, ", "))
			}
			return nil, fmt.Errorf("model %q not found for provider %s", modelName, copilotProviderID)
		}
	} else if modelInfo == nil && provider != "ollama" && config.ProviderURL == "" {
		// Model not in database — warn with suggestions but don't block.
		if suggestions := registry.SuggestModels(lookupProvider, modelName); len(suggestions) > 0 {
			fmt.Fprintf(os.Stderr, "Warning: model %q not found in model database for provider %s. Similar models: %s\n",
				modelName, lookupProvider, strings.Join(suggestions, ", "))
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

	// Apply per-model generation parameter defaults. Model-level params are
	// only applied for fields where the user hasn't explicitly set a value
	// via CLI flag or global config.
	ApplyModelSettings(config, modelInfo)

	// Auto-raise MaxTokens toward the model's known output ceiling when the
	// user hasn't explicitly set --max-tokens and no per-model override
	// applied. Runs after ApplyModelSettings so explicit modelSettings win.
	rightSizeMaxTokens(config, modelInfo)

	// Create the base provider
	var result *ProviderResult
	var createErr error

	switch provider {
	case "anthropic":
		result, createErr = createAnthropicProvider(ctx, config, modelName)
	case "openai":
		result, createErr = createOpenAIProvider(ctx, config, modelName)
	case "copilot", "github-copilot":
		result, createErr = createCopilotProvider(ctx, config, modelName)
	case "google", "gemini":
		result, createErr = createGoogleProvider(ctx, config, modelName)
	case "ollama":
		result, createErr = createOllamaProvider(ctx, config, modelName)
	case "azure", "azure-cognitive-services":
		result, createErr = createAzureProvider(ctx, config, modelName)
	case "google-vertex-anthropic":
		result, createErr = createVertexAnthropicProvider(ctx, config, modelName)
	case "google-vertex":
		result, createErr = createGoogleVertexProvider(ctx, config, modelName)
	case "openrouter":
		result, createErr = createOpenRouterProvider(ctx, config, modelName)
	case "bedrock", "amazon-bedrock":
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
			//
			// For OpenAI Responses API models, we skip merging entirely because
			// ResponsesProviderOptions and ProviderOptions are incompatible types
			// under the same options key. Detect this by type rather than by
			// provider name so providers auto-routed to the Responses wire
			// (wire: openai / npm @ai-sdk/openai) are guarded too, not just the
			// native "openai" provider.
			if !hasResponsesProviderOptions(result.ProviderOptions) {
				for k, v := range cacheOpts {
					if _, exists := result.ProviderOptions[k]; !exists {
						result.ProviderOptions[k] = v
					}
				}
			}
		}
	}

	return result, nil
}

// hasResponsesProviderOptions reports whether opts already carries OpenAI
// Responses API options (openai.ResponsesProviderOptions), which are
// type-incompatible with the regular prompt-cache options under the same
// options key. Used to guard the cache-options merge in CreateProvider for
// both the native openai provider and providers auto-routed to the Responses
// wire.
func hasResponsesProviderOptions(opts fantasy.ProviderOptions) bool {
	_, ok := opts[openai.Name].(*openai.ResponsesProviderOptions)
	return ok
}

// autoRouteProvider attempts to create a provider by resolving its wire
// protocol (openai, openai-compat, anthropic, google) and routing through the
// matching fantasy provider. Fantasy implements three native wire protocols
// (the OpenAI one split into Responses API and chat-completions flavors), and
// every other entry in its providers/ tree is a thin wrapper around one of
// them. Using the provider's api URL from the registry as the base URL, any
// proxy that re-flavors one of these protocols (e.g. opencode's Gemini
// routes) Just Works.
//
// Wire resolution precedence:
//  1. config.ProviderWire (--provider-wire flag) — always wins.
//  2. providerInfo.Wire — explicit declaration from the `providers` config
//     section.
//  3. npm-package heuristic (npmToWireProtocol), with per-model npm
//     overrides resolved first (e.g. opencode's claude-* uses
//     @ai-sdk/anthropic and its gemini-* uses @ai-sdk/google).
//  4. Providers with an api URL but no recognizable wire fall back to the
//     OpenAI-compatible wire.
//
// A provider absent from the registry is synthesized on the fly when both
// --provider-url and --provider-wire are given, so ad-hoc proxies work
// without a database entry or config override.
func autoRouteProvider(ctx context.Context, config *ProviderConfig, provider, modelName string, registry *ModelsRegistry) (*ProviderResult, error) {
	if config.ProviderWire != "" {
		if _, ok := parseWire(config.ProviderWire); !ok {
			return nil, fmt.Errorf("unknown wire protocol %q (expected one of: %s)", config.ProviderWire, wireNames())
		}
	}

	providerInfo := registry.GetProviderInfo(provider)
	if providerInfo == nil {
		// Unknown provider: synthesize an entry when the user supplied both
		// an endpoint and a wire — enough to route without a database entry.
		if config.ProviderURL != "" && config.ProviderWire != "" {
			providerInfo = &ProviderInfo{ID: provider, Name: provider}
		} else {
			return nil, fmt.Errorf("unsupported provider: %s (not found in model database; declare it in the `providers` config section or pass --provider-url with --provider-wire)", provider)
		}
	}

	// Resolve npm: per-model override > provider default.
	npmPackage := providerInfo.NPM
	if modelInfo := registry.LookupModel(provider, modelName); modelInfo != nil && modelInfo.ProviderNPM != "" {
		npmPackage = modelInfo.ProviderNPM
	}

	// Wire resolution: explicit flag > config override > npm heuristic.
	wire, known := parseWire(config.ProviderWire)
	if !known {
		wire, known = parseWire(providerInfo.Wire)
	}
	if !known {
		wire, known = npmToWireProtocol[npmPackage]
	}
	if !known {
		// Unknown npm but the provider has an API URL → assume OpenAI-compatible.
		// (Preserves the long-standing "any provider in models.dev with an api URL
		// is auto-routed through openaicompat" behaviour.)
		if providerInfo.API == "" {
			return nil, fmt.Errorf(
				"cannot auto-route provider %s: npm package %q has no known wire protocol "+
					"and the registry has no API URL (use --provider-url to override, or "+
					"declare a wire in the `providers` config section)",
				provider, npmPackage,
			)
		}
		wire = wireOpenAICompat
	}

	// All three wires use the provider's API URL from models.dev as the base.
	// When the registry has none, fall back to the SDK's hard-coded default for
	// this npm package (covers groq, cerebras, mistral, x.ai, etc. — providers
	// whose JS SDK ships a built-in baseURL that models.dev doesn't restate).
	if config.ProviderURL == "" {
		if providerInfo.API != "" {
			config.ProviderURL = providerInfo.API
		} else if defaultURL, ok := sdkDefaultBaseURL[npmPackage]; ok {
			config.ProviderURL = defaultURL
			providerInfo.API = defaultURL // for downstream helpers that read info.API
		}
	}

	// Provider templates a runtime account/region/deployment segment into the
	// URL (cloudflare-ai-gateway, databricks, snowflake-cortex, gitlab,
	// sap-ai-core). Resolve via environment variables, or surface a targeted
	// error pointing the user at the right knobs.
	if resolved, err := resolveTemplatedAPIURL(config.ProviderURL, providerInfo); err != nil {
		return nil, err
	} else if resolved != "" {
		config.ProviderURL = resolved
		providerInfo.API = resolved
	}

	switch wire {
	case wireOpenAI:
		// The native OpenAI wire speaks the Responses API; openai-compatible
		// proxies (and unknown-npm fallbacks) use the chat-completions wire
		// via fantasy's openaicompat provider.
		return createAutoRoutedOpenAIProvider(ctx, config, modelName, providerInfo)
	case wireOpenAICompat:
		return createAutoRoutedOpenAICompatProvider(ctx, config, modelName, providerInfo)
	case wireAnthropic:
		return createAutoRoutedAnthropicProvider(ctx, config, modelName, providerInfo)
	case wireGoogle:
		return createAutoRoutedGoogleProvider(ctx, config, modelName, providerInfo)
	default:
		return nil, fmt.Errorf("internal error: unknown wire protocol for provider %s (npm: %s)", provider, npmPackage)
	}
}

// resolveAutoRouteAPIKey looks up the API key for an auto-routed provider,
// returning a uniform error message when none can be resolved.
func resolveAutoRouteAPIKey(config *ProviderConfig, info *ProviderInfo) (string, error) {
	apiKey := resolveAPIKey(config.ProviderAPIKey, info.Env)
	if apiKey == "" {
		if len(info.Env) == 0 {
			return "", fmt.Errorf("%s API key not provided. Use --provider-api-key, or declare apiKeyEnv for this provider in the `providers` config section", info.Name)
		}
		return "", fmt.Errorf("%s API key not provided. Use --provider-api-key or set %s",
			info.Name, strings.Join(info.Env, " / "))
	}
	return apiKey, nil
}

// wrapProviderErr produces the uniform "failed to create X provider/model: %w"
// error wrap used by every createXxxProvider path. kind is typically
// "provider" or "model".
func wrapProviderErr(name, kind string, err error) error {
	return fmt.Errorf("failed to create %s %s: %w", name, kind, err)
}

// headerRoundTripper adds default headers to every outgoing request. Headers
// already present on the request are not overwritten.
type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, v := range h.headers {
		// Presence-aware check: an explicitly set but empty header (e.g. a
		// deliberately cleared Authorization) must not be replaced with the
		// configured default.
		if _, exists := req.Header[http.CanonicalHeaderKey(k)]; !exists {
			req.Header.Set(k, v)
		}
	}
	return h.base.RoundTrip(req)
}

// withDefaultHeaders wraps client's transport so that headers are added to
// every request. A nil client with non-empty headers produces a fresh
// client; a nil client with no headers stays nil (callers skip the
// WithHTTPClient option in that case).
func withDefaultHeaders(client *http.Client, headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return client
	}
	if client == nil {
		client = &http.Client{}
	}
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	client.Transport = &headerRoundTripper{base: base, headers: headers}
	return client
}

// autoRouteHTTPClient builds the HTTP client for an auto-routed provider,
// combining TLS verification config with the provider's default headers.
// Returns nil when no customization is needed (callers use the SDK default).
func autoRouteHTTPClient(config *ProviderConfig, info *ProviderInfo) *http.Client {
	var client *http.Client
	if config.TLSSkipVerify {
		client = createHTTPClientWithTLSConfig(true)
	}
	return withDefaultHeaders(client, info.Headers)
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

	apiKey, err := resolveAutoRouteAPIKey(config, info)
	if err != nil {
		return nil, err
	}

	var opts []openaicompat.Option
	opts = append(opts, openaicompat.WithBaseURL(apiURL))
	opts = append(opts, openaicompat.WithAPIKey(apiKey))
	opts = append(opts, openaicompat.WithName(info.ID))

	if httpClient := autoRouteHTTPClient(config, info); httpClient != nil {
		opts = append(opts, openaicompat.WithHTTPClient(httpClient))
	}

	p, err := openaicompat.New(opts...)
	if err != nil {
		return nil, wrapProviderErr(info.Name, "provider", err)
	}

	model, err := p.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr(info.Name, "model", err)
	}

	return &ProviderResult{Model: model}, nil
}

// createAutoRoutedAnthropicProvider creates an anthropic provider for
// third-party providers with anthropic-compatible APIs (e.g. minimax).
func createAutoRoutedAnthropicProvider(ctx context.Context, config *ProviderConfig, modelName string, info *ProviderInfo) (*ProviderResult, error) {
	clearConflictingAnthropicSamplingParams(config)

	apiKey, err := resolveAutoRouteAPIKey(config, info)
	if err != nil {
		return nil, err
	}

	var opts []anthropic.Option
	opts = append(opts, anthropic.WithAPIKey(apiKey))

	if config.ProviderURL != "" {
		// The anthropic client appends "/v1/messages" to the base URL.
		// If the provider URL ends with "/v1", strip it to avoid double "/v1/v1" paths.
		baseURL := strings.TrimSuffix(config.ProviderURL, "/v1")
		opts = append(opts, anthropic.WithBaseURL(baseURL))
	}

	if httpClient := autoRouteHTTPClient(config, info); httpClient != nil {
		opts = append(opts, anthropic.WithHTTPClient(httpClient))
	}

	p, err := anthropic.New(opts...)
	if err != nil {
		return nil, wrapProviderErr(info.Name, "provider", err)
	}

	model, err := p.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr(info.Name, "model", err)
	}

	return &ProviderResult{Model: model}, nil
}

// createAutoRoutedOpenAIProvider creates an openai provider for
// third-party providers with openai-compatible APIs.
func createAutoRoutedOpenAIProvider(ctx context.Context, config *ProviderConfig, modelName string, info *ProviderInfo) (*ProviderResult, error) {
	apiKey, err := resolveAutoRouteAPIKey(config, info)
	if err != nil {
		return nil, err
	}

	var opts []openai.Option
	opts = append(opts, openai.WithAPIKey(apiKey))
	opts = append(opts, openai.WithUseResponsesAPI())

	if config.ProviderURL != "" {
		opts = append(opts, openai.WithBaseURL(config.ProviderURL))
	}

	if httpClient := autoRouteHTTPClient(config, info); httpClient != nil {
		opts = append(opts, openai.WithHTTPClient(httpClient))
	}

	p, err := openai.New(opts...)
	if err != nil {
		return nil, wrapProviderErr(info.Name, "provider", err)
	}

	model, err := p.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr(info.Name, "model", err)
	}

	providerOpts := buildOpenAIProviderOptions(config, modelName)

	return &ProviderResult{Model: model, ProviderOptions: providerOpts}, nil
}

// createAutoRoutedGoogleProvider creates a Google (Gemini) provider for
// third-party providers that expose a Gemini-compatible API (e.g. opencode's
// Gemini routes, which carry an @ai-sdk/google per-model override).
//
// The underlying genai SDK always injects its own API version segment
// ("v1beta") between the base URL and the resource path. When the proxy's
// base URL from models.dev already carries a version segment (e.g. opencode's
// https://opencode.ai/zen/v1), that produces a doubled ".../v1/v1beta/..."
// path that the proxy rejects. In that case we install a transport that
// strips the injected segment so the proxy's own version is used.
func createAutoRoutedGoogleProvider(ctx context.Context, config *ProviderConfig, modelName string, info *ProviderInfo) (*ProviderResult, error) {
	apiKey, err := resolveAutoRouteAPIKey(config, info)
	if err != nil {
		return nil, err
	}

	opts := []google.Option{
		google.WithGeminiAPIKey(apiKey),
		google.WithName(info.ID),
	}

	if config.ProviderURL != "" {
		opts = append(opts, google.WithBaseURL(config.ProviderURL))
	}

	// Decide whether the genai-injected version segment needs stripping.
	var httpClient *http.Client
	if basePath := versionedBasePath(config.ProviderURL); basePath != "" {
		httpClient = newGeminiProxyHTTPClient(basePath, config.TLSSkipVerify)
	} else if config.TLSSkipVerify {
		httpClient = createHTTPClientWithTLSConfig(true)
	}
	httpClient = withDefaultHeaders(httpClient, info.Headers)
	if httpClient != nil {
		opts = append(opts, google.WithHTTPClient(httpClient))
	}

	p, err := google.New(opts...)
	if err != nil {
		return nil, wrapProviderErr(info.Name, "provider", err)
	}

	model, err := p.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr(info.Name, "model", err)
	}

	return &ProviderResult{Model: model}, nil
}

// versionSegmentRe matches a trailing API version segment in a URL path,
// e.g. "/v1", "/v1beta", "/v1beta1", "/v2alpha".
var versionSegmentRe = regexp.MustCompile(`/v\d+(?:beta\d*|alpha\d*)?$`)

// versionedBasePath returns the path component of rawURL when that path ends
// with an API version segment (e.g. opencode's ".../zen/v1" → "/zen/v1").
// It returns "" when rawURL is empty, unparseable, or has no version suffix
// — in which case the genai SDK's default version injection is correct and
// no rewriting is needed.
func versionedBasePath(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path := strings.TrimSuffix(u.Path, "/")
	if versionSegmentRe.MatchString(path) {
		return path
	}
	return ""
}

// newGeminiProxyHTTPClient builds an HTTP client whose transport strips the
// genai-injected version segment ("v1beta"/"v1beta1") that directly follows
// basePath, collapsing "{basePath}/v1beta/..." back to "{basePath}/...".
func newGeminiProxyHTTPClient(basePath string, skipVerify bool) *http.Client {
	var base http.RoundTripper
	if skipVerify {
		base = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	} else {
		base = http.DefaultTransport
	}
	return &http.Client{
		Transport: &geminiProxyTransport{base: base, basePath: basePath},
	}
}

// geminiProxyTransport removes the redundant API version segment that the
// genai SDK injects after a proxy base URL that already carries its own
// version segment.
type geminiProxyTransport struct {
	base     http.RoundTripper
	basePath string
}

func (t *geminiProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for _, injected := range []string{"/v1beta1", "/v1beta"} {
		prefix := t.basePath + injected + "/"
		if strings.HasPrefix(req.URL.Path, prefix) {
			newReq := req.Clone(req.Context())
			newReq.URL.Path = t.basePath + strings.TrimPrefix(req.URL.Path, t.basePath+injected)
			return t.base.RoundTrip(newReq)
		}
	}
	return t.base.RoundTrip(req)
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

// defaultRightSizeCap bounds auto-raised MaxTokens so that we don't silently
// allocate enormous output budgets for models with very high ceilings (e.g.
// Devstral at 262144, Mistral at 128000). Users who genuinely want more can
// pass --max-tokens explicitly or set modelSettings[...].maxTokens in config.
const defaultRightSizeCap = 32768

// rightSizeMaxTokens raises config.MaxTokens toward the model's known output
// ceiling when:
//   - the user has not explicitly set --max-tokens (or the KIT_MAX_TOKENS env
//     var, or the top-level max-tokens key in config.yaml), AND
//   - no per-model override already bumped MaxTokens (ApplyModelSettings runs
//     before this function), AND
//   - modelInfo.Limit.Output is known and larger than the current MaxTokens.
//
// The raised value is capped at defaultRightSizeCap to keep accidental
// allocations reasonable on very-large-output models. This prevents the
// common "ghost" where the agent's reply is silently truncated at the 8192
// default even though the selected model supports 64k or 262k output tokens.
func rightSizeMaxTokens(config *ProviderConfig, modelInfo *ModelInfo) {
	if modelInfo == nil || modelInfo.Limit.Output <= 0 {
		return
	}
	if isExplicitlySet(config.ConfigStore, "max-tokens") {
		return
	}
	target := min(modelInfo.Limit.Output, defaultRightSizeCap)
	if config.MaxTokens < target {
		config.MaxTokens = target
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
	case ThinkingNone:
		return new(openai.ReasoningEffortNone)
	case ThinkingMinimal:
		return new(openai.ReasoningEffortMinimal)
	case ThinkingLow:
		return new(openai.ReasoningEffortLow)
	case ThinkingMedium:
		return new(openai.ReasoningEffortMedium)
	case ThinkingHigh:
		return new(openai.ReasoningEffortHigh)
	default:
		return nil
	}
}

// IsValidThinkingLevelForModel checks if a thinking level is valid for the given
// model. Some OpenAI models like gpt-5.4 don't support "minimal" and require
// "none" instead.
func IsValidThinkingLevelForModel(level ThinkingLevel, modelName string) bool {
	if level == ThinkingOff {
		return true
	}

	// Check if this is an OpenAI model that doesn't support "minimal"
	// gpt-5.4 and newer gpt-5.x models use "none" instead of "minimal"
	if level == ThinkingMinimal {
		if strings.Contains(modelName, "gpt-5.4") ||
			strings.Contains(modelName, "gpt-5-pro") ||
			strings.Contains(modelName, "gpt-5-chat") {
			return false
		}
	}

	// Check if this is an OpenAI model that doesn't support "none"
	// Older gpt-5 models only support "minimal", not "none"
	if level == ThinkingNone {
		if strings.Contains(modelName, "gpt-5") &&
			!strings.Contains(modelName, "gpt-5.4") &&
			!strings.Contains(modelName, "gpt-5-pro") &&
			!strings.Contains(modelName, "gpt-5-chat") {
			// Older gpt-5 models might not support "none"
			// They only added "none" support in newer versions
			return false
		}
	}

	// All other levels are generally valid for reasoning models
	return true
}

// SuggestThinkingLevelFallback returns a recommended fallback level when the
// requested level is not valid for the model. Returns ThinkingOff if no
// suitable fallback exists.
func SuggestThinkingLevelFallback(level ThinkingLevel, modelName string) ThinkingLevel {
	if level == ThinkingMinimal && !IsValidThinkingLevelForModel(level, modelName) {
		// For models that don't support "minimal", suggest "none" (~same token budget)
		return ThinkingNone
	}
	if level == ThinkingNone && !IsValidThinkingLevelForModel(level, modelName) {
		// For models that don't support "none", suggest "minimal" (~same token budget)
		return ThinkingMinimal
	}
	return ThinkingOff
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
	if source == auth.CredentialSourceOAuth {
		httpClient := createOAuthHTTPClient(apiKey, config.TLSSkipVerify)
		opts = append(opts, anthropic.WithHTTPClient(httpClient))
		// Note: For OAuth, the API key is set as a placeholder; the transport handles auth
	} else if config.TLSSkipVerify {
		opts = append(opts, anthropic.WithHTTPClient(createHTTPClientWithTLSConfig(true)))
	}

	provider, err := anthropic.New(opts...)
	if err != nil {
		return nil, wrapProviderErr("Anthropic", "provider", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr("Anthropic", "model", err)
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
		return nil, wrapProviderErr("Vertex Anthropic", "provider", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr("Vertex Anthropic", "model", err)
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
		return nil, wrapProviderErr("OpenAI", "provider", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr("OpenAI", "model", err)
	}

	// Build provider options for OpenAI Responses API reasoning models.
	providerOpts := buildOpenAIProviderOptions(config, modelName)

	return &ProviderResult{Model: model, ProviderOptions: providerOpts}, nil
}

// createCopilotProvider builds a GitHub Copilot provider through fantasy's
// OpenAI-compatible provider. The catalog key is github-copilot, but the public
// model prefix may be either copilot/ or github-copilot/.
//
// Only gpt-* Copilot models are enabled here. The catalog also lists Claude and
// Gemini Copilot models, but those require different wire protocols and must be
// routed explicitly before they can be safely accepted.
func createCopilotProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	if !strings.HasPrefix(modelName, "gpt-") {
		return nil, fmt.Errorf("GitHub Copilot model %q is not supported yet: only gpt-* models use the OpenAI-compatible protocol", modelName)
	}

	cm, err := auth.NewCredentialManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize credential manager: %w", err)
	}

	token, err := cm.GetValidCopilotAccessTokenContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("GitHub Copilot credentials not available. Use 'kit auth login copilot': %w", err)
	}

	expiresAt := int64(0)
	if creds, err := cm.GetCopilotCredentials(); err == nil && creds != nil && creds.CopilotAccessToken == token {
		expiresAt = creds.ExpiresAt
	}

	baseURL := copilotBaseURL
	if providerInfo := GetGlobalRegistry().GetProviderInfo(copilotProviderID); providerInfo != nil && providerInfo.API != "" {
		baseURL = providerInfo.API
	}
	if config.ProviderURL != "" {
		baseURL = config.ProviderURL
	}

	opts := []openai.Option{
		openai.WithName(copilotAliasProviderID),
		openai.WithBaseURL(baseURL),
		openai.WithAPIKey(token),
		openai.WithHTTPClient(createCopilotHTTPClient(token, expiresAt, config.TLSSkipVerify)),
		openai.WithUseResponsesAPI(),
		openai.WithResponsesAPIFunc(copilotUsesResponsesAPI),
		openai.WithObjectMode(fantasy.ObjectModeTool),
	}

	provider, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub Copilot provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub Copilot model: %w", err)
	}

	providerOpts := buildOpenAIProviderOptions(config, modelName)

	return &ProviderResult{Model: model, ProviderOptions: providerOpts}, nil
}

// copilotUsesResponsesAPI selects the OpenAI Responses API for Copilot models
// known to support it. Non-gpt models are rejected before provider creation.
func copilotUsesResponsesAPI(modelID string) bool {
	return strings.HasPrefix(modelID, "gpt-5")
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
		return nil, wrapProviderErr("OpenAI Codex", "provider", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr("OpenAI Codex", "model", err)
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

// createCopilotHTTPClient returns an HTTP client that injects Copilot-specific
// authorization and client metadata headers. The token and expiry are cached in
// the transport so streaming requests do not hit credentials.json on every
// RoundTrip; the credential manager is consulted only near expiry.
func createCopilotHTTPClient(token string, expiresAt int64, skipVerify bool) *http.Client {
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
		Transport: &copilotTransport{
			base:      base,
			token:     token,
			expiresAt: expiresAt,
		},
		Timeout: 120 * time.Second,
	}
}

// copilotTransport decorates requests for api.githubcopilot.com.
//
// It owns a cached Copilot access token. When the token is still valid, the hot
// path is in-memory only. Near expiry it refreshes through CredentialManager,
// which updates both the cache here and credentials.json.
type copilotTransport struct {
	base      http.RoundTripper
	token     string
	expiresAt int64
	mu        sync.Mutex
}

func (t *copilotTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token := t.cachedToken(req.Context())

	newReq := req.Clone(req.Context())
	newReq.Header.Set("Authorization", "Bearer "+token)
	newReq.Header.Set("Copilot-Integration-Id", copilotIntegrationID)
	newReq.Header.Set("Editor-Version", copilotEditorVersion)
	newReq.Header.Set("Editor-Plugin-Version", copilotEditorPluginVersion)
	newReq.Header.Set("Openai-Intent", copilotOpenAIIntent)
	newReq.Header.Set("User-Agent", copilotUserAgent)
	newReq.Header.Set("X-GitHub-Api-Version", copilotGitHubAPIVersion)

	return t.base.RoundTrip(newReq)
}

// cachedToken returns the cached token unless it is within the five-minute
// refresh window. Refresh errors fall back to the last token so the request can
// surface any authoritative auth failure from the Copilot API.
func (t *copilotTransport) cachedToken(ctx context.Context) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.expiresAt == 0 || time.Now().Unix() < t.expiresAt-300 {
		return t.token
	}

	cm, err := auth.NewCredentialManager()
	if err != nil {
		return t.token
	}

	fresh, err := cm.GetValidCopilotAccessTokenContext(ctx)
	if err != nil || fresh == "" {
		return t.token
	}

	t.token = fresh
	if creds, err := cm.GetCopilotCredentials(); err == nil && creds != nil && creds.CopilotAccessToken == fresh {
		t.expiresAt = creds.ExpiresAt
	}
	return t.token
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
		return nil, wrapProviderErr("Google", "provider", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr("Google", "model", err)
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
		return nil, wrapProviderErr("Azure OpenAI", "provider", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr("Azure OpenAI", "model", err)
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
		return nil, wrapProviderErr("OpenRouter", "provider", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr("OpenRouter", "model", err)
	}

	return &ProviderResult{Model: model}, nil
}

func createBedrockProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	var opts []bedrock.Option

	// Bedrock uses AWS SDK default credential chain (env vars, shared config, etc.)
	provider, err := bedrock.New(opts...)
	if err != nil {
		return nil, wrapProviderErr("Bedrock", "provider", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr("Bedrock", "model", err)
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
		return nil, wrapProviderErr("Vercel", "provider", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr("Vercel", "model", err)
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
		return nil, wrapProviderErr("custom", "provider", err)
	}

	apiModelName := modelName
	if modelInfo != nil && modelInfo.APIModelName != "" {
		apiModelName = modelInfo.APIModelName
	}

	model, err := p.LanguageModel(ctx, apiModelName)
	if err != nil {
		return nil, wrapProviderErr("custom", "model", err)
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
		return nil, wrapProviderErr("Ollama", "provider", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr("Ollama", "model", err)
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
		if err := pullOllamaModel(ctx, client, baseURL, modelName, config.ProgressReaderFunc); err != nil {
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
	if config.FrequencyPenalty != nil {
		options["frequency_penalty"] = *config.FrequencyPenalty
	}
	if config.PresencePenalty != nil {
		options["presence_penalty"] = *config.PresencePenalty
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

func pullOllamaModel(ctx context.Context, client *http.Client, baseURL, modelName string, progressFn func(io.Reader) io.ReadCloser) error {
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

	if progressFn != nil {
		pr := progressFn(resp.Body)
		defer func() { _ = pr.Close() }()
		_, err = io.ReadAll(pr)
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
