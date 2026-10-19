package kit

import (
	"slices"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/mark3labs/kit/internal/codemode"
)

func TestResolveCodeModeFromConfig(t *testing.T) {
	v := viper.New()
	v.Set("codemode.enabled", true)
	v.Set("codemode.timeout", 12)
	v.Set("codemode.max-tool-calls", 7)
	v.Set("codemode.mcp-exposure", "deferred")
	v.Set("codemode.exposure", map[string]any{"github__*": "codemode"})

	enabled, cfg, err := resolveCodeMode(&Options{}, v)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("expected enabled")
	}
	if cfg.Limits.Timeout != 12*time.Second || cfg.Limits.MaxToolCalls != 7 {
		t.Fatalf("limits = %+v", cfg.Limits)
	}
	if got := cfg.Policy.Resolve("github__list"); got != codemode.ExposureCodeMode {
		t.Fatalf("github exposure = %s", got)
	}
	if got := cfg.Policy.Resolve("linear__x"); got != codemode.ExposureDeferred {
		t.Fatalf("mcp default = %s", got)
	}
}

func TestResolveCodeModeOptionsOverride(t *testing.T) {
	v := viper.New()
	v.Set("codemode.max-tool-calls", 7)
	enabled, cfg, err := resolveCodeMode(&Options{CodeMode: &CodeModeOptions{
		Enabled:       true,
		MaxToolCalls:  3,
		MemoryLimitMB: -1,
		Exposure:      map[string]string{"subagent": CodeModeExposureModelOnly},
	}}, v)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled || cfg.Limits.MaxToolCalls != 3 || cfg.Limits.MemoryLimit != -1 {
		t.Fatalf("enabled=%v limits=%+v", enabled, cfg.Limits)
	}
	if cfg.Policy.Resolve("subagent") != codemode.ExposureModelOnly {
		t.Fatal("option exposure rule not applied")
	}

	if _, _, err := resolveCodeMode(&Options{CodeMode: &CodeModeOptions{MCPExposure: "bogus"}}, viper.New()); err == nil {
		t.Fatal("expected error for invalid exposure")
	}
}

func TestCodeModeDefaultToolList(t *testing.T) {
	if !slices.Contains(handleCoreToolList(nil, false), CodeModeToolName) {
		t.Fatal("code mode must be in the default core tool set")
	}
	if !slices.Contains(handleCoreToolList([]string{"read", CodeModeToolName}, false), CodeModeToolName) {
		t.Fatal("naming code mode must enable it")
	}
	got := withCodeModeTool(handleCoreToolList(nil, true), true)
	if !slices.Equal(got, []string{CodeModeToolName}) {
		t.Fatalf("code mode with core tools disabled = %v", got)
	}
	if got := withCodeModeTool([]string{CodeModeToolName}, true); len(got) != 1 {
		t.Fatalf("duplicate code mode entry: %v", got)
	}
}

func TestNewCodeModeTool(t *testing.T) {
	tool, err := NewCodeModeTool(CodeModeOptions{MaxToolCalls: 2})
	if err != nil {
		t.Fatal(err)
	}
	if tool.Info().Name != CodeModeToolName {
		t.Fatalf("name = %s", tool.Info().Name)
	}
	if _, err := NewCodeModeTool(CodeModeOptions{Exposure: map[string]string{"x": "nope"}}); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveCodeModeDefaultsAndDisabling(t *testing.T) {
	for _, tc := range []struct {
		name    string
		config  *bool
		options *CodeModeOptions
		want    bool
	}{
		{name: "default", want: true},
		{name: "zero options", options: &CodeModeOptions{}, want: true},
		{name: "config disabled", config: new(bool)},
		{name: "SDK disabled", options: &CodeModeOptions{Disabled: true}},
		{name: "SDK enable overrides config", config: new(bool), options: &CodeModeOptions{Enabled: true}, want: true},
		{name: "disabled wins", options: &CodeModeOptions{Enabled: true, Disabled: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := viper.New()
			if tc.config != nil {
				v.Set("codemode.enabled", *tc.config)
			}
			enabled, cfg, err := resolveCodeMode(&Options{CodeMode: tc.options}, v)
			if err != nil {
				t.Fatal(err)
			}
			if enabled != tc.want {
				t.Fatalf("enabled = %v, want %v", enabled, tc.want)
			}
			for _, name := range []string{"read", "server__tool", "custom"} {
				if got := cfg.Policy.Resolve(name); got != codemode.ExposureDirect {
					t.Fatalf("%s exposure = %s", name, got)
				}
			}
			original := []string{"read", CodeModeToolName}
			got := withCodeModeTool(original, enabled)
			if slices.Contains(got, CodeModeToolName) != tc.want {
				t.Fatalf("tools = %v", got)
			}
			if original[1] != CodeModeToolName {
				t.Fatal("modified caller tool list")
			}
		})
	}
}

func TestCodeModeResolvedToolSelection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		opts   Options
		values map[string]any
		want   bool
	}{
		{name: "default", want: true},
		{name: "disable core", opts: Options{DisableCoreTools: true}},
		{name: "explicit enable with no core", opts: Options{DisableCoreTools: true, CodeMode: &CodeModeOptions{Enabled: true}}, want: true},
		{name: "SDK selection", opts: Options{CoreToolList: []string{"read"}}},
		{name: "exclude", values: map[string]any{"exclude-core-tools": []string{CodeModeToolName}}},
		{name: "include", values: map[string]any{"include-core-tools": []string{"read"}}},
		{name: "config disable", values: map[string]any{"codemode.enabled": false}},
		{name: "SDK disable named tool", opts: Options{CoreToolList: []string{CodeModeToolName}, CodeMode: &CodeModeOptions{Disabled: true}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := viper.New()
			setSDKDefaults(v)
			for key, value := range tc.values {
				v.Set(key, value)
			}
			rc := &resolvedConfig{}
			if err := resolveModelConfig(v, &tc.opts, nil, rc); err != nil {
				t.Fatal(err)
			}
			if got := slices.Contains(rc.toolList, CodeModeToolName); got != tc.want {
				t.Fatalf("tools = %v, want code mode %v", rc.toolList, tc.want)
			}
		})
	}
}
