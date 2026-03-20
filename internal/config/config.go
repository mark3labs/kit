package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// MCPServerConfig represents configuration for an MCP server, supporting both
// local (stdio) and remote (StreamableHTTP/SSE) server types.
// It maintains backward compatibility with legacy configuration formats.
type MCPServerConfig struct {
	Type          string            `json:"type"`
	Command       []string          `json:"command,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	URL           string            `json:"url,omitempty"`
	AllowedTools  []string          `json:"allowedTools,omitempty" yaml:"allowedTools,omitempty"`
	ExcludedTools []string          `json:"excludedTools,omitempty" yaml:"excludedTools,omitempty"`

	// Legacy fields for backward compatibility
	Transport string         `json:"transport,omitempty"`
	Args      []string       `json:"args,omitempty"`
	Env       map[string]any `json:"env,omitempty"`
	Headers   []string       `json:"headers,omitempty"`
}

// UnmarshalJSON handles both new and legacy config formats for backward compatibility.
// New format uses "type" field with "local", "remote", or "builtin" values.
// Legacy format uses "transport", "command", "args", and "env" fields.
func (s *MCPServerConfig) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as the new format
	type newFormat struct {
		Type          string            `json:"type"`
		Command       []string          `json:"command,omitempty"`
		Environment   map[string]string `json:"environment,omitempty"`
		URL           string            `json:"url,omitempty"`
		Headers       []string          `json:"headers,omitempty"`
		AllowedTools  []string          `json:"allowedTools,omitempty" yaml:"allowedTools,omitempty"`
		ExcludedTools []string          `json:"excludedTools,omitempty" yaml:"excludedTools,omitempty"`
	}

	// Also try legacy format
	type legacyFormat struct {
		Transport     string         `json:"transport,omitempty"`
		Command       string         `json:"command,omitempty"`
		Args          []string       `json:"args,omitempty"`
		Env           map[string]any `json:"env,omitempty"`
		URL           string         `json:"url,omitempty"`
		Headers       []string       `json:"headers,omitempty"`
		AllowedTools  []string       `json:"allowedTools,omitempty" yaml:"allowedTools,omitempty"`
		ExcludedTools []string       `json:"excludedTools,omitempty" yaml:"excludedTools,omitempty"`
	}

	// Try new format first
	var newConfig newFormat
	if err := json.Unmarshal(data, &newConfig); err == nil && newConfig.Type != "" {
		s.Type = newConfig.Type
		s.Command = newConfig.Command
		s.Environment = newConfig.Environment
		s.URL = newConfig.URL
		s.Headers = newConfig.Headers
		s.AllowedTools = newConfig.AllowedTools
		s.ExcludedTools = newConfig.ExcludedTools
		return nil
	}

	// Fall back to legacy format
	var legacyConfig legacyFormat
	if err := json.Unmarshal(data, &legacyConfig); err != nil {
		return err
	}

	// Convert legacy format to new format
	s.Transport = legacyConfig.Transport
	if legacyConfig.Command != "" {
		s.Command = append([]string{legacyConfig.Command}, legacyConfig.Args...)
	}
	s.Args = legacyConfig.Args
	s.Env = legacyConfig.Env
	s.URL = legacyConfig.URL
	s.Headers = legacyConfig.Headers
	s.AllowedTools = legacyConfig.AllowedTools
	s.ExcludedTools = legacyConfig.ExcludedTools

	// Infer type from legacy format for better compatibility
	// Only set Type when it doesn't change existing transport behavior
	if legacyConfig.Command != "" {
		s.Type = "local" // This maps to "stdio" which matches legacy behavior
	}
	// Don't set Type for URL-only configs to preserve legacy "sse" behavior
	// The URL will be handled by the legacy fallback logic in GetTransportType()

	return nil
}

// AdaptiveColor represents a color that adapts to light and dark themes.
// Either light or dark can be specified, or both for theme-aware coloring.
type AdaptiveColor struct {
	Light string `json:"light,omitempty" yaml:"light,omitempty"`
	Dark  string `json:"dark,omitempty" yaml:"dark,omitempty"`
}

// MarkdownThemeConfig defines color overrides for markdown rendering and
// syntax highlighting.
type MarkdownThemeConfig struct {
	Text    AdaptiveColor `json:"text,omitzero" yaml:"text,omitempty"`
	Muted   AdaptiveColor `json:"muted,omitzero" yaml:"muted,omitempty"`
	Heading AdaptiveColor `json:"heading,omitzero" yaml:"heading,omitempty"`
	Emph    AdaptiveColor `json:"emph,omitzero" yaml:"emph,omitempty"`
	Strong  AdaptiveColor `json:"strong,omitzero" yaml:"strong,omitempty"`
	Link    AdaptiveColor `json:"link,omitzero" yaml:"link,omitempty"`
	Code    AdaptiveColor `json:"code,omitzero" yaml:"code,omitempty"`
	Error   AdaptiveColor `json:"error,omitzero" yaml:"error,omitempty"`
	Keyword AdaptiveColor `json:"keyword,omitzero" yaml:"keyword,omitempty"`
	String  AdaptiveColor `json:"string,omitzero" yaml:"string,omitempty"`
	Number  AdaptiveColor `json:"number,omitzero" yaml:"number,omitempty"`
	Comment AdaptiveColor `json:"comment,omitzero" yaml:"comment,omitempty"`
}

// Theme defines the color scheme for the application UI with adaptive colors
// that support both light and dark modes.
type Theme struct {
	Primary     AdaptiveColor `json:"primary,omitzero" yaml:"primary,omitempty"`
	Secondary   AdaptiveColor `json:"secondary,omitzero" yaml:"secondary,omitempty"`
	Success     AdaptiveColor `json:"success,omitzero" yaml:"success,omitempty"`
	Warning     AdaptiveColor `json:"warning,omitzero" yaml:"warning,omitempty"`
	Error       AdaptiveColor `json:"error,omitzero" yaml:"error,omitempty"`
	Info        AdaptiveColor `json:"info,omitzero" yaml:"info,omitempty"`
	Text        AdaptiveColor `json:"text,omitzero" yaml:"text,omitempty"`
	Muted       AdaptiveColor `json:"muted,omitzero" yaml:"muted,omitempty"`
	VeryMuted   AdaptiveColor `json:"very-muted,omitzero" yaml:"very-muted,omitempty"`
	Background  AdaptiveColor `json:"background,omitzero" yaml:"background,omitempty"`
	Border      AdaptiveColor `json:"border,omitzero" yaml:"border,omitempty"`
	MutedBorder AdaptiveColor `json:"muted-border,omitzero" yaml:"muted-border,omitempty"`
	System      AdaptiveColor `json:"system,omitzero" yaml:"system,omitempty"`
	Tool        AdaptiveColor `json:"tool,omitzero" yaml:"tool,omitempty"`
	Accent      AdaptiveColor `json:"accent,omitzero" yaml:"accent,omitempty"`
	Highlight   AdaptiveColor `json:"highlight,omitzero" yaml:"highlight,omitempty"`

	// Diff block backgrounds
	DiffInsertBg  AdaptiveColor `json:"diff-insert-bg,omitzero" yaml:"diff-insert-bg,omitempty"`
	DiffDeleteBg  AdaptiveColor `json:"diff-delete-bg,omitzero" yaml:"diff-delete-bg,omitempty"`
	DiffEqualBg   AdaptiveColor `json:"diff-equal-bg,omitzero" yaml:"diff-equal-bg,omitempty"`
	DiffMissingBg AdaptiveColor `json:"diff-missing-bg,omitzero" yaml:"diff-missing-bg,omitempty"`

	// Code/output block backgrounds
	CodeBg   AdaptiveColor `json:"code-bg,omitzero" yaml:"code-bg,omitempty"`
	GutterBg AdaptiveColor `json:"gutter-bg,omitzero" yaml:"gutter-bg,omitempty"`
	WriteBg  AdaptiveColor `json:"write-bg,omitzero" yaml:"write-bg,omitempty"`

	// Markdown rendering and syntax highlighting
	Markdown MarkdownThemeConfig `json:"markdown,omitzero" yaml:"markdown,omitempty"`
}

// Config represents the complete application configuration including MCP servers,
// model settings, UI preferences, and API credentials. It supports both command-line
// flags and configuration file settings.
type Config struct {
	MCPServers     map[string]MCPServerConfig `json:"mcpServers" yaml:"mcpServers"`
	Model          string                     `json:"model,omitempty" yaml:"model,omitempty"`
	MaxSteps       int                        `json:"max-steps,omitempty" yaml:"max-steps,omitempty"`
	Debug          bool                       `json:"debug,omitempty" yaml:"debug,omitempty"`
	Compact        bool                       `json:"compact,omitempty" yaml:"compact,omitempty"`
	SystemPrompt   string                     `json:"system-prompt,omitempty" yaml:"system-prompt,omitempty"`
	ProviderAPIKey string                     `json:"provider-api-key,omitempty" yaml:"provider-api-key,omitempty"`
	ProviderURL    string                     `json:"provider-url,omitempty" yaml:"provider-url,omitempty"`
	Stream         *bool                      `json:"stream,omitempty" yaml:"stream,omitempty"`
	Theme          any                        `json:"theme" yaml:"theme"`
	// Model generation parameters
	MaxTokens     int      `json:"max-tokens,omitempty" yaml:"max-tokens,omitempty"`
	Temperature   *float32 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	TopP          *float32 `json:"top-p,omitempty" yaml:"top-p,omitempty"`
	TopK          *int32   `json:"top-k,omitempty" yaml:"top-k,omitempty"`
	StopSequences []string `json:"stop-sequences,omitempty" yaml:"stop-sequences,omitempty"`

	// Thinking / extended reasoning
	ThinkingLevel string `json:"thinking-level,omitempty" yaml:"thinking-level,omitempty"`

	// TLS configuration
	TLSSkipVerify bool `json:"tls-skip-verify,omitempty" yaml:"tls-skip-verify,omitempty"`
}

// GetTransportType returns the transport type for the server config, mapping
// simplified type names to actual transport protocols. Supports legacy format
// detection and automatic type inference from configuration.
func (s *MCPServerConfig) GetTransportType() string {
	// Legacy format support - check explicit transport first
	if s.Transport != "" {
		return s.Transport
	}

	// New simplified format
	if s.Type != "" {
		switch s.Type {
		case "local":
			return "stdio"
		case "remote":
			return "streamable"
		default:
			return s.Type
		}
	}

	// Backward compatibility: infer transport type
	if len(s.Command) > 0 {
		return "stdio"
	}
	if s.URL != "" {
		return "sse"
	}
	return "stdio" // default
}

// Validate validates the configuration, ensuring required fields are present
// for each server type and that tool filters are used correctly. Returns an
// error describing any validation failures.
func (c *Config) Validate() error {
	for serverName, serverConfig := range c.MCPServers {
		if len(serverConfig.AllowedTools) > 0 && len(serverConfig.ExcludedTools) > 0 {
			return fmt.Errorf("server %s: allowedTools and excludedTools are mutually exclusive", serverName)
		}

		transport := serverConfig.GetTransportType()
		switch transport {
		case "stdio":
			// Check both new and legacy command formats
			if len(serverConfig.Command) == 0 && serverConfig.Transport == "" {
				return fmt.Errorf("server %s: command is required for stdio transport", serverName)
			}
		case "sse", "streamable":
			if serverConfig.URL == "" {
				return fmt.Errorf("server %s: url is required for %s transport", serverName, transport)
			}
		default:
			return fmt.Errorf("server %s: unsupported transport type '%s'. Supported types: stdio, sse, streamable", serverName, transport)
		}
	}
	return nil
}

// LoadSystemPrompt loads system prompt from file or returns the string directly.
// If input is a path to an existing file, its contents are read and returned.
// Otherwise, the input string is returned as-is.
func LoadSystemPrompt(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	// Check if input is a file that exists
	if _, err := os.Stat(input); err == nil {
		// Read the entire file as plain text
		content, err := os.ReadFile(input)
		if err != nil {
			return "", fmt.Errorf("error reading system prompt file: %v", err)
		}
		return strings.TrimSpace(string(content)), nil
	}

	// Treat as direct string
	return input, nil
}

// EnsureConfigExists checks if a config file exists and creates a default one if not.
// It searches for .kit.{yml,yaml,json} files in the user's home directory.
// If none exist, creates a default .kit.yml with examples.
func EnsureConfigExists() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting home directory: %v", err)
	}

	// Check for existing config files
	configNames := []string{".kit"}
	configTypes := []string{"yml", "yaml", "json"}

	for _, configName := range configNames {
		for _, configType := range configTypes {
			configPath := filepath.Join(homeDir, configName+"."+configType)
			if _, err := os.Stat(configPath); err == nil {
				// Config file exists, no need to create
				return nil
			}
		}
	}

	// No config file found, create default
	return createDefaultConfig(homeDir)
}

// createDefaultConfig creates a default .kit.yml file in the user's home directory
func createDefaultConfig(homeDir string) error {
	configPath := filepath.Join(homeDir, ".kit.yml")

	// Create the file
	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("error creating config file: %v", err)
	}
	defer func() { _ = file.Close() }()

	// Write a comprehensive YAML template with examples
	content := `# KIT Configuration File
# All command-line flags can be configured here

# MCP Servers configuration (for external tool servers)
# Core tools (bash, read, write, edit, grep, find, ls) are built-in and always available.
# Add external MCP servers here for additional tools:
# mcpServers:
#   # Local MCP servers - run commands locally via stdio transport
#   filesystem:
#     type: "local"
#     command: ["npx", "@modelcontextprotocol/server-filesystem", "/tmp"]
#     environment:
#       DEBUG: "true"
#   
#   # Remote MCP servers - connect via StreamableHTTP transport
#   websearch:
#     type: "remote"
#     url: "https://api.example.com/mcp"

mcpServers:

# Application settings (all optional)
# model: "anthropic/claude-sonnet-4-5-20250929"  # Default model to use
# max-steps: 10                                # Maximum agent steps (0 for unlimited)
# debug: false                                 # Enable debug logging
# system-prompt: "/path/to/system-prompt.txt" # System prompt text file

# Model generation parameters (all optional)
# max-tokens: 4096                             # Maximum tokens in response
# temperature: 0.7                             # Randomness (0.0-1.0)
# top-p: 0.95                                  # Nucleus sampling (0.0-1.0)
# top-k: 40                                    # Top K sampling
# stop-sequences: ["Human:", "Assistant:"]     # Custom stop sequences

# API Configuration (can also use environment variables)
# provider-api-key: "your-api-key"         # API key for OpenAI, Anthropic, or Google
# provider-url: "https://api.openai.com/v1" # Base URL for OpenAI, Anthropic, or Ollama
`

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("error writing config content: %v", err)
	}

	return nil
}

// FilepathOr reads a configuration value that can be either a direct value or a
// filepath to a JSON/YAML file containing the value. If the value is a string
// starting with "~/" or a relative path, it's expanded to an absolute path.
// The contents of the file are then unmarshaled into the provided value pointer.
func FilepathOr[T any](key string, value *T) error {
	var field any
	err := viper.UnmarshalKey(key, &field)
	if err != nil {
		return err
	}
	switch f := field.(type) {
	case string:
		{
			absPath := f
			if strings.HasPrefix(absPath, "~/") {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				filepath.Join(home, absPath[2:])
			}
			if !filepath.IsAbs(absPath) {
				// base := GetConfigPath()
				base := configPath
				if base == "" {
					fmt.Fprintf(os.Stderr, "unable to build relative path to config.")
					os.Exit(1)
				}
				absPath = filepath.Join(filepath.Dir(base), absPath)
			}
			b, err := os.ReadFile(absPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%q", err)
				os.Exit(1)
			}
			switch filepath.Ext(absPath) {
			case ".json":
				return json.Unmarshal(b, value)
			case ".yaml", ".yml":
				return yaml.Unmarshal(b, value)
			}
		}
	case map[string]any:
		return viper.UnmarshalKey(key, value)
	default:
		return fmt.Errorf("invalid type for field %q", key)
	}
	return nil
}

var configPath string

// SetConfigPath sets the configuration file path for resolving relative paths
// in configuration values. This should be called when the configuration file
// location is known.
func SetConfigPath(path string) {
	configPath = path
}
