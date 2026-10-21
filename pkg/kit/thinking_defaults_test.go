package kit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/models"
	"github.com/spf13/viper"
)

// TestThinkingDefaultsEffectiveState checks resolved defaults without making them explicit overrides.
func TestThinkingDefaultsEffectiveState(t *testing.T) {
	for _, tc := range []struct{ name, settings, override, want string }{
		{"model settings", "modelSettings:\n  custom/default-effort:\n    thinkingLevel: medium\n", "", "medium"},
		{"custom params", "", "", "low"},
		{"explicit off", "", "off", "off"},
		{"named override", "", "Future", "Future"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("KIT_THINKING_LEVEL", "")
			config := `model: custom/default-effort
no-extensions: true
no-skills: true
no-mcp: true
customModels:
  default-effort:
    limit: {context: 32768, output: 4096}
    baseUrl: http://127.0.0.1:1/v1
    reasoning: true
    reasoningLevels: [low, medium, Future]
    params:
      thinkingLevel: low
  other-effort:
    limit: {context: 32768, output: 4096}
    baseUrl: http://127.0.0.1:1/v1
    reasoning: true
    reasoningLevels: [low, medium, Future]
    params:
      thinkingLevel: Future
  no-effort:
    limit: {context: 32768, output: 4096}
    baseUrl: http://127.0.0.1:1/v1
    reasoning: true
` + tc.settings
			path := filepath.Join(home, ".kit.yml")
			if err := os.WriteFile(path, []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			viper.Reset()
			viper.SetConfigFile(path)
			if err := viper.ReadInConfig(); err != nil {
				t.Fatal(err)
			}
			models.ReloadGlobalRegistry()
			t.Cleanup(func() { viper.Reset(); models.ReloadGlobalRegistry() })
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			k, err := New(ctx, &Options{ConfigFile: path, ThinkingLevel: tc.override, WorkDir: t.TempDir(), Quiet: true, NoSession: true, NoExtensions: true, NoSkills: true, NoContextFiles: true})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = k.Close() }()
			if got := k.GetThinkingLevel(); got != tc.want {
				t.Fatalf("effective level = %q, want %q", got, tc.want)
			}
			if tc.override == "" {
				if k.v.IsSet("thinking-level") {
					t.Fatal("model default became explicit")
				}
				if err := k.SetModel(ctx, "custom/default-effort"); err != nil {
					t.Fatal(err)
				}
				if got := k.GetThinkingLevel(); got != tc.want {
					t.Fatalf("recreated level = %q", got)
				}
				if err := k.SetModel(ctx, "custom/other-effort"); err != nil {
					t.Fatal(err)
				}
				if got := k.GetThinkingLevel(); got != "Future" {
					t.Fatalf("switched level = %q", got)
				}
				if k.v.IsSet("thinking-level") {
					t.Fatal("switch made default explicit")
				}
				if err := k.SetModel(ctx, "custom/no-effort"); err != nil {
					t.Fatal(err)
				}
				if got := k.GetThinkingLevel(); got != "off" {
					t.Fatalf("unconfigured level = %q", got)
				}
			}
		})
	}
}
