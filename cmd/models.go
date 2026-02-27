package cmd

import (
	"fmt"
	"sort"

	kit "github.com/mark3labs/kit/pkg/kit"
	"github.com/spf13/cobra"
)

var modelsAllFlag bool

var modelsCmd = &cobra.Command{
	Use:   "models [provider]",
	Short: "List available models from the model database",
	Long: `List models known to kit from the models.dev database.

By default, shows only providers that kit can use (native fantasy
providers plus openai-compatible auto-routed providers). Use --all
to show every provider in the database.

When a provider name is given, shows only that provider's models.

Note: models not listed here can still be used — the database is
advisory. Run 'kit update-models' to refresh.

Examples:
  kit models
  kit models --all
  kit models anthropic
  kit models deepseek`,
	Args: cobra.MaximumNArgs(1),
	RunE: runModels,
}

func init() {
	modelsCmd.Flags().BoolVar(&modelsAllFlag, "all", false, "show all providers in the database, not just fantasy-compatible ones")
	rootCmd.AddCommand(modelsCmd)
}

func runModels(_ *cobra.Command, args []string) error {
	if len(args) == 1 {
		return printProvider(args[0])
	}

	return printAllProviders(modelsAllFlag)
}

func printAllProviders(showAll bool) error {
	var providerIDs []string
	if showAll {
		providerIDs = kit.GetSupportedProviders()
	} else {
		providerIDs = kit.GetFantasyProviders()
	}
	sort.Strings(providerIDs)

	// Filter to providers that have models
	var withModels []string
	for _, id := range providerIDs {
		m, _ := kit.GetModelsForProvider(id)
		if len(m) > 0 {
			withModels = append(withModels, id)
		}
	}

	if len(withModels) == 0 {
		fmt.Println("No models in database. Run 'kit update-models' to fetch.")
		return nil
	}

	for i, id := range withModels {
		m, _ := kit.GetModelsForProvider(id)
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

func printProvider(provider string) error {
	m, err := kit.GetModelsForProvider(provider)
	if err != nil {
		return fmt.Errorf("unknown provider %q. Run 'kit models' to see all providers", provider)
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

func sortedModelIDs(m map[string]kit.ModelInfo) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
