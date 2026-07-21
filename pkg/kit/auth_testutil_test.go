package kit_test

import (
	"testing"

	"github.com/mark3labs/kit/internal/auth"
)

// requireAnthropicAuth skips the test when Anthropic credentials are missing
// or unusable (e.g. stored OAuth with an expired refresh token).
//
// Tests previously only checked that ANTHROPIC_API_KEY was set, but
// GetAnthropicAPIKey prefers stored OAuth credentials over the env var. When
// the OAuth refresh token is expired, kit.New fails even though the env var
// is present — so we validate the full auth resolution path here.
func requireAnthropicAuth(t *testing.T) {
	t.Helper()
	if _, _, err := auth.GetAnthropicAPIKey(""); err != nil {
		t.Skipf("Skipping test: Anthropic auth unavailable: %v", err)
	}
}
