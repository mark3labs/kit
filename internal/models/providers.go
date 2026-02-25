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
		"claude-sonnet-latest":   "claude-sonnet-4-20250514",
		"claude-4-opus-latest":   "claude-opus-4-20250514",
		"claude-4-sonnet-latest": "claude-sonnet-4-20250514",

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
}

// CreateProvider creates a fantasy LanguageModel based on the provider configuration.
// It validates the model, checks required environment variables, and initializes
// the appropriate provider.
//
// Supported providers: anthropic, openai, google, ollama, azure, google-vertex-anthropic,
// openrouter, bedrock
func CreateProvider(ctx context.Context, config *ProviderConfig) (*ProviderResult, error) {
	parts := strings.SplitN(config.ModelString, ":", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid model format. Expected provider:model, got %s", config.ModelString)
	}

	provider := parts[0]
	modelName := parts[1]

	// Resolve model aliases before validation (for OAuth compatibility)
	if provider == "anthropic" || provider == "google-vertex-anthropic" {
		modelName = resolveModelAlias(provider, modelName)
	}

	// Get the global registry for validation
	registry := GetGlobalRegistry()

	// Validate the model exists (skip for ollama as it's not in models.dev, and skip when using custom provider URL)
	if provider != "ollama" && config.ProviderURL == "" {
		modelInfo, err := registry.ValidateModel(provider, modelName)
		if err != nil {
			suggestions := registry.SuggestModels(provider, modelName)
			if len(suggestions) > 0 {
				return nil, fmt.Errorf("%v. Did you mean one of: %s", err, strings.Join(suggestions, ", "))
			}
			return nil, err
		}

		if err := registry.ValidateEnvironment(provider, config.ProviderAPIKey); err != nil {
			return nil, err
		}

		if err := validateModelConfig(config, modelInfo); err != nil {
			return nil, err
		}
	}

	switch provider {
	case "anthropic":
		return createAnthropicProvider(ctx, config, modelName)
	case "openai":
		return createOpenAIProvider(ctx, config, modelName)
	case "google":
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
	default:
		return nil, fmt.Errorf("unsupported provider: %s. Supported: anthropic, openai, google, ollama, azure, google-vertex-anthropic, openrouter, bedrock", provider)
	}
}

// validateModelConfig validates configuration parameters against model capabilities
func validateModelConfig(config *ProviderConfig, modelInfo *ModelInfo) error {
	if config.Temperature != nil && !modelInfo.Temperature {
		config.Temperature = nil
	}

	if config.MaxTokens > modelInfo.Limit.Output {
		return fmt.Errorf("max_tokens (%d) exceeds model's output limit (%d) for %s",
			config.MaxTokens, modelInfo.Limit.Output, modelInfo.ID)
	}

	return nil
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
		return nil, fmt.Errorf("Google Vertex project ID not provided. Set ANTHROPIC_VERTEX_PROJECT_ID, GOOGLE_CLOUD_PROJECT, or GCLOUD_PROJECT environment variable")
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
		return nil, fmt.Errorf("Google API key not provided. Use --provider-api-key flag or GOOGLE_API_KEY/GEMINI_API_KEY/GOOGLE_GENERATIVE_AI_API_KEY environment variable")
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
		return nil, fmt.Errorf("Azure OpenAI API key not provided. Use --provider-api-key flag or AZURE_OPENAI_API_KEY environment variable")
	}

	baseURL := config.ProviderURL
	if baseURL == "" {
		baseURL = os.Getenv("AZURE_OPENAI_BASE_URL")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("Azure OpenAI Base URL not provided. Use --provider-url flag or AZURE_OPENAI_BASE_URL environment variable")
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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to pull model (status %d): %s", resp.StatusCode, string(body))
	}

	if showProgress {
		progressReader := progress.NewProgressReader(resp.Body)
		defer progressReader.Close()
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
	defer resp.Body.Close()

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
