package message

import (
	"testing"
)

func TestSanitizeToolCallID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid alphanumeric ID",
			input:    "call_123abc",
			expected: "call_123abc",
		},
		{
			name:     "ID with dots (OpenCode/Kimi style)",
			input:    "call.123.abc",
			expected: "call_123_abc",
		},
		{
			name:     "ID with colons",
			input:    "tool:123:abc",
			expected: "tool_123_abc",
		},
		{
			name:     "ID with special characters",
			input:    "tool@#$%^&*()",
			expected: "tool_________",
		},
		{
			name:     "Anthropic style ID (already valid)",
			input:    "toolu_0123456789ABCDEF",
			expected: "toolu_0123456789ABCDEF",
		},
		{
			name:     "OpenAI style ID (already valid)",
			input:    "call_O17Uplv4lJvD6DVdIvFFeRMw",
			expected: "call_O17Uplv4lJvD6DVdIvFFeRMw",
		},
		{
			name:     "ID with hyphens",
			input:    "my-tool-call-123",
			expected: "my-tool-call-123",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "tool_0",
		},
		{
			name:     "only special characters",
			input:    "@#$%",
			expected: "____",
		},
		{
			name:     "mixed valid and invalid",
			input:    "call_123.abc-def@ghi",
			expected: "call_123_abc-def_ghi",
		},
		{
			name:     "Unicode characters",
			input:    "tool_日本語",
			expected: "tool____",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeToolCallID(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeToolCallID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeToolCallID_MatchesAnthropicPattern(t *testing.T) {
	// Test that sanitized IDs match Anthropic's required pattern: ^[a-zA-Z0-9_-]+$
	// This is a simplified check - in reality the pattern allows alphanumeric, underscore, hyphen
	testIDs := []string{
		"call.123.abc",
		"tool:123:def",
		"id@#$%^&*()",
		"mixed.valid-id_test",
		"",
	}

	for _, id := range testIDs {
		sanitized := sanitizeToolCallID(id)

		// Verify each character is valid
		for i, r := range sanitized {
			valid := (r >= 'a' && r <= 'z') ||
				(r >= 'A' && r <= 'Z') ||
				(r >= '0' && r <= '9') ||
				r == '_' ||
				r == '-'

			if !valid {
				t.Errorf("sanitizeToolCallID(%q) = %q, contains invalid character at position %d: %q",
					id, sanitized, i, string(r))
			}
		}

		// Verify non-empty
		if sanitized == "" {
			t.Errorf("sanitizeToolCallID(%q) returned empty string", id)
		}
	}
}
