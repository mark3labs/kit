package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Manifest tracks installed git packages.
type Manifest struct {
	Packages []ManifestEntry `json:"packages"`
}

// ManifestEntry represents a single installed package.
type ManifestEntry struct {
	// Source is the canonical string representation (e.g., "git:github.com/user/repo@v1.0.0")
	Source string `json:"source"`
	// Repo is the clone URL
	Repo string `json:"repo"`
	// Host is the git host (e.g., github.com)
	Host string `json:"host"`
	// Path is the path on the host (e.g., user/repo)
	Path string `json:"path"`
	// Ref is the optional pinned ref (tag/branch/commit)
	Ref string `json:"ref,omitempty"`
	// Pinned indicates if the ref is pinned
	Pinned bool `json:"pinned"`
	// Scope is where the package is installed (global or project)
	Scope InstallScope `json:"scope"`
	// Installed is when the package was first installed
	Installed time.Time `json:"installed"`
	// Updated is when the package was last updated (only for unpinned, zero time means never updated)
	Updated time.Time `json:"updated,omitzero"`
	// Include is a list of relative paths to extensions that should be loaded.
	// If empty, all extensions in the package are loaded.
	// Paths are relative to the package root (e.g., "./git/main.go", "./weather.go")
	Include []string `json:"include,omitempty"`
}

// Identity returns the normalized identity for deduplication.
func (e ManifestEntry) Identity() string {
	return fmt.Sprintf("%s/%s", e.Host, e.Path)
}

// loadManifest loads the manifest from the given scope.
func loadManifestFromScope(scope InstallScope) (*Manifest, error) {
	path := manifestPathForScope(scope)
	return loadManifestFromPath(path)
}

// loadManifestFromPath loads a manifest from a specific file path.
func loadManifestFromPath(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Packages: []ManifestEntry{}}, nil
		}
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	return &manifest, nil
}

// manifestPathForScope returns the manifest file path for a scope.
func manifestPathForScope(scope InstallScope) string {
	if scope == ScopeProject {
		return filepath.Join(".kit", "git", "packages.json")
	}

	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "kit", "git", "packages.json")
}

// FindInManifest finds an entry by identity in either global or project manifest.
// Returns the entry and its scope, or nil if not found.
func FindInManifest(identity string) (*ManifestEntry, InstallScope, error) {
	global, err := loadManifestFromScope(ScopeGlobal)
	if err != nil {
		return nil, "", fmt.Errorf("loading global manifest: %w", err)
	}
	for _, p := range global.Packages {
		if p.Identity() == identity {
			return &p, ScopeGlobal, nil
		}
	}

	project, err := loadManifestFromScope(ScopeProject)
	if err != nil {
		return nil, "", fmt.Errorf("loading project manifest: %w", err)
	}
	for _, p := range project.Packages {
		if p.Identity() == identity {
			return &p, ScopeProject, nil
		}
	}

	return nil, "", nil
}

// ExtensionPreview represents a discovered extension in a package before installation.
type ExtensionPreview struct {
	// Path is the relative path from the package root (e.g., "./git/main.go")
	Path string `json:"path"`
	// Name is a display name for the extension (derived from path or metadata)
	Name string `json:"name"`
	// Description is an optional description (could be extracted from comments)
	Description string `json:"description,omitempty"`
	// IsMain indicates if this is a main.go in a subdirectory
	IsMain bool `json:"is_main"`
}

// ScanForExtensions discovers all extensions in a directory using opinionated conventions.
// Extensions are ONLY recognized in these specific locations:
//  1. Root-level *.go files
//  2. Files in examples/extensions/ or examples/ext/ subdirectories
//  3. Files in any top-level ext/ directory
//  4. Files in any subdirectory that ends in -ext/ or -extensions/
//
// Everything else (cmd/, internal/, pkg/, etc.) is ignored.
func ScanForExtensions(dir string) ([]ExtensionPreview, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}

	candidates, err := walkExtensionCandidates(dir)
	if err != nil {
		return nil, err
	}

	var previews []ExtensionPreview
	for _, candidate := range candidates {
		previews = append(previews, ExtensionPreview{
			Path:   "./" + candidate.relPath,
			Name:   deriveExtensionName(candidate.relPath, candidate.isMain),
			IsMain: candidate.isMain,
		})
	}

	return previews, nil
}

// deriveExtensionName creates a display name from a file path.
func deriveExtensionName(relPath string, isMain bool) string {
	// Convert path to a readable name
	// e.g., "git/main.go" -> "Git Extension"
	// e.g., "weather.go" -> "Weather"

	dir := filepath.Dir(relPath)
	base := filepath.Base(relPath)

	if isMain && dir != "." {
		// Use immediate parent directory name for main.go files
		name := filepath.Base(dir)
		name = strings.ReplaceAll(name, "_", " ")
		name = strings.ReplaceAll(name, "-", " ")
		return cases.Title(language.English).String(name) + " Extension"
	}

	// Use filename without extension
	name := strings.TrimSuffix(base, ".go")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return cases.Title(language.English).String(name)
}
