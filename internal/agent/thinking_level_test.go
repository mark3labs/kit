package agent

import (
	"testing"

	"github.com/mark3labs/kit/internal/models"
)

// TestGetThinkingLevel checks the effective provider level and defensive off fallback.
func TestGetThinkingLevel(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config *models.ProviderConfig
		want   string
	}{
		{"missing config", nil, "off"},
		{"empty config", &models.ProviderConfig{}, "off"},
		{"resolved default", &models.ProviderConfig{ThinkingLevel: models.ThinkingMedium}, "medium"},
		{"named effort", &models.ProviderConfig{ThinkingLevel: models.ThinkingLevel("Future")}, "Future"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &Agent{modelConfig: tc.config}
			if got := a.GetThinkingLevel(); got != tc.want {
				t.Fatalf("level = %q, want %q", got, tc.want)
			}
		})
	}
}
