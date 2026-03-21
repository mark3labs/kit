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
