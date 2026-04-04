package extensions

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// InstallScope defines where a package should be installed.
type InstallScope string

const (
	ScopeGlobal  InstallScope = "global"
	ScopeProject InstallScope = "project"
)

// GitSource represents a parsed git repository URL.
type GitSource struct {
	Repo   string // Clone URL (e.g., https://github.com/user/repo.git)
	Host   string // Host (e.g., github.com)
	Path   string // Path (e.g., user/repo)
	Ref    string // Optional ref (tag, branch, commit)
	Pinned bool   // Whether a specific ref is pinned
}

// String returns the canonical string representation.
func (g GitSource) String() string {
	if g.Pinned {
		return fmt.Sprintf("git:%s/%s@%s", g.Host, g.Path, g.Ref)
	}
	return fmt.Sprintf("git:%s/%s", g.Host, g.Path)
}

// Identity returns a normalized identity string for deduplication.
func (g GitSource) Identity() string {
	return fmt.Sprintf("%s/%s", g.Host, g.Path)
}

// ParseGitSource parses a git source string into a GitSource.
// Supports formats like:
//   - git:github.com/user/repo
//   - git:github.com/user/repo@v1.0.0
//   - https://github.com/user/repo
//   - https://github.com/user/repo@v1.0.0
//   - ssh://git@github.com/user/repo
//   - git@github.com:user/repo
//   - github.com/user/repo (shorthand, defaults to https)
func ParseGitSource(source string) (*GitSource, error) {
	source = strings.TrimSpace(source)

	// Check for @ref suffix
	ref := ""
	pinned := false
	if atIdx := strings.LastIndex(source, "@"); atIdx > 0 {
		// Make sure it's not part of the protocol (e.g., @ in ssh://git@)
		after := source[atIdx+1:]
		if !strings.Contains(after, "/") && !strings.Contains(after, ":") {
			ref = after
			pinned = true
			source = source[:atIdx]
		}
	}

	// Handle git: prefix
	source, _ = strings.CutPrefix(source, "git:")

	var repo, host, path string

	// Handle explicit URLs
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		u, err := url.Parse(source)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		host = u.Host
		path = strings.TrimPrefix(u.Path, "/")
		path, _ = strings.CutSuffix(path, ".git")
		repo = source
		if !strings.HasSuffix(repo, ".git") {
			repo += ".git"
		}
	} else if strings.HasPrefix(source, "ssh://") {
		u, err := url.Parse(source)
		if err != nil {
			return nil, fmt.Errorf("invalid SSH URL: %w", err)
		}
		host = u.Host
		path = strings.TrimPrefix(u.Path, "/")
		path, _ = strings.CutSuffix(path, ".git")
		repo = source
	} else if strings.HasPrefix(source, "git@") {
		// SSH shorthand: git@github.com:user/repo
		parts := strings.SplitN(source, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid SSH shorthand format")
		}
		host = strings.TrimPrefix(parts[0], "git@")
		path = parts[1]
		path, _ = strings.CutSuffix(path, ".git")
		repo = source
	} else if strings.HasPrefix(source, "github.com/") || strings.HasPrefix(source, "gitlab.com/") || strings.HasPrefix(source, "bitbucket.org/") {
		// Shorthand for known hosts: host/path
		parts := strings.SplitN(source, "/", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid shorthand format, expected host/path")
		}
		host = parts[0]
		path = parts[1]
		repo = fmt.Sprintf("https://%s/%s.git", host, path)
	} else if strings.HasPrefix(source, ".") || strings.HasPrefix(source, "/") || strings.HasPrefix(source, "~") {
		// Local paths are not supported
		return nil, fmt.Errorf("local paths not supported, use explicit extension path with -e flag")
	} else {
		// Generic shorthand: host/user/repo (3+ path segments)
		parts := strings.Split(source, "/")
		if len(parts) >= 3 {
			host = parts[0]
			path = strings.Join(parts[1:], "/")
			repo = fmt.Sprintf("https://%s/%s.git", host, path)
		} else {
			return nil, fmt.Errorf("unrecognized source format: %s", source)
		}
	}

	return &GitSource{
		Repo:   repo,
		Host:   host,
		Path:   path,
		Ref:    ref,
		Pinned: pinned,
	}, nil
}

// Installer handles installing, updating, and removing git-based extensions.
type Installer struct {
	// Global packages root: $XDG_DATA_HOME/kit/git/ (default ~/.local/share/kit/git/)
	globalGitRoot string
	// Project packages root: .kit/git/
	projectGitRoot string
}

// NewInstaller creates a new Installer.
func NewInstaller(projectDir string) *Installer {
	return &Installer{
		globalGitRoot:  globalGitInstallRoot(),
		projectGitRoot: filepath.Join(projectDir, ".kit", "git"),
	}
}

// Install clones a git repository to the appropriate scope.
func (i *Installer) Install(source *GitSource, scope InstallScope) error {
	return i.install(source, scope, nil)
}

