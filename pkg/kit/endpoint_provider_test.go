package kit

import (
	"testing"

	"github.com/spf13/viper"
)

// The provider-url / provider-api-key / provider-wire overrides belong to the
// provider of the model they were configured with. A model switch to another
// provider must not carry them along, or the other provider's requests would
// be sent, with the wrong key, to the first provider's endpoint.

func TestEndpointProviderFor(t *testing.T) {
	t.Run("no override binds to nothing", func(t *testing.T) {
		v := viper.New()
		if got := endpointProviderFor(v, "anthropic/claude"); got != "" {
			t.Errorf("endpointProviderFor = %q, want empty", got)
		}
	})

	t.Run("url binds to the model's provider", func(t *testing.T) {
		v := viper.New()
		v.Set("provider-url", "http://127.0.0.1:1234/v1")
		if got := endpointProviderFor(v, "custom/local-model"); got != "custom" {
			t.Errorf("endpointProviderFor = %q, want custom", got)
		}
	})

	t.Run("api key alone binds", func(t *testing.T) {
		v := viper.New()
		v.Set("provider-api-key", "sk-x")
		if got := endpointProviderFor(v, "openai/gpt-x"); got != "openai" {
			t.Errorf("endpointProviderFor = %q, want openai", got)
		}
	})

	t.Run("wire alone binds", func(t *testing.T) {
		v := viper.New()
		v.Set("provider-wire", "anthropic")
		if got := endpointProviderFor(v, "proxy/model"); got != "proxy" {
			t.Errorf("endpointProviderFor = %q, want proxy", got)
		}
	})

	t.Run("unparseable model binds to nothing", func(t *testing.T) {
		v := viper.New()
		v.Set("provider-url", "http://127.0.0.1:1234/v1")
		if got := endpointProviderFor(v, ""); got != "" {
			t.Errorf("endpointProviderFor = %q, want empty", got)
		}
	})
}

func TestEndpointOverridesApply(t *testing.T) {
	t.Run("unbound applies to every model", func(t *testing.T) {
		k := &Kit{}
		if !k.endpointOverridesApply("anthropic/claude") {
			t.Error("unbound overrides must apply")
		}
	})

	t.Run("same provider applies", func(t *testing.T) {
		k := &Kit{endpointProvider: "custom"}
		if !k.endpointOverridesApply("custom/other-model") {
			t.Error("overrides must apply to the bound provider")
		}
	})

	t.Run("other provider does not apply", func(t *testing.T) {
		k := &Kit{endpointProvider: "custom"}
		if k.endpointOverridesApply("opencode/grok-4.6") {
			t.Error("overrides must not leak to another provider")
		}
	})

	t.Run("unparseable model does not apply", func(t *testing.T) {
		k := &Kit{endpointProvider: "custom"}
		if k.endpointOverridesApply("") {
			t.Error("overrides must not apply to an unparseable model")
		}
	})
}
