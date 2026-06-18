package config

import (
	"sync"
	"testing"
)

// TestConfigPathConcurrentAccess exercises the mutex guarding the package-level
// configPath global. Run with -race to detect the data race that motivated the
// guard (concurrent kit.New() calls discovering a .kit.yml).
func TestConfigPathConcurrentAccess(t *testing.T) {
	t.Cleanup(func() { SetConfigPath("") })

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)
	for range goroutines {
		go func() {
			defer wg.Done()
			SetConfigPath("/tmp/kit.yml")
		}()
		go func() {
			defer wg.Done()
			_ = GetConfigPath()
		}()
	}
	wg.Wait()

	SetConfigPath("/tmp/final.yml")
	if got := GetConfigPath(); got != "/tmp/final.yml" {
		t.Fatalf("GetConfigPath() = %q, want /tmp/final.yml", got)
	}
}
