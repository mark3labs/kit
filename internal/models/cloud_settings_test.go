package models

import (
	"context"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/spf13/viper"

	"github.com/mark3labs/kit/internal/auth"
)

// withProviderSettings installs a copy of the global registry in which the
// provider has the given region and project, as if they came from the
// `providers` config section.
func withProviderSettings(t *testing.T, id, region, project string) {
	t.Helper()
	old := GetGlobalRegistry()
	reg := &ModelsRegistry{providers: maps.Clone(old.providers)}
	info := reg.providers[id]
	info.Region, info.Project = region, project
	reg.providers[id] = info
	globalRegistry.Store(reg)
	t.Cleanup(func() { globalRegistry.Store(old) })
}

// storeTestAPIKey stores key for provider in the credentials file under the
// test's XDG_CONFIG_HOME.
func storeTestAPIKey(t *testing.T, provider, key string) {
	t.Helper()
	storeTestBedrockKey(t, provider, key, "")
}

// storeTestBedrockKey stores key with its region.
func storeTestBedrockKey(t *testing.T, provider, key, region string) {
	t.Helper()
	cm, err := auth.NewCredentialManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := cm.SetProviderAPIKeyWithRegion(provider, key, region); err != nil {
		t.Fatal(err)
	}
}

// freshAWSEnv keeps the AWS SDK away from the developer's own AWS setup.
func freshAWSEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, name := range []string{"AWS_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_BEARER_TOKEN_BEDROCK"} {
		t.Setenv(name, "")
	}
}

// capturedRequest records the host, path and Authorization header of the
// model request a provider sends.
type capturedRequest struct {
	host, path, authorization string
}

// sendTestRequest creates the provider for model and sends one request,
// which the mocked transport captures. Token requests of the Google
// credentials are answered and not captured.
func sendTestRequest(t *testing.T, model string) capturedRequest {
	t.Helper()
	var got capturedRequest
	mockDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Host, "credentials.test") || strings.Contains(r.URL.Host, "oauth2") {
			return jsonTestResponse(r, `{"access_token":"fake-google-token","token_type":"Bearer","expires_in":3600}`), nil
		}
		got = capturedRequest{host: r.URL.Host, path: r.URL.Path, authorization: r.Header.Get("Authorization")}
		return jsonTestResponse(r, `{}`), nil
	})
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	result, err := CreateProvider(ctx, &ProviderConfig{ModelString: model})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = result.Model.Generate(ctx, fantasy.Call{Prompt: []fantasy.Message{fantasy.NewUserMessage("hi")}})
	if got.host == "" {
		t.Fatal("no model request")
	}
	return got
}

// TestBedrockStoredKey checks that a stored key is sent as the bearer token,
// with no other AWS credentials and when AWS_BEARER_TOKEN_BEDROCK is set,
// for both names of the provider.
func TestBedrockStoredKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	freshAWSEnv(t)
	t.Setenv("AWS_REGION", "us-west-2")
	storeTestAPIKey(t, "amazon-bedrock", "from-store")

	for _, envToken := range []string{"", "from-env"} {
		for _, model := range []string{"amazon-bedrock/us.anthropic.claude-opus-5-5", "bedrock/us.anthropic.claude-opus-5-5"} {
			t.Run("token="+envToken+"/"+model, func(t *testing.T) {
				t.Setenv("AWS_BEARER_TOKEN_BEDROCK", envToken)
				got := sendTestRequest(t, model)
				if got.host != "bedrock-runtime.us-west-2.amazonaws.com" {
					t.Errorf("host = %q", got.host)
				}
				if got.authorization != "Bearer from-store" {
					t.Errorf("Authorization = %q, want the stored key", got.authorization)
				}
			})
		}
	}
}

