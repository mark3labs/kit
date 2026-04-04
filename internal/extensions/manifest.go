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

// saveManifestToScope saves the manifest to the given scope.
func saveManifestToScope(manifest *Manifest, scope InstallScope) error {
	path := manifestPathForScope(scope)
	return saveManifestToPath(manifest, path)
}

// saveManifestToPath saves a manifest to a specific file path.
func saveManifestToPath(manifest *Manifest, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
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

// GetGlobalManifest returns the global manifest.
func GetGlobalManifest() (*Manifest, error) {
	return loadManifestFromScope(ScopeGlobal)
}

// GetProjectManifest returns the project manifest.
func GetProjectManifest() (*Manifest, error) {
	return loadManifestFromScope(ScopeProject)
}

// addEntryToManifest adds or replaces an entry in the manifest for a scope.
func addEntryToManifest(entry ManifestEntry, scope InstallScope) error {
	manifest, err := loadManifestFromScope(scope)
	if err != nil {
		return err
	}

	// Remove any existing entry with same identity
	identity := entry.Identity()
	filtered := make([]ManifestEntry, 0, len(manifest.Packages))
	for _, p := range manifest.Packages {
		if p.Identity() != identity {
			filtered = append(filtered, p)
		}
	}
	filtered = append(filtered, entry)
	manifest.Packages = filtered

	return saveManifestToScope(manifest, scope)
}

// removeEntryFromManifest removes an entry by identity from the manifest for a scope.
func removeEntryFromManifest(identity string, scope InstallScope) error {
	manifest, err := loadManifestFromScope(scope)
	if err != nil {
		return err
	}

	filtered := make([]ManifestEntry, 0, len(manifest.Packages))
	for _, p := range manifest.Packages {
		if p.Identity() != identity {
			filtered = append(filtered, p)
		}
	}
	manifest.Packages = filtered

	return saveManifestToScope(manifest, scope)
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

	var previews []ExtensionPreview
	multiFileDirs := make(map[string]bool)

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(dir, path)
		relPath = filepath.ToSlash(relPath)

		// Skip directories we know don't contain extensions
		if info.IsDir() {
			// Never scan these directories
			switch info.Name() {
			case ".git", ".github", "node_modules", "vendor", "dist", "build":
				return filepath.SkipDir
			}

			// Skip internal code directories
			if strings.HasPrefix(relPath, "internal/") ||
				strings.HasPrefix(relPath, "cmd/") ||
				strings.HasPrefix(relPath, "pkg/") ||
				strings.HasPrefix(relPath, "test/") ||
				strings.HasPrefix(relPath, "tests/") {
				return filepath.SkipDir
			}

			// Root directory - scan it
			if relPath == "." {
				return nil
			}

			// Check if this directory is an extension location by name
			// Pattern: must be named "extensions", "ext", or end with those
			base := info.Name()
			isExtDir := base == "extensions" || base == "ext" ||
				strings.HasSuffix(base, "-extensions") || strings.HasSuffix(base, "-ext")

			// Allow walking into examples/ so we can reach examples/extensions/ etc,
			// but don't treat examples/ itself or non-extension subdirs as extension locations.
			if relPath == "examples" {
				return nil
			}

			if !isExtDir {
				// Check for main.go before skipping
				mainPath := filepath.Join(path, "main.go")
				if _, err := os.Stat(mainPath); err == nil {
					// This is a package with main.go at root level
					if relPath == base { // Top-level directory
						if !multiFileDirs[relPath] {
							multiFileDirs[relPath] = true
							previews = append(previews, ExtensionPreview{
								Path:   "./" + relPath + "/main.go",
								Name:   deriveExtensionName(relPath+"/main.go", true),
								IsMain: true,
							})
						}
						return filepath.SkipDir
					}
				}

				// Not an extension location
				return filepath.SkipDir
			}

			// Check for main.go in this directory
			mainPath := filepath.Join(path, "main.go")
			if _, err := os.Stat(mainPath); err == nil {
				if !multiFileDirs[relPath] {
					multiFileDirs[relPath] = true
					previews = append(previews, ExtensionPreview{
						Path:   "./" + relPath + "/main.go",
						Name:   deriveExtensionName(relPath+"/main.go", true),
						IsMain: true,
					})
				}
				return filepath.SkipDir
			}

			// Scan this extensions directory
			return nil
		}

		// It's a file - check if it's a valid extension
		if !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		if info.Name() == "main.go" {
			return nil // Already handled above
		}

		// Check if parent is a valid extension location
		parentDir := filepath.Dir(relPath)
		if parentDir == "." {
			// Root-level .go file - valid extension
			previews = append(previews, ExtensionPreview{
				Path:   "./" + relPath,
				Name:   deriveExtensionName(relPath, false),
				IsMain: false,
			})
			return nil
		}

		// Check if we're in a valid extension directory
		// Valid locations are:
		// - examples/extensions/*
		// - examples/ext/*
		// - ext/* (top-level)
		// - Any *-extensions/* or *-ext/* directory
		isValidExtDir := false
		if strings.HasPrefix(parentDir, "examples/extensions/") ||
			parentDir == "examples/extensions" {
			isValidExtDir = true
		} else if strings.HasPrefix(parentDir, "examples/ext/") ||
			parentDir == "examples/ext" {
			isValidExtDir = true
		} else if strings.HasPrefix(parentDir, "ext/") ||
			parentDir == "ext" {
			isValidExtDir = true
		} else if strings.Contains(parentDir, "-extensions/") ||
			strings.HasSuffix(parentDir, "-extensions") {
			isValidExtDir = true
		} else if strings.Contains(parentDir, "-ext/") ||
			strings.HasSuffix(parentDir, "-ext") {
			isValidExtDir = true
		}

		if !isValidExtDir {
			return nil
		}

		previews = append(previews, ExtensionPreview{
			Path:   "./" + relPath,
			Name:   deriveExtensionName(relPath, false),
			IsMain: false,
		})

		return nil
	})

	if err != nil {
		return nil, err
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
