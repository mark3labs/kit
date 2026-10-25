package cmd

import (
	"fmt"
	"os"

	"github.com/mark3labs/kit/internal/models"
	"github.com/spf13/cobra"
)

var updateModelsCmd = &cobra.Command{
	Use:   "update-models [source]",
	Short: "Update the local model database from models.dev",
	Long: `Update the local model database used for cost tracking, capability
detection, and model suggestions.

When run without arguments, fetches from models.dev.

Sources:
  (none)      Fetch from models.dev (or KIT_MODELS_URL override)
  <url>       Fetch from a custom URL
  <file>      Load from a local JSON file
  embedded    Reset to the built-in database shipped with this binary

Examples:
  kit update-models
  kit update-models "https://models.dev/api.json?type=all"
  kit update-models /path/to/models.json
  kit update-models embedded`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUpdateModels,
}

func init() {
	rootCmd.AddCommand(updateModelsCmd)
}

func runUpdateModels(cmd *cobra.Command, args []string) error {
	opts := models.UpdateModelsOptions{}
	if len(args) > 0 {
		opts.Source = args[0]
	}
	source, err := models.ResolveModelsUpdateSource(opts.Source)
	if err != nil {
		return err
	}
	if source.HTTP {
		fmt.Fprintf(os.Stderr, "Fetching models from %s...\n", source.Location)
	}
	result, err := models.UpdateModels(cmd.Context(), opts)
	if err != nil {
		return err
	}
	switch result.Status {
	case models.UpdateModelsStatusReset:
		fmt.Fprintln(os.Stderr, "Model database reset to embedded version.")
	case models.UpdateModelsStatusNotModified:
		fmt.Fprintln(os.Stderr, "Model database is already up to date.")
	case models.UpdateModelsStatusUpdated:
		if source.HTTP {
			fmt.Fprintf(os.Stderr, "Model database updated: %d providers, %d models.\n", result.ProviderCount, result.ModelCount)
		} else {
			fmt.Fprintf(os.Stderr, "Model database updated from file: %d providers, %d models.\n", result.ProviderCount, result.ModelCount)
		}
	}
	return nil
}
