package ui

import (
	"slices"
	"testing"

	"github.com/mark3labs/kit/internal/models"
	"github.com/spf13/viper"
)

// TestThinkingPickerAndCycleUseModelLevels checks picker/cycle agreement and wraparound.
func TestThinkingPickerAndCycleUseModelLevels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	viper.Set("customModels", map[string]any{"effort-fixture": map[string]any{"reasoning": true, "reasoningLevels": []string{"low", "medium", "xhigh", "Future"}}})
	models.ReloadGlobalRegistry()
	t.Cleanup(func() { viper.Reset(); models.ReloadGlobalRegistry() })
	picker := NewThinkingLevelSelector("custom", "effort-fixture", "Future", 100, 30)
	var got []string
	for _, item := range picker.popup.Items() {
		got = append(got, item.Label)
		if item.Label == "Future" && !item.Active {
			t.Fatal("future selection not restored")
		}
	}
	if !slices.Equal(got, []string{"off", "low", "medium", "xhigh", "Future"}) {
		t.Fatal(got)
	}
	m := &AppModel{conversationPane: &conversationPane{}, providerName: "custom", modelName: "effort-fixture", thinkingLevel: "xhigh"}
	m.cycleThinkingLevel()
	if m.thinkingLevel != "Future" {
		t.Fatal(m.thinkingLevel)
	}
	m.cycleThinkingLevel()
	if m.thinkingLevel != "off" {
		t.Fatal(m.thinkingLevel)
	}
}