// install is the internal implementation that supports optional include paths.
func (i *Installer) install(source *GitSource, scope InstallScope, includePaths []string) error {
	targetDir := i.getInstallPath(source, scope)

	// Check if already installed
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("extension already installed at %s", targetDir)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	// Clone the repository
	cmd := exec.Command("git", "clone", "--depth=1", source.Repo, targetDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\n%s", err, string(output))
	}

	// Checkout specific ref if pinned
	if source.Pinned && source.Ref != "" {
		checkoutCmd := exec.Command("git", "checkout", source.Ref)
		checkoutCmd.Dir = targetDir
		if output, err := checkoutCmd.CombinedOutput(); err != nil {
			// Clean up on failed checkout
			_ = os.RemoveAll(targetDir)
			return fmt.Errorf("git checkout failed: %w\n%s", err, string(output))
		}
	}

	// Validate that the package contains valid extensions
	if err := i.validatePackage(targetDir); err != nil {
		_ = os.RemoveAll(targetDir)
		return fmt.Errorf("validation failed: %w", err)
	}

	// Add to manifest
	entry := ManifestEntry{
		Source:    source.String(),
		Repo:      source.Repo,
		Host:      source.Host,
		Path:      source.Path,
		Ref:       source.Ref,
		Pinned:    source.Pinned,
		Scope:     scope,
		Installed: time.Now(),
		Include:   includePaths,
	}
	if err := i.addToManifest(entry, scope); err != nil {
		// Don't fail the install, just log the error
		// The package is installed, manifest update failed
		return fmt.Errorf("installed but failed to update manifest: %w", err)
	}

	return nil
}

// Uninstall removes an installed package.
func (i *Installer) Uninstall(source *GitSource, scope InstallScope) error {
	targetDir := i.getInstallPath(source, scope)

	if _, err := os.Stat(targetDir); err != nil {
		return fmt.Errorf("extension not found at %s", targetDir)
	}

	// Remove the directory
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("removing extension directory: %w", err)
	}

	// Remove from manifest
	if err := i.removeFromManifest(source.Identity(), scope); err != nil {
		return fmt.Errorf("removed but failed to update manifest: %w", err)
	}

	return nil
}

// Update fetches and resets a git package to the latest.
// For pinned packages, this does nothing.
func (i *Installer) Update(source *GitSource, scope InstallScope) error {
	if source.Pinned {
		return nil // Don't update pinned packages
	}

	targetDir := i.getInstallPath(source, scope)

	if _, err := os.Stat(targetDir); err != nil {
		return i.Install(source, scope)
	}

	// Fetch latest
	fetchCmd := exec.Command("git", "fetch", "--prune", "origin")
	fetchCmd.Dir = targetDir
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w\n%s", err, string(output))
	}

	// Reset to tracking branch or origin/HEAD
	resetCmd := exec.Command("git", "reset", "--hard", "@{upstream}")
	resetCmd.Dir = targetDir
	if _, err := resetCmd.CombinedOutput(); err != nil {
		// Try alternative: set HEAD and reset to origin/HEAD
		_ = exec.Command("git", "remote", "set-head", "origin", "-a").Run()
		resetCmd = exec.Command("git", "reset", "--hard", "origin/HEAD")
		resetCmd.Dir = targetDir
		if output, err := resetCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git reset failed: %w\n%s", err, string(output))
		}
	}

	// Clean untracked files
	cleanCmd := exec.Command("git", "clean", "-fdx")
	cleanCmd.Dir = targetDir
	_ = cleanCmd.Run() // Ignore errors - clean is best effort

	// Update manifest timestamp, preserving existing fields like Include
	existing, _ := i.loadManifest(scope)
	var include []string
	var installed time.Time
	if existing != nil {
		for _, p := range existing.Packages {
			if p.Host+"/"+p.Path == source.Identity() {
				include = p.Include
				installed = p.Installed
				break
			}
		}
	}
	if installed.IsZero() {
		installed = time.Now()
	}
	entry := ManifestEntry{
		Source:    source.String(),
		Repo:      source.Repo,
		Host:      source.Host,
		Path:      source.Path,
		Ref:       "",
		Pinned:    false,
		Scope:     scope,
		Installed: installed,
		Updated:   time.Now(),
		Include:   include,
	}
	_ = i.addToManifest(entry, scope) // Best effort - don't fail update if manifest fails

	return nil
}

// getInstallPath returns the target directory for a source.
func (i *Installer) getInstallPath(source *GitSource, scope InstallScope) string {
	root := i.globalGitRoot
	if scope == ScopeProject {
		root = i.projectGitRoot
	}
	return filepath.Join(root, source.Host, source.Path)
}

// validatePackage checks that the cloned repo contains valid .go extension files.
func (i *Installer) validatePackage(dir string) error {
	// Find all .go files in the directory
	var goFiles []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking directory: %w", err)
	}

	if len(goFiles) == 0 {
		return fmt.Errorf("no .go files found in package")
	}

	// Try to load the first .go file to validate it's a valid extension
	// We don't fail if validation fails - the extension might be fine but
	// have dependencies that aren't available during install time
	_, err = loadSingleExtension(goFiles[0])
	if err != nil {
		// Log but don't fail - the extension might need runtime deps
		// User can use `kit extensions validate` to check later
		return nil
	}

	return nil
}

