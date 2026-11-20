package cmd

import (
	"testing"

	"github.com/mark3labs/kit/internal/models"
)

// TestDefaultModelsAreCurrent checks that the CLI's built-in default models
// are in the model catalog and not marked deprecated, so a default never
// points at a model the provider has scheduled for removal.
func TestDefaultModelsAreCurrent(t *testing.T) {
	defaults := map[string]string{
		"--model flag":             rootCmd.PersistentFlags().Lookup("model").DefValue,
		"GitHub workflow":          defaultGitHubModel,
		"auth login --set-default": defaultModels["anthropic"],
	}
	for name, model := range defaults {
		provider, modelID, err := models.ParseModelString(model)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		info := models.GetGlobalRegistry().LookupModel(provider, modelID)
		if info == nil {
			t.Errorf("%s: %q is not in the model catalog", name, model)
			continue
		}
		if info.Status == models.StatusDeprecated {
			t.Errorf("%s: %q is deprecated", name, model)
		}
	}
}
