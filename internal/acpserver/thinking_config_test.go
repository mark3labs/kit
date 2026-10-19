package acpserver

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/pkg/kit"
	"github.com/spf13/viper"
)

func TestThinkingConfigPreservesDynamicLevels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	viper.Set("customModels", map[string]any{"acp-effort-fixture": map[string]any{"reasoning": true, "baseUrl": "http://127.0.0.1:1/v1", "reasoningLevels": []string{"low", "medium", "xhigh", "Future"}}})
	models.ReloadGlobalRegistry()
	t.Cleanup(func() { viper.Reset(); models.ReloadGlobalRegistry() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	k, err := kit.New(ctx, &kit.Options{Model: "custom/acp-effort-fixture", ThinkingLevel: "Future", Quiet: true, NoSession: true, NoExtensions: true, NoSkills: true, NoContextFiles: true, SkipConfig: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = k.Close() }()
	option, ok := thinkingConfigOption(k)
	if !ok {
		t.Fatal("thinking option absent")
	}
	if string(option.Select.CurrentValue) != "Future" {
		t.Fatal(option.Select.CurrentValue)
	}
	var got []string
	for _, choice := range *option.Select.Options.Ungrouped {
		got = append(got, string(choice.Value))
	}
	if !slices.Equal(got, []string{"off", "low", "medium", "xhigh", "Future"}) {
		t.Fatal(got)
	}
}
