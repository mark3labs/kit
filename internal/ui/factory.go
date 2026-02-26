package ui

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcphost/internal/auth"
	"github.com/mark3labs/mcphost/internal/models"
)

// AgentInterface defines the minimal interface required from the agent package
// to avoid circular dependencies while still accessing necessary agent functionality.
type AgentInterface interface {
	GetLoadingMessage() string
	GetTools() []any                // Using any to avoid importing tool types
	GetLoadedServerNames() []string // Add this method for debug config
}

// CLISetupOptions encapsulates all configuration parameters needed to initialize
// and set up a CLI instance, including display preferences, model information,
// and debugging settings.
type CLISetupOptions struct {
	Agent          AgentInterface
	ModelString    string
	Debug          bool
	Compact        bool
	Quiet          bool
	ShowDebug      bool   // Whether to show debug config
	ProviderAPIKey string // For OAuth detection
}

// parseModelName extracts provider and model name from model string
func parseModelName(modelString string) (provider, model string) {
	p, m, err := models.ParseModelString(modelString)
	if err != nil {
		return "unknown", "unknown"
	}
	return p, m
}

// CreateUsageTracker creates a UsageTracker for the given model string and
// provider API key. It returns nil when usage tracking is unavailable (e.g.
// ollama or unrecognised models). This is used by the interactive TUI path
// which doesn't go through SetupCLI.
func CreateUsageTracker(modelString, providerAPIKey string) *UsageTracker {
	provider, model := parseModelName(modelString)
	if provider == "unknown" || model == "unknown" || provider == "ollama" {
		return nil
	}

	registry := models.GetGlobalRegistry()
	modelInfo, err := registry.ValidateModel(provider, model)
	if err != nil {
		return nil
	}

	isOAuth := false
	if provider == "anthropic" {
		_, source, err := auth.GetAnthropicAPIKey(providerAPIKey)
		if err == nil && strings.HasPrefix(source, "stored OAuth") {
			isOAuth = true
		}
	}

	return NewUsageTracker(modelInfo, provider, 80, isOAuth)
}

// SetupCLI creates, configures, and initializes a CLI instance with the provided
// options. It sets up model display, usage tracking for supported providers, and
// shows initial loading information. Returns nil in quiet mode or an initialized
// CLI instance ready for user interaction.
func SetupCLI(opts *CLISetupOptions) (*CLI, error) {
	if opts.Quiet {
		return nil, nil // No CLI in quiet mode
	}

	cli, err := NewCLI(opts.Debug, opts.Compact)
	if err != nil {
		return nil, fmt.Errorf("failed to create CLI: %v", err)
	}

	// Parse model string for display and usage tracking
	provider, model := parseModelName(opts.ModelString)

	// Set the model name for consistent display
	if model != "unknown" {
		cli.SetModelName(model)
	}

	// Set up usage tracking for supported providers
	if provider != "unknown" && model != "unknown" {
		// Skip usage tracking for ollama as it's not in models.dev
		if provider != "ollama" {
			registry := models.GetGlobalRegistry()
			if modelInfo, err := registry.ValidateModel(provider, model); err == nil {
				// Check if OAuth credentials are being used for Anthropic models
				isOAuth := false
				if provider == "anthropic" {
					_, source, err := auth.GetAnthropicAPIKey(opts.ProviderAPIKey)
					if err == nil && strings.HasPrefix(source, "stored OAuth") {
						isOAuth = true
					}
				}

				usageTracker := NewUsageTracker(modelInfo, provider, 80, isOAuth) // Will be updated with actual width
				cli.SetUsageTracker(usageTracker)
			}
		}
	}

	fmt.Println("")

	// Display model info
	if provider != "unknown" && model != "unknown" {
		cli.DisplayInfo(fmt.Sprintf("Model loaded: %s (%s)", provider, model))
	}

	// Display loading message if available (e.g., GPU fallback info)
	if loadingMessage := opts.Agent.GetLoadingMessage(); loadingMessage != "" {
		cli.DisplayInfo(loadingMessage)
	}

	// Display tool count
	tools := opts.Agent.GetTools()
	cli.DisplayInfo(fmt.Sprintf("Loaded %d tools from MCP servers", len(tools)))

	// Display usage information (for both streaming and non-streaming)
	if !opts.Quiet && cli != nil {
		cli.DisplayUsageAfterResponse()
	}

	return cli, nil
}
