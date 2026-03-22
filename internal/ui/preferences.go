package ui

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// preferences holds user-mutable runtime state that persists across sessions.
// Stored at ~/.config/kit/preferences.yml, separate from the declarative
// .kit.yml config so we never clobber user comments or formatting.
type preferences struct {
	Theme         string `yaml:"theme,omitempty"`
	Model         string `yaml:"model,omitempty"`
	ThinkingLevel string `yaml:"thinking_level,omitempty"`
}

// preferencesPath returns ~/.config/kit/preferences.yml.
// Returns "" if the config directory cannot be determined.
func preferencesPath() string {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cfgDir, "kit", "preferences.yml")
}

// loadPreferences reads and parses the preferences file.
// Returns zero-value preferences if the file is missing or invalid.
func loadPreferences() preferences {
	path := preferencesPath()
	if path == "" {
		return preferences{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return preferences{}
	}
	var prefs preferences
	if err := yaml.Unmarshal(data, &prefs); err != nil {
		return preferences{}
	}
	return prefs
}

// savePreferences atomically writes the preferences file, merging into any
// existing content. The mutate function receives the current preferences and
// should modify them in place.
func savePreferences(mutate func(*preferences)) error {
	path := preferencesPath()
	if path == "" {
		return nil // silently skip if config dir unavailable
	}

	// Load existing preferences to preserve other fields.
	var prefs preferences
	if data, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(data, &prefs)
	}

	mutate(&prefs)

	data, err := yaml.Marshal(&prefs)
	if err != nil {
		return err
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// Atomic write: write to temp file, then rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ── Theme preference ────────────────────────────────────────────────────────

// LoadThemePreference reads the persisted theme name from preferences.yml.
// Returns "" if no preference is saved or the file doesn't exist.
func LoadThemePreference() string {
	return strings.TrimSpace(loadPreferences().Theme)
}

// SaveThemePreference persists the theme name to ~/.config/kit/preferences.yml.
// Preserves other preference fields. Uses atomic write (temp + rename) to
// avoid corruption from concurrent Kit instances.
func SaveThemePreference(name string) error {
	return savePreferences(func(p *preferences) {
		p.Theme = name
	})
}

// ── Model preference ────────────────────────────────────────────────────────

// LoadModelPreference reads the persisted model string (e.g.
// "anthropic/claude-sonnet-4-5-20250929") from preferences.yml.
// Returns "" if no preference is saved.
func LoadModelPreference() string {
	return strings.TrimSpace(loadPreferences().Model)
}

// SaveModelPreference persists the model string to preferences.yml.
func SaveModelPreference(model string) error {
	return savePreferences(func(p *preferences) {
		p.Model = model
	})
}

// ── Thinking level preference ───────────────────────────────────────────────

// LoadThinkingLevelPreference reads the persisted thinking level from
// preferences.yml. Returns "" if no preference is saved.
func LoadThinkingLevelPreference() string {
	return strings.TrimSpace(loadPreferences().ThinkingLevel)
}

// SaveThinkingLevelPreference persists the thinking level to preferences.yml.
func SaveThinkingLevelPreference(level string) error {
	return savePreferences(func(p *preferences) {
		p.ThinkingLevel = level
	})
}
