package kit

import (
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/kit/internal/config"
	"github.com/spf13/viper"
)

// setSDKDefaults registers the same viper defaults that the CLI sets via
// cobra flag bindings. This ensures the SDK behaves identically to the CLI
// even when cobra is not used.
func setSDKDefaults() {
	viper.SetDefault("model", "anthropic/claude-sonnet-4-5-20250929")
	viper.SetDefault("max-tokens", 4096)
	viper.SetDefault("temperature", 0.7)
	viper.SetDefault("top-p", 0.95)
	viper.SetDefault("top-k", 40)
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
	configNames := []string{".kit"}

	for _, name := range configNames {
		viper.SetConfigName(name)
		if err := viper.ReadInConfig(); err == nil {
			configPath := viper.ConfigFileUsed()
			if err := LoadConfigWithEnvSubstitution(configPath); err != nil {
				if strings.Contains(err.Error(), "environment variable substitution failed") {
					return fmt.Errorf("error reading config file '%s': %w", configPath, err)
				}
				continue
			}
			configLoaded = true
			break
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
