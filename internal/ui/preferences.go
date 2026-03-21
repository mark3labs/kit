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
	Theme string `yaml:"theme,omitempty"`
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

// LoadThemePreference reads the persisted theme name from preferences.yml.
// Returns "" if no preference is saved or the file doesn't exist.
func LoadThemePreference() string {
	path := preferencesPath()
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var prefs preferences
	if err := yaml.Unmarshal(data, &prefs); err != nil {
		return ""
	}
	return strings.TrimSpace(prefs.Theme)
}

// SaveThemePreference persists the theme name to ~/.config/kit/preferences.yml.
// Preserves other preference fields. Uses atomic write (temp + rename) to
// avoid corruption from concurrent Kit instances.
func SaveThemePreference(name string) error {
	path := preferencesPath()
	if path == "" {
		return nil // silently skip if config dir unavailable
	}

	// Load existing preferences to preserve other fields.
	var prefs preferences
	if data, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(data, &prefs)
	}

	prefs.Theme = name

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
