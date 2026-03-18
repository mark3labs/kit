package extensions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGitSource(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantRepo   string
		wantHost   string
		wantPath   string
		wantRef    string
		wantPinned bool
		wantErr    bool
	}{
		{
			name:       "github shorthand",
			source:     "github.com/user/repo",
			wantRepo:   "https://github.com/user/repo.git",
			wantHost:   "github.com",
			wantPath:   "user/repo",
			wantRef:    "",
			wantPinned: false,
		},
		{
			name:       "github shorthand with version",
			source:     "github.com/user/repo@v1.0.0",
			wantRepo:   "https://github.com/user/repo.git",
			wantHost:   "github.com",
			wantPath:   "user/repo",
			wantRef:    "v1.0.0",
			wantPinned: true,
		},
		{
			name:       "git prefix shorthand",
			source:     "git:github.com/user/repo",
			wantRepo:   "https://github.com/user/repo.git",
			wantHost:   "github.com",
			wantPath:   "user/repo",
			wantRef:    "",
			wantPinned: false,
		},
		{
			name:       "https URL",
			source:     "https://github.com/user/repo",
			wantRepo:   "https://github.com/user/repo.git",
			wantHost:   "github.com",
			wantPath:   "user/repo",
			wantRef:    "",
			wantPinned: false,
		},
		{
			name:       "https URL with .git suffix",
			source:     "https://github.com/user/repo.git",
			wantRepo:   "https://github.com/user/repo.git",
			wantHost:   "github.com",
			wantPath:   "user/repo",
			wantRef:    "",
			wantPinned: false,
		},
		{
			name:       "ssh shorthand",
			source:     "git@github.com:user/repo",
			wantRepo:   "git@github.com:user/repo",
			wantHost:   "github.com",
			wantPath:   "user/repo",
			wantRef:    "",
			wantPinned: false,
		},
		{
			name:       "ssh URL",
			source:     "ssh://git@github.com/user/repo",
			wantRepo:   "ssh://git@github.com/user/repo",
			wantHost:   "github.com",
			wantPath:   "user/repo",
			wantRef:    "",
			wantPinned: false,
		},
		{
			name:       "gitlab shorthand",
			source:     "gitlab.com/user/repo",
			wantRepo:   "https://gitlab.com/user/repo.git",
			wantHost:   "gitlab.com",
			wantPath:   "user/repo",
			wantRef:    "",
			wantPinned: false,
		},
		{
			name:       "bitbucket shorthand",
			source:     "bitbucket.org/user/repo",
			wantRepo:   "https://bitbucket.org/user/repo.git",
			wantHost:   "bitbucket.org",
			wantPath:   "user/repo",
			wantRef:    "",
			wantPinned: false,
		},
		{
			name:       "generic host",
			source:     "gitea.example.com/user/repo",
			wantRepo:   "https://gitea.example.com/user/repo.git",
			wantHost:   "gitea.example.com",
			wantPath:   "user/repo",
			wantRef:    "",
			wantPinned: false,
		},
		{
			name:       "with branch ref",
			source:     "github.com/user/repo@main",
			wantRepo:   "https://github.com/user/repo.git",
			wantHost:   "github.com",
			wantPath:   "user/repo",
			wantRef:    "main",
			wantPinned: true,
		},
		{
			name:       "with commit ref",
			source:     "github.com/user/repo@abc1234",
			wantRepo:   "https://github.com/user/repo.git",
			wantHost:   "github.com",
			wantPath:   "user/repo",
			wantRef:    "abc1234",
			wantPinned: true,
		},
		{
			name:    "local path should error",
			source:  "./local/path",
			wantErr: true,
		},
		{
			name:    "absolute path should error",
			source:  "/absolute/path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGitSource(tt.source)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGitSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if got.Repo != tt.wantRepo {
				t.Errorf("ParseGitSource() Repo = %v, want %v", got.Repo, tt.wantRepo)
			}
			if got.Host != tt.wantHost {
				t.Errorf("ParseGitSource() Host = %v, want %v", got.Host, tt.wantHost)
			}
			if got.Path != tt.wantPath {
				t.Errorf("ParseGitSource() Path = %v, want %v", got.Path, tt.wantPath)
			}
			if got.Ref != tt.wantRef {
				t.Errorf("ParseGitSource() Ref = %v, want %v", got.Ref, tt.wantRef)
			}
			if got.Pinned != tt.wantPinned {
				t.Errorf("ParseGitSource() Pinned = %v, want %v", got.Pinned, tt.wantPinned)
			}
		})
	}
}

func TestGitSourceIdentity(t *testing.T) {
	source := &GitSource{
		Host: "github.com",
		Path: "user/repo",
	}
	if got := source.Identity(); got != "github.com/user/repo" {
		t.Errorf("Identity() = %v, want %v", got, "github.com/user/repo")
	}
}

