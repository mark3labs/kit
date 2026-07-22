package footer

import (
	"strings"
	"testing"
	"time"
)

func TestConfig_DefaultsAndValidation(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Errorf("Expected DefaultConfig to be enabled")
	}
	if len(cfg.Fields) != len(AllFields) {
		t.Errorf("Expected %d fields in DefaultConfig, got %d", len(AllFields), len(cfg.Fields))
	}

	if !IsFieldValid("mode") || !IsFieldValid("COST") {
		t.Errorf("Expected mode and COST to be valid fields")
	}
	if IsFieldValid("invalid_field") {
		t.Errorf("Expected invalid_field to be invalid")
	}
}

func TestConfig_FieldToggling(t *testing.T) {
	cfg := DefaultConfig()

	// Disable clock
	ok := cfg.DisableField("clock")
	if !ok {
		t.Errorf("Expected DisableField('clock') to succeed")
	}
	if cfg.IsFieldEnabled("clock") {
		t.Errorf("Expected clock to be disabled")
	}

	// Enable clock
	ok = cfg.EnableField("clock")
	if !ok {
		t.Errorf("Expected EnableField('clock') to succeed")
	}
	if !cfg.IsFieldEnabled("clock") {
		t.Errorf("Expected clock to be enabled")
	}

	// Toggle mode off
	enabled, valid := cfg.ToggleField("mode")
	if !valid || enabled {
		t.Errorf("Expected ToggleField('mode') to return enabled=false, valid=true; got enabled=%v, valid=%v", enabled, valid)
	}

	// Set exact fields
	cfg.SetFields([]string{"model", "cost", "invalid"})
	if len(cfg.Fields) != 2 {
		t.Errorf("Expected 2 valid fields, got %d (%v)", len(cfg.Fields), cfg.Fields)
	}
	if !cfg.IsFieldEnabled("model") || !cfg.IsFieldEnabled("cost") || cfg.IsFieldEnabled("mode") {
		t.Errorf("Unexpected field enablement: %v", cfg.Fields)
	}
}

func TestRender_DisabledFooterPreservesSpinner(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SetEnabled(false)

	data := RenderData{
		Width:         100,
		ModelName:     "claude-3-7-sonnet",
		SpinnerPrefix: "working ",
	}

	out := Render(cfg, data)
	if out != data.SpinnerPrefix {
		t.Errorf("Render() = %q; want spinner %q", out, data.SpinnerPrefix)
	}
}

func TestRender_EmptyFieldsStayEmpty(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SetFields(nil)

	if out := Render(cfg, RenderData{Width: 100}); out != "" {
		t.Errorf("Render() = %q; want empty output", out)
	}
}

func TestRender_SelectedFieldsOnly(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SetFields([]string{"model", "cost"})

	data := RenderData{
		Width:       200,
		ModelName:   "anthropic/claude-3-7-sonnet",
		SessionCost: 0.12345,
	}

	out := Render(cfg, data)
	if !strings.Contains(out, "sonnet 3.7") {
		t.Errorf("Expected model 'sonnet 3.7' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "💰 $0.12345") {
		t.Errorf("Expected cost '💰 $0.12345' in output, got:\n%s", out)
	}
	if strings.Contains(out, "[KIT]") || strings.Contains(out, "ctx") {
		t.Errorf("Unexpected fields rendered in output:\n%s", out)
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := FormatShortModelName("openai/gpt-4o"); got != "gpt 4o" {
		t.Errorf("FormatShortModelName(openai/gpt-4o) = %q; want 'gpt 4o'", got)
	}
	if got := FormatTokenCount(1234567); got != "1.2M" {
		t.Errorf("FormatTokenCount(1234567) = %q; want '1.2M'", got)
	}
	if got := FormatDuration(75 * time.Second); got != "1m15s" {
		t.Errorf("FormatDuration(75s) = %q; want '1m15s'", got)
	}
}
