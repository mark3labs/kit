package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Variable substitution patterns
var envVarPattern = regexp.MustCompile(`\$\{env://([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)

// parseVariableWithDefault extracts variable name and default value
// Works for both ${var:-default} and ${env://var:-default} patterns
func parseVariableWithDefault(varPart string) (varName, defaultValue string, hasDefault bool) {
	// Handle the case where varPart is like "VAR:-default" or just "VAR"
	if strings.Contains(varPart, ":-") {
		parts := strings.SplitN(varPart, ":-", 2)
		return parts[0], parts[1], true
	}
	return varPart, "", false
}

// EnvSubstituter handles environment variable substitution in configuration strings,
// supporting both ${env://VAR} and ${env://VAR:-default} patterns.
type EnvSubstituter struct{}

// SubstituteEnvVars replaces ${env://VAR} and ${env://VAR:-default} patterns with environment variables.
// If a variable is not set and has a default value, the default is used. Returns an error
// if required variables (those without defaults) are not set.
func (e *EnvSubstituter) SubstituteEnvVars(content string) (string, error) {
	var errors []string

	result := envVarPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the variable part from ${env://VAR:-default}
		// Remove ${env:// prefix and } suffix
		varPart := strings.TrimPrefix(strings.TrimSuffix(match, "}"), "${env://")

		varName, defaultValue, hasDefault := parseVariableWithDefault(varPart)

		if envValue := os.Getenv(varName); envValue != "" {
			return envValue
		}

		if hasDefault {
			return defaultValue
		}

		errors = append(errors, fmt.Sprintf("required environment variable %s not set in %s", varName, match))
		return match // Keep original if error
	})

	if len(errors) > 0 {
		return "", fmt.Errorf("environment variable substitution failed: %s", strings.Join(errors, ", "))
	}

	return result, nil
}

// HasEnvVars checks if content contains environment variable patterns (${env://...}).
// This is useful for determining if substitution is needed before processing.
func HasEnvVars(content string) bool {
	return envVarPattern.MatchString(content)
}
