package models

import "strings"

// ModelsDBProviders is the top-level type for models.dev/api.json data:
// a map of provider ID → provider object.
type ModelsDBProviders = map[string]modelsDBProvider

// modelsDBProvider represents a provider entry from models.dev/api.json.
type modelsDBProvider struct {
	ID     string                   `json:"id"`
	Env    []string                 `json:"env"`
	NPM    string                   `json:"npm"`
	API    string                   `json:"api,omitempty"`
	Name   string                   `json:"name"`
	Doc    string                   `json:"doc,omitempty"`
	Models map[string]modelsDBModel `json:"models"`
}

// modelsDBModel represents a model entry from models.dev/api.json.
type modelsDBModel struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Family      string                 `json:"family,omitempty"`
	Attachment  bool                   `json:"attachment"`
	Reasoning   bool                   `json:"reasoning"`
	ToolCall    bool                   `json:"tool_call"`
	Temperature bool                   `json:"temperature"`
	Cost        modelsDBCost           `json:"cost"`
	Limit       modelsDBLimit          `json:"limit"`
	Provider    *modelsDBModelProvider `json:"provider,omitempty"` // Model-specific provider override
}

// modelsDBModelProvider represents a provider reference within a model.
type modelsDBModelProvider struct {
	NPM string `json:"npm"`
}

// modelsDBCost represents model pricing from models.dev.
type modelsDBCost struct {
	Input      float64  `json:"input"`
	Output     float64  `json:"output"`
	CacheRead  *float64 `json:"cache_read,omitempty"`
	CacheWrite *float64 `json:"cache_write,omitempty"`
}

// modelsDBLimit represents model context/output limits from models.dev.
type modelsDBLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

// wireProtocol identifies which LLM API protocol a provider speaks.
// Fantasy implements three native protocols (openai, anthropic, google);
// everything else in its providers/ tree is a thin wrapper around one of
// them with a pre-baked default URL or auth scheme. The OpenAI protocol is
// split into two wires: wireOpenAI (the Responses API, spoken only by
// api.openai.com and true mirrors of it) and wireOpenAICompat (the
// chat-completions wire spoken by virtually every OpenAI-compatible
// provider and proxy).
type wireProtocol int

const (
	wireUnknown      wireProtocol = iota
	wireOpenAI                    // OpenAI Responses API
	wireOpenAICompat              // OpenAI chat-completions API
	wireAnthropic
	wireGoogle
)

// Wire protocol names accepted in provider overrides (config `providers`
// section) and the --provider-wire flag. These are the user-facing,
// dependency-agnostic names for the wire protocols.
const (
	WireNameOpenAI       = "openai"
	WireNameOpenAICompat = "openai-compat"
	WireNameAnthropic    = "anthropic"
	WireNameGoogle       = "google"
)

// parseWire maps a user-facing wire protocol name to its wireProtocol value.
// Accepts a few common aliases. Returns (wireUnknown, false) for empty or
// unrecognized names.
func parseWire(name string) (wireProtocol, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case WireNameOpenAI, "openai-responses":
		return wireOpenAI, true
	case WireNameOpenAICompat, "openai-compatible", "openai-chat":
		return wireOpenAICompat, true
	case WireNameAnthropic:
		return wireAnthropic, true
	case WireNameGoogle, "gemini":
		return wireGoogle, true
	}
	return wireUnknown, false
}

// wireNames lists the accepted canonical wire names for error messages.
func wireNames() string {
	return strings.Join([]string{WireNameOpenAI, WireNameOpenAICompat, WireNameAnthropic, WireNameGoogle}, ", ")
}

