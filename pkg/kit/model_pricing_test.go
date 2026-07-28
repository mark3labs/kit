package kit

import (
	"testing"

	"github.com/mark3labs/kit/internal/models"
)

// TestModelPricingFrom_MapsRegistryCosts verifies the registry-to-extension
// pricing conversion, in particular that optional cache rates are reported
// via the Has* flags instead of collapsing "no published rate" into 0.
func TestModelPricingFrom_MapsRegistryCosts(t *testing.T) {
	cacheRead := 0.30
	cacheWrite := 3.75

	tests := []struct {
		name string
		info *models.ModelInfo
		want ModelPricing
	}{
		{
			name: "nil model info is unknown pricing",
			info: nil,
			want: ModelPricing{},
		},
		{
			name: "full pricing with cache rates",
			info: &models.ModelInfo{
				Cost: models.Cost{
					Input:      3.0,
					Output:     15.0,
					CacheRead:  &cacheRead,
					CacheWrite: &cacheWrite,
				},
			},
			want: ModelPricing{
				Input:         3.0,
				Output:        15.0,
				CacheRead:     0.30,
				CacheWrite:    3.75,
				HasCacheRead:  true,
				HasCacheWrite: true,
				Known:         true,
			},
		},
		{
			name: "no cache rates published",
			info: &models.ModelInfo{
				Cost: models.Cost{Input: 1.0, Output: 2.0},
			},
			want: ModelPricing{
				Input:  1.0,
				Output: 2.0,
				Known:  true,
			},
		},
		{
			name: "genuinely free model is known, not unknown",
			info: &models.ModelInfo{
				Cost: models.Cost{Input: 0, Output: 0},
			},
			want: ModelPricing{Known: true},
		},
		{
			name: "cache read only",
			info: &models.ModelInfo{
				Cost: models.Cost{Input: 3.0, Output: 15.0, CacheRead: &cacheRead},
			},
			want: ModelPricing{
				Input:        3.0,
				Output:       15.0,
				CacheRead:    0.30,
				HasCacheRead: true,
				Known:        true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modelPricingFrom(tt.info); got != tt.want {
				t.Errorf("modelPricingFrom() = %+v; want %+v", got, tt.want)
			}
		})
	}
}

// TestModelPricingFrom_CopiesCacheRates guards against the converter aliasing
// the registry's *float64 fields: mutating the source after conversion must
// not change an already-returned ModelPricing.
func TestModelPricingFrom_CopiesCacheRates(t *testing.T) {
	cacheRead := 0.30
	info := &models.ModelInfo{
		Cost: models.Cost{Input: 3.0, CacheRead: &cacheRead},
	}

	got := modelPricingFrom(info)
	cacheRead = 99.0

	if got.CacheRead != 0.30 {
		t.Errorf("CacheRead = %v after mutating source; want 0.30 (value must be copied)", got.CacheRead)
	}
}

// TestGetModelCapabilities_IncludesPricing verifies pricing reaches extensions
// through the public capability lookup, and that unresolvable models report an
// error with zero (unknown) pricing rather than a plausible-looking zero cost.
func TestGetModelCapabilities_IncludesPricing(t *testing.T) {
	caps, errStr := GetModelCapabilities("")
	if errStr == "" {
		t.Error("GetModelCapabilities(\"\") error = \"\"; want a message")
	}
	if caps.Pricing.Known {
		t.Error("Pricing.Known = true for an empty model; want false")
	}

	caps, errStr = GetModelCapabilities("nonexistent-provider/nonexistent-model")
	if errStr == "" {
		t.Error("GetModelCapabilities(unknown) error = \"\"; want a message")
	}
	if caps.Pricing.Known {
		t.Error("Pricing.Known = true for an unknown model; want false")
	}
}

// TestGetCurrentModel_NilSafe verifies the accessor tolerates an uninitialized
// Kit. The extension bridge calls it on every GetModelCapabilities("") and
// must not panic in headless or partially constructed setups.
func TestGetCurrentModel_NilSafe(t *testing.T) {
	var k *Kit
	if got := k.GetCurrentModel(); got != "" {
		t.Errorf("(*Kit)(nil).GetCurrentModel() = %q; want \"\"", got)
	}

	if got := (&Kit{}).GetCurrentModel(); got != "" {
		t.Errorf("(&Kit{}).GetCurrentModel() = %q; want \"\"", got)
	}
}

// TestGetAvailableModels_CarryPricing verifies pricing survives the registry
// walk in GetAvailableModels. The loop takes the address of the range variable,
// so this also guards against every entry aliasing the final iteration.
func TestGetAvailableModels_CarryPricing(t *testing.T) {
	k := &Kit{}
	entries := k.GetAvailableModels()
	if len(entries) == 0 {
		t.Skip("no models in registry")
	}

	var priced, distinct int
	seen := map[float64]bool{}
	for _, e := range entries {
		if e.Pricing.Known {
			priced++
			if e.Pricing.Input > 0 && !seen[e.Pricing.Input] {
				seen[e.Pricing.Input] = true
				distinct++
			}
		}
	}

	if priced == 0 {
		t.Error("no entry reported Known pricing; want registry costs to flow through")
	}
	if distinct < 2 {
		t.Errorf("only %d distinct input rates across %d entries; entries likely alias one model",
			distinct, len(entries))
	}
}
