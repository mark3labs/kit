package cmd

import (
	"fmt"
	"sort"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/kitsetup"
	"github.com/mark3labs/kit/internal/models"
	kit "github.com/mark3labs/kit/pkg/kit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var modelsAllFlag bool
var modelsExtensionsFlag bool

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
	modelsCmd.Flags().BoolVar(&modelsExtensionsFlag, "extensions", false, "execute trusted extension Init code and include its declared models")
	rootCmd.AddCommand(modelsCmd)
}

func runModels(_ *cobra.Command, args []string) error {
	if modelsExtensionsFlag && !viper.GetBool("no-extensions") {
		loaded, err := extensions.LoadExtensionsScoped(viper.GetStringSlice("extension"), bareFlag)
		if err != nil {
			return err
		}
		if _, err := kitsetup.PrepareProviderFactories(loaded, nil); err != nil {
			return err
		}
		entries := kitsetup.PrepareProviderResolver(loaded).List()
		if len(args) == 1 {
			if items, ok := entries[args[0]]; ok {
				for _, id := range sortedModelIDs(items) {
					fmt.Println(id)
				}
				return nil
			}
		}
		if len(args) == 0 {
			providers := make([]string, 0, len(entries))
			for provider := range entries {
				providers = append(providers, provider)
			}
			sort.Strings(providers)
			for _, provider := range providers {
				for _, id := range sortedModelIDs(entries[provider]) {
					fmt.Printf("%s/%s\\n", provider, id)
				}
			}
		}
	}
	if len(args) == 1 {
		return printProvider(args[0])
	}

	return printAllProviders(modelsAllFlag)
}

func printAllProviders(showAll bool) error {
	// Reload the registry to pick up any custom models from config
	models.ReloadGlobalRegistry()

	var providerIDs []string
	if showAll {
		providerIDs = kit.GetSupportedProviders()
	} else {
		providerIDs = kit.GetLLMProviders()
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
	// Reload the registry to pick up any custom models from config
	models.ReloadGlobalRegistry()

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
