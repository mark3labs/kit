package kit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestSetModelKeepsRuntimePromptChanges checks that a model switch keeps a
// context file added at runtime. The switch used to put back the prompt
// that New had composed.
func TestSetModelKeepsRuntimePromptChanges(t *testing.T) {
	k := newBareTestKit(t, false)

	const marker = "RUNTIME-CONTEXT-MARKER"
	if err := k.AddContextFile(&ContextFile{Path: "/runtime/AGENTS.md", Content: marker}); err != nil {
		t.Fatalf("AddContextFile: %v", err)
	}
	if err := k.SetModel(context.Background(), "openai/gpt-4o-mini"); err != nil {
		t.Fatalf("SetModel: %v", err)
	}

	prompt, _ := k.agentSnapshotForTest()
	if !strings.Contains(prompt, marker) {
		t.Errorf("prompt after SetModel lacks the context file added at runtime:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Always speak in Latin.") {
		t.Errorf("prompt after SetModel lacks the AGENTS.md content loaded by New:\n%s", prompt)
	}
}

// TestSetModelPerModelPrompt switches to a model with its own system prompt
// and back. The per-model prompt replaces the default, stays in place when
// the context files change, and goes away with the next switch.
func TestSetModelPerModelPrompt(t *testing.T) {
	requireCatalogModel(t, testCatalogModel)
	k := newBareTestKit(t, false)

	const perModel = "PER-MODEL-PROMPT-MARKER"
	k.v.Set("modelSettings", map[string]any{
		testCatalogModel: map[string]any{"systemPrompt": perModel},
	})

	if err := k.SetModel(context.Background(), testCatalogModel); err != nil {
		t.Fatalf("SetModel(%s): %v", testCatalogModel, err)
	}
	prompt, _ := k.agentSnapshotForTest()
	if !strings.Contains(prompt, perModel) || strings.Contains(prompt, defaultSystemPrompt) {
		t.Fatalf("prompt after the switch to %s: want the per-model prompt instead of the default:\n%s", testCatalogModel, prompt)
	}

	// A runtime change composes the prompt again from the per-model prompt.
	const marker = "RUNTIME-CONTEXT-MARKER"
	if err := k.AddContextFile(&ContextFile{Path: "/runtime/AGENTS.md", Content: marker}); err != nil {
		t.Fatalf("AddContextFile: %v", err)
	}
	prompt, _ = k.agentSnapshotForTest()
	if !strings.Contains(prompt, perModel) || !strings.Contains(prompt, marker) {
		t.Errorf("prompt after AddContextFile: want the per-model prompt and the new context file:\n%s", prompt)
	}

	// A model without its own prompt gets the default back.
	if err := k.SetModel(context.Background(), "openai/gpt-4o-mini"); err != nil {
		t.Fatalf("SetModel(openai/gpt-4o-mini): %v", err)
	}
	prompt, _ = k.agentSnapshotForTest()
	if strings.Contains(prompt, perModel) || !strings.Contains(prompt, defaultSystemPrompt) {
		t.Errorf("prompt after the switch back: want the default instead of the per-model prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, marker) {
		t.Errorf("prompt after the switch back lacks the context file added at runtime:\n%s", prompt)
	}
}

// TestSetModelEmptyPerModelPrompt checks that a per-model prompt that is
// configured but empty replaces the default on a model switch, as it does
// in New (#122).
func TestSetModelEmptyPerModelPrompt(t *testing.T) {
	requireCatalogModel(t, testCatalogModel)
	k := newBareTestKit(t, false)

	emptyFile := filepath.Join(t.TempDir(), "empty-prompt.md")
	if err := os.WriteFile(emptyFile, []byte("  \n"), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}
	k.v.Set("modelSettings", map[string]any{
		testCatalogModel: map[string]any{"systemPrompt": emptyFile},
	})

	if err := k.SetModel(context.Background(), testCatalogModel); err != nil {
		t.Fatalf("SetModel(%s): %v", testCatalogModel, err)
	}
	if prompt, _ := k.agentSnapshotForTest(); strings.Contains(prompt, defaultSystemPrompt) {
		t.Errorf("prompt still contains the built-in default; the empty per-model prompt must replace it:\n%s", prompt)
	}
}

// TestBuildSystemPromptKeepsGlobalBase checks that New keeps the global base
// prompt when a per-model prompt replaces it. A later switch to a model
// without its own prompt starts from the global base prompt again.
func TestBuildSystemPromptKeepsGlobalBase(t *testing.T) {
	requireCatalogModel(t, testCatalogModel)

	const perModel = "PER-MODEL-PROMPT-MARKER"
	v := viper.New()
	v.Set("model", testCatalogModel)
	v.Set("system-prompt", defaultSystemPrompt)
	v.Set("modelSettings", map[string]any{
		testCatalogModel: map[string]any{"systemPrompt": perModel},
	})

	rc := &resolvedConfig{}
	buildSystemPrompt(v, &Options{}, rc)

	if rc.basePrompt != perModel {
		t.Errorf("basePrompt = %q, want the per-model prompt", rc.basePrompt)
	}
	if rc.globalBasePrompt != defaultSystemPrompt {
		t.Errorf("globalBasePrompt = %q, want the built-in default", rc.globalBasePrompt)
	}
}

// TestComposeSystemPromptUsesSessionDir checks that a composed prompt names
// the session directory, as the prompt from New does. ACP sets one
// directory per session, which differs from the directory of the process.
func TestComposeSystemPromptUsesSessionDir(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Chdir(t.TempDir())
	sessionDir := t.TempDir()

	k, err := New(context.Background(), &Options{
		Model:            "openai/gpt-4o-mini",
		SessionDir:       sessionDir,
		Quiet:            true,
		NoSession:        true,
		NoExtensions:     true,
		DisableCoreTools: true,
		SkipConfig:       true,
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	t.Cleanup(func() { _ = k.Close() })

	want := "Current working directory: " + sessionDir
	if prompt, _ := k.agentSnapshotForTest(); !strings.Contains(prompt, want) {
		t.Fatalf("prompt after New lacks %q:\n%s", want, prompt)
	}

	if err := k.AddContextFile(&ContextFile{Path: "/runtime/AGENTS.md", Content: "runtime"}); err != nil {
		t.Fatalf("AddContextFile: %v", err)
	}
	if prompt, _ := k.agentSnapshotForTest(); !strings.Contains(prompt, want) {
		t.Errorf("prompt after AddContextFile lacks %q:\n%s", want, prompt)
	}

	if err := k.SetModel(context.Background(), "openai/gpt-4o-mini"); err != nil {
		t.Fatalf("SetModel: %v", err)
	}
	if prompt, _ := k.agentSnapshotForTest(); !strings.Contains(prompt, want) {
		t.Errorf("prompt after SetModel lacks %q:\n%s", want, prompt)
	}
}

// TestSetModelConcurrentWithPromptRefresh switches the model while another
// goroutine composes the prompt again. Run with -race: Agent.SetModel must
// swap the provider and the prompt under the lock that SetSystemPrompt
// takes for its rebuild.
func TestSetModelConcurrentWithPromptRefresh(t *testing.T) {
	k := newBareTestKit(t, false)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 20 {
			k.RefreshSystemPrompt()
		}
	}()
	for range 3 {
		if err := k.SetModel(context.Background(), "openai/gpt-4o-mini"); err != nil {
			t.Errorf("SetModel: %v", err)
			break
		}
	}
	<-done
}
