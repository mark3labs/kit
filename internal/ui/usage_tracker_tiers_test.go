package ui

import (
	"math"
	"testing"

	"github.com/mark3labs/kit/internal/models"
)

// tieredModel mirrors a real long-context model: 3/15 per million at the base
// rate, doubling to 6/22.5 once a request's prompt passes 200k tokens.
//
//go:fix inline
func tieredModel(t *testing.T) *models.ModelInfo {
	t.Helper()
	info := &models.ModelInfo{
		ID:   "tiered",
		Name: "Tiered",
		Cost: models.Cost{
			Input: 3, Output: 15,
			CacheRead: new(0.3), CacheWrite: new(3.75),
			Published: true,
			Tiers: []models.CostTier{{
				Threshold: 200_000,
				Input:     6, Output: 22.5,
				CacheRead: new(0.6), CacheWrite: new(7.5),
			}},
		},
		Limit: models.Limit{Context: 1_000_000, Output: 64_000},
	}
	return info
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestUsageTrackerAppliesBaseRateBelowThreshold(t *testing.T) {
	ut := NewUsageTracker(tieredModel(t), "test", 80, false)
	ut.UpdateUsage(100_000, 1_000, 0, 0)

	stats := ut.GetSessionStats()
	want := 100_000*3.0/1e6 + 1_000*15.0/1e6
	if !approx(stats.TotalCost, want) {
		t.Errorf("TotalCost = %v, want %v (base rate)", stats.TotalCost, want)
	}
}

func TestUsageTrackerAppliesTierRateAboveThreshold(t *testing.T) {
	ut := NewUsageTracker(tieredModel(t), "test", 80, false)
	ut.UpdateUsage(250_000, 1_000, 0, 0)

	stats := ut.GetSessionStats()
	want := 250_000*6.0/1e6 + 1_000*22.5/1e6
	if !approx(stats.TotalCost, want) {
		t.Errorf("TotalCost = %v, want %v (long-context rate)", stats.TotalCost, want)
	}

	// The bug this replaces: billing a 250k-token prompt at the base rate.
	base := 250_000*3.0/1e6 + 1_000*15.0/1e6
	if approx(stats.TotalCost, base) {
		t.Error("long-context request was charged at the base rate")
	}
}

// TestUsageTrackerCountsCachedTokensTowardTier pins the tier-selection input.
// Cached tokens are part of the context the provider prices, so a request that
// only crosses the threshold once cache reads are counted must still be billed
// at the long-context rate.
func TestUsageTrackerCountsCachedTokensTowardTier(t *testing.T) {
	ut := NewUsageTracker(tieredModel(t), "test", 80, false)
	// 50k fresh + 180k cached = 230k prompt, over the 200k threshold even
	// though the fresh input alone is well under it.
	ut.UpdateUsage(50_000, 500, 180_000, 0)

	stats := ut.GetSessionStats()
	want := 50_000*6.0/1e6 + 500*22.5/1e6 + 180_000*0.6/1e6
	if !approx(stats.TotalCost, want) {
		t.Errorf("TotalCost = %v, want %v (tier chosen from the whole prompt)", stats.TotalCost, want)
	}
}

func TestUsageTrackerFlatPricingUnaffected(t *testing.T) {
	flat := &models.ModelInfo{
		ID:    "flat",
		Cost:  models.Cost{Input: 1, Output: 2, Published: true},
		Limit: models.Limit{Context: 1_000_000, Output: 64_000},
	}
	ut := NewUsageTracker(flat, "test", 80, false)
	ut.UpdateUsage(500_000, 1_000, 0, 0)

	stats := ut.GetSessionStats()
	want := 500_000*1.0/1e6 + 1_000*2.0/1e6
	if !approx(stats.TotalCost, want) {
		t.Errorf("TotalCost = %v, want %v; flat pricing must ignore tiers", stats.TotalCost, want)
	}
}

func TestUsageTrackerOAuthStaysFree(t *testing.T) {
	ut := NewUsageTracker(tieredModel(t), "test", 80, true)
	ut.UpdateUsage(500_000, 10_000, 0, 0)

	if stats := ut.GetSessionStats(); stats.TotalCost != 0 {
		t.Errorf("OAuth usage must cost 0, got %v", stats.TotalCost)
	}
}
