package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPServerConfig_NewFormat(t *testing.T) {
	// Test new simplified format
	jsonData := `{
		"type": "local",
		"command": ["bun", "x", "my-mcp-command"],
		"environment": {
			"MY_ENV_VAR": "my_env_var_value"
		}
	}`

	var config MCPServerConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal new format: %v", err)
	}

	if config.Type != "local" {
		t.Errorf("Expected type 'local', got '%s'", config.Type)
	}

	if len(config.Command) != 3 {
		t.Errorf("Expected 3 command parts, got %d", len(config.Command))
	}

	if config.Command[0] != "bun" || config.Command[1] != "x" || config.Command[2] != "my-mcp-command" {
		t.Errorf("Command parts incorrect: %v", config.Command)
	}

	if config.Environment["MY_ENV_VAR"] != "my_env_var_value" {
		t.Errorf("Environment variable not set correctly")
	}

	// Test transport type detection
	transportType := config.GetTransportType()
	if transportType != "stdio" {
		t.Errorf("Expected transport type 'stdio', got '%s'", transportType)
	}
}

func TestMCPServerConfig_RemoteFormat(t *testing.T) {
	// Test remote format
	jsonData := `{
		"type": "remote",
		"url": "https://my-mcp-server.com"
	}`

	var config MCPServerConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal remote format: %v", err)
	}

	if config.Type != "remote" {
		t.Errorf("Expected type 'remote', got '%s'", config.Type)
	}

	if config.URL != "https://my-mcp-server.com" {
		t.Errorf("Expected URL 'https://my-mcp-server.com', got '%s'", config.URL)
	}

	// Test transport type detection
	transportType := config.GetTransportType()
	if transportType != "streamable" {
		t.Errorf("Expected transport type 'streamable', got '%s'", transportType)
	}
}

func TestMCPServerConfig_LegacyFormat(t *testing.T) {
	// Test legacy format still works
	jsonData := `{
		"command": "npx",
		"args": ["@modelcontextprotocol/server-filesystem", "/path"],
		"env": {
			"MY_VAR": "value"
		}
	}`

	var config MCPServerConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal legacy format: %v", err)
	}

	// Verify Type field is now set correctly for legacy format
	if config.Type != "local" {
		t.Errorf("Expected type 'local' for legacy command format, got '%s'", config.Type)
	}

	if len(config.Command) != 3 {
		t.Errorf("Expected 3 command parts, got %d", len(config.Command))
	}

	if config.Command[0] != "npx" || config.Command[1] != "@modelcontextprotocol/server-filesystem" || config.Command[2] != "/path" {
		t.Errorf("Command parts incorrect: %v", config.Command)
	}

	if config.Env["MY_VAR"] != "value" {
		t.Errorf("Legacy environment variable not set correctly")
	}

	// Test transport type detection
	transportType := config.GetTransportType()
	if transportType != "stdio" {
		t.Errorf("Expected transport type 'stdio', got '%s'", transportType)
	}
}

func TestMCPServerConfig_LocalFormat(t *testing.T) {
	jsonData := `{
		"type": "local",
		"command": ["npx", "@modelcontextprotocol/server-filesystem", "/tmp"]
	}`

	var config MCPServerConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal local format: %v", err)
	}

	if config.Type != "local" {
		t.Errorf("Expected type 'local', got '%s'", config.Type)
	}

	transportType := config.GetTransportType()
	if transportType != "stdio" {
		t.Errorf("Expected transport type 'stdio', got '%s'", transportType)
	}
}

func TestConfig_Validate(t *testing.T) {
	config := &Config{
		MCPServers: map[string]MCPServerConfig{
			"local-server": {
				Type:    "local",
				Command: []string{"echo", "hello"},
			},
			"remote-server": {
				Type: "remote",
				URL:  "https://example.com",
			},
			"another-local": {
				Type:    "local",
				Command: []string{"echo", "world"},
			},
		},
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}
}

func TestEnsureConfigExists(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "kit_config_test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tempDir)
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	// Test config creation
	err = EnsureConfigExists()
	if err != nil {
		t.Fatalf("Error creating config: %v", err)
	}

	// Verify the config file was created
	configPath := filepath.Join(tempDir, ".kit.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file was not created at %s", configPath)
	}

	// Read and verify the content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Error reading config: %v", err)
	}

	contentStr := string(content)

	// Verify it contains the expected sections
	expectedSections := []string{
		"# KIT Configuration File",
		"mcpServers:",
		"# Local MCP servers",
		"# Remote MCP servers",
		"type: \"local\"",
		"type: \"remote\"",
		"Core tools",
	}

	for _, expected := range expectedSections {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("Config content missing expected section: %s", expected)
		}
	}

	// Verify it's valid YAML structure (basic check)
	if !strings.Contains(contentStr, "mcpServers:") {
		t.Error("Config should contain mcpServers section")
	}
}

