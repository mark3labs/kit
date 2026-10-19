package kit

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/models"
	"github.com/spf13/viper"
)

func TestDynamicEffortSDKRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	viper.Set("customModels", map[string]any{"effort-fixture": map[string]any{"reasoning": true, "baseUrl": "http://127.0.0.1:1/v1", "reasoningLevels": []string{"low", "xhigh", "Future"}}})
	models.ReloadGlobalRegistry()
	t.Cleanup(func() { viper.Reset(); models.ReloadGlobalRegistry() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := &Options{Model: "custom/effort-fixture", Quiet: true, NoSession: true, NoExtensions: true, NoSkills: true, NoContextFiles: true, SkipConfig: true}
	WithThinkingLevel("Future")(opts)
	k, err := New(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = k.Close() }()
	if k.GetThinkingLevel() != "Future" {
		t.Fatal(k.GetThinkingLevel())
	}
	if !IsThinkingLevelSupported("custom", "effort-fixture", "Future") || IsThinkingLevelSupported("custom", "effort-fixture", "future") {
		t.Fatal("predicate lost exact spelling")
	}
	if SuggestThinkingLevel("custom", "effort-fixture", "typo") != "off" {
		t.Fatal("unranked typo got ranked")
	}
	if err := k.SetThinkingLevel(ctx, "xhigh"); err != nil {
		t.Fatal(err)
	}
	if err := k.SetThinkingLevel(ctx, "typo"); err == nil {
		t.Fatal("invalid setter accepted")
	}
	if k.GetThinkingLevel() != "xhigh" {
		t.Fatal("invalid setter changed selection")
	}
	if err := k.SetThinkingLevel(ctx, "Future"); err != nil {
		t.Fatal(err)
	}
	if err := k.SetModel(ctx, "custom/effort-fixture"); err != nil {
		t.Fatal(err)
	}
	if k.GetThinkingLevel() != "Future" {
		t.Fatal(k.GetThinkingLevel())
	}
	invalid := *opts
	invalid.ThinkingLevel = "typo"
	if rejected, err := New(ctx, &invalid); err == nil {
		_ = rejected.Close()
		t.Fatal("startup accepted an unsupported graded effort")
	}
}