func TestGitSourceString(t *testing.T) {
	tests := []struct {
		name   string
		source GitSource
		want   string
	}{
		{
			name: "unpinned",
			source: GitSource{
				Host:   "github.com",
				Path:   "user/repo",
				Pinned: false,
			},
			want: "git:github.com/user/repo",
		},
		{
			name: "pinned",
			source: GitSource{
				Host:   "github.com",
				Path:   "user/repo",
				Ref:    "v1.0.0",
				Pinned: true,
			},
			want: "git:github.com/user/repo@v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.source.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInstallerGetInstallPath(t *testing.T) {
	tempDir := t.TempDir()
	installer := NewInstaller(tempDir)

	source := &GitSource{
		Host: "github.com",
		Path: "user/repo",
	}

	// Test global scope
	globalPath := installer.getInstallPath(source, ScopeGlobal)
	if !filepath.IsAbs(globalPath) {
		t.Error("Global install path should be absolute")
	}

	// Test project scope
	projectPath := installer.getInstallPath(source, ScopeProject)
	expectedProjectPath := filepath.Join(tempDir, ".kit", "git", "github.com", "user", "repo")
	if projectPath != expectedProjectPath {
		t.Errorf("Project path = %v, want %v", projectPath, expectedProjectPath)
	}
}

func TestManifestEntryIdentity(t *testing.T) {
	entry := ManifestEntry{
		Host: "github.com",
		Path: "user/repo",
	}
	if got := entry.Identity(); got != "github.com/user/repo" {
		t.Errorf("Identity() = %v, want %v", got, "github.com/user/repo")
	}
}

func TestLoadAndSaveManifest(t *testing.T) {
	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "packages.json")

	// Test loading non-existent manifest
	manifest, err := loadManifestFromPath(manifestPath)
	if err != nil {
		t.Fatalf("loadManifestFromPath() error = %v", err)
	}
	if len(manifest.Packages) != 0 {
		t.Errorf("Expected empty packages, got %d", len(manifest.Packages))
	}

	// Create a manifest
	manifest = &Manifest{
		Packages: []ManifestEntry{
			{
				Source: "git:github.com/user/repo",
				Repo:   "https://github.com/user/repo.git",
				Host:   "github.com",
				Path:   "user/repo",
				Pinned: false,
				Scope:  ScopeGlobal,
			},
		},
	}

	// Save it
	err = saveManifestToPath(manifest, manifestPath)
	if err != nil {
		t.Fatalf("saveManifestToPath() error = %v", err)
	}

	// Load it back
	loaded, err := loadManifestFromPath(manifestPath)
	if err != nil {
		t.Fatalf("loadManifestFromPath() error = %v", err)
	}
	if len(loaded.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(loaded.Packages))
	}
	if loaded.Packages[0].Host != "github.com" {
		t.Errorf("Expected host github.com, got %s", loaded.Packages[0].Host)
	}
}

func TestAddAndRemoveFromManifest(t *testing.T) {
	tempDir := t.TempDir()

	// Set up environment for manifest path
	if err := os.Setenv("XDG_DATA_HOME", tempDir); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_DATA_HOME"); err != nil {
			t.Logf("Unsetenv() error = %v", err)
		}
	}()

	// The manifest path when XDG_DATA_HOME is set
	manifestPath := filepath.Join(tempDir, "kit", "git", "packages.json")

	// Add an entry
	entry := ManifestEntry{
		Source: "git:github.com/user/repo",
		Host:   "github.com",
		Path:   "user/repo",
		Scope:  ScopeGlobal,
	}

	err := addEntryToManifest(entry, ScopeGlobal)
	if err != nil {
		t.Fatalf("addEntryToManifest() error = %v", err)
	}

	// Verify it was added
	manifest, err := loadManifestFromPath(manifestPath)
	if err != nil {
		t.Fatalf("loadManifestFromPath() error = %v", err)
	}
	if len(manifest.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(manifest.Packages))
	}

	// Remove it
	err = removeEntryFromManifest("github.com/user/repo", ScopeGlobal)
	if err != nil {
		t.Fatalf("removeEntryFromManifest() error = %v", err)
	}

	// Verify it was removed
	manifest, err = loadManifestFromPath(manifestPath)
	if err != nil {
		t.Fatalf("loadManifestFromPath() error = %v", err)
	}
	if len(manifest.Packages) != 0 {
		t.Errorf("Expected 0 packages, got %d", len(manifest.Packages))
	}
}

func TestFindInManifest(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.Setenv("XDG_DATA_HOME", tempDir); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_DATA_HOME"); err != nil {
			t.Logf("Unsetenv() error = %v", err)
		}
	}()

	// Add an entry to global manifest
	entry := ManifestEntry{
		Source: "git:github.com/user/repo",
		Host:   "github.com",
		Path:   "user/repo",
		Scope:  ScopeGlobal,
	}

	err := addEntryToManifest(entry, ScopeGlobal)
	if err != nil {
		t.Fatalf("addEntryToManifest() error = %v", err)
	}

	// Find it
	found, scope, err := FindInManifest("github.com/user/repo")
	if err != nil {
		t.Fatalf("FindInManifest() error = %v", err)
	}
	if found == nil {
		t.Fatal("Expected to find entry, got nil")
	}
	if scope != ScopeGlobal {
		t.Errorf("Expected scope global, got %s", scope)
	}

	// Try to find non-existent
	notFound, _, err := FindInManifest("github.com/other/repo")
	if err != nil {
		t.Fatalf("FindInManifest() error = %v", err)
	}
	if notFound != nil {
		t.Error("Expected nil for non-existent entry")
	}
}