// addToManifest adds an entry to the manifest.
func (i *Installer) addToManifest(entry ManifestEntry, scope InstallScope) error {
	manifest, err := i.loadManifest(scope)
	if err != nil {
		return err
	}

	// Remove any existing entry with same identity
	identity := entry.Host + "/" + entry.Path
	filtered := make([]ManifestEntry, 0, len(manifest.Packages))
	for _, p := range manifest.Packages {
		if p.Host+"/"+p.Path != identity {
			filtered = append(filtered, p)
		}
	}
	filtered = append(filtered, entry)
	manifest.Packages = filtered

	return i.saveManifest(manifest, scope)
}

// removeFromManifest removes an entry from the manifest by identity.
func (i *Installer) removeFromManifest(identity string, scope InstallScope) error {
	manifest, err := i.loadManifest(scope)
	if err != nil {
		return err
	}

	filtered := make([]ManifestEntry, 0, len(manifest.Packages))
	for _, p := range manifest.Packages {
		if p.Host+"/"+p.Path != identity {
			filtered = append(filtered, p)
		}
	}
	manifest.Packages = filtered

	return i.saveManifest(manifest, scope)
}

// loadManifest loads the manifest for the given scope.
func (i *Installer) loadManifest(scope InstallScope) (*Manifest, error) {
	path := i.manifestPath(scope)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Packages: []ManifestEntry{}}, nil
		}
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	return &manifest, nil
}

// saveManifest saves the manifest for the given scope.
func (i *Installer) saveManifest(manifest *Manifest, scope InstallScope) error {
	path := i.manifestPath(scope)

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

// manifestPath returns the path to the manifest file.
func (i *Installer) manifestPath(scope InstallScope) string {
	if scope == ScopeProject {
		return filepath.Join(i.projectGitRoot, "packages.json")
	}
	return filepath.Join(i.globalGitRoot, "packages.json")
}

// globalGitInstallRoot returns the global git install root.
func globalGitInstallRoot() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "kit", "git")
}

// GetInstalledPackages returns all installed packages from both scopes.
func (i *Installer) GetInstalledPackages() ([]ManifestEntry, error) {
	var all []ManifestEntry

	global, err := i.loadManifest(ScopeGlobal)
	if err != nil {
		return nil, fmt.Errorf("loading global manifest: %w", err)
	}
	all = append(all, global.Packages...)

	project, err := i.loadManifest(ScopeProject)
	if err != nil {
		return nil, fmt.Errorf("loading project manifest: %w", err)
	}
	all = append(all, project.Packages...)

	return all, nil
}

// IsInstalled checks if a package is installed in either scope.
// Returns (scope, true) if installed, ("", false) otherwise.
func (i *Installer) IsInstalled(source *GitSource) (InstallScope, bool) {
	globalPath := i.getInstallPath(source, ScopeGlobal)
	if _, err := os.Stat(globalPath); err == nil {
		return ScopeGlobal, true
	}

	projectPath := i.getInstallPath(source, ScopeProject)
	if _, err := os.Stat(projectPath); err == nil {
		return ScopeProject, true
	}

	return "", false
}

// PreviewExtensions clones a repo to a temporary directory and scans for extensions.
// Returns the preview list and the temp directory path (caller should clean up).
func (i *Installer) PreviewExtensions(source *GitSource) ([]ExtensionPreview, string, error) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "kit-install-preview-*")
	if err != nil {
		return nil, "", fmt.Errorf("creating temp directory: %w", err)
	}

	// Clone to temp
	cloneDir := filepath.Join(tempDir, "repo")
	cmd := exec.Command("git", "clone", "--depth=1", source.Repo, cloneDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("git clone failed: %w\n%s", err, string(output))
	}

	// Checkout specific ref if pinned
	if source.Pinned && source.Ref != "" {
		checkoutCmd := exec.Command("git", "checkout", source.Ref)
		checkoutCmd.Dir = cloneDir
		if output, err := checkoutCmd.CombinedOutput(); err != nil {
			_ = os.RemoveAll(tempDir)
			return nil, "", fmt.Errorf("git checkout failed: %w\n%s", err, string(output))
		}
	}

	// Scan for extensions
	previews, err := ScanForExtensions(cloneDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("scanning extensions: %w", err)
	}

	return previews, tempDir, nil
}

// InstallWithInclude clones a repo and installs only the specified extensions.
// includePaths are relative paths like "./git/main.go" - if empty, installs all.
func (i *Installer) InstallWithInclude(source *GitSource, scope InstallScope, includePaths []string) error {
	return i.install(source, scope, includePaths)
}

// CleanupTempDir removes a temporary directory used for preview.
func CleanupTempDir(tempDir string) {
	if tempDir != "" {
		_ = os.RemoveAll(tempDir)
	}
}
