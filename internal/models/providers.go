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

	"github.com/mark3labs/mcphost/internal/auth"
	"github.com/mark3labs/mcphost/internal/ui/progress"
)

const (
	// ClaudeCodePrompt is the required system prompt for OAuth authentication.
	ClaudeCodePrompt = "You are Claude Code, Anthropic's official CLI for Claude."
)

// resolveModelAlias resolves model aliases to their full names using the registry
func resolveModelAlias(provider, modelName string) string {
	registry := GetGlobalRegistry()

	aliasMap := map[string]string{
		"claude-opus-latest":     "claude-opus-4-20250514",
		"claude-sonnet-latest":   "claude-sonnet-4-5-20250929",
		"claude-4-opus-latest":   "claude-opus-4-20250514",
		"claude-4-sonnet-latest": "claude-sonnet-4-5-20250929",

		"claude-3-5-haiku-latest":  "claude-3-5-haiku-20241022",
		"claude-3-5-sonnet-latest": "claude-3-5-sonnet-20241022",
		"claude-3-7-sonnet-latest": "claude-3-7-sonnet-20250219",
		"claude-3-opus-latest":     "claude-3-opus-20240229",
	}

	if resolved, exists := aliasMap[modelName]; exists {
		if _, err := registry.ValidateModel(provider, resolved); err == nil {
			return resolved
		}
	}

	return modelName
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

	// Legacy colon-separated format
	if strings.Contains(modelString, ":") {
		parts := strings.SplitN(modelString, ":", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			fmt.Fprintf(os.Stderr, "Warning: model format %q uses deprecated colon separator. Use %s/%s instead.\n",
				modelString, parts[0], parts[1])
			return parts[0], parts[1], nil
		}
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

	// Resolve model aliases (for OAuth compatibility)
	if provider == "anthropic" || provider == "google-vertex-anthropic" {
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

	// Validate environment variables
	if err := registry.ValidateEnvironment(provider, config.ProviderAPIKey); err != nil {
		return nil, err
	}

	// Validate config against known model limits when metadata is available
	if modelInfo != nil {
		validateModelConfig(config, modelInfo)
	}

	switch provider {
	case "anthropic":
		return createAnthropicProvider(ctx, config, modelName)
	case "openai":
		return createOpenAIProvider(ctx, config, modelName)
	case "google", "gemini":
		return createGoogleProvider(ctx, config, modelName)
	case "ollama":
		return createOllamaProvider(ctx, config, modelName)
	case "azure":
		return createAzureProvider(ctx, config, modelName)
	case "google-vertex-anthropic":
		return createVertexAnthropicProvider(ctx, config, modelName)
	case "openrouter":
		return createOpenRouterProvider(ctx, config, modelName)
	case "bedrock":
		return createBedrockProvider(ctx, config, modelName)
	case "vercel":
		return createVercelProvider(ctx, config, modelName)
	default:
		return autoRouteProvider(ctx, config, provider, modelName, registry)
	}
}

// autoRouteProvider attempts to create a provider by looking up its npm package
// in the models.dev database and routing through the appropriate fantasy provider.
// For openai-compatible providers, it uses the api URL from models.dev.
func autoRouteProvider(ctx context.Context, config *ProviderConfig, provider, modelName string, registry *ModelsRegistry) (*ProviderResult, error) {
	providerInfo := registry.GetProviderInfo(provider)
	if providerInfo == nil {
		return nil, fmt.Errorf("unsupported provider: %s (not found in model database)", provider)
	}

	// Determine the fantasy provider for this npm package
	fantasyProvider := npmToFantasyProvider[providerInfo.NPM]
	if fantasyProvider == "" && providerInfo.API != "" {
		// Unknown npm but has API URL → route through openaicompat
		fantasyProvider = "openaicompat"
	}

	switch fantasyProvider {
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
		return nil, fmt.Errorf("unsupported provider: %s (npm: %s has no fantasy mapping)", provider, providerInfo.NPM)
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
	apiKey := resolveAPIKey(config.ProviderAPIKey, info.Env)
	if apiKey == "" {
		return nil, fmt.Errorf("%s API key not provided. Use --provider-api-key or set %s",
			info.Name, strings.Join(info.Env, " / "))
	}

	var opts []anthropic.Option
	opts = append(opts, anthropic.WithAPIKey(apiKey))

	if config.ProviderURL != "" {
		opts = append(opts, anthropic.WithBaseURL(config.ProviderURL))
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

	return &ProviderResult{Model: model}, nil
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

func createAnthropicProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	apiKey, source, err := auth.GetAnthropicAPIKey(config.ProviderAPIKey)
	if err != nil {
		return nil, err
	}

	if os.Getenv("DEBUG") != "" || os.Getenv("MCPHOST_DEBUG") != "" {
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

	return &ProviderResult{Model: model}, nil
}

func createVertexAnthropicProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
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
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not provided. Use --provider-api-key flag or OPENAI_API_KEY environment variable")
	}

	var opts []openai.Option
	opts = append(opts, openai.WithAPIKey(apiKey))

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

	return &ProviderResult{Model: model}, nil
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
	newReq := req.Clone(req.Context())
	newReq.Header.Del("x-api-key")
	newReq.Header.Set("Authorization", "Bearer "+t.accessToken)
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