// TestBedrockStoredRegion checks that the region stored with the key takes
// precedence over AWS_REGION, and that without a stored key the AWS SDK
// still takes the region from the environment.
func TestBedrockStoredRegion(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	freshAWSEnv(t)
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "from-env")

	got := sendTestRequest(t, "amazon-bedrock/us.anthropic.claude-opus-5-5")
	if got.host != "bedrock-runtime.us-west-2.amazonaws.com" {
		t.Errorf("without a stored key: host = %q", got.host)
	}
	if got.authorization != "Bearer from-env" {
		t.Errorf("without a stored key: Authorization = %q, want the environment token", got.authorization)
	}

	storeTestBedrockKey(t, "amazon-bedrock", "from-store", "eu-central-1")
	got = sendTestRequest(t, "amazon-bedrock/eu.anthropic.claude-opus-5-5")
	if got.host != "bedrock-runtime.eu-central-1.amazonaws.com" {
		t.Errorf("with a stored key: host = %q", got.host)
	}
	if got.authorization != "Bearer from-store" {
		t.Errorf("with a stored key: Authorization = %q", got.authorization)
	}
}

// TestBedrockCredentials covers the region rules for a stored key, which has
// no AWS shared config to fall back on.
func TestBedrockCredentials(t *testing.T) {
	tests := []struct {
		name                 string
		stored, storedRegion string
		env, envDflt         string
		wantKey, wantRegion  string
		wantErr              bool
	}{
		{name: "no key, nothing set"},
		{name: "no key leaves the region to the SDK", env: "us-west-2"},
		{name: "key, stored region wins", stored: "k", storedRegion: "eu-central-1", env: "us-west-2", wantKey: "k", wantRegion: "eu-central-1"},
		{name: "key, AWS_REGION", stored: "k", env: "us-west-2", envDflt: "ap-northeast-1", wantKey: "k", wantRegion: "us-west-2"},
		{name: "key, AWS_DEFAULT_REGION", stored: "k", envDflt: "ap-northeast-1", wantKey: "k", wantRegion: "ap-northeast-1"},
		{name: "key without a region", stored: "k", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("AWS_REGION", tt.env)
			t.Setenv("AWS_DEFAULT_REGION", tt.envDflt)
			if tt.stored != "" {
				storeTestBedrockKey(t, "amazon-bedrock", tt.stored, tt.storedRegion)
			}
			key, region, err := bedrockCredentials()
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "--region") {
					t.Fatalf("err = %v, want one that names --region", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if key != tt.wantKey || region != tt.wantRegion {
				t.Errorf("got (%q, %q), want (%q, %q)", key, region, tt.wantKey, tt.wantRegion)
			}
		})
	}
}

// TestBedrockStoredKeyNeedsRegion checks that Kit stops instead of sending
// the requests to a region nobody chose.
func TestBedrockStoredKeyNeedsRegion(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	freshAWSEnv(t)
	storeTestAPIKey(t, "amazon-bedrock", "from-store")
	_, err := CreateProvider(t.Context(), &ProviderConfig{ModelString: "amazon-bedrock/us.anthropic.claude-opus-5-5"})
	if err == nil || !strings.Contains(err.Error(), "kit auth login amazon-bedrock --region") {
		t.Fatalf("err = %v, want a missing region error", err)
	}
}

// TestBearerTokenRoundTripper checks that the header is replaced and that
// the caller's request is left as it was.
func TestBearerTokenRoundTripper(t *testing.T) {
	var sent string
	base := codexTestRoundTripper(func(r *http.Request) (*http.Response, error) {
		sent = r.Header.Get("Authorization")
		return jsonTestResponse(r, `{}`), nil
	})
	client := withBearerToken(&http.Client{Transport: base}, "stored")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://bedrock.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer from-env")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if sent != "Bearer stored" {
		t.Errorf("sent Authorization = %q", sent)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer from-env" {
		t.Errorf("caller's request changed: %q", got)
	}

	// The HTTP client strips the header from a redirect to another host;
	// the token must not come back there.
	req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, "https://elsewhere.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if sent != "" {
		t.Errorf("request without Authorization sent %q", sent)
	}
}

