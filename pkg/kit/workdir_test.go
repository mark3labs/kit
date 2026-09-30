package kit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWorkDir_CoreToolsUseIt checks that Options.WorkDir, not the process
// working directory, is the base directory of the built-in tools. The ACP
// server depends on this: one process serves sessions for several projects.
func TestWorkDir_CoreToolsUseIt(t *testing.T) {
	dir := t.TempDir()
	const marker = "workdir_marker.txt"
	if err := os.WriteFile(filepath.Join(dir, marker), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	model := &scriptedToolModel{script: []string{"ls"}}
	model.provider, model.model = "script", "m"
	factory := func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
		return &ProviderResult{Model: model}, nil
	}

	k, err := New(context.Background(), &Options{
		Model:        "script/m",
		Providers:    map[string]ProviderFactory{"script": factory},
		CoreToolList: []string{"ls"},
		WorkDir:      dir,
		Quiet:        true,
		NoSession:    true,
		NoExtensions: true,
		SkipConfig:   true,
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	t.Cleanup(func() { _ = k.Close() })

	var result string
	unsub := k.Subscribe(func(e Event) {
		if ev, ok := e.(ToolResultEvent); ok && ev.ToolName == "ls" {
			result = ev.Result
		}
	})
	defer unsub()

	if _, err := k.PromptResult(context.Background(), "list"); err != nil {
		t.Fatalf("PromptResult: %v", err)
	}
	if !strings.Contains(result, marker) {
		t.Fatalf("ls ran outside WorkDir %s; result:\n%s", dir, result)
	}
}
