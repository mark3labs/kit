package models

import (
	"net/http"
	"testing"
)

func TestCreateHTTPClientWithTLSConfig(t *testing.T) {
	tests := []struct {
		name         string
		skipVerify   bool
		wantInsecure bool
	}{
		{
			name:         "skip verify disabled",
			skipVerify:   false,
			wantInsecure: false,
		},
		{
			name:         "skip verify enabled",
			skipVerify:   true,
			wantInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createHTTPClientWithTLSConfig(tt.skipVerify)

			if client == nil {
				t.Fatal("expected non-nil client")
			}

			// Check if the client has a custom transport when skipVerify is true
			if tt.skipVerify {
				transport, ok := client.Transport.(*http.Transport)
				if !ok {
					t.Fatal("expected *http.Transport when skipVerify is true")
				}

				if transport.TLSClientConfig == nil {
					t.Fatal("expected non-nil TLSClientConfig when skipVerify is true")
				}

				if transport.TLSClientConfig.InsecureSkipVerify != tt.wantInsecure {
					t.Errorf("InsecureSkipVerify = %v, want %v",
						transport.TLSClientConfig.InsecureSkipVerify, tt.wantInsecure)
				}
			}
		})
	}
}

func TestCreateOAuthHTTPClient(t *testing.T) {
	tests := []struct {
		name         string
		accessToken  string
		skipVerify   bool
		wantInsecure bool
	}{
		{
			name:         "oauth with skip verify disabled",
			accessToken:  "test-token",
			skipVerify:   false,
			wantInsecure: false,
		},
		{
			name:         "oauth with skip verify enabled",
			accessToken:  "test-token",
			skipVerify:   true,
			wantInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createOAuthHTTPClient(tt.accessToken, tt.skipVerify)

			if client == nil {
				t.Fatal("expected non-nil client")
				return
			}

			// Check that the transport is an oauthTransport
			oauthTransport, ok := client.Transport.(*oauthTransport)
			if !ok {
				t.Fatal("expected *oauthTransport")
			}

			if oauthTransport.accessToken != tt.accessToken {
				t.Errorf("accessToken = %v, want %v", oauthTransport.accessToken, tt.accessToken)
			}

			// Check the base transport when skipVerify is true
			if tt.skipVerify {
				baseTransport, ok := oauthTransport.base.(*http.Transport)
				if !ok {
					t.Fatal("expected base transport to be *http.Transport when skipVerify is true")
				}

				if baseTransport.TLSClientConfig == nil {
					t.Fatal("expected non-nil TLSClientConfig when skipVerify is true")
				}

				if baseTransport.TLSClientConfig.InsecureSkipVerify != tt.wantInsecure {
					t.Errorf("InsecureSkipVerify = %v, want %v",
						baseTransport.TLSClientConfig.InsecureSkipVerify, tt.wantInsecure)
				}
			}
		})
	}
}

func TestProviderConfigTLSSkipVerify(t *testing.T) {
	// Test that ProviderConfig properly stores TLSSkipVerify
	config := &ProviderConfig{
		ModelString:   "test:model",
		TLSSkipVerify: true,
	}

	if !config.TLSSkipVerify {
		t.Error("expected TLSSkipVerify to be true")
	}
}

// TestModelAliasTargetsExist guards against alias drift. Every alias target
// must still be published by at least one provider in the embedded
// models.dev catalog. An alias whose target has disappeared everywhere can
// never resolve, so it should be pruned or repointed rather than left to
// silently no-op.
func TestModelAliasTargetsExist(t *testing.T) {
	registry := GetGlobalRegistry()
	providers := registry.GetSupportedProviders()

	for alias, target := range modelAliases {
		t.Run(alias, func(t *testing.T) {
			for _, provider := range providers {
				if registry.LookupModel(provider, target) != nil {
					return
				}
			}
			t.Errorf("alias %q -> %q: target is not published by any provider; "+
				"prune the alias or repoint it at a current model", alias, target)
		})
	}
}

// TestModelAliasTargetsNotFullyDeprecated reports aliases whose target is
// marked deprecated everywhere it is published. Such an alias still resolves,
// so it is not broken, but it steers users onto a model its provider is
// retiring — a "-latest" shorthand in particular should mean "the current
// model". This is advisory: it logs rather than fails, because the catalog can
// mark a model deprecated long before it stops answering.
func TestModelAliasTargetsNotFullyDeprecated(t *testing.T) {
	registry := GetGlobalRegistry()
	providers := registry.GetSupportedProviders()

	for alias, target := range modelAliases {
		var live, deprecated []string
		for _, provider := range providers {
			info := registry.LookupModel(provider, target)
			if info == nil {
				continue
			}
			if info.IsDeprecated() {
				deprecated = append(deprecated, provider)
			} else {
				live = append(live, provider)
			}
		}
		if len(deprecated) > 0 && len(live) == 0 {
			t.Logf("alias %q -> %q: deprecated at every provider that publishes it (%v); "+
				"consider repointing the alias", alias, target, deprecated)
		}
	}
}

// TestModelAliasNoSelfMapping rejects identity entries. They are pure no-ops:
// resolveModelAlias returns the input unchanged whether or not they exist.
func TestModelAliasNoSelfMapping(t *testing.T) {
	for alias, target := range modelAliases {
		if alias == target {
			t.Errorf("alias %q maps to itself; remove it", alias)
		}
	}
}

// TestResolveModelAliasFallsBackWhenTargetMissing pins the documented
// behaviour: an alias that does not resolve for the requested provider must
// return the original name so the caller can report "unknown model".
func TestResolveModelAliasFallsBackWhenTargetMissing(t *testing.T) {
	// claude-3-5-haiku-latest resolves on some aggregators but not on openai.
	if got := resolveModelAlias("openai", "claude-3-5-haiku-latest"); got != "claude-3-5-haiku-latest" {
		t.Errorf("resolveModelAlias(openai, claude-3-5-haiku-latest) = %q, want the input unchanged", got)
	}
	// A name that is not an alias at all passes through untouched.
	if got := resolveModelAlias("anthropic", "some-unknown-model"); got != "some-unknown-model" {
		t.Errorf("resolveModelAlias(anthropic, some-unknown-model) = %q, want the input unchanged", got)
	}
	// A live alias still resolves.
	if got := resolveModelAlias("anthropic", "claude-sonnet-latest"); got != "claude-sonnet-4-6" {
		t.Errorf("resolveModelAlias(anthropic, claude-sonnet-latest) = %q, want claude-sonnet-4-6", got)
	}
}