// TestBedrockAliasUsesCatalog checks that the documented bedrock/ prefix
// finds the catalog entries of amazon-bedrock.
func TestBedrockAliasUsesCatalog(t *testing.T) {
	reg := GetGlobalRegistry()
	if info := reg.GetProviderInfo("bedrock"); info == nil || info.ID != "amazon-bedrock" {
		t.Fatalf("GetProviderInfo(bedrock) = %+v", info)
	}
	if reg.LookupModel("bedrock", "us.anthropic.claude-opus-5-5") == nil {
		t.Error("bedrock/us.anthropic.claude-opus-5-5 has no catalog entry")
	}
}

// writeTestGoogleCredentials points Application Default Credentials at a
// fake authorized-user file whose token endpoint the mocked transport
// answers.
func writeTestGoogleCredentials(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "google.json")
	if err := os.WriteFile(path, []byte(`{"type":"authorized_user","client_id":"fake","client_secret":"fake","refresh_token":"fake","token_uri":"http://credentials.test/token"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
}

// TestVertexConfiguredSettings checks that the project and region from the
// `providers` section take precedence over the environment.
func TestVertexConfiguredSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeTestGoogleCredentials(t)
	t.Setenv("GOOGLE_VERTEX_PROJECT", "env-project")
	t.Setenv("GOOGLE_VERTEX_LOCATION", "us-central1")

	for _, tt := range []struct{ provider, model string }{
		{"google-vertex-anthropic", "claude-opus-5-5@default"},
		{"google-vertex", "gemini-2.5-flash"},
	} {
		t.Run(tt.provider, func(t *testing.T) {
			withProviderSettings(t, tt.provider, "europe-west1", "cfg-project")
			got := sendTestRequest(t, tt.provider+"/"+tt.model)
			if got.host != "europe-west1-aiplatform.googleapis.com" {
				t.Errorf("host = %q", got.host)
			}
			if !strings.Contains(got.path, "/projects/cfg-project/locations/europe-west1/") {
				t.Errorf("path = %q", got.path)
			}
		})
	}
}

// TestApplyProviderOverrides_RegionProject checks the new fields and that
// an entry under an alias is not registered as a provider of its own.
func TestApplyProviderOverrides_RegionProject(t *testing.T) {
	providers := map[string]ProviderInfo{
		"amazon-bedrock":          {ID: "amazon-bedrock", Name: "Amazon Bedrock"},
		"google-vertex-anthropic": {ID: "google-vertex-anthropic"},
	}
	applyProviderOverrides(providers, map[string]ProviderOverrideConfig{
		"google-vertex-anthropic": {Project: "cfg-project", Region: "europe-west1"},
		"bedrock":                 {Name: "Bedrock under its alias"},
	})
	if got := providers["google-vertex-anthropic"]; got.Project != "cfg-project" || got.Region != "europe-west1" {
		t.Errorf("google-vertex-anthropic = %+v", got)
	}
	if got := providers["amazon-bedrock"]; got.Name != "Amazon Bedrock" {
		t.Errorf("alias entry changed amazon-bedrock: %+v", got)
	}
	if _, ok := providers["bedrock"]; ok {
		t.Error("alias registered as a provider")
	}
}

// TestLoadProviderOverridesFrom_RegionProject checks the config keys.
func TestLoadProviderOverridesFrom_RegionProject(t *testing.T) {
	v := viper.New()
	v.Set("providers", map[string]any{
		"google-vertex-anthropic": map[string]any{"project": "cfg-project", "region": "europe-west1"},
	})
	overrides := loadProviderOverridesFrom(v)
	if got := overrides["google-vertex-anthropic"]; got.Project != "cfg-project" || got.Region != "europe-west1" {
		t.Errorf("google-vertex-anthropic = %+v", got)
	}
}
