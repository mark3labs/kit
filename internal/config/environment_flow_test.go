package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestEnvironmentVariableFlow(t *testing.T) {
	// Create a test config file that demonstrates the full flow
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	configContent := `
mcpServers:
  github:
    type: local
    command: ["docker", "run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN=${env://GITHUB_TOKEN}", "ghcr.io/github/github-mcp-server"]
    environment:
      GITHUB_TOKEN: "${env://GITHUB_TOKEN}"
      DEBUG_MODE: "true"
      LOG_LEVEL: "${env://LOG_LEVEL:-info}"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set environment variables
	_ = os.Setenv("GITHUB_TOKEN", "ghp_test_token_123")
	_ = os.Setenv("LOG_LEVEL", "debug")
	defer func() {
		_ = os.Unsetenv("GITHUB_TOKEN")
		_ = os.Unsetenv("LOG_LEVEL")
	}()

	// Step 1: Load config with environment substitution (simulating what happens in cmd/root.go)
	viper.Reset()
	rawContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	substituter := &EnvSubstituter{}
	processedContent, err := substituter.SubstituteEnvVars(string(rawContent))
	if err != nil {
		t.Fatalf("Environment substitution failed: %v", err)
	}

	// Verify substitution happened correctly
	if !strings.Contains(processedContent, "ghp_test_token_123") {
		t.Error("Environment substitution didn't replace GITHUB_TOKEN in command")
	}

	// Step 2: Parse with viper
	viper.SetConfigType("yaml")
	err = viper.ReadConfig(strings.NewReader(processedContent))
	if err != nil {
		t.Fatalf("Viper failed to read config: %v", err)
	}

	// Step 3: Load and validate config (which includes case fixing)
	config, err := LoadAndValidateConfig()
	if err != nil {
		t.Fatalf("Failed to load and validate config: %v", err)
	}

	// Step 4: Verify the final result
	githubServer, exists := config.MCPServers["github"]
	if !exists {
		t.Fatal("github server not found")
	}

	// Check command has substituted value
	expectedInCommand := "GITHUB_PERSONAL_ACCESS_TOKEN=ghp_test_token_123"
	found := slices.Contains(githubServer.Command, expectedInCommand)
	if !found {
		t.Errorf("Expected '%s' in command, got: %v", expectedInCommand, githubServer.Command)
	}

	// Check environment variables are uppercase after case fixing
	expectedEnv := map[string]string{
		"GITHUB_TOKEN": "ghp_test_token_123",
		"DEBUG_MODE":   "true",
		"LOG_LEVEL":    "debug",
	}

	for key, expectedValue := range expectedEnv {
		actualValue, exists := githubServer.Environment[key]
		if !exists {
			t.Errorf("Environment variable %s not found. Available: %+v", key, githubServer.Environment)
		} else if actualValue != expectedValue {
			t.Errorf("Environment variable %s: expected '%s', got '%s'", key, expectedValue, actualValue)
		}
	}

	// Verify no lowercase versions exist
	for key := range githubServer.Environment {
		if key != strings.ToUpper(key) && strings.Contains(key, "_") {
			t.Errorf("Found lowercase environment variable with underscore: %s", key)
		}
	}
}
