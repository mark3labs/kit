package models

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/mark3labs/kit/internal/httpclient"
)

// Decision model errors.
var (
	// ErrNotDecisionModel indicates that a model string does not name a
	// decision model in the catalog.
	ErrNotDecisionModel = errors.New("not a decision model")

	// ErrDecisionModelNotChat indicates that a decision model was used where
	// a chat model is required.
	ErrDecisionModelNotChat = errors.New("decision models cannot chat")

	// ErrUnknownDecisionWire indicates that Kit cannot tell which protocol a
	// decision model speaks.
	ErrUnknownDecisionWire = errors.New("unknown decision wire protocol")
)

// DecisionWire identifies the HTTP protocol a decision model speaks.
type DecisionWire string

const (
	// DecisionWireSystemOne is the TypeSafe request shape (noul, choice,
	// score), sent to POST {base}/systemone. Most hosts speak it.
	DecisionWireSystemOne DecisionWire = "systemone"
	// DecisionWireWorkersAI is the SystemOne body sent to Cloudflare Workers
	// AI at POST {base}/{model}, with the answer inside Cloudflare's
	// response envelope.
	DecisionWireWorkersAI DecisionWire = "workers-ai"
	// DecisionWireGateway is the Vercel AI Gateway decision model protocol,
	// sent to POST {base}/decision-model.
	DecisionWireGateway DecisionWire = "gateway"
)

// DecisionImageMode identifies how a decision wire carries images.
type DecisionImageMode string

const (
	// DecisionImagesMultipart sends the request as multipart form data: the
	// JSON request plus one part per image, referenced from state by asset
	// name.
	DecisionImagesMultipart DecisionImageMode = "multipart"
	// DecisionImagesDataURL sends images as data URLs in an "images" array.
	DecisionImagesDataURL DecisionImageMode = "data-url"
	// DecisionImagesBase64 sends images as bare base64 in an "images" array.
	DecisionImagesBase64 DecisionImageMode = "base64"
	// DecisionImagesStateParts sends images as file parts of the state.
	DecisionImagesStateParts DecisionImageMode = "state-parts"
	// DecisionImagesImageURLParts sends the state as an array of parts, with
	// each image as an {"type": "image_url"} part that holds a data URL
	// (OpenRouter).
	DecisionImagesImageURLParts DecisionImageMode = "image-url-parts"
)

// defaultGatewayDecisionURL is the base URL of the Vercel AI Gateway decision
// protocol. The catalog's Vercel URL is the chat endpoint, so it is not used.
const defaultGatewayDecisionURL = "https://ai-gateway.vercel.sh/v4/ai"

// npmToDecisionWire maps the effective catalog npm package of a decision
// model to the decision wire it speaks. It is separate from
// npmToWireProtocol because one package can mean a different endpoint for
// chat and for decisions.
var npmToDecisionWire = map[string]DecisionWire{
	"@ai-sdk/typesafe-ai": DecisionWireSystemOne,
	"@ai-sdk/gateway":     DecisionWireGateway,
}

// decisionProviderOverride covers a provider whose catalog npm describes its
// chat wire, or which is not in the catalog.
type decisionProviderOverride struct {
	Wire DecisionWire
	// BaseURL replaces the catalog URL. Empty keeps the catalog URL.
	BaseURL func() string
	// Env lists extra environment variables that may hold the API key.
	Env    []string
	Images DecisionImageMode
	// ImageTypes lists the accepted image media types. Nil keeps the
	// default (PNG and JPEG).
	ImageTypes []string
	// MaxImages limits the images per request. 0 means no known limit.
	MaxImages int
	// NoAuth sends no Authorization header.
	NoAuth bool
	// AnyModel accepts model names that are not in the catalog.
	AnyModel bool
}

