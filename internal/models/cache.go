package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// cacheFile is the file name for the cached provider data.
const cacheFile = "providers.json"

// cacheEnvelope wraps the provider data with an ETag for HTTP caching.
type cacheEnvelope struct {
	ETag      string                      `json:"etag,omitempty"`
	Providers map[string]modelsDBProvider `json:"providers"`
}

// dataDir returns the kit data directory following XDG Base Directory spec.
//
//	Linux/macOS: $XDG_DATA_HOME/kit  (default ~/.local/share/kit)
//	Windows:     %LOCALAPPDATA%/kit
func dataDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "kit"), nil
	}

	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "kit"), nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "kit"), nil
}

// cachePath returns the full path to the cache file.
func cachePath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheFile), nil
}

// LoadCachedProviders reads the cached provider data from disk.
// Returns nil, "" if no cache exists or the cache is unreadable.
func LoadCachedProviders() (map[string]modelsDBProvider, string) {
	path, err := cachePath()
	if err != nil {
		return nil, ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ""
	}

	var env cacheEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, ""
	}

	if len(env.Providers) == 0 {
		return nil, ""
	}

	return env.Providers, env.ETag
}

// StoreCachedProviders writes provider data to the cache file on disk.
func StoreCachedProviders(providers map[string]modelsDBProvider, etag string) error {
	path, err := cachePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	env := cacheEnvelope{
		ETag:      etag,
		Providers: providers,
	}

	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to marshal provider data: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// RemoveCachedProviders deletes the cache file, causing the registry to
// fall back to the embedded model database on next load.
func RemoveCachedProviders() error {
	path, err := cachePath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
