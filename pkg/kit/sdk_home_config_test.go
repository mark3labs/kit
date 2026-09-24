package kit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// isolateHome points every home/XDG lookup at fresh temp directories and
// moves into an empty working directory, returning the fake home.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir on Windows
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Chdir(t.TempDir())
	return home
}

// newHomeTestKit builds a Kit the way an embedding application would:
// through New, with config discovery enabled (no SkipConfig).
func newHomeTestKit(t *testing.T) *Kit {
	t.Helper()
	t.Setenv("OPENAI_API_KEY", "sk-test")
	k, err := New(context.Background(), &Options{
		Model:            "openai/gpt-4o-mini",
		Quiet:            true,
		NoSession:        true,
		NoExtensions:     true,
		DisableCoreTools: true,
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	t.Cleanup(func() { _ = k.Close() })
	return k
}

// TestNew_DoesNotCreateHomeConfig guards SDK embedders: constructing a Kit
// must never write a ~/.kit.yml into the user's home directory. Only the CLI
// seeds that template.
func TestNew_DoesNotCreateHomeConfig(t *testing.T) {
	home := isolateHome(t)
	newHomeTestKit(t)

	for _, name := range []string{".kit.yml", ".kit.yaml", ".kit.json"} {
		if _, err := os.Stat(filepath.Join(home, name)); err == nil {
			t.Errorf("kit.New created %s in the home directory", name)
		}
	}
}

// TestNew_StillLoadsExistingHomeConfig confirms the SDK keeps reading a
// config the user already has; only creating one was removed.
func TestNew_StillLoadsExistingHomeConfig(t *testing.T) {
	home := isolateHome(t)
	cfg := "temperature: 0.123\n"
	if err := os.WriteFile(filepath.Join(home, ".kit.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	k := newHomeTestKit(t)
	if got := k.v.GetFloat64("temperature"); got != 0.123 {
		t.Errorf("home config not loaded: temperature = %v, want 0.123", got)
	}
}

// TestInitConfig_CLICreatesHomeConfig confirms the CLI entry point
// (InitConfigWithOptions, used by cmd/root.go) still gives first-time users
// a template.
func TestInitConfig_CLICreatesHomeConfig(t *testing.T) {
	home := isolateHome(t)
	// InitConfigWithOptions writes to the process-global store.
	t.Cleanup(viper.Reset)

	if err := InitConfigWithOptions(ConfigInitOptions{}); err != nil {
		t.Fatalf("InitConfigWithOptions: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".kit.yml")); err != nil {
		t.Errorf("CLI path did not create ~/.kit.yml: %v", err)
	}
}