// npmToWireProtocol maps npm package names from models.dev to the wire
// protocol they speak. Provider-specific bundles that need bespoke auth or
// URL templating (azure, bedrock, openrouter, google-vertex, google-vertex-
// anthropic, and @ai-sdk/gateway which is the Vercel AI Gateway) are
// intentionally absent — they have native top-level cases in CreateProvider
// and never reach the auto-router. Providers not in this map but with an
// api URL are auto-routed through the OpenAI-compatible wire.
//
// The thin OpenAI-compatible npm wrappers (groq, cerebras, mistral, …) are
// listed explicitly so that auto-routing can recover their hard-coded base
// URL from sdkDefaultBaseURL when the registry entry has no api field.
var npmToWireProtocol = map[string]wireProtocol{
	// Native wires.
	"@ai-sdk/openai":            wireOpenAI,
	"@ai-sdk/openai-compatible": wireOpenAICompat,
	"@ai-sdk/anthropic":         wireAnthropic,
	"@ai-sdk/google":            wireGoogle,

	// Thin OpenAI-compatible wrappers. Each ships with a hard-coded base URL
	// in its JS SDK (see sdkDefaultBaseURL) but speaks the plain OpenAI chat
	// completions wire — so we can route them all through fantasy's
	// openaicompat provider once we supply the URL.
	"@ai-sdk/groq":                  wireOpenAICompat,
	"@ai-sdk/cerebras":              wireOpenAICompat,
	"@ai-sdk/perplexity":            wireOpenAICompat,
	"@ai-sdk/togetherai":            wireOpenAICompat,
	"@ai-sdk/xai":                   wireOpenAICompat,
	"@ai-sdk/deepinfra":             wireOpenAICompat,
	"@ai-sdk/mistral":               wireOpenAICompat,
	"@ai-sdk/cohere":                wireOpenAICompat,
	"@ai-sdk/vercel":                wireOpenAICompat, // v0 API (api.v0.dev), distinct from @ai-sdk/gateway
	"@aihubmix/ai-sdk-provider":     wireOpenAICompat,
	"venice-ai-sdk-provider":        wireOpenAICompat,
	"merge-gateway-ai-sdk-provider": wireOpenAICompat,
}

// sdkDefaultBaseURL maps an npm package name to the base URL its JavaScript
// SDK uses by default. This lets us recover a working endpoint for providers
// whose models.dev entry omits the `api` field because the JS SDK hard-codes
// the URL (e.g. groq, cerebras, mistral, x.ai…).
//
// Only OpenAI-compatible and native-wire SDKs are listed; providers needing
// bespoke auth or URL templating (bedrock SigV4, azure resource URLs,
// google-vertex project/location, cloudflare gateway account IDs, gitlab,
// sap-ai-core) are handled by native CreateProvider cases or surface a
// targeted error that asks the user to supply --provider-url.
var sdkDefaultBaseURL = map[string]string{
	// Native wires.
	"@ai-sdk/openai":    "https://api.openai.com/v1",
	"@ai-sdk/anthropic": "https://api.anthropic.com/v1",
	"@ai-sdk/google":    "https://generativelanguage.googleapis.com/v1beta",

	// Thin OpenAI-compatible wrappers.
	"@ai-sdk/groq":                  "https://api.groq.com/openai/v1",
	"@ai-sdk/cerebras":              "https://api.cerebras.ai/v1",
	"@ai-sdk/perplexity":            "https://api.perplexity.ai",
	"@ai-sdk/togetherai":            "https://api.together.xyz/v1",
	"@ai-sdk/xai":                   "https://api.x.ai/v1",
	"@ai-sdk/deepinfra":             "https://api.deepinfra.com/v1/openai",
	"@ai-sdk/mistral":               "https://api.mistral.ai/v1",
	"@ai-sdk/cohere":                "https://api.cohere.com/compatibility/v1",
	"@ai-sdk/vercel":                "https://api.v0.dev/v1",
	"@aihubmix/ai-sdk-provider":     "https://aihubmix.com/v1",
	"venice-ai-sdk-provider":        "https://api.venice.ai/api/v1",
	"merge-gateway-ai-sdk-provider": "https://api-gateway.merge.dev/v1/ai-sdk",

	// Native handlers — included for ResolveProviderBaseURL introspection
	// even though CreateProvider routes these via dedicated cases.
	"@ai-sdk/gateway":             "https://ai-gateway.vercel.sh/v1",
	"@openrouter/ai-sdk-provider": "https://openrouter.ai/api/v1",
}
