package extensions

import (
	"math"
	"testing"
)

func TestProviderRegistrationValidation(t *testing.T) {
	base := func() ProviderDef {
		return ProviderDef{ID: "fake", Name: "Fake", Create: func(ProviderCreateRequest) ProviderCreateResult { return ProviderCreateResult{} }, Models: []ProviderModel{{ID: "echo", Name: "Echo"}}}
	}
	tests := []struct {
		name   string
		mutate func(*ProviderDef)
	}{
		{"id slash", func(d *ProviderDef) { d.ID = "fake/other" }},
		{"id leading digit", func(d *ProviderDef) { d.ID = "1fake" }},
		{"nil create", func(d *ProviderDef) { d.Create = nil }},
		{"empty name", func(d *ProviderDef) { d.Name = " " }},
		{"control name", func(d *ProviderDef) { d.Name = "bad\nname" }},
		{"empty model", func(d *ProviderDef) { d.Models[0].ID = " " }},
		{"duplicate model", func(d *ProviderDef) { d.Models = append(d.Models, d.Models[0]) }},
		{"invalid limits", func(d *ProviderDef) { d.Models[0].ContextLimit = 2; d.Models[0].OutputLimit = 3 }},
		{"negative price", func(d *ProviderDef) { d.Models[0].InputPrice.Value = -1 }},
		{"nan price", func(d *ProviderDef) { d.Models[0].InputPrice.Value = math.NaN() }},
		{"nonobject settings", func(d *ProviderDef) { d.SettingsJSON = `[]` }},
		{"malformed settings", func(d *ProviderDef) { d.SettingsJSON = `{` }},
		{"invalid modality", func(d *ProviderDef) { d.Models[0].Capabilities.InputModalities = []string{"hologram"} }},
		{"reasoning levels without reasoning", func(d *ProviderDef) { d.Models[0].Capabilities.ReasoningLevels = []string{"high"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			def := base()
			test.mutate(&def)
			if _, err := freezeProvider(def); err == nil {
				t.Fatal("invalid registration accepted")
			}
		})
	}
	valid := base()
	valid.ID = "FAKE"
	valid.Models[0].ID = "path/model"
	valid.Models[0].InputPrice = ProviderPrice{Known: true, Value: 0}
	if _, err := freezeProvider(valid); err != nil {
		t.Fatal(err)
	}
}
