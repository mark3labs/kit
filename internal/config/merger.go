package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// LoadAndValidateConfig loads configuration from the process-global viper
// store, fixes environment variable casing issues, and validates the
// configuration. Returns an error if loading or validation fails.
//
// This is a convenience wrapper around [LoadAndValidateConfigFrom] using the
// shared global store; it is retained for the CLI and other callers that rely
// on viper's process-global state.
func LoadAndValidateConfig() (*Config, error) {
	return LoadAndValidateConfigFrom(viper.GetViper())
}

// LoadAndValidateConfigFrom loads configuration from the supplied per-instance
// store, fixes environment variable casing issues, and validates the
// configuration. When v is nil, the process-global store is used. Threading an
// explicit store lets each Kit instance own an isolated configuration without
// clobbering other instances in the same process.
func LoadAndValidateConfigFrom(v *viper.Viper) (*Config, error) {
	if v == nil {
		v = viper.GetViper()
	}
	config := &Config{
		MCPServers: make(map[string]MCPServerConfig),
	}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Fix environment variable case sensitivity issue
	// Viper lowercases all keys, but we need to preserve the original case for environment variables
	fixEnvironmentCase(v, config)

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// fixEnvironmentCase fixes the case of environment variable keys that were lowercased by Viper
func fixEnvironmentCase(v *viper.Viper, config *Config) {
	// Get the raw config data from viper
	rawConfig := v.AllSettings()

	// Check if we have mcpServers in the raw config
	if mcpServersRaw, ok := rawConfig["mcpservers"]; ok {
		if mcpServersMap, ok := mcpServersRaw.(map[string]any); ok {
			// Iterate through each server
			for serverName, serverDataRaw := range mcpServersMap {
				if serverData, ok := serverDataRaw.(map[string]any); ok {
					// Check if this server has an environment field
					if _, hasEnv := serverData["environment"]; hasEnv {
						// Get the server config from our parsed config
						if serverConfig, exists := config.MCPServers[serverName]; exists {
							// Create a new environment map with proper casing
							newEnv := make(map[string]string)

							// For each environment variable, check if it should be uppercase
							for key, value := range serverConfig.Environment {
								// Convert to uppercase if it looks like an environment variable
								// (contains underscore or is all uppercase in typical usage)
								upperKey := strings.ToUpper(key)
								if strings.Contains(key, "_") || key == strings.ToLower(upperKey) {
									newEnv[upperKey] = value
								} else {
									newEnv[key] = value
								}
							}

							// Update the server config with the fixed environment map
							serverConfig.Environment = newEnv
							config.MCPServers[serverName] = serverConfig
						}
					}
				}
			}
		}
	}
}
