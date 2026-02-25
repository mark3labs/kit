package cmd

import (
	"fmt"
	"sort"

	"github.com/mark3labs/mcphost/internal/models"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models [provider]",
	Short: "List available models from the model database",
	Long: `List models known to mcphost from the catwalk database.

When run without arguments, shows all providers and their models.
When a provider name is given, shows only that provider's models.

Note: models not listed here can still be used — the database is
advisory. Run 'mcphost update-models' to refresh.

Examples:
  mcphost models
  mcphost models anthropic
  mcphost models openai`,
	Args: cobra.MaximumNArgs(1),
	RunE: runModels,
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}

func runModels(_ *cobra.Command, args []string) error {
	registry := models.GetGlobalRegistry()

	if len(args) == 1 {
		return printProvider(registry, args[0])
	}

	return printAllProviders(registry)
}

func printAllProviders(registry *models.ModelsRegistry) error {
	providerIDs := registry.GetSupportedProviders()
	sort.Strings(providerIDs)

	// Filter to providers that have models
	var withModels []string
	for _, id := range providerIDs {
		m, _ := registry.GetModelsForProvider(id)
		if len(m) > 0 {
			withModels = append(withModels, id)
		}
	}

	if len(withModels) == 0 {
		fmt.Println("No models in database. Run 'mcphost update-models' to fetch.")
		return nil
	}

	for i, id := range withModels {
		m, _ := registry.GetModelsForProvider(id)
		modelIDs := sortedModelIDs(m)

		isLast := i == len(withModels)-1
		branch := "├── "
		if isLast {
			branch = "└── "
		}
		fmt.Printf("%s%s\n", branch, id)

		childPrefix := "│   "
		if isLast {
			childPrefix = "    "
		}

		for j, modelID := range modelIDs {
			modelBranch := "├── "
			if j == len(modelIDs)-1 {
				modelBranch = "└── "
			}
			fmt.Printf("%s%s%s\n", childPrefix, modelBranch, modelID)
		}
	}

	return nil
}

func printProvider(registry *models.ModelsRegistry, provider string) error {
	m, err := registry.GetModelsForProvider(provider)
	if err != nil {
		return fmt.Errorf("unknown provider %q. Run 'mcphost models' to see all providers", provider)
	}

	if len(m) == 0 {
		fmt.Printf("No models listed for %s.\n", provider)
		return nil
	}

	modelIDs := sortedModelIDs(m)
	for _, id := range modelIDs {
		fmt.Println(id)
	}

	return nil
}

func sortedModelIDs(m map[string]models.ModelInfo) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
