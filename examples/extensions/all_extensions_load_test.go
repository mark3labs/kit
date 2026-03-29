package main

import (
	"testing"

	"github.com/mark3labs/kit/pkg/extensions/test"
)

// TestAllExtensions_Load is a smoke test that verifies every single-file
// example extension in this directory can be loaded by the Yaegi interpreter
// without errors. This catches syntax errors, missing symbols, bad imports,
// and Init signature mismatches.
func TestAllExtensions_Load(t *testing.T) {
	files := extensionFiles(t)

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			harness := test.New(t)
			ext := harness.LoadFile(file)
			if ext == nil {
				t.Fatalf("%s: extension should not be nil after loading", file)
			}
		})
	}

	t.Logf("successfully loaded %d extensions", len(files))
}
