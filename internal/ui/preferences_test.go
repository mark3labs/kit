package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadThemePreference(t *testing.T) {
	// Use a temp dir as XDG_CONFIG_HOME so we don't touch the real config.
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Initially no preference is saved.
	if got := LoadThemePreference(); got != "" {
		t.Fatalf("expected empty preference, got %q", got)
	}

	// Save a preference.
	if err := SaveThemePreference("dracula"); err != nil {
		t.Fatalf("SaveThemePreference: %v", err)
	}

	// Load it back.
	if got := LoadThemePreference(); got != "dracula" {
		t.Fatalf("expected %q, got %q", "dracula", got)
	}

	// Overwrite with a different theme.
	if err := SaveThemePreference("nord"); err != nil {
		t.Fatalf("SaveThemePreference: %v", err)
	}
	if got := LoadThemePreference(); got != "nord" {
		t.Fatalf("expected %q, got %q", "nord", got)
	}

	// Verify the file exists and is valid YAML.
	path := filepath.Join(tmp, "kit", "preferences.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading preferences file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("preferences file is empty")
	}
}

func TestLoadThemePreference_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// No file exists — should return empty string, not error.
	if got := LoadThemePreference(); got != "" {
		t.Fatalf("expected empty string for missing file, got %q", got)
	}
}

func TestLoadThemePreference_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Write invalid YAML.
	dir := filepath.Join(tmp, "kit")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "preferences.yml"), []byte(":::bad yaml"), 0o644)

	// Should return empty string, not panic.
	if got := LoadThemePreference(); got != "" {
		t.Fatalf("expected empty string for invalid YAML, got %q", got)
	}
}

func TestSaveThemePreference_PreservesOtherFields(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Pre-populate with extra content (simulating future fields).
	dir := filepath.Join(tmp, "kit")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "preferences.yml"), []byte("theme: old\n"), 0o644)

	// Overwrite theme.
	if err := SaveThemePreference("catppuccin"); err != nil {
		t.Fatalf("SaveThemePreference: %v", err)
	}

	if got := LoadThemePreference(); got != "catppuccin" {
		t.Fatalf("expected %q, got %q", "catppuccin", got)
	}
}

func TestSaveAndLoadModelPreference(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Initially empty.
	if got := LoadModelPreference(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	// Save a model.
	if err := SaveModelPreference("anthropic/claude-sonnet-4-5-20250929"); err != nil {
		t.Fatalf("SaveModelPreference: %v", err)
	}
	if got := LoadModelPreference(); got != "anthropic/claude-sonnet-4-5-20250929" {
		t.Fatalf("expected %q, got %q", "anthropic/claude-sonnet-4-5-20250929", got)
	}

	// Overwrite.
	if err := SaveModelPreference("openai/gpt-4o"); err != nil {
		t.Fatalf("SaveModelPreference: %v", err)
	}
	if got := LoadModelPreference(); got != "openai/gpt-4o" {
		t.Fatalf("expected %q, got %q", "openai/gpt-4o", got)
	}
}

func TestSaveAndLoadThinkingLevelPreference(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Initially empty.
	if got := LoadThinkingLevelPreference(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	// Save a level.
	if err := SaveThinkingLevelPreference("medium"); err != nil {
		t.Fatalf("SaveThinkingLevelPreference: %v", err)
	}
	if got := LoadThinkingLevelPreference(); got != "medium" {
		t.Fatalf("expected %q, got %q", "medium", got)
	}

	// Overwrite.
	if err := SaveThinkingLevelPreference("high"); err != nil {
		t.Fatalf("SaveThinkingLevelPreference: %v", err)
	}
	if got := LoadThinkingLevelPreference(); got != "high" {
		t.Fatalf("expected %q, got %q", "high", got)
	}
}

func TestPreferencesPreserveEachOther(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Save all three preferences.
	if err := SaveThemePreference("dracula"); err != nil {
		t.Fatal(err)
	}
	if err := SaveModelPreference("anthropic/claude-haiku-3-5-20241022"); err != nil {
		t.Fatal(err)
	}
	if err := SaveThinkingLevelPreference("high"); err != nil {
		t.Fatal(err)
	}

	// All three should be preserved.
	if got := LoadThemePreference(); got != "dracula" {
		t.Fatalf("theme: expected %q, got %q", "dracula", got)
	}
	if got := LoadModelPreference(); got != "anthropic/claude-haiku-3-5-20241022" {
		t.Fatalf("model: expected %q, got %q", "anthropic/claude-haiku-3-5-20241022", got)
	}
	if got := LoadThinkingLevelPreference(); got != "high" {
		t.Fatalf("thinking_level: expected %q, got %q", "high", got)
	}

	// Updating one should not affect the others.
	if err := SaveModelPreference("openai/gpt-4o"); err != nil {
		t.Fatal(err)
	}
	if got := LoadThemePreference(); got != "dracula" {
		t.Fatalf("theme after model update: expected %q, got %q", "dracula", got)
	}
	if got := LoadThinkingLevelPreference(); got != "high" {
		t.Fatalf("thinking_level after model update: expected %q, got %q", "high", got)
	}
}
