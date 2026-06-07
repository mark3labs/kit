package models

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"charm.land/fantasy/providers/google"
)

// templatePlaceholderRe matches "${NAME}" placeholders in URL templates from
// models.dev (e.g. "https://${DATABRICKS_HOST}/ai-gateway/mlflow/v1").
var templatePlaceholderRe = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

// templateEnvVarOverrides supplies fallback environment variable names for
// placeholders that providers commonly use under non-obvious env names.
// The placeholder name itself is always tried first; this map adds extra
// names to try when the placeholder doesn't match the canonical env var.
var templateEnvVarOverrides = map[string][]string{
	"CLOUDFLARE_ACCOUNT_ID":   {"CF_ACCOUNT_ID"},
	"CLOUDFLARE_GATEWAY_NAME": {"CF_GATEWAY", "CLOUDFLARE_GATEWAY"},
	"DATABRICKS_HOST":         {"DATABRICKS_WORKSPACE_URL"},
	"SNOWFLAKE_ACCOUNT":       {"SNOWFLAKE_ACCOUNT_ID"},
}

// resolveTemplatedAPIURL substitutes "${VAR}" placeholders in apiURL with the
// values of the named environment variables. Returns:
//   - ("", nil) when apiURL contains no placeholders (caller keeps current URL),
//   - (resolved, nil) when every placeholder was resolved,
//   - ("", error) when one or more placeholders are unset, with a message that
//     names the missing env vars and points at the relevant provider.
//
// The info parameter is used purely for error messaging (provider name).
func resolveTemplatedAPIURL(apiURL string, info *ProviderInfo) (string, error) {
	if apiURL == "" || !strings.Contains(apiURL, "${") {
		return "", nil
	}

	var missing []string
	resolved := templatePlaceholderRe.ReplaceAllStringFunc(apiURL, func(match string) string {
		// match is "${NAME}". Extract NAME.
		name := match[2 : len(match)-1]
		if v := os.Getenv(name); v != "" {
			return v
		}
		for _, alt := range templateEnvVarOverrides[name] {
			if v := os.Getenv(alt); v != "" {
				return v
			}
		}
		missing = append(missing, name)
		return match
	})

	if len(missing) > 0 {
		providerName := info.ID
		if info.Name != "" {
			providerName = info.Name
		}
		return "", fmt.Errorf(
			"provider %s requires environment variable(s) %s to construct its API URL (%s); "+
				"set them or pass --provider-url to override",
			providerName, strings.Join(missing, ", "), apiURL,
		)
	}
	return resolved, nil
}

// ResolveProviderBaseURL returns the base API URL kit will use when talking to
// the given provider, applying the same resolution order as CreateProvider:
//
//  1. The provider's `api` field from the models.dev registry.
//  2. The hard-coded default base URL of its npm SDK package (e.g.
//     @ai-sdk/groq → https://api.groq.com/openai/v1).
//  3. Template substitution against the current process environment when the
//     URL contains "${VAR}" placeholders (e.g. cloudflare-workers-ai needs
//     CLOUDFLARE_ACCOUNT_ID).
//
// It returns an error when the provider is unknown, when no URL can be derived,
// or when a templated URL has unset placeholders. The error message is suitable
// for direct display to end users.
//
// Note: providers handled by bespoke auth schemes (amazon-bedrock SigV4,
// azure resource URLs, google-vertex project/location, sap-ai-core customer
// deployments) may return either an empty URL or a regional/templated URL —
// the actual endpoint is finalised inside their native handlers and depends on
// runtime credentials.
func ResolveProviderBaseURL(providerID string) (string, error) {
	registry := GetGlobalRegistry()
	info := registry.GetProviderInfo(providerID)
	if info == nil {
		return "", fmt.Errorf("unknown provider: %s", providerID)
	}

	apiURL := info.API
	if apiURL == "" {
		if defaultURL, ok := sdkDefaultBaseURL[info.NPM]; ok {
			apiURL = defaultURL
		}
	}

	if apiURL == "" {
		return "", fmt.Errorf(
			"provider %s has no default API URL: its npm package %q does not "+
				"ship a built-in baseURL (likely Bedrock SigV4, Azure deployment, "+
				"Vertex project/location, or a customer-hosted endpoint). "+
				"Pass --provider-url or set the provider's URL env var",
			providerID, info.NPM,
		)
	}

	if strings.Contains(apiURL, "${") {
		resolved, err := resolveTemplatedAPIURL(apiURL, info)
		if err != nil {
			return apiURL, err
		}
		return resolved, nil
	}
	return apiURL, nil
}

// createGoogleVertexProvider creates a Google Gemini provider that targets the
// Vertex AI backend (rather than the public generativelanguage.googleapis.com
// endpoint). It requires the same project/region environment variables as
// google-vertex-anthropic.
func createGoogleVertexProvider(ctx context.Context, config *ProviderConfig, modelName string) (*ProviderResult, error) {
	projectID := firstNonEmpty(
		os.Getenv("GOOGLE_VERTEX_PROJECT"),
		os.Getenv("GOOGLE_CLOUD_PROJECT"),
		os.Getenv("GCLOUD_PROJECT"),
		os.Getenv("CLOUDSDK_CORE_PROJECT"),
	)
	if projectID == "" {
		return nil, fmt.Errorf(
			"google Vertex project ID not provided, set GOOGLE_VERTEX_PROJECT, " +
				"GOOGLE_CLOUD_PROJECT, or GCLOUD_PROJECT environment variable",
		)
	}

	region := firstNonEmpty(
		os.Getenv("GOOGLE_VERTEX_LOCATION"),
		os.Getenv("CLOUD_ML_REGION"),
	)
	if region == "" {
		region = "global"
	}

	opts := []google.Option{
		google.WithVertex(projectID, region),
		google.WithName("google-vertex"),
	}

	if config.TLSSkipVerify {
		opts = append(opts, google.WithHTTPClient(createHTTPClientWithTLSConfig(true)))
	}

	provider, err := google.New(opts...)
	if err != nil {
		return nil, wrapProviderErr("Google Vertex", "provider", err)
	}

	model, err := provider.LanguageModel(ctx, modelName)
	if err != nil {
		return nil, wrapProviderErr("Google Vertex", "model", err)
	}

	return &ProviderResult{Model: model}, nil
}
