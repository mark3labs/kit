package models

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
//	2            — adds modalities (used to stop images for a model
//	               without image input).
const cacheSchemaVersion = 3

// cacheEnvelope wraps the provider data with an ETag for HTTP caching.
type cacheEnvelope struct {
	// Version is the schema the cache was written with. Absent (zero) in
	// caches written before versioning was introduced.
	Version           int                         `json:"version,omitempty"`
	ETag              string                      `json:"etag,omitempty"`
	SourceFingerprint string                      `json:"source_fingerprint,omitempty"`
	Providers         map[string]modelsDBProvider `json:"providers"`
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
	env := loadCachedEnvelope()
	return env.Providers, env.ETag
}

func loadCachedEnvelope() cacheEnvelope {
	path, err := cachePath()
	if err != nil {
		return cacheEnvelope{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheEnvelope{}
	}
	providers, err := decodeModels(data)
	if err != nil {
		return cacheEnvelope{}
	}
	var env cacheEnvelope
	if err := json.Unmarshal(data, &env); err != nil || env.Version != cacheSchemaVersion {
		return cacheEnvelope{}
	}
	env.Providers = providers
	return env
}

// StoreCachedProviders writes provider data to the cache file on disk.
func StoreCachedProviders(providers map[string]modelsDBProvider, etag string) error {
	return storeCachedEnvelope(context.Background(), cacheEnvelope{Version: cacheSchemaVersion, ETag: etag, Providers: providers})
}

func storeCachedEnvelope(ctx context.Context, env cacheEnvelope) error {
	if err := validateCatalog(env.Providers); err != nil {
		return err
	}
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to marshal provider data: %w", err)
	}
	path, err := cachePath()
	if err != nil {
		return err
	}
	return replaceCache(ctx, path, data, cacheFileOps{create: func(dir string) (cacheTempFile, error) { return os.CreateTemp(dir, ".providers-*") }, rename: os.Rename})
}

type cacheTempFile interface {
	io.Writer
	Name() string
	Chmod(os.FileMode) error
	Sync() error
	Close() error
}

type cacheFileOps struct {
	create func(string) (cacheTempFile, error)
	rename func(string, string) error
}

// replaceCache has one commit point: rename. Never delete the destination as a fallback.
func replaceCache(ctx context.Context, path string, data []byte, ops cacheFileOps) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	file, err := ops.create(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("create temporary cache: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
		_ = os.Remove(file.Name())
	}()
	if n, err := file.Write(data); err != nil {
		return fmt.Errorf("write cache: %w", err)
	} else if n != len(data) {
		return fmt.Errorf("write cache: %w", io.ErrShortWrite)
	}
	if err := file.Chmod(0o644); err != nil {
		return fmt.Errorf("set cache permissions: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync cache: %w", err)
	}
	err = file.Close()
	closed = true
	if err != nil {
		return fmt.Errorf("close cache: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ops.rename(file.Name(), path); err != nil {
		return fmt.Errorf("replace cache: %w", err)
	}
	return nil
}

// RemoveCachedProviders deletes the cache file, causing the registry to
// fall back to the embedded model database on next load.
func RemoveCachedProviders() error {
	return removeCachedProviders(context.Background())
}

func removeCachedProviders(ctx context.Context) error {
	path, err := cachePath()
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