var decisionProviderOverrides = map[string]decisionProviderOverride{
	"cloudflare-workers-ai": {
		Wire:       DecisionWireWorkersAI,
		Env:        []string{"CLOUDFLARE_API_TOKEN", "CF_API_TOKEN"},
		Images:     DecisionImagesDataURL,
		ImageTypes: []string{"image/png", "image/jpeg", "image/webp"},
		MaxImages:  4,
	},
	"openrouter": {
		// OpenRouter speaks SystemOne but rejects multipart and the
		// top-level "images" array.
		Wire:   DecisionWireSystemOne,
		Images: DecisionImagesImageURLParts,
	},
	"ollama": {
		Wire:       DecisionWireSystemOne,
		BaseURL:    func() string { return strings.TrimRight(ollamaBaseURL(), "/") + "/v1" },
		Images:     DecisionImagesBase64,
		ImageTypes: []string{"image/png", "image/jpeg", "image/webp"},
		NoAuth:     true,
		AnyModel:   true,
	},
	"typesafe": {
		Wire:     DecisionWireSystemOne,
		BaseURL:  func() string { return "https://api.typesafe.ai/v1" },
		Env:      []string{"TYPESAFE_AI_API_KEY", "TYPESAFE_API_KEY"},
		Images:   DecisionImagesMultipart,
		AnyModel: true,
	},
	"vercel": {
		Wire:    DecisionWireGateway,
		BaseURL: func() string { return defaultGatewayDecisionURL },
		Env:     []string{"VERCEL_API_KEY"},
		Images:  DecisionImagesStateParts,
	},
}

// ollamaBaseURL returns the local Ollama server URL.
func ollamaBaseURL() string {
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		return host
	}
	return "http://localhost:11434"
}

// DecisionEndpoint is everything needed to send one decision request.
type DecisionEndpoint struct {
	// Provider is the catalog provider ID.
	Provider string
	// Model is the model ID sent in the request body.
	Model string
	Wire  DecisionWire
	// URL is the full URL of the POST request.
	URL string
	// APIKey is sent as a bearer token. Empty sends no Authorization header.
	APIKey string
	Images DecisionImageMode
	// ImageTypes lists the accepted image media types.
	ImageTypes []string
	// MaxImages limits the images per request. 0 means no known limit.
	MaxImages int
	// AcceptsImages is false when the catalog says the model takes no
	// image input.
	AcceptsImages bool
	// Info is the catalog entry. Nil for models that are not in the catalog
	// (for example local Ollama models).
	Info       *ModelInfo
	HTTPClient *http.Client
}

// DecisionModelEntry is one decision model of the catalog.
type DecisionModelEntry struct {
	Provider string
	ID       string
	Info     ModelInfo
}

