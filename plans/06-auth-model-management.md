# Plan 06: Auth & Model Management APIs

**Priority**: P2
**Effort**: Medium
**Goal**: Expose provider management, model validation, and API key handling in the SDK; CLI auth commands consume SDK APIs

## Background

Pi exports `AuthStorage`, `ModelRegistry`, `SettingsManager` for programmatic auth/model management. Kit has this internally (`internal/models/registry.go`, `internal/auth/credentials.go`, `internal/models/providers.go`) but none is exposed through the SDK.

## Prerequisites

- Plan 00 (Create `pkg/kit/`)
- Plan 02 (Richer type exports)

## Step-by-Step

### Step 1: Export model registry functions

**File**: `pkg/kit/models.go` (new)

```go
package kit

import (
    "fmt"
    "github.com/mark3labs/kit/internal/models"
)

// LookupModel returns information about a model, or nil if unknown.
func LookupModel(provider, modelID string) *ModelInfo {
    return models.GetGlobalRegistry().LookupModel(provider, modelID)
}

// GetSupportedProviders returns all known provider names.
func GetSupportedProviders() []string {
    return models.GetGlobalRegistry().GetSupportedProviders()
}

// GetModelsForProvider returns all known models for a provider.
func GetModelsForProvider(provider string) (map[string]ModelInfo, error) {
    return models.GetGlobalRegistry().GetModelsForProvider(provider)
}

// GetProviderInfo returns information about a provider (env vars, API URL, etc.).
func GetProviderInfo(provider string) *ProviderInfo {
    return models.GetGlobalRegistry().GetProviderInfo(provider)
}

// ValidateEnvironment checks if required API keys are set for a provider.
func ValidateEnvironment(provider string, apiKey string) error {
    return models.GetGlobalRegistry().ValidateEnvironment(provider, apiKey)
}

// SuggestModels returns model names similar to an invalid model string.
func SuggestModels(provider, invalidModel string) []string {
    return models.GetGlobalRegistry().SuggestModels(provider, invalidModel)
}

// RefreshModelRegistry reloads the model database from models.dev.
func RefreshModelRegistry() {
    models.ReloadGlobalRegistry()
}

// ParseModelString splits a "provider/model" string into components.
func ParseModelString(modelString string) (provider, model string, err error) {
    return models.ParseModelString(modelString)
}

// CheckProviderReady validates that a provider is properly configured.
func CheckProviderReady(provider string) error {
    info := models.GetGlobalRegistry().GetProviderInfo(provider)
    if info == nil {
        return fmt.Errorf("unknown provider: %s", provider)
    }
    return models.GetGlobalRegistry().ValidateEnvironment(provider, "")
}
```

### Step 2: Add model info to Kit instance

**File**: `pkg/kit/kit.go`

```go
// GetModel returns the current model string (e.g., "anthropic/claude-sonnet-4-5-20250929").
func (m *Kit) GetModel() string {
    return m.modelString
}

// GetModelInfo returns detailed information about the current model.
// Returns nil if the model is not in the registry.
func (m *Kit) GetModelInfo() *ModelInfo {
    provider, modelID, err := models.ParseModelString(m.modelString)
    if err != nil {
        return nil
    }
    return models.GetGlobalRegistry().LookupModel(provider, modelID)
}
```

### Step 3: Export auth credential management

**File**: `pkg/kit/auth.go` (new)

```go
package kit

import "github.com/mark3labs/kit/internal/auth"

// CredentialManager manages API keys and OAuth credentials.
type CredentialManager = auth.CredentialManager

// NewCredentialManager creates a credential manager.
func NewCredentialManager() (*CredentialManager, error) {
    return auth.NewCredentialManager()
}

// HasAnthropicCredentials checks if Anthropic credentials are stored.
func HasAnthropicCredentials() bool {
    cm, err := auth.NewCredentialManager()
    if err != nil {
        return false
    }
    return cm.GetAnthropicCredentials() != nil
}

// GetAnthropicAPIKey resolves the Anthropic API key using the standard
// resolution order: stored credentials -> ANTHROPIC_API_KEY env var.
func GetAnthropicAPIKey() string {
    key, err := auth.GetAnthropicAPIKey("")
    if err != nil {
        return ""
    }
    return key
}
```

### Step 4: App-as-Consumer — CLI commands use SDK APIs

Currently CLI commands like `kit models`, `kit update-models`, and provider validation logic directly import `internal/models` and `internal/auth`. They should use `pkg/kit` functions instead.

**File**: `cmd/root.go` or wherever model validation happens

```go
// Before:
import "github.com/mark3labs/kit/internal/models"
registry := models.GetGlobalRegistry()
info := registry.LookupModel(provider, model)

// After:
import kit "github.com/mark3labs/kit/pkg/kit"
info := kit.LookupModel(provider, model)
```

**File**: `cmd/` auth-related commands

```go
// Before:
import "github.com/mark3labs/kit/internal/auth"
cm, _ := auth.NewCredentialManager()

// After:
import kit "github.com/mark3labs/kit/pkg/kit"
cm, _ := kit.NewCredentialManager()
```

Since these are type aliases, existing code continues to work during gradual migration.

### Step 5: Write tests and verify

```bash
go build -o output/kit ./cmd/kit
go test -race ./...
go vet ./...
```

## Files Changed Summary

| Action | File | Change |
|--------|------|--------|
| CREATE | `pkg/kit/models.go` | Model registry, parsing, validation, suggestions |
| CREATE | `pkg/kit/auth.go` | Credential management exports |
| EDIT | `pkg/kit/kit.go` | Add GetModel(), GetModelInfo() |
| EDIT | `cmd/` | Migrate to use pkg/kit functions (gradual) |

## Verification Checklist

- [ ] `ParseModelString` handles "provider/model" format
- [ ] `GetSupportedProviders` returns provider list
- [ ] `LookupModel` returns info for known models
- [ ] `CheckProviderReady` gives clear error messages
- [ ] CLI commands use SDK functions instead of internal imports
