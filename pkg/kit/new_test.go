package kit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/models"

	"github.com/spf13/viper"
)

// testCatalogModel must exist in the embedded model catalog: per-model
// settings only apply to models that LookupModelForSettings resolves.
const testCatalogModel = "openai/gpt-4o"

func requireCatalogModel(t *testing.T, model string) {
	t.Helper()
	if models.LookupModelForSettings(model) == nil {
		t.Fatalf("test model %q is not in the model catalog; pick another catalog model", model)
	}
}

// TestPerModelSystemPromptEmptyFileReplacesDefault guards the behaviour kept
// by the New split (#122): a per-model systemPrompt that is configured but
// resolves to empty content (an empty file) still replaces the built-in
// default base prompt instead of being ignored.
func TestPerModelSystemPromptEmptyFileReplacesDefault(t *testing.T) {
	const model = testCatalogModel
	requireCatalogModel(t, model)

	emptyFile := filepath.Join(t.TempDir(), "empty-prompt.md")
	if err := os.WriteFile(emptyFile, []byte("  \n"), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	v := viper.New()
	v.Set("model", model)
	v.Set("system-prompt", defaultSystemPrompt)
	v.Set("modelSettings", map[string]any{
		model: map[string]any{"systemPrompt": emptyFile},
	})

	p, ok := perModelSystemPrompt(v)
	if !ok {
		t.Fatalf("perModelSystemPrompt ok = false, want true for a configured prompt")
	}
	if p != "" {
		t.Fatalf("perModelSystemPrompt = %q, want empty content", p)
	}

	rc := &resolvedConfig{}
	buildSystemPrompt(v, &Options{}, rc)

	if rc.basePrompt != "" {
		t.Errorf("basePrompt = %q, want empty (per-model prompt must replace the default)", rc.basePrompt)
	}
	if strings.Contains(v.GetString("system-prompt"), defaultSystemPrompt) {
		t.Errorf("composed system prompt still contains the built-in default")
	}
}

// TestPerModelSystemPromptNotConfigured checks that the default base prompt
// stays in place when the model has no per-model systemPrompt.
func TestPerModelSystemPromptNotConfigured(t *testing.T) {
	requireCatalogModel(t, testCatalogModel)

	v := viper.New()
	v.Set("model", testCatalogModel)
	v.Set("system-prompt", defaultSystemPrompt)

	if _, ok := perModelSystemPrompt(v); ok {
		t.Fatalf("perModelSystemPrompt ok = true, want false when nothing is configured")
	}

	rc := &resolvedConfig{}
	buildSystemPrompt(v, &Options{}, rc)
	if rc.basePrompt != defaultSystemPrompt {
		t.Errorf("basePrompt = %q, want the built-in default", rc.basePrompt)
	}
}
