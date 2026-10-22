package models

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDecisionModelsAreNotChatModels(t *testing.T) {
	registry := GetGlobalRegistry()

	if registry.LookupModel("openrouter", "typesafe/jev-1.14") != nil {
		t.Error("LookupModel returned a decision model")
	}
	info := registry.LookupDecisionModel("openrouter", "typesafe/jev-1.14")
	if info == nil {
		t.Fatal("LookupDecisionModel(openrouter, typesafe/jev-1.14) = nil")
	}
	if info.Type != ModelTypeDecision {
		t.Errorf("Type = %q, want %q", info.Type, ModelTypeDecision)
	}

	models, err := registry.GetModelsForProvider("openrouter")
	if err != nil {
		t.Fatal(err)
	}
	for id, m := range models {
		if m.Type == ModelTypeDecision {
			t.Errorf("chat model list contains decision model %s", id)
		}
	}
	for _, s := range registry.SuggestModels("openrouter", "jev") {
		if strings.Contains(s, "jev") {
			t.Errorf("SuggestModels offered decision model %s", s)
		}
	}
}

func TestCreateProviderRejectsDecisionModel(t *testing.T) {
	for _, url := range []string{"", "http://proxy.test/v1"} {
		_, err := CreateProvider(context.Background(), &ProviderConfig{ModelString: "openrouter/typesafe/jev-1.14", ProviderURL: url})
		if !errors.Is(err, ErrDecisionModelNotChat) {
			t.Errorf("provider URL %q: CreateProvider error = %v, want ErrDecisionModelNotChat", url, err)
		}
	}
}

// isolateCredentials keeps stored developer credentials out of a test.
func isolateCredentials(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

// Every embedded decision model resolves to a wire, except the providers
// whose decision endpoint Kit does not know.
func TestEmbeddedDecisionModelsHaveWires(t *testing.T) {
	registry := GetGlobalRegistry()
	entries := registry.DecisionModels()
	if len(entries) < 50 {
		t.Fatalf("embedded catalog has %d decision models; want at least 50", len(entries))
	}
	unknown := map[string]bool{"cloudflare-ai-gateway": true}
	for _, e := range entries {
		wire, ok := registry.DecisionWireFor(e.Provider, e.ID)
		if unknown[e.Provider] {
			if ok {
				t.Errorf("%s/%s: wire %q, want none", e.Provider, e.ID, wire)
			}
			continue
		}
		if !ok {
			t.Errorf("%s/%s (npm %q): no decision wire", e.Provider, e.ID, e.Info.ProviderNPM)
		}
	}
}

func TestResolveDecisionEndpoint(t *testing.T) {
	isolateCredentials(t)
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "acct123")
	t.Setenv("CLOUDFLARE_API_KEY", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cf-token")
	t.Setenv("AI_GATEWAY_API_KEY", "gw-key")
	t.Setenv("TYPESAFE_AI_API_KEY", "ts-key")
	t.Setenv("OLLAMA_HOST", "http://ollama.test:11434")

	tests := []struct {
		model     string
		wire      DecisionWire
		url       string
		bodyModel string
		key       string
		images    DecisionImageMode
	}{
		{"openrouter/typesafe/jev-1.14", DecisionWireSystemOne, "https://openrouter.ai/api/v1/systemone", "typesafe/jev-1.14", "or-key", DecisionImagesImageURLParts},
		{"cloudflare-workers-ai/@cf/cloudflare/clef-flash", DecisionWireWorkersAI, "https://api.cloudflare.com/client/v4/accounts/acct123/ai/run/@cf/cloudflare/clef-flash", "clef-flash", "cf-token", DecisionImagesDataURL},
		{"vercel/typesafe-ai/jev", DecisionWireGateway, "https://ai-gateway.vercel.sh/v4/ai/decision-model", "typesafe-ai/jev", "gw-key", DecisionImagesStateParts},
		{"typesafe/jev-latest", DecisionWireSystemOne, "https://api.typesafe.ai/v1/systemone", "jev-latest", "ts-key", DecisionImagesMultipart},
		{"ollama/pplx-decider", DecisionWireSystemOne, "http://ollama.test:11434/v1/systemone", "pplx-decider", "", DecisionImagesBase64},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			ep, err := ResolveDecisionEndpoint(&ProviderConfig{ModelString: tc.model})
			if err != nil {
				t.Fatal(err)
			}
			if ep.Wire != tc.wire || ep.URL != tc.url || ep.Model != tc.bodyModel || ep.APIKey != tc.key || ep.Images != tc.images {
				t.Errorf("got wire=%s url=%s model=%s key=%q images=%s\nwant wire=%s url=%s model=%s key=%q images=%s",
					ep.Wire, ep.URL, ep.Model, ep.APIKey, ep.Images, tc.wire, tc.url, tc.bodyModel, tc.key, tc.images)
			}
			if ep.HTTPClient == nil {
				t.Error("no HTTP client")
			}
		})
	}
}

func TestResolveDecisionEndpointErrors(t *testing.T) {
	isolateCredentials(t)
	t.Setenv("OPENROUTER_API_KEY", "or-key")

	_, err := ResolveDecisionEndpoint(&ProviderConfig{ModelString: "anthropic/claude-sonnet-4-6"})
	if !errors.Is(err, ErrNotDecisionModel) {
		t.Errorf("chat model: error = %v, want ErrNotDecisionModel", err)
	}

	_, err = ResolveDecisionEndpoint(&ProviderConfig{ModelString: "openrouter/no-such-decider"})
	if !errors.Is(err, ErrNotDecisionModel) {
		t.Errorf("unknown model: error = %v, want ErrNotDecisionModel", err)
	}

	_, err = ResolveDecisionEndpoint(&ProviderConfig{ModelString: "cloudflare-ai-gateway/typesafe/jev"})
	if !errors.Is(err, ErrUnknownDecisionWire) {
		t.Errorf("cloudflare-ai-gateway: error = %v, want ErrUnknownDecisionWire", err)
	}

	t.Setenv("OPENROUTER_API_KEY", "")
	_, err = ResolveDecisionEndpoint(&ProviderConfig{ModelString: "openrouter/typesafe/jev-1.14"})
	if err == nil || !strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
		t.Errorf("missing key: error = %v, want a missing credentials error", err)
	}
}

func TestResolveDecisionEndpointOverrides(t *testing.T) {
	ep, err := ResolveDecisionEndpoint(&ProviderConfig{
		ModelString:    "openrouter/typesafe/jev-1.14",
		ProviderURL:    "http://proxy.test/v1/",
		ProviderAPIKey: "explicit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ep.URL != "http://proxy.test/v1/systemone" || ep.APIKey != "explicit" {
		t.Errorf("url=%s key=%q; want the explicit URL and key", ep.URL, ep.APIKey)
	}

	// An Ollama URL gets exactly one /v1, as for chat.
	for _, url := range []string{"http://gpu:11434", "http://gpu:11434/", "http://gpu:11434/v1"} {
		ep, err := ResolveDecisionEndpoint(&ProviderConfig{ModelString: "ollama/pplx-decider", ProviderURL: url})
		if err != nil {
			t.Fatal(err)
		}
		if ep.URL != "http://gpu:11434/v1/systemone" {
			t.Errorf("ollama URL %q: got %s", url, ep.URL)
		}
	}
}
