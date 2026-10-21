package acpserver

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/mark3labs/kit/internal/models"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// TestNewSessionThinkingDefaults checks startup precedence, runtime changes, and session isolation.
func TestNewSessionThinkingDefaults(t *testing.T) {
	for _, tc := range []struct {
		name, flag, env, top, want string
		noDefault                  bool
	}{
		{name: "unchanged bound flag", want: "medium"},
		{name: "explicit off", flag: "off", want: "off"},
		{name: "explicit low", flag: "low", env: "xhigh", top: "off", want: "low"},
		{name: "environment", env: "xhigh", top: "low", want: "xhigh"},
		{name: "top-level file", top: "low", want: "low"},
		{name: "no default", noDefault: true, want: "off"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("KIT_THINKING_LEVEL", tc.env)
			config := `model: custom/acp-default-effort
no-extensions: true
no-skills: true
no-mcp: true
no-agents: true
customModels:
  acp-default-effort:
    limit: {context: 262144, output: 32768}
    baseUrl: http://127.0.0.1:1/v1
    reasoning: true
    reasoningLevels: [low, medium, xhigh]
`
			if !tc.noDefault {
				config += "modelSettings:\n  custom/acp-default-effort:\n    thinkingLevel: medium\n"
			}
			if tc.top != "" {
				config += "thinking-level: " + tc.top + "\n"
			}
			path := filepath.Join(home, ".kit.yml")
			if err := os.WriteFile(path, []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			viper.Reset()
			t.Cleanup(func() { viper.Reset(); models.ReloadGlobalRegistry() })
			flags := pflag.NewFlagSet("thinking fixture", pflag.ContinueOnError)
			flags.String("thinking-level", "off", "")
			if tc.flag != "" {
				if err := flags.Set("thinking-level", tc.flag); err != nil {
					t.Fatal(err)
				}
			}
			if err := viper.BindPFlag("thinking-level", flags.Lookup("thinking-level")); err != nil {
				t.Fatal(err)
			}
			viper.SetEnvPrefix("KIT")
			viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
			viper.AutomaticEnv()
			viper.SetConfigFile(path)
			if err := viper.ReadInConfig(); err != nil {
				t.Fatal(err)
			}
			models.ReloadGlobalRegistry()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			a := NewAgent()
			defer a.registry.closeAll()
			cwd := t.TempDir()
			resp, err := a.NewSession(ctx, acp.NewSessionRequest{Cwd: cwd})
			if err != nil {
				t.Fatal(err)
			}
			assertOption := func(options []acp.SessionConfigOption, want string) {
				t.Helper()
				for _, option := range options {
					if option.Select == nil || string(option.Select.Id) != configIDThinkingLevel {
						continue
					}
					opt := option.Select
					if string(opt.CurrentValue) != want {
						t.Fatalf("current value = %q, want %q", opt.CurrentValue, want)
					}
					if opt.Category == nil || string(*opt.Category) != "thought_level" {
						t.Fatalf("category = %v", opt.Category)
					}
					var choices []string
					for _, choice := range *opt.Options.Ungrouped {
						choices = append(choices, string(choice.Value))
					}
					if !slices.Equal(choices, []string{"off", "low", "medium", "xhigh"}) {
						t.Fatalf("choices = %v", choices)
					}
					return
				}
				t.Fatal("thinking option missing")
			}
			assertOption(resp.ConfigOptions, tc.want)
			sess, ok := a.registry.get(string(resp.SessionId))
			if !ok {
				t.Fatal("session missing")
			}
			if got := sess.kit.GetThinkingLevel(); got != tc.want {
				t.Fatalf("effective level = %q", got)
			}
			set := func(value string) (acp.SetSessionConfigOptionResponse, error) {
				return a.SetSessionConfigOption(ctx, acp.SetSessionConfigOptionRequest{ValueId: &acp.SetSessionConfigOptionValueId{SessionId: resp.SessionId, ConfigId: configIDThinkingLevel, Value: acp.SessionConfigValueId(value)}})
			}
			changed, err := set("xhigh")
			if err != nil {
				t.Fatal(err)
			}
			assertOption(changed.ConfigOptions, "xhigh")
			if _, err := set("invalid"); err == nil {
				t.Fatal("unsupported level accepted")
			}
			if got := sess.kit.GetThinkingLevel(); got != "xhigh" {
				t.Fatalf("runtime level = %q", got)
			}
			second, err := a.NewSession(ctx, acp.NewSessionRequest{Cwd: cwd})
			if err != nil {
				t.Fatal(err)
			}
			assertOption(second.ConfigOptions, tc.want)
		})
	}
}
