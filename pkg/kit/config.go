package kit

import (
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/kit/internal/config"
	"github.com/spf13/viper"
)

// defaultSystemPrompt is the built-in system prompt used when no custom
// prompt is configured. It describes the available core tools and provides
// usage guidelines.
//
// NOTE: Keep this in sync with the CLI default in cmd/root.go (search for
// defaultSystemPrompt or system-prompt flag default). Changes here should
// generally be reflected there, and vice versa.
const defaultSystemPrompt = `You are an expert coding assistant operating inside kit, a coding agent harness. You help users by reading files, executing commands, editing code, and writing new files.

Available tools:
- read: Read file or directory contents (supports pagination via offset/limit)
- write: Create or overwrite files
- edit: Make surgical edits to files (find exact text and replace)
- bash: Execute bash commands with timeout support
- grep: Search file contents using regex patterns (respects .gitignore)
- find: Search for files by glob pattern (respects .gitignore)
- ls: List directory contents

In addition to the tools above, you may have access to other custom tools from MCP servers and extensions.

Guidelines:
- Prefer grep/find/ls tools over bash for file exploration (faster, respects .gitignore)
- Use read to examine files before editing
- Use edit for precise changes (old text must match exactly, including whitespace)
- Use write only for new files or complete rewrites
- When summarizing your actions, output plain text directly - do NOT use cat or bash to display what you did
- Be concise in your responses
- Show file paths clearly when working with files`

// sdkDefaultMaxTokens is the last-resort ceiling applied when the SDK caller
// has not configured max-tokens via Options, env, config, or a per-model
// default. It is intentionally applied on the *models.ProviderConfig struct
// (not via viper) so that viper.IsSet("max-tokens") remains false and the
// right-sizing + per-model-default paths continue to work.
const sdkDefaultMaxTokens = 4096

// setSDKDefaults registers viper defaults that match the CLI's cobra flag
// defaults for keys where SetDefault does not interfere with downstream
// viper.IsSet() checks.
//
// Keys that participate in "explicit vs unset" precedence downstream —
// max-tokens, temperature, top-p, top-k, frequency-penalty, presence-penalty,
// thinking-level — are deliberately NOT registered here. viper.SetDefault
// causes viper.IsSet() to return true, which would suppress per-model
// defaults (ApplyModelSettings) and automatic right-sizing (rightSizeMaxTokens)
// for every SDK-created Kit. Those defaults are instead applied:
//
//   - max-tokens: as a last-resort struct-level floor (sdkDefaultMaxTokens)
//     in kit.New() after BuildProviderConfig returns, when the resolved
//     value is still zero.
//   - thinking-level: handled implicitly by models.ParseThinkingLevel("")
//     which returns models.ThinkingOff.
//   - sampling params (temperature, top-p, top-k, frequency/presence-penalty):
//     left as nil pointers so provider libraries apply their own defaults.
func setSDKDefaults() {
	viper.SetDefault("model", "anthropic/claude-sonnet-4-5-20250929")
	viper.SetDefault("system-prompt", defaultSystemPrompt)
	viper.SetDefault("stream", true)
	viper.SetDefault("num-gpu-layers", -1)
	viper.SetDefault("main-gpu", 0)
}

// InitConfig initializes the viper configuration system.
// It searches for config files in standard locations and loads them with
// environment variable substitution.
//
// configFile: explicit config file path (empty = search defaults).
// debug: if true, print warnings about missing configs to stderr.
func InitConfig(configFile string, debug bool) error {
	if configFile != "" {
		return LoadConfigWithEnvSubstitution(configFile)
	}

	// Ensure a config file exists (create default if none found).
	if err := config.EnsureConfigExists(); err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "Warning: Could not create default config file: %v\n", err)
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}

	// Current directory has higher priority than home directory.
	viper.AddConfigPath(".")
	viper.AddConfigPath(home)

	configLoaded := false

	viper.SetConfigName(".kit")
	if err := viper.ReadInConfig(); err == nil {
		configPath := viper.ConfigFileUsed()
		if err := LoadConfigWithEnvSubstitution(configPath); err != nil {
			if strings.Contains(err.Error(), "environment variable substitution failed") {
				return fmt.Errorf("error reading config file '%s': %w", configPath, err)
			}
		} else {
			configLoaded = true
		}
	}

	if !configLoaded && debug {
		fmt.Fprintf(os.Stderr, "No config file found in current directory or home directory\n")
	}

	viper.SetEnvPrefix("KIT")
	viper.AutomaticEnv()
	return nil
}

// LoadConfigWithEnvSubstitution loads a config file with ${ENV_VAR} expansion.
func LoadConfigWithEnvSubstitution(configPath string) error {
	rawContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	substituter := &config.EnvSubstituter{}
	processedContent, err := substituter.SubstituteEnvVars(string(rawContent))
	if err != nil {
		return fmt.Errorf("config env substitution failed: %w", err)
	}

	configType := "yaml"
	if strings.HasSuffix(configPath, ".json") {
		configType = "json"
	}

	config.SetConfigPath(configPath)
	viper.SetConfigType(configType)
	return viper.ReadConfig(strings.NewReader(processedContent))
}
