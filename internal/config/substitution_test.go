package config

import (
	"os"
	"testing"
)

func TestParseVariableWithDefault(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		expectedVar        string
		expectedDefault    string
		expectedHasDefault bool
	}{
		{
			name:               "variable without default",
			input:              "GITHUB_TOKEN",
			expectedVar:        "GITHUB_TOKEN",
			expectedDefault:    "",
			expectedHasDefault: false,
		},
		{
			name:               "variable with default",
			input:              "DEBUG:-false",
			expectedVar:        "DEBUG",
			expectedDefault:    "false",
			expectedHasDefault: true,
		},
		{
			name:               "variable with empty default",
			input:              "OPTIONAL:-",
			expectedVar:        "OPTIONAL",
			expectedDefault:    "",
			expectedHasDefault: true,
		},
		{
			name:               "variable with complex default",
			input:              "DATABASE_URL:-sqlite:///tmp/default.db",
			expectedVar:        "DATABASE_URL",
			expectedDefault:    "sqlite:///tmp/default.db",
			expectedHasDefault: true,
		},
		{
			name:               "variable with default containing colon",
			input:              "URL:-https://api.example.com:8080/path",
			expectedVar:        "URL",
			expectedDefault:    "https://api.example.com:8080/path",
			expectedHasDefault: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varName, defaultValue, hasDefault := parseVariableWithDefault(tt.input)

			if varName != tt.expectedVar {
				t.Errorf("Expected var name %s, got %s", tt.expectedVar, varName)
			}
			if defaultValue != tt.expectedDefault {
				t.Errorf("Expected default value %s, got %s", tt.expectedDefault, defaultValue)
			}
			if hasDefault != tt.expectedHasDefault {
				t.Errorf("Expected hasDefault %v, got %v", tt.expectedHasDefault, hasDefault)
			}
		})
	}
}

func TestEnvSubstituter_SubstituteEnvVars(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		envVars     map[string]string
		expected    string
		expectError bool
	}{
		{
			name:     "basic env substitution",
			input:    `{"token": "${env://GITHUB_TOKEN}"}`,
			envVars:  map[string]string{"GITHUB_TOKEN": "ghp_123"},
			expected: `{"token": "ghp_123"}`,
		},
		{
			name:     "env with default value used",
			input:    `{"debug": "${env://DEBUG:-false}"}`,
			envVars:  map[string]string{},
			expected: `{"debug": "false"}`,
		},
		{
			name:     "env with default value overridden",
			input:    `{"debug": "${env://DEBUG:-false}"}`,
			envVars:  map[string]string{"DEBUG": "true"},
			expected: `{"debug": "true"}`,
		},
		{
			name:     "env with empty default",
			input:    `{"optional": "${env://OPTIONAL:-}"}`,
			envVars:  map[string]string{},
			expected: `{"optional": ""}`,
		},
		{
			name:     "multiple env vars in same string",
			input:    `{"url": "${env://HOST:-localhost}:${env://PORT:-8080}"}`,
			envVars:  map[string]string{"HOST": "example.com"},
			expected: `{"url": "example.com:8080"}`,
		},
		{
			name:     "mixed env and script args (env processed first)",
			input:    `{"token": "${env://TOKEN:-default}", "name": "${username}"}`,
			envVars:  map[string]string{},
			expected: `{"token": "default", "name": "${username}"}`,
		},
		{
			name:     "complex default with special characters",
			input:    `{"db": "${env://DATABASE_URL:-sqlite:///tmp/default.db?cache=shared&mode=rwc}"}`,
			envVars:  map[string]string{},
			expected: `{"db": "sqlite:///tmp/default.db?cache=shared&mode=rwc"}`,
		},
		{
			name:     "no env vars in content",
			input:    `{"normal": "value", "script": "${arg}"}`,
			envVars:  map[string]string{},
			expected: `{"normal": "value", "script": "${arg}"}`,
		},
		{
			name:        "missing required env var",
			input:       `{"token": "${env://REQUIRED_TOKEN}"}`,
			envVars:     map[string]string{},
			expectError: true,
		},
		{
			name:        "multiple missing required env vars",
			input:       `{"token": "${env://TOKEN1}", "key": "${env://TOKEN2}"}`,
			envVars:     map[string]string{},
			expectError: true,
		},
		{
			name:     "yaml format",
			input:    "token: ${env://GITHUB_TOKEN:-default}\ndebug: ${env://DEBUG:-false}",
			envVars:  map[string]string{"GITHUB_TOKEN": "ghp_456"},
			expected: "token: ghp_456\ndebug: false",
		},
		{
			name:     "env var with underscores and numbers",
			input:    `{"var": "${env://MY_VAR_123}"}`,
			envVars:  map[string]string{"MY_VAR_123": "test_value"},
			expected: `{"var": "test_value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			originalEnv := make(map[string]string)
			for k, v := range tt.envVars {
				originalEnv[k] = os.Getenv(k)
				_ = os.Setenv(k, v)
			}

			// Clean up environment variables after test
			defer func() {
				for k := range tt.envVars {
					if originalValue, existed := originalEnv[k]; existed {
						_ = os.Setenv(k, originalValue)
					} else {
						_ = os.Unsetenv(k)
					}
				}
			}()

			substituter := &EnvSubstituter{}
			result, err := substituter.SubstituteEnvVars(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
				}
			}
		})
	}
}

func TestHasEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "has env vars",
			content:  `{"token": "${env://GITHUB_TOKEN}"}`,
			expected: true,
		},
		{
			name:     "has env vars with default",
			content:  `{"debug": "${env://DEBUG:-false}"}`,
			expected: true,
		},
		{
			name:     "no env vars",
			content:  `{"name": "${username}", "normal": "value"}`,
			expected: false,
		},
		{
			name:     "empty content",
			content:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasEnvVars(tt.content)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
