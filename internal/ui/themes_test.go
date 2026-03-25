package ui

import (
	"testing"
)

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		hex     string
		r, g, b int
	}{
		{"#000000", 0, 0, 0},
		{"#ffffff", 255, 255, 255},
		{"#1e1e2e", 0x1e, 0x1e, 0x2e},
		{"#a6e3a1", 0xa6, 0xe3, 0xa1},
		{"#f38ba8", 0xf3, 0x8b, 0xa8},
	}
	for _, tt := range tests {
		r, g, b := parseHexColor(tt.hex)
		if r != tt.r || g != tt.g || b != tt.b {
			t.Errorf("parseHexColor(%q) = (%d,%d,%d), want (%d,%d,%d)",
				tt.hex, r, g, b, tt.r, tt.g, tt.b)
		}
	}
}

func TestBlendHex(t *testing.T) {
	// Blending with 0 amount should return the base color.
	got := blendHex("#1e1e2e", "#a6e3a1", 0.0)
	if got != "#1e1e2e" {
		t.Errorf("blendHex with 0.0 = %q, want #1e1e2e", got)
	}

	// Blending with 1.0 amount should return the tint color.
	got = blendHex("#1e1e2e", "#a6e3a1", 1.0)
	if got != "#a6e3a1" {
		t.Errorf("blendHex with 1.0 = %q, want #a6e3a1", got)
	}

	// Blending black and white at 0.5 should give mid gray.
	got = blendHex("#000000", "#ffffff", 0.5)
	// 127 = int(0 + 255*0.5) — truncated, so #7f7f7f
	if got != "#7f7f7f" {
		t.Errorf("blendHex black/white at 0.5 = %q, want #7f7f7f", got)
	}
}

func TestDeriveDiffBgProducesDifferentColorsPerTheme(t *testing.T) {
	// Catppuccin palette
	catBg := [2]string{"#eff1f5", "#1e1e2e"}
	catSuccess := [2]string{"#40a02b", "#a6e3a1"}
	catError := [2]string{"#d20f39", "#f38ba8"}

	// KITT palette
	kittBg := [2]string{"#F0F0F0", "#0D0D0D"}
	kittSuccess := [2]string{"#998800", "#CCAA00"}
	kittError := [2]string{"#CC0000", "#FF3333"}

	catInsert, catDelete, _, _, _, _, _ := deriveDiffBg(catBg, catSuccess, catError)
	kittInsert, kittDelete, _, _, _, _, _ := deriveDiffBg(kittBg, kittSuccess, kittError)

	if catInsert == kittInsert {
		t.Error("catppuccin DiffInsertBg should differ from kitt DiffInsertBg")
	}
	if catDelete == kittDelete {
		t.Error("catppuccin DiffDeleteBg should differ from kitt DiffDeleteBg")
	}
}

func TestMakeThemeDerivesUniqueDiffColors(t *testing.T) {
	themes := builtinThemes()
	kitt := themes["kitt"]
	cat := themes["catppuccin"]

	// The catppuccin diff backgrounds should NOT equal the kitt defaults.
	if cat.DiffInsertBg == kitt.DiffInsertBg {
		t.Error("catppuccin DiffInsertBg should differ from kitt default")
	}
	if cat.DiffDeleteBg == kitt.DiffDeleteBg {
		t.Error("catppuccin DiffDeleteBg should differ from kitt default")
	}
	if cat.DiffEqualBg == kitt.DiffEqualBg {
		t.Error("catppuccin DiffEqualBg should differ from kitt default")
	}
}
