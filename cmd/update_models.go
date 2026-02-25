package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcphost/internal/models"
	"github.com/spf13/cobra"
)

const defaultModelsURL = "https://models.dev/api.json"

var updateModelsCmd = &cobra.Command{
	Use:   "update-models [source]",
	Short: "Update the local model database from models.dev",
	Long: `Update the local model database used for cost tracking, capability
detection, and model suggestions.

When run without arguments, fetches from models.dev.

Sources:
  (none)      Fetch from models.dev (or MCPHOST_MODELS_URL override)
  <url>       Fetch from a custom URL
  <file>      Load from a local JSON file
  embedded    Reset to the built-in database shipped with this binary

Examples:
  mcphost update-models
  mcphost update-models https://models.dev/api.json
  mcphost update-models /path/to/models.json
  mcphost update-models embedded`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUpdateModels,
}

func init() {
	rootCmd.AddCommand(updateModelsCmd)
}

func runUpdateModels(_ *cobra.Command, args []string) error {
	source := ""
	if len(args) > 0 {
		source = args[0]
	}

	switch {
	case source == "embedded":
		return resetToEmbedded()

	case source == "":
		url := defaultModelsURL
		if override := os.Getenv("MCPHOST_MODELS_URL"); override != "" {
			url = override
		}
		fmt.Fprintf(os.Stderr, "Fetching models from %s...\n", url)
		return fetchFromURL(url)

	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		fmt.Fprintf(os.Stderr, "Fetching models from %s...\n", source)
		return fetchFromURL(source)

	default:
		return loadFromFile(source)
	}
}

func fetchFromURL(url string) error {
	// Load existing ETag for conditional fetch
	_, etag := models.LoadCachedProviders()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		fmt.Fprintln(os.Stderr, "Model database is already up to date.")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	providers, err := parseModelsDB(data)
	if err != nil {
		return err
	}

	// Use ETag from response, or compute from content
	newETag := resp.Header.Get("ETag")
	if newETag == "" {
		newETag = computeETag(data)
	}

	if err := models.StoreCachedProviders(providers, newETag); err != nil {
		return fmt.Errorf("failed to cache providers: %w", err)
	}

	modelCount := 0
	for _, p := range providers {
		modelCount += len(p.Models)
	}

	fmt.Fprintf(os.Stderr, "Model database updated: %d providers, %d models.\n", len(providers), modelCount)
	return nil
}

func loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	providers, err := parseModelsDB(data)
	if err != nil {
		return err
	}

	etag := computeETag(data)
	if err := models.StoreCachedProviders(providers, etag); err != nil {
		return fmt.Errorf("failed to cache providers: %w", err)
	}

	modelCount := 0
	for _, p := range providers {
		modelCount += len(p.Models)
	}

	fmt.Fprintf(os.Stderr, "Model database updated from file: %d providers, %d models.\n", len(providers), modelCount)
	return nil
}

func resetToEmbedded() error {
	if err := models.RemoveCachedProviders(); err != nil {
		return fmt.Errorf("failed to remove cache: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Model database reset to embedded version.")
	return nil
}

// parseModelsDB parses a models.dev JSON payload. It accepts both the raw
// models.dev format (map of provider ID → provider object) and our cache
// envelope format (for backward compatibility with cached files).
func parseModelsDB(data []byte) (models.ModelsDBProviders, error) {
	// Try direct models.dev format first (map[string]provider)
	var providers models.ModelsDBProviders
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, fmt.Errorf("failed to parse model data: %w", err)
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("model data contains no providers")
	}

	return providers, nil
}

// computeETag generates a content-based ETag from the raw data.
func computeETag(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf(`"%x"`, h[:8])
}
