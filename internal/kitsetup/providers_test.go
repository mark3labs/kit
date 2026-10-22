package kitsetup

import (
	"context"
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/models"
)

func TestProviderFactoryCollisions(t *testing.T) {
	factory := func(context.Context, *models.ProviderConfig, string) (*models.ProviderResult, error) { return nil, nil }
	for _, id := range []string{"openai", "anthropic", "copilot", "ollama"} {
		t.Run(id, func(t *testing.T) {
			_, err := PrepareProviderFactories([]extensions.LoadedExtension{{Path: "owner.go", Providers: []extensions.ProviderDef{{ID: id}}}}, nil)
			if err == nil {
				t.Fatal("reserved provider accepted")
			}
		})
	}
	if _, err := PrepareProviderFactories([]extensions.LoadedExtension{{Path: "owner.go", Providers: []extensions.ProviderDef{{ID: "local"}}}}, map[string]models.ProviderFactory{"local": factory}); err == nil {
		t.Fatal("instance collision accepted")
	}
	if err := models.RegisterProviderFactory("extension-collision-test", factory); err != nil {
		t.Fatal(err)
	}
	defer models.UnregisterProviderFactory("extension-collision-test")
	if _, err := PrepareProviderFactories([]extensions.LoadedExtension{{Path: "owner.go", Providers: []extensions.ProviderDef{{ID: "extension-collision-test"}}}}, nil); err == nil {
		t.Fatal("global collision accepted")
	}
	if _, err := PrepareProviderFactories([]extensions.LoadedExtension{{Path: "b.go", Providers: []extensions.ProviderDef{{ID: "fake"}}}, {Path: "a.go", Providers: []extensions.ProviderDef{{ID: "fake"}}}}, nil); err == nil {
		t.Fatal("cross-owner collision accepted")
	}
}