func TestMCPServerConfig_LegacyFormatTypeInference(t *testing.T) {
	tests := []struct {
		name              string
		jsonData          string
		expectedType      string
		expectedTransport string
	}{
		{
			name: "Legacy command format should infer local type",
			jsonData: `{
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
			}`,
			expectedType:      "local",
			expectedTransport: "stdio",
		},
		{
			name: "Legacy URL format should preserve legacy behavior",
			jsonData: `{
				"url": "https://api.example.com/mcp"
			}`,
			expectedType:      "", // Don't set Type to preserve legacy transport behavior
			expectedTransport: "sse",
		},
		{
			name: "Legacy format with explicit transport should still infer type",
			jsonData: `{
				"command": "python",
				"args": ["-m", "my_mcp_server"],
				"transport": "stdio"
			}`,
			expectedType:      "local",
			expectedTransport: "stdio",
		},
		{
			name: "Legacy format with URL and explicit transport should preserve explicit transport",
			jsonData: `{
				"url": "https://remote-server.com",
				"transport": "sse"
			}`,
			expectedType:      "", // Don't set Type to preserve legacy behavior
			expectedTransport: "sse",
		},
		{
			name: "Empty legacy format should not set type",
			jsonData: `{
				"env": {"VAR": "value"}
			}`,
			expectedType:      "",
			expectedTransport: "stdio",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config MCPServerConfig
			err := json.Unmarshal([]byte(tt.jsonData), &config)
			if err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if config.Type != tt.expectedType {
				t.Errorf("Expected type '%s', got '%s'", tt.expectedType, config.Type)
			}

			transportType := config.GetTransportType()
			if transportType != tt.expectedTransport {
				t.Errorf("Expected transport type '%s', got '%s'", tt.expectedTransport, transportType)
			}
		})
	}
}

func TestIssue76_ExactReproduction(t *testing.T) {
	// Test the exact config from GitHub issue #76
	jsonData := `{
		"mcpServers": {
			"filesystem": {
				"command": "npx",
				"args": [
					"-y",
					"@modelcontextprotocol/server-filesystem",
					"C:\\test"
				]
			}
		}
	}`

	var cfg Config
	err := json.Unmarshal([]byte(jsonData), &cfg)
	if err != nil {
		t.Fatalf("Error unmarshaling config: %v", err)
	}

	// Verify the server config was parsed correctly
	if len(cfg.MCPServers) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(cfg.MCPServers))
	}

	serverConfig, exists := cfg.MCPServers["filesystem"]
	if !exists {
		t.Fatal("Expected 'filesystem' server to exist")
	}

	// Verify Type field is now set correctly
	if serverConfig.Type != "local" {
		t.Errorf("Expected type 'local', got '%s'", serverConfig.Type)
	}

	// Verify command was parsed correctly
	expectedCommand := []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "C:\\test"}
	if len(serverConfig.Command) != len(expectedCommand) {
		t.Errorf("Expected %d command parts, got %d", len(expectedCommand), len(serverConfig.Command))
	}
	for i, expected := range expectedCommand {
		if i >= len(serverConfig.Command) || serverConfig.Command[i] != expected {
			t.Errorf("Command part %d: expected '%s', got '%s'", i, expected, serverConfig.Command[i])
		}
	}

	// Verify transport type detection works
	transportType := serverConfig.GetTransportType()
	if transportType != "stdio" {
		t.Errorf("Expected transport type 'stdio', got '%s'", transportType)
	}

	// Most importantly, verify validation passes
	err = cfg.Validate()
	if err != nil {
		t.Errorf("Validation should pass but failed with: %v", err)
	}
}

