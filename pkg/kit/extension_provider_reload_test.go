package kit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtensionProviderReloadRetainsOldRuntimeAndRollsBackFailure(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "examples", "extensions", "fake-provider.go"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "provider.go")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	// Reuse the real fixture's Kit setup, then point reload at the mutable copy.
	k := extensionProviderKit(t, "fake-provider.go", "fake/echo", nil)
	k.v.Set("extension", []string{path})
	if err := os.WriteFile(path, []byte("package main\nfunc broken syntax"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := k.ReloadExtensions(); err == nil {
		t.Fatal("invalid candidate accepted")
	}
	got, err := k.Prompt(context.Background(), "old works")
	if err != nil || got != "Hello from a fake provider." {
		t.Fatalf("old=%q err=%v", got, err)
	}
	replacement := strings.ReplaceAll(string(source), "Hello from a fake provider.", "New generation.")
	if err := os.WriteFile(path, []byte(replacement), 0600); err != nil {
		t.Fatal(err)
	}
	if err := k.ReloadExtensions(); err != nil {
		t.Fatal(err)
	}
	got, err = k.Prompt(context.Background(), "existing runtime")
	if err != nil || got != "Hello from a fake provider." {
		t.Fatalf("retained=%q err=%v", got, err)
	}
	result, err := k.ExecuteCompletion(context.Background(), CompleteRequest{Model: "fake/echo", Prompt: "new runtime"})
	if err != nil || result.Text != "New generation." {
		t.Fatalf("new=%+v err=%v", result, err)
	}
}
