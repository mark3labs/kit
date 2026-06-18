//go:build testing

package kit

import "github.com/mark3labs/kit/internal/config"

// ResetForTesting clears package-global state that survives across tests in
// the same binary. It is intended for test-binary teardown / between-test
// cleanup. Safe to call concurrently with no in-flight kit.New() calls.
//
// This function is only compiled under the "testing" build tag so it never
// ships in production binaries.
func ResetForTesting() {
	config.SetConfigPath("")
}
