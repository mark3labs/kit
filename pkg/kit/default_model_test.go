package kit

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/mark3labs/kit/internal/models"
)

// TestSDKDefaultModelIsCurrent checks that the SDK's default model is in the
// model catalog and not marked deprecated.
func TestSDKDefaultModelIsCurrent(t *testing.T) {
	v := viper.New()
	setSDKDefaults(v)
	model := v.GetString("model")

	provider, modelID, err := models.ParseModelString(model)
	if err != nil {
		t.Fatalf("default model %q: %v", model, err)
	}
	info := models.GetGlobalRegistry().LookupModel(provider, modelID)
	if info == nil {
		t.Fatalf("default model %q is not in the model catalog", model)
	}
	if info.Status == models.StatusDeprecated {
		t.Errorf("default model %q is deprecated", model)
	}
}