// DecisionModels lists the catalog's decision models, sorted by provider
// and model ID.
func (r *ModelsRegistry) DecisionModels() []DecisionModelEntry {
	var out []DecisionModelEntry
	for providerID, info := range r.providers {
		for modelID, m := range info.DecisionModels {
			out = append(out, DecisionModelEntry{Provider: providerID, ID: modelID, Info: m})
		}
	}
	slices.SortFunc(out, func(a, b DecisionModelEntry) int {
		if c := strings.Compare(a.Provider, b.Provider); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// DecisionWireFor reports the decision wire of a catalog provider and
// model. ok is false when Kit cannot tell.
func (r *ModelsRegistry) DecisionWireFor(provider, modelID string) (DecisionWire, bool) {
	provider = catalogProviderID(provider)
	if o, ok := decisionProviderOverrides[provider]; ok {
		return o.Wire, true
	}
	providerInfo := r.GetProviderInfo(provider)
	if providerInfo == nil {
		return "", false
	}
	npm := providerInfo.NPM
	if info, ok := providerInfo.DecisionModels[modelID]; ok && info.ProviderNPM != "" {
		npm = info.ProviderNPM
	}
	wire, ok := npmToDecisionWire[npm]
	return wire, ok
}

// ResolveDecisionEndpoint resolves config.ModelString to the endpoint of a
// decision model. The wire comes from a provider override, else from the
// effective catalog npm package (per-model first, then provider). The base
// URL comes from config.ProviderURL, a provider override, the per-model
// catalog URL or the provider catalog URL, in that order.
func ResolveDecisionEndpoint(config *ProviderConfig) (*DecisionEndpoint, error) {
	provider, modelName, err := ParseModelString(config.ModelString)
	if err != nil {
		return nil, err
	}
	identity, err := httpclient.ResolveUserAgent(config.UserAgent)
	if err != nil {
		return nil, err
	}
	config.UserAgent = identity

	registry := GetGlobalRegistry()
	providerID := catalogProviderID(provider)
	override, hasOverride := decisionProviderOverrides[providerID]
	providerInfo := registry.GetProviderInfo(providerID)
	info := registry.LookupDecisionModel(providerID, modelName)

	if info == nil && (!hasOverride || !override.AnyModel) {
		if registry.LookupModel(providerID, modelName) != nil {
			return nil, fmt.Errorf("%w: %s is a chat model", ErrNotDecisionModel, config.ModelString)
		}
		return nil, fmt.Errorf("%w: %s is not in the model database as a decision model (run 'kit update-models' to refresh it)", ErrNotDecisionModel, config.ModelString)
	}

	wire, ok := registry.DecisionWireFor(providerID, modelName)
	if !ok {
		npm := ""
		if providerInfo != nil {
			npm = providerInfo.NPM
		}
		if info != nil && info.ProviderNPM != "" {
			npm = info.ProviderNPM
		}
		return nil, fmt.Errorf("%w: %s (npm package %q)", ErrUnknownDecisionWire, config.ModelString, npm)
	}

	baseURL := config.ProviderURL
	if baseURL == "" && hasOverride && override.BaseURL != nil {
		baseURL = override.BaseURL()
	}
	if baseURL == "" && info != nil {
		baseURL = info.ProviderAPI
	}
	if baseURL == "" && providerInfo != nil {
		baseURL = providerInfo.API
	}
	if baseURL == "" && wire == DecisionWireGateway {
		baseURL = defaultGatewayDecisionURL
	}
	if baseURL == "" {
		return nil, fmt.Errorf("decision model %s: the model database has no API URL for provider %s", config.ModelString, providerID)
	}
	// Variables that only fill URL placeholders (for example
	// CLOUDFLARE_ACCOUNT_ID) are not API keys.
	urlVars := templateVars(baseURL)
	tmplInfo := &ProviderInfo{ID: providerID}
	if providerInfo != nil {
		tmplInfo = providerInfo
	}
	if resolved, err := resolveTemplatedAPIURL(baseURL, tmplInfo); err != nil {
		return nil, err
	} else if resolved != "" {
		baseURL = resolved
	}
	baseURL = strings.TrimRight(baseURL, "/")

	wireModel := modelName
	if info != nil && info.ID != "" {
		wireModel = info.ID
	}

	ep := &DecisionEndpoint{
		Provider:      providerID,
		Model:         wireModel,
		Wire:          wire,
		Info:          info,
		AcceptsImages: true,
		ImageTypes:    []string{"image/png", "image/jpeg"},
	}
	if info != nil {
		if supported, known := info.SupportsImageInput(); known {
			ep.AcceptsImages = supported
		}
	}

	switch wire {
	case DecisionWireSystemOne:
		ep.URL = baseURL + "/systemone"
		ep.Images = DecisionImagesMultipart
	case DecisionWireWorkersAI:
		// The URL takes the full model ID; the body takes its last segment
		// (e.g. "@cf/cloudflare/clef-flash" → "clef-flash").
		ep.URL = baseURL + "/" + wireModel
		ep.Model = path.Base(wireModel)
		ep.Images = DecisionImagesDataURL
	case DecisionWireGateway:
		ep.URL = baseURL + "/decision-model"
		ep.Images = DecisionImagesStateParts
	}
	if hasOverride {
		if override.Images != "" {
			ep.Images = override.Images
		}
		if override.ImageTypes != nil {
			ep.ImageTypes = override.ImageTypes
		}
		ep.MaxImages = override.MaxImages
	}

	if !hasOverride || !override.NoAuth {
		var env []string
		name := providerID
		if providerInfo != nil {
			env = append(env, providerInfo.Env...)
			if providerInfo.Name != "" {
				name = providerInfo.Name
			}
		}
		env = append(env, override.Env...)
		env = slices.DeleteFunc(env, func(name string) bool { return slices.Contains(urlVars, name) })
		ep.APIKey = resolveAPIKey(config.ProviderAPIKey, providerID, env)
		if ep.APIKey == "" {
			return nil, missingKeyError(providerID, name, env)
		}
	}

	client := providerHTTPClient(createHTTPClientWithTLSConfig(config.TLSSkipVerify), config)
	if providerInfo != nil {
		client = withDefaultHeaders(client, providerInfo.Headers)
	}
	ep.HTTPClient = client
	return ep, nil
}

// templateVars returns the names of the "${NAME}" placeholders in apiURL.
func templateVars(apiURL string) []string {
	var names []string
	for _, m := range templatePlaceholderRe.FindAllString(apiURL, -1) {
		names = append(names, m[2:len(m)-1])
	}
	return names
}
