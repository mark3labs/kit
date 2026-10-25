package models

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/kit/internal/httpclient"
)

const defaultModelsURL = "https://models.dev/api.json?type=all"

// UpdateModelsOptions selects the catalog source. Empty uses KIT_MODELS_URL or models.dev.
type UpdateModelsOptions struct {
	Source string
}

// UpdateModelsStatus describes the committed update outcome.
type UpdateModelsStatus string

const (
	// UpdateModelsStatusUpdated means a new catalog was stored.
	UpdateModelsStatusUpdated UpdateModelsStatus = "updated"
	// UpdateModelsStatusNotModified means the server confirmed the cached catalog.
	UpdateModelsStatusNotModified UpdateModelsStatus = "not_modified"
	// UpdateModelsStatusReset means the cache was removed (or was already absent).
	UpdateModelsStatusReset UpdateModelsStatus = "reset"
)

// UpdateModelsResult reports imported catalog counts, not merged registry counts.
// Counts are zero for reset, where they are not applicable.
type UpdateModelsResult struct {
	Status        UpdateModelsStatus
	ProviderCount int
	ModelCount    int
}

// ModelsUpdateSource is a resolved source for the internal updater and CLI.
type ModelsUpdateSource struct {
	Location string
	HTTP     bool
	Reset    bool
}

// ResolveModelsUpdateSource applies source precedence without accessing the network.
func ResolveModelsUpdateSource(source string) (ModelsUpdateSource, error) {
	if source == "embedded" {
		return ModelsUpdateSource{Reset: true}, nil
	}
	network := strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
	if source == "" {
		source = os.Getenv("KIT_MODELS_URL")
		if source == "" {
			source = defaultModelsURL
		}
		network = true
	}
	if network {
		u, err := url.Parse(source)
		if err != nil {
			return ModelsUpdateSource{}, fmt.Errorf("invalid models URL: %w", err)
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
			return ModelsUpdateSource{}, fmt.Errorf("models URL must use HTTP(S) and have a host")
		}
	}
	return ModelsUpdateSource{Location: source, HTTP: network}, nil
}

func modelsUpdateContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 30*time.Second)
}

// UpdateModels updates shared catalog storage without loading or refreshing a registry.
// Filesystem syscalls are not interruptible; cancellation after commit cannot undo it.
func UpdateModels(ctx context.Context, opts UpdateModelsOptions) (UpdateModelsResult, error) {
	if ctx == nil {
		return UpdateModelsResult{}, fmt.Errorf("models update requires a context")
	}
	if err := ctx.Err(); err != nil {
		return UpdateModelsResult{}, err
	}
	source, err := ResolveModelsUpdateSource(opts.Source)
	if err != nil {
		return UpdateModelsResult{}, err
	}
	if source.Reset {
		if err := removeCachedProviders(ctx); err != nil {
			return UpdateModelsResult{}, fmt.Errorf("failed to remove cache: %w", err)
		}
		return UpdateModelsResult{Status: UpdateModelsStatusReset}, nil
	}
	if source.HTTP {
		ctx, cancel := modelsUpdateContext(ctx)
		defer cancel()
		return fetchModels(ctx, source.Location)
	}
	data, err := os.ReadFile(source.Location)
	if err != nil {
		return UpdateModelsResult{}, fmt.Errorf("failed to read file: %w", err)
	}
	return importModels(ctx, data, "", "")
}

func sourceFingerprint(source string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
}

func fetchModels(ctx context.Context, source string) (UpdateModelsResult, error) {
	cached := loadCachedEnvelope()
	fingerprint := sourceFingerprint(source)
	conditional := cached.ETag != "" && cached.SourceFingerprint == fingerprint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return UpdateModelsResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	if conditional {
		req.Header.Set("If-None-Match", cached.ETag)
	}
	client := httpclient.WithUserAgent(nil, "")
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		if req.URL.String() != source {
			req.Header.Del("If-None-Match")
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return UpdateModelsResult{}, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := ctx.Err(); err != nil {
		return UpdateModelsResult{}, err
	}
	if resp.StatusCode == http.StatusNotModified {
		if !conditional || resp.Request.Header.Get("If-None-Match") == "" {
			return UpdateModelsResult{}, fmt.Errorf("unsolicited models HTTP 304")
		}
		return catalogResult(UpdateModelsStatusNotModified, cached.Providers), nil
	}
	if resp.StatusCode != http.StatusOK {
		return UpdateModelsResult{}, fmt.Errorf("unexpected models HTTP status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return UpdateModelsResult{}, fmt.Errorf("failed to read response: %w", err)
	}
	// A redirected resource's validator does not belong to the original URL.
	etag := resp.Header.Get("ETag")
	if resp.Request.URL.String() != source {
		etag = ""
	}
	return importModels(ctx, data, etag, fingerprint)
}

func importModels(ctx context.Context, data []byte, etag, fingerprint string) (UpdateModelsResult, error) {
	providers, err := decodeModels(data)
	if err != nil {
		return UpdateModelsResult{}, err
	}
	if err := storeCachedEnvelope(ctx, cacheEnvelope{Version: cacheSchemaVersion, Providers: providers, ETag: etag, SourceFingerprint: fingerprint}); err != nil {
		return UpdateModelsResult{}, fmt.Errorf("failed to cache providers: %w", err)
	}
	return catalogResult(UpdateModelsStatusUpdated, providers), nil
}

func catalogResult(status UpdateModelsStatus, providers ModelsDBProviders) UpdateModelsResult {
	result := UpdateModelsResult{Status: status, ProviderCount: len(providers)}
	for _, p := range providers {
		result.ModelCount += len(p.Models)
	}
	return result
}

func validateCatalog(providers ModelsDBProviders) error {
	for key, p := range providers {
		if strings.TrimSpace(key) == "" || p.Models == nil {
			return fmt.Errorf("invalid provider %q: key and model map are required", key)
		}
		for model := range p.Models {
			if strings.TrimSpace(model) == "" {
				return fmt.Errorf("invalid blank model key in provider %q", key)
			}
		}
	}
	if catalogResult("", providers).ModelCount == 0 {
		return fmt.Errorf("model data contains no models")
	}
	return nil
}

func decodeModels(data []byte) (ModelsDBProviders, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("failed to parse model data: %w", err)
	}
	// Envelope metadata is reserved at the top level, so it cannot become providers.
	if _, ok := object["providers"]; ok {
		var env cacheEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, fmt.Errorf("failed to parse cache envelope: %w", err)
		}
		if env.Version != cacheSchemaVersion {
			return nil, fmt.Errorf("unsupported model cache version %d", env.Version)
		}
		data = object["providers"]
	} else if object["version"] != nil || object["etag"] != nil || object["source_fingerprint"] != nil {
		return nil, fmt.Errorf("model cache envelope is missing providers")
	}
	// Check null provider/model objects before decoding into value structs.
	var raw map[string]*struct {
		Models map[string]json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse providers: %w", err)
	}
	for key, p := range raw {
		if p == nil {
			return nil, fmt.Errorf("null provider %q", key)
		}
		for key, model := range p.Models {
			if strings.TrimSpace(string(model)) == "null" {
				return nil, fmt.Errorf("null model %q", key)
			}
		}
	}
	var providers ModelsDBProviders
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, fmt.Errorf("failed to parse providers: %w", err)
	}
	if err := validateCatalog(providers); err != nil {
		return nil, err
	}
	return providers, nil
}
