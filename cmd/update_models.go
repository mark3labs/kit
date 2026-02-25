package cmd

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"

	"github.com/mark3labs/mcphost/internal/models"
	"github.com/spf13/cobra"
)

const defaultCatwalkURL = "https://catwalk.charm.sh"

var updateModelsCmd = &cobra.Command{
	Use:   "update-models [source]",
	Short: "Update the local model database from a catwalk server",
	Long: `Update the local model database used for cost tracking, capability
detection, and model suggestions.

When run without arguments, fetches from the default catwalk server
(https://catwalk.charm.sh). Override with CATWALK_URL env var.

Sources:
  (none)      Fetch from default catwalk server (or CATWALK_URL)
  <url>       Fetch from a custom catwalk server
  <file>      Load from a local JSON file
  embedded    Reset to the built-in database shipped with this binary

Examples:
  mcphost update-models
  mcphost update-models https://catwalk.example.com
  mcphost update-models /path/to/providers.json
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
		// Default: fetch from CATWALK_URL or the public Charm server
		url := cmp.Or(os.Getenv("CATWALK_URL"), defaultCatwalkURL)
		fmt.Fprintf(os.Stderr, "Fetching models from %s...\n", url)
		return fetchFromServer(catwalk.NewWithURL(url))

	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		fmt.Fprintf(os.Stderr, "Fetching models from %s...\n", source)
		return fetchFromServer(catwalk.NewWithURL(source))

	default:
		return loadFromFile(source)
	}
}

func fetchFromServer(client *catwalk.Client) error {
	// Load existing ETag for conditional fetch
	_, etag := models.LoadCachedProviders()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	providers, err := client.GetProviders(ctx, etag)
	if err != nil {
		if errors.Is(err, catwalk.ErrNotModified) {
			fmt.Fprintln(os.Stderr, "Model database is already up to date.")
			return nil
		}
		return fmt.Errorf("failed to fetch providers: %w", err)
	}

	// Compute new ETag from the fetched data
	data, err := json.Marshal(providers)
	if err != nil {
		return fmt.Errorf("failed to marshal providers: %w", err)
	}
	newETag := catwalk.Etag(data)

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

	var providers []catwalk.Provider
	if err := json.Unmarshal(data, &providers); err != nil {
		return fmt.Errorf("failed to parse provider data: %w", err)
	}

	if len(providers) == 0 {
		return fmt.Errorf("file contains no provider data")
	}

	etag := catwalk.Etag(data)
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
