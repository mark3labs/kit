package models

import "testing"

// TestModelConfigToModelInfo_CostPresence guards the distinction between a
// custom model whose pricing was omitted (unknown) and one explicitly
// configured as free. A value-typed CostConfig collapsed both to zero, which
// surfaced to extensions as a confident — and wrong — "$0.00".
func TestModelConfigToModelInfo_CostPresence(t *testing.T) {
	tests := []struct {
		name          string
		cfg           CustomModelConfig
		wantPublished bool
		wantInput     float64
	}{
		{
			name:          "omitted cost block is unknown pricing",
			cfg:           CustomModelConfig{Name: "m"},
			wantPublished: false,
		},
		{
			name:          "explicit zero rate is a declared free model",
			cfg:           CustomModelConfig{Name: "m", Cost: &CostConfig{Input: 0, Output: 0}},
			wantPublished: true,
		},
		{
			name:          "real rates are published",
			cfg:           CustomModelConfig{Name: "m", Cost: &CostConfig{Input: 1.5, Output: 3}},
			wantPublished: true,
			wantInput:     1.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modelConfigToModelInfo("m", tt.cfg)
			if got.Cost.Published != tt.wantPublished {
				t.Errorf("Cost.Published = %v; want %v", got.Cost.Published, tt.wantPublished)
			}
			if got.Cost.Input != tt.wantInput {
				t.Errorf("Cost.Input = %v; want %v", got.Cost.Input, tt.wantInput)
			}
		})
	}
}

// TestCustomPlaceholderIsUnpriced covers the built-in custom/custom entry. It
// stands in for whatever endpoint the user configures, so its real rate is
// unknown; reporting it as free would let extensions render "$0.00" for a
// billed provider.
func TestCustomPlaceholderIsUnpriced(t *testing.T) {
	info := GetGlobalRegistry().LookupModel("custom", "custom")
	if info == nil {
		t.Skip("custom/custom placeholder not present in registry")
	}
	if info.Cost.Published {
		t.Error("custom/custom Cost.Published = true; want false (price is unknown, not zero)")
	}
}
