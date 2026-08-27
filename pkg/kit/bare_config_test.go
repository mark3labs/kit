package kit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// TestBareConfig_SkipsProjectConfig is the security-relevant half of bare
// mode. A project .kit.yml can define mcpServers, which spawn processes, and
// can override system-prompt. Bare mode must not read it.
func TestBareConfig_SkipsProjectConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	project := t.TempDir()
	cfg := "model: project/should-not-load\n"
	if err := os.WriteFile(filepath.Join(project, ".kit.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)

	// Control: without bare, the project config is picked up.
	v := viper.New()
	if err := initConfig(v, "", false, false); err != nil {
		t.Fatalf("initConfig: %v", err)
	}
	if got := v.GetString("model"); got != "project/should-not-load" {
		t.Fatalf("non-bare: want project config to load, got model %q", got)
	}

	// Bare must not see it.
	vBare := viper.New()
	if err := initConfig(vBare, "", false, true); err != nil {
		t.Fatalf("initConfig bare: %v", err)
	}
	if got := vBare.GetString("model"); got != "" {
		t.Errorf("bare: project config leaked, model = %q", got)
	}
}

// TestBareConfig_KeepsExplicitConfigFile confirms --config still wins under
// bare mode. Bare drops discovery, not an explicitly named file.
func TestBareConfig_KeepsExplicitConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yml")
	if err := os.WriteFile(path, []byte("model: explicit/model\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	v := viper.New()
	if err := initConfig(v, path, false, true); err != nil {
		t.Fatalf("initConfig: %v", err)
	}
	if got := v.GetString("model"); got != "explicit/model" {
		t.Errorf("bare with --config: want explicit/model, got %q", got)
	}
}

// TestBareConfig_KeepsHomeConfig confirms the user's own config still loads.
// Without it bare mode would lose API keys and the default model, making the
// flag unusable rather than merely quiet.
func TestBareConfig_KeepsHomeConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".kit.yml"), []byte("model: home/model\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	v := viper.New()
	if err := initConfig(v, "", false, true); err != nil {
		t.Fatalf("initConfig: %v", err)
	}
	if got := v.GetString("model"); got != "home/model" {
		t.Errorf("bare: want home config to load, got model %q", got)
	}
}
