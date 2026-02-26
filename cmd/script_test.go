package cmd

import (
	"reflect"
	"testing"
)

func TestFindVariablesWithDefaults(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []Variable
	}{
		{
			name:    "simple variable without default",
			content: "Hello ${name}!",
			expected: []Variable{
				{Name: "name", DefaultValue: "", HasDefault: false},
			},
		},
		{
			name:    "variable with default value",
			content: "Hello ${name:-World}!",
			expected: []Variable{
				{Name: "name", DefaultValue: "World", HasDefault: true},
			},
		},
		{
			name:    "variable with empty default",
			content: "Hello ${name:-}!",
			expected: []Variable{
				{Name: "name", DefaultValue: "", HasDefault: true},
			},
		},
		{
			name:    "multiple variables mixed",
			content: "Hello ${name:-World}! Your directory is ${directory} and your age is ${age:-25}.",
			expected: []Variable{
				{Name: "name", DefaultValue: "World", HasDefault: true},
				{Name: "directory", DefaultValue: "", HasDefault: false},
				{Name: "age", DefaultValue: "25", HasDefault: true},
			},
		},
		{
			name:    "duplicate variables",
			content: "Hello ${name:-World}! Again, hello ${name:-Universe}!",
			expected: []Variable{
				{Name: "name", DefaultValue: "World", HasDefault: true},
			},
		},
		{
			name:     "no variables",
			content:  "Hello World!",
			expected: nil,
		},
		{
			name:    "complex default values",
			content: "Path: ${path:-/tmp/default/path} and URL: ${url:-https://example.com/api}",
			expected: []Variable{
				{Name: "path", DefaultValue: "/tmp/default/path", HasDefault: true},
				{Name: "url", DefaultValue: "https://example.com/api", HasDefault: true},
			},
		},
		{
			name:    "default with spaces",
			content: "Message: ${msg:-Hello World}",
			expected: []Variable{
				{Name: "msg", DefaultValue: "Hello World", HasDefault: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findVariablesWithDefaults(tt.content)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("findVariablesWithDefaults() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestFindVariablesBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "simple variables",
			content:  "Hello ${name} from ${location}!",
			expected: []string{"name", "location"},
		},
		{
			name:     "variables with defaults should still return names",
			content:  "Hello ${name:-World} from ${location:-Earth}!",
			expected: []string{"name", "location"},
		},
		{
			name:     "mixed variables",
			content:  "Hello ${name} from ${location:-Earth}!",
			expected: []string{"name", "location"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findVariables(tt.content)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("findVariables() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateVariables(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		variables map[string]string
		wantError bool
	}{
		{
			name:      "all required variables provided",
			content:   "Hello ${name} from ${location}!",
			variables: map[string]string{"name": "John", "location": "NYC"},
			wantError: false,
		},
		{
			name:      "missing required variable",
			content:   "Hello ${name} from ${location}!",
			variables: map[string]string{"name": "John"},
			wantError: true,
		},
		{
			name:      "variable with default not provided - should not error",
			content:   "Hello ${name:-World}!",
			variables: map[string]string{},
			wantError: false,
		},
		{
			name:      "mixed required and optional variables",
			content:   "Hello ${name} from ${location:-Earth}!",
			variables: map[string]string{"name": "John"},
			wantError: false,
		},
		{
			name:      "mixed variables with missing required",
			content:   "Hello ${name} from ${location:-Earth}!",
			variables: map[string]string{},
			wantError: true,
		},
		{
			name:      "all variables have defaults",
			content:   "Hello ${name:-World} from ${location:-Earth}!",
			variables: map[string]string{},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVariables(tt.content, tt.variables)
			if (err != nil) != tt.wantError {
				t.Errorf("validateVariables() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestSubstituteVariables(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		variables map[string]string
		expected  string
	}{
		{
			name:      "simple substitution",
			content:   "Hello ${name}!",
			variables: map[string]string{"name": "John"},
			expected:  "Hello John!",
		},
		{
			name:      "substitution with default - value provided",
			content:   "Hello ${name:-World}!",
			variables: map[string]string{"name": "John"},
			expected:  "Hello John!",
		},
		{
			name:      "substitution with default - value not provided",
			content:   "Hello ${name:-World}!",
			variables: map[string]string{},
			expected:  "Hello World!",
		},
		{
			name:      "multiple variables mixed",
			content:   "Hello ${name:-World} from ${location}!",
			variables: map[string]string{"location": "NYC"},
			expected:  "Hello World from NYC!",
		},
		{
			name:      "empty default value",
			content:   "Hello ${name:-}!",
			variables: map[string]string{},
			expected:  "Hello !",
		},
		{
			name:      "complex default values",
			content:   "Path: ${path:-/tmp/default} URL: ${url:-https://example.com}",
			variables: map[string]string{},
			expected:  "Path: /tmp/default URL: https://example.com",
		},
		{
			name:      "variable not found and no default",
			content:   "Hello ${name}!",
			variables: map[string]string{},
			expected:  "Hello ${name}!",
		},
		{
			name:      "default with spaces",
			content:   "Message: ${msg:-Hello World}",
			variables: map[string]string{},
			expected:  "Message: Hello World",
		},
		{
			name:      "override default with provided value",
			content:   "Message: ${msg:-Hello World}",
			variables: map[string]string{"msg": "Custom Message"},
			expected:  "Message: Custom Message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteVariables(tt.content, tt.variables)
			if result != tt.expected {
				t.Errorf("substituteVariables() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that existing scripts without default syntax continue to work
	content := `---
model: "anthropic/claude-sonnet-4-5-20250929"
---
Hello ${name}! Please analyze ${directory}.`

	variables := map[string]string{
		"name":      "John",
		"directory": "/tmp",
	}

	// Should not error during validation
	err := validateVariables(content, variables)
	if err != nil {
		t.Errorf("validateVariables() should not error for backward compatibility, got: %v", err)
	}

	// Should substitute correctly
	result := substituteVariables(content, variables)
	expected := `---
model: "anthropic/claude-sonnet-4-5-20250929"
---
Hello John! Please analyze /tmp.`

	if result != expected {
		t.Errorf("substituteVariables() backward compatibility failed.\nGot:\n%s\nWant:\n%s", result, expected)
	}
}

func TestParseScriptContentWithCompactMode(t *testing.T) {
	content := `---
compact: true
mcpServers:
  echo:
    type: "local"
    command: ["echo", "hello"]
---
Test prompt with compact mode`

	variables := make(map[string]string)
	config, err := parseScriptContent(content, variables)
	if err != nil {
		t.Fatalf("parseScriptContent() failed: %v", err)
	}

	if !config.Compact {
		t.Errorf("Expected compact mode to be true, got false")
	}

	if config.Prompt != "Test prompt with compact mode" {
		t.Errorf("Expected prompt 'Test prompt with compact mode', got '%s'", config.Prompt)
	}

	if len(config.MCPServers) != 1 {
		t.Errorf("Expected 1 MCP server, got %d", len(config.MCPServers))
	}
}

func TestParseScriptContentMCPServersNewFormat(t *testing.T) {
	content := `---
model: "anthropic/claude-sonnet-4-5-20250929"
mcpServers:
  filesystem:
    type: "local"
    command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    environment:
      NODE_ENV: "production"
  remote-server:
    type: "remote"
    url: "https://example.com/mcp"
---
Test prompt with new format MCP servers`

	variables := make(map[string]string)
	config, err := parseScriptContent(content, variables)
	if err != nil {
		t.Fatalf("parseScriptContent() failed: %v", err)
	}

	if len(config.MCPServers) != 2 {
		t.Errorf("Expected 2 MCP servers, got %d", len(config.MCPServers))
	}

	// Test local server
	fs, exists := config.MCPServers["filesystem"]
	if !exists {
		t.Error("Expected filesystem server to exist")
	}
	if fs.Type != "local" {
		t.Errorf("Expected filesystem server type 'local', got '%s'", fs.Type)
	}
	expectedCommand := []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"}
	if len(fs.Command) != len(expectedCommand) {
		t.Errorf("Expected filesystem server command length %d, got %d", len(expectedCommand), len(fs.Command))
	}
	for i, expected := range expectedCommand {
		if i >= len(fs.Command) || fs.Command[i] != expected {
			t.Errorf("Expected filesystem server command %v, got %v", expectedCommand, fs.Command)
			break
		}
	}
	if fs.Environment["node_env"] != "production" {
		t.Errorf("Expected node_env=production, got %s", fs.Environment["node_env"])
	}

	// Test remote server
	remote, exists := config.MCPServers["remote-server"]
	if !exists {
		t.Error("Expected remote-server to exist")
	}
	if remote.Type != "remote" {
		t.Errorf("Expected remote server type 'remote', got '%s'", remote.Type)
	}
	if remote.URL != "https://example.com/mcp" {
		t.Errorf("Expected remote server URL 'https://example.com/mcp', got '%s'", remote.URL)
	}
}

func TestParseScriptContentMCPServersLegacyFormat(t *testing.T) {
	content := `---
model: "anthropic/claude-sonnet-4-5-20250929"
mcpServers:
  legacy-stdio:
    transport: "stdio"
    command: "npx"
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    env:
      node_env: "development"
  legacy-sse:
    transport: "sse"
    url: "https://legacy.example.com/mcp"
    headers: ["Authorization: Bearer token"]
---
Test prompt with legacy format MCP servers`

	variables := make(map[string]string)
	config, err := parseScriptContent(content, variables)
	if err != nil {
		t.Fatalf("parseScriptContent() failed: %v", err)
	}

	if len(config.MCPServers) != 2 {
		t.Errorf("Expected 2 MCP servers, got %d", len(config.MCPServers))
	}

	// Test legacy stdio server - Note: Viper parsing doesn't trigger custom UnmarshalJSON
	// so legacy format has limited support in script frontmatter
	stdio, exists := config.MCPServers["legacy-stdio"]
	if !exists {
		t.Error("Expected legacy-stdio server to exist")
	}
	if stdio.Transport != "stdio" {
		t.Errorf("Expected legacy stdio transport 'stdio', got '%s'", stdio.Transport)
	}
	// Command field only gets the single command value, not combined with args
	if stdio.Command[0] != "npx" {
		t.Errorf("Expected legacy stdio command to start with 'npx', got %v", stdio.Command)
	}
	expectedArgs := []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"}
	if len(stdio.Args) != len(expectedArgs) {
		t.Errorf("Expected legacy stdio args length %d, got %d", len(expectedArgs), len(stdio.Args))
	}
	for i, expected := range expectedArgs {
		if i >= len(stdio.Args) || stdio.Args[i] != expected {
			t.Errorf("Expected legacy stdio args %v, got %v", expectedArgs, stdio.Args)
			break
		}
	}
	// Env field should contain the environment variables (with lowercase keys due to Viper)
	if stdio.Env["node_env"] != "development" {
		t.Errorf("Expected legacy stdio env node_env=development, got %v", stdio.Env["node_env"])
	}

	// Test legacy SSE server
	sse, exists := config.MCPServers["legacy-sse"]
	if !exists {
		t.Error("Expected legacy-sse server to exist")
	}
	if sse.Transport != "sse" {
		t.Errorf("Expected legacy sse transport 'sse', got '%s'", sse.Transport)
	}
	if sse.URL != "https://legacy.example.com/mcp" {
		t.Errorf("Expected legacy sse URL 'https://legacy.example.com/mcp', got '%s'", sse.URL)
	}
	if len(sse.Headers) != 1 || sse.Headers[0] != "Authorization: Bearer token" {
		t.Errorf("Expected legacy sse headers [Authorization: Bearer token], got %v", sse.Headers)
	}
}
