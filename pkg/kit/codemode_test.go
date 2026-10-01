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

func TestCodeModeIsOptIn(t *testing.T) {
	if slices.Contains(handleCoreToolList(nil, false), CodeModeToolName) {
		t.Fatal("code mode must not be in the default core tool set")
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
