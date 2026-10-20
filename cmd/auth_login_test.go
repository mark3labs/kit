package cmd

import (
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/auth"
)

// TestLoginProviderAPIKeyRegion checks that 'kit auth login amazon-bedrock
// --api-key ... --region ...' stores the region with the key, that a new key
// without --region keeps it, and that --region is refused for other
// providers before anything is stored.
func TestLoginProviderAPIKeyRegion(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { loginAPIKey, loginRegion = "", "" })

	loginAPIKey, loginRegion = "ABSK-test", "eu-central-1"
	if err := loginProviderAPIKey("amazon-bedrock"); err != nil {
		t.Fatal(err)
	}
	if key, region := auth.LookupStoredAPIKeyWithRegion("amazon-bedrock"); key != "ABSK-test" || region != "eu-central-1" {
		t.Errorf("stored (%q, %q), want (ABSK-test, eu-central-1)", key, region)
	}

	// A rotated key without --region keeps the region.
	loginAPIKey, loginRegion = "ABSK-rotated", ""
	if err := loginProviderAPIKey("bedrock"); err != nil {
		t.Fatal(err)
	}
	if key, region := auth.LookupStoredAPIKeyWithRegion("amazon-bedrock"); key != "ABSK-rotated" || region != "eu-central-1" {
		t.Errorf("after rotation (%q, %q), want (ABSK-rotated, eu-central-1)", key, region)
	}

	loginAPIKey, loginRegion = "gsk-test", "eu-central-1"
	err := loginProviderAPIKey("groq")
	if err == nil || !strings.Contains(err.Error(), "--region") {
		t.Fatalf("err = %v, want --region refused for groq", err)
	}
	if key := auth.LookupStoredAPIKey("groq"); key != "" {
		t.Errorf("groq key stored despite the error: %q", key)
	}

	// The providers with their own login flows refuse it too.
	for _, provider := range []string{"anthropic", "openai", "copilot"} {
		if err := runAuthLogin(authLoginCmd, []string{provider}); err == nil || !strings.Contains(err.Error(), "--region") {
			t.Errorf("%s: err = %v, want --region refused", provider, err)
		}
	}
}