func TestMCPServerConfig_RemoteFormatWithHeaders(t *testing.T) {
	tests := []struct {
		name              string
		jsonData          string
		expectedType      string
		expectedURL       string
		expectedHeaders   []string
		expectedTransport string
	}{
		{
			name: "Remote server with headers",
			jsonData: `{
				"type": "remote",
				"url": "https://api.example.com/mcp",
				"headers": ["Authorization: Bearer token123", "X-API-Key: key456"]
			}`,
			expectedType:      "remote",
			expectedURL:       "https://api.example.com/mcp",
			expectedHeaders:   []string{"Authorization: Bearer token123", "X-API-Key: key456"},
			expectedTransport: "streamable",
		},
		{
			name: "Remote server without headers",
			jsonData: `{
				"type": "remote",
				"url": "https://api.example.com/mcp"
			}`,
			expectedType:      "remote",
			expectedURL:       "https://api.example.com/mcp",
			expectedHeaders:   nil,
			expectedTransport: "streamable",
		},
		{
			name: "Legacy remote server with headers",
			jsonData: `{
				"url": "https://legacy.example.com/mcp",
				"headers": ["Content-Type: application/json", "User-Agent: KIT/1.0"]
			}`,
			expectedType:      "",
			expectedURL:       "https://legacy.example.com/mcp",
			expectedHeaders:   []string{"Content-Type: application/json", "User-Agent: KIT/1.0"},
			expectedTransport: "sse",
		},
		{
			name: "Legacy remote server with explicit transport and headers",
			jsonData: `{
				"url": "https://legacy.example.com/mcp",
				"transport": "sse",
				"headers": ["Authorization: Basic dXNlcjpwYXNz"]
			}`,
			expectedType:      "",
			expectedURL:       "https://legacy.example.com/mcp",
			expectedHeaders:   []string{"Authorization: Basic dXNlcjpwYXNz"},
			expectedTransport: "sse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config MCPServerConfig
			err := json.Unmarshal([]byte(tt.jsonData), &config)
			if err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if config.Type != tt.expectedType {
				t.Errorf("Expected type '%s', got '%s'", tt.expectedType, config.Type)
			}

			if config.URL != tt.expectedURL {
				t.Errorf("Expected URL '%s', got '%s'", tt.expectedURL, config.URL)
			}

			if len(config.Headers) != len(tt.expectedHeaders) {
				t.Errorf("Expected %d headers, got %d", len(tt.expectedHeaders), len(config.Headers))
			}

			for i, expectedHeader := range tt.expectedHeaders {
				if i >= len(config.Headers) || config.Headers[i] != expectedHeader {
					t.Errorf("Header %d: expected '%s', got '%s'", i, expectedHeader, config.Headers[i])
				}
			}

			transportType := config.GetTransportType()
			if transportType != tt.expectedTransport {
				t.Errorf("Expected transport type '%s', got '%s'", tt.expectedTransport, transportType)
			}
		})
	}
}

func TestMCPServerConfig_HeadersParsing(t *testing.T) {
	// Test that headers are properly parsed in both new and legacy formats
	tests := []struct {
		name     string
		jsonData string
		expected []string
	}{
		{
			name: "New format with multiple headers",
			jsonData: `{
				"type": "remote",
				"url": "https://api.example.com",
				"headers": [
					"Authorization: Bearer abc123",
					"Content-Type: application/json",
					"X-Custom-Header: custom-value"
				]
			}`,
			expected: []string{
				"Authorization: Bearer abc123",
				"Content-Type: application/json",
				"X-Custom-Header: custom-value",
			},
		},
		{
			name: "Legacy format with headers",
			jsonData: `{
				"url": "https://legacy.example.com",
				"headers": ["API-Key: secret123", "Accept: application/json"]
			}`,
			expected: []string{"API-Key: secret123", "Accept: application/json"},
		},
		{
			name: "Empty headers array",
			jsonData: `{
				"type": "remote",
				"url": "https://api.example.com",
				"headers": []
			}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config MCPServerConfig
			err := json.Unmarshal([]byte(tt.jsonData), &config)
			if err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if len(config.Headers) != len(tt.expected) {
				t.Errorf("Expected %d headers, got %d", len(tt.expected), len(config.Headers))
			}

			for i, expected := range tt.expected {
				if i >= len(config.Headers) || config.Headers[i] != expected {
					t.Errorf("Header %d: expected '%s', got '%s'", i, expected, config.Headers[i])
				}
			}
		})
	}
}

func TestEnsureConfigExistsWhenFileExists(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "kit_config_test")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tempDir)
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	// Create an existing config file
	configPath := filepath.Join(tempDir, ".kit.yml")
	existingContent := "# Existing config\nmcpServers:\n  test: {}\n"
	err = os.WriteFile(configPath, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("Error creating existing config: %v", err)
	}

	// Test that EnsureConfigExists doesn't overwrite
	err = EnsureConfigExists()
	if err != nil {
		t.Fatalf("Error in EnsureConfigExists: %v", err)
	}

	// Verify the content wasn't changed
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Error reading config: %v", err)
	}

	if string(content) != existingContent {
		t.Error("Existing config file was modified when it shouldn't have been")
	}
}
