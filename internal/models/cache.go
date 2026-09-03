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

// cacheSchemaVersion identifies the shape of the cached provider data.
//
// The cache stores provider data re-serialized through modelsDBProvider, so it
// only ever contains the catalog fields Kit modelled at the time it was
// written; anything else is dropped. Whenever Kit starts reading a new field,
// caches written by earlier versions are missing it and would silently mask
// the embedded catalog, which does have it. Bumping this constant retires
// those caches: a mismatched version is ignored in favour of the embedded
// data, and the next `kit update-models` writes a complete cache.
//
// Bump this whenever a field is added to modelsDBProvider or modelsDBModel.
//
// Version history:
//
//	0 (implicit) — no version recorded; predates status, reasoning_options
//	               and tiered pricing.
//	1            — adds status, reasoning_options, cost.context_over_200k
//	               and cost.tiers.
const cacheSchemaVersion = 1

// cacheEnvelope wraps the provider data with an ETag for HTTP caching.
type cacheEnvelope struct {
	// Version is the schema the cache was written with. Absent (zero) in
	// caches written before versioning was introduced.
	Version   int                         `json:"version,omitempty"`
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
//
// Returns nil, "" if no cache exists, the cache is unreadable, or it was
// written under an older schema. A stale-schema cache is discarded rather than
// merged: it would shadow the embedded catalog with entries missing whatever
// fields the old binary did not model. Returning no ETag as well forces the
// next `kit update-models` to make an unconditional fetch, so the cache is
// rewritten in full instead of being confirmed as "not modified".
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

	if env.Version != cacheSchemaVersion {
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
		Version:   cacheSchemaVersion,
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
