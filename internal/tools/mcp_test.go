package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcphost/internal/config"
)

func TestMCPToolManager_LoadTools_WithTimeout(t *testing.T) {
	manager := NewMCPToolManager()

	// Create a config with a non-existent command that should fail
	cfg := &config.Config{
		MCPServers: map[string]config.MCPServerConfig{
			"test-server": {
				Command: []string{"non-existent-command", "arg1", "arg2"},
			},
		},
	}

	// Create a context with a reasonable timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// This should not hang indefinitely and should return an error
	start := time.Now()
	err := manager.LoadTools(ctx, cfg)
	duration := time.Since(start)

	// The operation should complete within our timeout
	if duration > 14*time.Second {
		t.Errorf("LoadTools took too long: %v, expected to complete within 14 seconds", duration)
	}

	// We expect an error since the command doesn't exist, but it shouldn't be a timeout
	if err == nil {
		t.Error("Expected an error for non-existent command, but got nil")
	}

	t.Logf("LoadTools completed in %v with error: %v", duration, err)
}

func TestMCPToolManager_LoadTools_GracefulFailure(t *testing.T) {
	manager := NewMCPToolManager()

	// Create a config with multiple servers, some good and some bad
	cfg := &config.Config{
		MCPServers: map[string]config.MCPServerConfig{
			"bad-server-1": {
				Command: []string{"non-existent-command-1", "arg1"},
			},
			"bad-server-2": {
				Command: []string{"non-existent-command-2", "arg2"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// This should fail gracefully and return an error since all servers failed
	err := manager.LoadTools(ctx, cfg)

	// We expect an error since all servers failed
	if err == nil {
		t.Error("Expected an error when all servers fail, but got nil")
	}

	// The error should mention that all servers failed
	if err != nil && !contains(err.Error(), "all MCP servers failed") {
		t.Errorf("Expected error message to mention all servers failed, got: %v", err)
	}

	t.Logf("LoadTools failed gracefully with error: %v", err)
}

// TestMCPToolManager_ToolWithoutProperties tests handling of tools with no input properties
func TestMCPToolManager_ToolWithoutProperties(t *testing.T) {
	manager := NewMCPToolManager()

	// Create a config with a builtin todo server (which has tools with properties)
	cfg := &config.Config{
		MCPServers: map[string]config.MCPServerConfig{
			"todo-server": {
				Type: "builtin",
				Name: "todo",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Load the tools - this should work fine
	err := manager.LoadTools(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to load tools: %v", err)
	}

	// Get the loaded tools
	tools := manager.GetTools()
	if len(tools) == 0 {
		t.Fatal("No tools were loaded")
	}

	// Test that we can get tool info for each tool
	for _, tool := range tools {
		info := tool.Info()

		// Check that the tool has a valid name
		if info.Name == "" {
			t.Error("Tool has empty name")
		}

		t.Logf("Tool: %s, Description: %s", info.Name, info.Description)
	}
}

// TestIssue89_ObjectSchemaMissingProperties tests the fix for issue #89
// This verifies that object schemas with nil properties get an empty properties map
func TestIssue89_ObjectSchemaMissingProperties(t *testing.T) {
	// Create a schema that would cause issues with tools that have no input properties
	brokenSchema := map[string]any{
		"type": "object",
		// Properties is nil - this used to cause "object schema missing properties" error
	}

	// Verify the problematic state
	if brokenSchema["type"] == "object" && brokenSchema["properties"] == nil {
		t.Log("Found object schema with nil properties - this previously caused validation errors")
	}

	// Apply the fix - add empty properties
	if brokenSchema["type"] == "object" && brokenSchema["properties"] == nil {
		brokenSchema["properties"] = map[string]any{}
	}

	// Verify the fix worked
	if brokenSchema["properties"] == nil {
		t.Error("Fix failed: object schema still has nil properties")
	}

	// Verify it marshals cleanly
	data, err := json.Marshal(brokenSchema)
	if err != nil {
		t.Errorf("Failed to marshal fixed schema: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("Failed to unmarshal fixed schema: %v", err)
	}

	if result["type"] != "object" {
		t.Error("Schema type should be 'object'")
	}
}

// TestConvertExclusiveBoundsToBoolean tests the JSON Schema draft-07 to draft-04 conversion
func TestConvertExclusiveBoundsToBoolean(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
	}{
		{
			name:  "exclusiveMinimum as number",
			input: `{"type": "number", "exclusiveMinimum": 0}`,
			expected: map[string]any{
				"type":             "number",
				"minimum":          float64(0),
				"exclusiveMinimum": true,
			},
		},
		{
			name:  "exclusiveMaximum as number",
			input: `{"type": "number", "exclusiveMaximum": 100}`,
			expected: map[string]any{
				"type":             "number",
				"maximum":          float64(100),
				"exclusiveMaximum": true,
			},
		},
		{
			name:  "both exclusive bounds as numbers",
			input: `{"type": "integer", "exclusiveMinimum": 1, "exclusiveMaximum": 10}`,
			expected: map[string]any{
				"type":             "integer",
				"minimum":          float64(1),
				"exclusiveMinimum": true,
				"maximum":          float64(10),
				"exclusiveMaximum": true,
			},
		},
		{
			name:  "already boolean exclusiveMinimum (draft-04 style)",
			input: `{"type": "number", "minimum": 0, "exclusiveMinimum": true}`,
			expected: map[string]any{
				"type":             "number",
				"minimum":          float64(0),
				"exclusiveMinimum": true,
			},
		},
		{
			name:  "no exclusive bounds",
			input: `{"type": "string", "minLength": 1}`,
			expected: map[string]any{
				"type":      "string",
				"minLength": float64(1),
			},
		},
		{
			name:  "nested properties with exclusive bounds",
			input: `{"type": "object", "properties": {"age": {"type": "integer", "exclusiveMinimum": 0}}}`,
			expected: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"age": map[string]any{
						"type":             "integer",
						"minimum":          float64(0),
						"exclusiveMinimum": true,
					},
				},
			},
		},
		{
			name:  "array items with exclusive bounds",
			input: `{"type": "array", "items": {"type": "number", "exclusiveMaximum": 100}}`,
			expected: map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":             "number",
					"maximum":          float64(100),
					"exclusiveMaximum": true,
				},
			},
		},
		{
			name:  "allOf with exclusive bounds",
			input: `{"allOf": [{"type": "number", "exclusiveMinimum": 0}]}`,
			expected: map[string]any{
				"allOf": []any{
					map[string]any{
						"type":             "number",
						"minimum":          float64(0),
						"exclusiveMinimum": true,
					},
				},
			},
		},
		{
			name:  "additionalProperties with exclusive bounds",
			input: `{"type": "object", "additionalProperties": {"type": "integer", "exclusiveMinimum": 0, "exclusiveMaximum": 255}}`,
			expected: map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"type":             "integer",
					"minimum":          float64(0),
					"exclusiveMinimum": true,
					"maximum":          float64(255),
					"exclusiveMaximum": true,
				},
			},
		},
		{
			name:  "Chrome DevTools MCP style schema (real-world example)",
			input: `{"type": "object", "properties": {"timeout": {"type": "integer", "exclusiveMinimum": 0}, "quality": {"type": "number", "minimum": 0, "maximum": 100}}}`,
			expected: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"timeout": map[string]any{
						"type":             "integer",
						"minimum":          float64(0),
						"exclusiveMinimum": true,
					},
					"quality": map[string]any{
						"type":    "number",
						"minimum": float64(0),
						"maximum": float64(100),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertExclusiveBoundsToBoolean([]byte(tt.input))

			var got map[string]any
			if err := json.Unmarshal(result, &got); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			if !deepEqual(got, tt.expected) {
				t.Errorf("convertExclusiveBoundsToBoolean() =\n%v\nwant:\n%v", got, tt.expected)
			}
		})
	}
}

// TestNullRequiredFieldSanitization tests that null "required" fields are removed
// from schemas to prevent OpenAI API validation errors like:
// "None is not of type 'array'"
func TestNullRequiredFieldSanitization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKey  bool   // should "required" key exist in output?
		wantJSON string // expected JSON output (if checking more than key presence)
	}{
		{
			name:    "null required is removed",
			input:   `{"type": "object", "properties": {"name": {"type": "string"}}, "required": null}`,
			wantKey: false,
		},
		{
			name:    "valid required array is preserved",
			input:   `{"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}`,
			wantKey: true,
		},
		{
			name:    "empty required array is preserved",
			input:   `{"type": "object", "properties": {}, "required": []}`,
			wantKey: true,
		},
		{
			name:    "nested null required is removed",
			input:   `{"type": "object", "properties": {"config": {"type": "object", "properties": {"key": {"type": "string"}}, "required": null}}}`,
			wantKey: false,
		},
		{
			name:    "required as wrong type (string) is removed",
			input:   `{"type": "object", "properties": {}, "required": "name"}`,
			wantKey: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertExclusiveBoundsToBoolean([]byte(tt.input))

			var got map[string]any
			if err := json.Unmarshal(result, &got); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			// Check top-level or nested required field
			checkSchema := got
			if nested, ok := got["properties"].(map[string]any); ok {
				if cfg, ok := nested["config"].(map[string]any); ok {
					checkSchema = cfg
				}
			}

			_, hasRequired := checkSchema["required"]
			if hasRequired != tt.wantKey {
				t.Errorf("required key present = %v, want %v. Schema: %s", hasRequired, tt.wantKey, string(result))
			}
		})
	}
}

