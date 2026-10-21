package extensions

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// discoveryTree includes both accepted paths and paths that must be ignored.
func discoveryTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, path := range []string{
		"root.go", "ext/x.go", "examples/extensions/y.go",
		"foo-ext/main.go", "foo-ext/helper.go", "pkg2/main.go", "pkg2/helper.go",
		"internal/skip.go", "x_test.go", "main.go", "README.md",
		"examples/main.go", "examples/skip.go", "examples/other/main.go",
		"examples/ext/z.go", "bar-extensions/a.go", "extensions/skip.go",
		"ext/nested-ext/a.go", "ext/other/skip.go",
		".git/ext/a.go", ".github/ext/a.go", "node_modules/ext/a.go",
		"vendor/ext/a.go", "dist/ext/a.go", "build/ext/a.go",
		"internal/ext/a.go", "cmd/ext/a.go", "pkg/ext/a.go",
		"test/ext/a.go", "tests/ext/a.go",
	} {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("package main\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestWalkExtensionCandidates(t *testing.T) {
	root := discoveryTree(t)
	candidates, err := walkExtensionCandidates(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []extensionCandidate{
		{relPath: "bar-extensions/a.go"},
		{relPath: "examples/ext/z.go"},
		{relPath: "examples/extensions/y.go"},
		{relPath: "ext/nested-ext/a.go"},
		{relPath: "ext/x.go"},
		{relPath: "foo-ext/main.go", isMain: true},
		{relPath: "pkg2/main.go", isMain: true},
		{relPath: "root.go"},
	}
	if len(candidates) != len(want) {
		t.Fatalf("got candidates %+v, want %+v", candidates, want)
	}
	for i, expected := range want {
		t.Run(expected.relPath, func(t *testing.T) {
			expected.absPath = filepath.Join(root, filepath.FromSlash(expected.relPath))
			if candidates[i] != expected {
				t.Errorf("got %+v, want %+v", candidates[i], expected)
			}
		})
	}
}

func TestExtensionDiscoveryParity(t *testing.T) {
	root := discoveryTree(t)
	loaded := findExtensionsInRepo(root)
	previews, err := ScanForExtensions(root)
	if err != nil {
		t.Fatal(err)
	}
	var previewPaths []string
	for _, preview := range previews {
		if !strings.HasPrefix(preview.Path, "./") {
			t.Errorf("preview path %q has no ./ prefix", preview.Path)
		}
		previewPaths = append(previewPaths, filepath.Join(root, filepath.FromSlash(preview.Path)))
		isMain := strings.HasSuffix(preview.Path, "/main.go")
		if preview.IsMain != isMain {
			t.Errorf("preview %q: IsMain = %v, want %v", preview.Path, preview.IsMain, isMain)
		}
		wantName := deriveExtensionName(strings.TrimPrefix(preview.Path, "./"), isMain)
		if preview.Name != wantName {
			t.Errorf("preview %q: name = %q, want %q", preview.Path, preview.Name, wantName)
		}
	}
	slices.Sort(loaded)
	slices.Sort(previewPaths)
	if !reflect.DeepEqual(loaded, previewPaths) {
		t.Errorf("loader paths %v differ from preview paths %v", loaded, previewPaths)
	}
}

func TestExtensionDiscoveryErrors(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	if candidates, err := walkExtensionCandidates(missing); err == nil || len(candidates) != 0 {
		t.Errorf("missing root: got %v, %v; want no candidates and an error", candidates, err)
	}
	if paths := findExtensionsInRepo(missing); len(paths) != 0 {
		t.Errorf("loader returned paths for missing root: %v", paths)
	}
	file := filepath.Join(root, "file.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{missing, file} {
		if previews, err := ScanForExtensions(path); err == nil || previews != nil {
			t.Errorf("ScanForExtensions(%q) = %v, %v; want nil and an error", path, previews, err)
		}
	}
}

func TestIsValidExtDir(t *testing.T) {
	for _, dir := range []string{
		"examples/extensions", "examples/extensions/nested", "examples/ext", "examples/ext/nested",
		"ext", "ext/nested", "foo-extensions", "foo-extensions/nested", "foo-ext", "foo-ext/nested",
	} {
		t.Run(dir, func(t *testing.T) {
			if !isValidExtDir(dir) {
				t.Errorf("rejected valid extension directory %q", dir)
			}
		})
	}
	for _, dir := range []string{".", "examples", "extensions", "internal", "extra", "examples/extra", "foo-ext2"} {
		t.Run(dir, func(t *testing.T) {
			if isValidExtDir(dir) {
				t.Errorf("accepted invalid extension directory %q", dir)
			}
		})
	}
}
