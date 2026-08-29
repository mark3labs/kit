package daemon

import (
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed all:embedded
var embeddedTunnelFS embed.FS

// errNoEmbeddedTunnel means the build carries no sidecar for this platform
// (fresh checkout without `task tunnel`).
var errNoEmbeddedTunnel = errors.New("daemon: no embedded kit-tunnel for this platform")

// embeddedTunnelName is the file name staged for the running platform.
func embeddedTunnelName() string {
	return "kit-tunnel-" + runtime.GOOS + "-" + runtime.GOARCH
}

// embeddedTunnelBytes returns the sidecar binary baked into the kit build,
// if the build was staged for this platform.
func embeddedTunnelBytes() ([]byte, bool) {
	b, err := fs.ReadFile(embeddedTunnelFS, "embedded/"+embeddedTunnelName())
	if err != nil {
		return nil, false
	}
	// Guard against picking up documentation placeholders.
	if len(b) < 1<<20 { // 1 MiB: real sidecars are multi-MB binaries
		return nil, false
	}
	return b, true
}

// extractEmbeddedTunnel writes the embedded sidecar to the user cache dir,
// keyed by its SHA-256 so upgrades land in fresh files, and returns the
// executable path. Extraction is atomic (temp file + rename) and idempotent.
func extractEmbeddedTunnel() (string, error) {
	b, ok := embeddedTunnelBytes()
	if !ok {
		return "", errNoEmbeddedTunnel
	}
	sum := sha256.Sum256(b)

	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "kit", "tunnel")
	target := filepath.Join(dir, fmt.Sprintf("kit-tunnel-%x", sum[:10]))

	if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() && info.Size() == int64(len(b)) {
		return target, nil // already extracted
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("daemon: cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".extract-*")
	if err != nil {
		return "", fmt.Errorf("daemon: cache temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(b); err != nil {
		cleanup()
		return "", fmt.Errorf("daemon: cache write: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		cleanup()
		return "", fmt.Errorf("daemon: cache chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("daemon: cache close: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("daemon: cache rename: %w", err)
	}
	return target, nil
}
