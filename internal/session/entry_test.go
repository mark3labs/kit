package session

import (
	"encoding/json"
	"testing"
)

func TestSystemPromptEntry(t *testing.T) {
	// Test creation
	content := "You are a helpful coding assistant."
	model := "claude-sonnet-4-5"
	provider := "anthropic"
	entry := NewSystemPromptEntry(content, model, provider)

	if entry.Type != EntryTypeSystemPrompt {
		t.Errorf("Expected type %q, got %q", EntryTypeSystemPrompt, entry.Type)
	}

	if entry.Content != content {
		t.Errorf("Expected content %q, got %q", content, entry.Content)
	}

	if entry.Model != model {
		t.Errorf("Expected model %q, got %q", model, entry.Model)
	}

	if entry.Provider != provider {
		t.Errorf("Expected provider %q, got %q", provider, entry.Provider)
	}

	if entry.ID == "" {
		t.Error("Expected non-empty ID")
	}

	// Test marshaling
	data, err := MarshalEntry(entry)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Test unmarshaling
	unmarshaled, err := UnmarshalEntry(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	sysPrompt, ok := unmarshaled.(*SystemPromptEntry)
	if !ok {
		t.Fatalf("Expected *SystemPromptEntry, got %T", unmarshaled)
	}

	if sysPrompt.Type != EntryTypeSystemPrompt {
		t.Errorf("Unmarshaled: expected type %q, got %q", EntryTypeSystemPrompt, sysPrompt.Type)
	}

	if sysPrompt.Content != content {
		t.Errorf("Unmarshaled: expected content %q, got %q", content, sysPrompt.Content)
	}

	if sysPrompt.Model != model {
		t.Errorf("Unmarshaled: expected model %q, got %q", model, sysPrompt.Model)
	}

	if sysPrompt.Provider != provider {
		t.Errorf("Unmarshaled: expected provider %q, got %q", provider, sysPrompt.Provider)
	}

	if sysPrompt.ID != entry.ID {
		t.Errorf("Unmarshaled: expected ID %q, got %q", entry.ID, sysPrompt.ID)
	}
}

func TestSystemPromptEntryJSONStructure(t *testing.T) {
	content := "Test system prompt content"
	model := "gpt-4o"
	provider := "openai"
	entry := NewSystemPromptEntry(content, model, provider)

	data, err := MarshalEntry(entry)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify JSON structure
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal to raw map: %v", err)
	}

	if raw["type"] != "system_prompt" {
		t.Errorf("Expected type 'system_prompt', got %v", raw["type"])
	}

	if raw["content"] != content {
		t.Errorf("Expected content %q, got %v", content, raw["content"])
	}

	if raw["model"] != model {
		t.Errorf("Expected model %q, got %v", model, raw["model"])
	}

	if raw["provider"] != provider {
		t.Errorf("Expected provider %q, got %v", provider, raw["provider"])
	}

	if raw["id"] == "" || raw["id"] == nil {
		t.Error("Expected non-empty id field")
	}

	if raw["timestamp"] == "" || raw["timestamp"] == nil {
		t.Error("Expected non-empty timestamp field")
	}
}
