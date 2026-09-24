package models

import (
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/spf13/viper"
)

// lazyProbeEnv makes TestGlobalRegistryLazyProbe run its assertions; it is
// set only in the child process spawned by TestGlobalRegistryNotBuiltAtInit.
const lazyProbeEnv = "KIT_TEST_LAZY_REGISTRY_PROBE"

// TestGlobalRegistryNotBuiltAtInit guards the startup cost for SDK embedders:
// importing the package must not decode the models database. It runs the
// probe in a fresh process because any earlier test in this binary may
// already have built the registry.
func TestGlobalRegistryNotBuiltAtInit(t *testing.T) {
	if os.Getenv(lazyProbeEnv) != "" {
		t.Skip("running as probe child")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestGlobalRegistryLazyProbe$", "-test.v")
	cmd.Env = append(os.Environ(), lazyProbeEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("probe failed: %v\n%s", err, out)
	}
}

// TestGlobalRegistryLazyProbe is the child half of
// TestGlobalRegistryNotBuiltAtInit; it is a no-op when run directly.
func TestGlobalRegistryLazyProbe(t *testing.T) {
	if os.Getenv(lazyProbeEnv) == "" {
		t.Skip("only meaningful in the probe child process")
	}
	if globalRegistry.Load() != nil {
		t.Fatal("global registry was built during package initialization")
	}
	r := GetGlobalRegistry()
	if r == nil {
		t.Fatal("GetGlobalRegistry returned nil")
	}
	if len(r.providers) == 0 {
		t.Fatal("lazily built registry has no providers")
	}
	if GetGlobalRegistry() != r {
		t.Fatal("GetGlobalRegistry built the registry more than once")
	}
}

// reloadProbeEnv makes TestReloadGlobalRegistryPicksUpConfig run its
// assertions; it is set only in the child process that the test spawns.
const reloadProbeEnv = "KIT_TEST_RELOAD_REGISTRY_PROBE"

// TestReloadGlobalRegistryPicksUpConfig covers the CLI ordering in a cold
// process: config is loaded after package init, then the registry is
// reloaded before anything calls GetGlobalRegistry. The first getter call
// must keep that config-aware registry, and lookups must see custom models
// from the config. It runs in a child process so the global registry is not
// yet built and so the Viper override does not leak into other tests.
func TestReloadGlobalRegistryPicksUpConfig(t *testing.T) {
	if os.Getenv(reloadProbeEnv) == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestReloadGlobalRegistryPicksUpConfig$", "-test.v")
		cmd.Env = append(os.Environ(), reloadProbeEnv+"=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("probe failed: %v\n%s", err, out)
		}
		return
	}

	if globalRegistry.Load() != nil {
		t.Fatal("global registry was built before the probe started")
	}
	viper.Set("customModels", map[string]any{
		"lazy-registry-test-model": map[string]any{
			"name":  "Lazy Registry Test",
			"limit": map[string]any{"context": 8192, "output": 1024},
		},
	})

	ReloadGlobalRegistry()
	installed := globalRegistry.Load()
	if installed == nil {
		t.Fatal("ReloadGlobalRegistry did not install a registry")
	}
	if got := GetGlobalRegistry(); got != installed {
		t.Fatal("first GetGlobalRegistry replaced the registry installed by ReloadGlobalRegistry")
	}
	if installed.LookupModel("custom", "lazy-registry-test-model") == nil {
		t.Fatal("custom model from config not visible after ReloadGlobalRegistry")
	}
}

// TestGlobalRegistryConcurrentAccess must pass under -race: readers and
// reloads may overlap, e.g. a model lookup during a cache update.
func TestGlobalRegistryConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			if i%4 == 0 {
				ReloadGlobalRegistry()
				return
			}
			if GetGlobalRegistry() == nil {
				t.Error("GetGlobalRegistry returned nil")
			}
		})
	}
	wg.Wait()
}