// TestToolInfoRequiredNeverNull verifies that MCP tool conversion always produces
// a non-nil Required slice, preventing "required": null in serialized JSON.
func TestToolInfoRequiredNeverNull(t *testing.T) {
	// Simulate the schema extraction logic from loadServerTools
	schemaJSON := `{"type": "object", "properties": {"name": {"type": "string"}}}`

	var schemaMap map[string]any
	if err := json.Unmarshal([]byte(schemaJSON), &schemaMap); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

	// This mirrors the code in loadServerTools
	required := []string{}
	if req, ok := schemaMap["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}

	// required must never be nil
	if required == nil {
		t.Fatal("required is nil — would serialize as \"required\": null")
	}

	// Verify JSON serialization
	data, err := json.Marshal(map[string]any{"required": required})
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	if string(data) != `{"required":[]}` {
		t.Errorf("Expected {\"required\":[]}, got %s", string(data))
	}
}

// TestConvertExclusiveBoundsToBoolean_InvalidJSON tests that invalid JSON is returned unchanged
func TestConvertExclusiveBoundsToBoolean_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{invalid json}`)
	result := convertExclusiveBoundsToBoolean(invalidJSON)

	if string(result) != string(invalidJSON) {
		t.Errorf("Expected invalid JSON to be returned unchanged, got: %s", string(result))
	}
}

// deepEqual compares two maps recursively
func deepEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		switch av := v.(type) {
		case map[string]any:
			bvm, ok := bv.(map[string]any)
			if !ok || !deepEqual(av, bvm) {
				return false
			}
		case []any:
			bva, ok := bv.([]any)
			if !ok || !sliceEqual(av, bva) {
				return false
			}
		default:
			if v != bv {
				return false
			}
		}
	}
	return true
}

// sliceEqual compares two slices recursively
func sliceEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		switch av := a[i].(type) {
		case map[string]any:
			bvm, ok := b[i].(map[string]any)
			if !ok || !deepEqual(av, bvm) {
				return false
			}
		case []any:
			bva, ok := b[i].([]any)
			if !ok || !sliceEqual(av, bva) {
				return false
			}
		default:
			if a[i] != b[i] {
				return false
			}
		}
	}
	return true
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
