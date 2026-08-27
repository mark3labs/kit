package extensions

import (
	"os"
	"path/filepath"
	"testing"
)

// writeBareExt creates a minimal extension file and returns its path.
func writeBareExt(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestDiscoverExtensionPaths_BareSkipsDirectories asserts that bare mode
// ignores every discovery directory — system, user and project-local — so no
// extension runs merely because Kit started in a particular directory.
func TestDiscoverExtensionPaths_BareSkipsDirectories(t *testing.T) {
	sysDir := t.TempDir()
	writeBareExt(t, sysDir, "sys.go")
	t.Setenv(SystemExtensionsDirEnv, sysDir)

	configHome := t.TempDir()
	writeBareExt(t, filepath.Join(configHome, "kit", "extensions"), "user.go")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	// Project-local discovery uses a relative path, so run from a temp cwd.
	project := t.TempDir()
	writeBareExt(t, filepath.Join(project, ".kit", "extensions"), "project.go")
	t.Chdir(project)

	// Sanity check: without bare, all three are discovered.
	if got := discoverExtensionPaths(nil, false); len(got) != 3 {
		t.Fatalf("non-bare discovery: want 3 extensions, got %d (%v)", len(got), got)
	}

	if got := discoverExtensionPaths(nil, true); len(got) != 0 {
		t.Errorf("bare discovery: want no extensions, got %v", got)
	}
}

// TestDiscoverExtensionPaths_BareKeepsExplicitPaths asserts that --extension
// still loads under bare mode. Bare suppresses implicit context, not the
// choices the user made on the command line.
func TestDiscoverExtensionPaths_BareKeepsExplicitPaths(t *testing.T) {
	t.Setenv(SystemExtensionsDirEnv, t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	project := t.TempDir()
	writeBareExt(t, filepath.Join(project, ".kit", "extensions"), "project.go")
	t.Chdir(project)

	explicit := writeBareExt(t, t.TempDir(), "explicit.go")

	got := discoverExtensionPaths([]string{explicit}, true)
	want, _ := filepath.Abs(explicit)
	if len(got) != 1 || got[0] != want {
		t.Errorf("bare discovery with explicit path: want [%s], got %v", want, got)
	}
}
