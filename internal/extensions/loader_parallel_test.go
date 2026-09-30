package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadExtensionsScoped_InitRunsInDiscoveryOrder checks that compiling
// extensions in parallel did not change the order Init runs in: each
// extension appends its name to a shared log file from Init.
func TestLoadExtensionsScoped_InitRunsInDiscoveryOrder(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "init.log")

	var paths []string
	for i := range 8 {
		name := fmt.Sprintf("ext%d", i)
		src := fmt.Sprintf(`package main

import (
	"os"

	"kit/ext"
)

func Init(api ext.API) {
	f, err := os.OpenFile(%q, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.WriteString(%q + "\n")
	api.OnSessionStart(func(_ ext.SessionStartEvent, _ ext.Context) {})
}
`, logPath, name)
		p := filepath.Join(dir, name+".go")
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}

	loaded, err := LoadExtensionsScoped(paths, true)
	if err != nil {
		t.Fatalf("LoadExtensionsScoped: %v", err)
	}
	if len(loaded) != len(paths) {
		t.Fatalf("loaded %d extensions, want %d", len(loaded), len(paths))
	}
	for i := range loaded {
		if loaded[i].Path != paths[i] {
			t.Errorf("loaded[%d].Path = %s, want %s", i, loaded[i].Path, paths[i])
		}
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "ext0\next1\next2\next3\next4\next5\next6\next7\n"
	if string(data) != want {
		t.Errorf("Init order:\n%s\nwant:\n%s", data, want)
	}
}

// TestLoadExtensionsScoped_ExamplesConcurrently compiles every example
// extension at once. Its real value is under `go test -race`: it catches
// shared state between Yaegi interpreters.
func TestLoadExtensionsScoped_ExamplesConcurrently(t *testing.T) {
	files, err := filepath.Glob("../../examples/extensions/*.go")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range files {
		if !strings.HasSuffix(f, "_test.go") {
			paths = append(paths, f)
		}
	}
	if len(paths) < 2 {
		t.Skip("no example extensions found")
	}

	loaded, err := LoadExtensionsScoped(paths, true)
	if err != nil {
		t.Fatalf("LoadExtensionsScoped: %v", err)
	}
	// Every example is expected to load; compare with a serial load so a
	// file that is broken for unrelated reasons does not fail this test.
	serial := 0
	for _, p := range paths {
		if _, err := loadSingleExtension(p); err == nil {
			serial++
		}
	}
	if len(loaded) != serial {
		t.Errorf("parallel load got %d extensions, serial load got %d", len(loaded), serial)
	}
}
