# Plan 02: Richer Type Exports

**Priority**: P0
**Effort**: Low
**Goal**: Export 40+ internal types so SDK users and the CLI app share the same type surface

## Background

Currently only 3 type aliases are exported: `Message`, `ToolCall`, `ToolResult`. Pi exports 50+ types. SDK users and the CLI app both need access to messages, sessions, config, agents, models, and callback types. By exporting from `pkg/kit`, both external consumers and the CLI share the same types — no parallel definitions.

## Prerequisites

- Plan 00 (Create `pkg/kit/`)

## Key Principle: Shared Types

After this plan, `cmd/` should progressively adopt types from `pkg/kit/` instead of importing from `internal/` directly. For example:
- `cmd/setup.go` should reference `kit.ProviderConfig` rather than `models.ProviderConfig`
- `cmd/root.go` session setup should use `kit.SessionInfo` rather than `session.SessionInfo`

This is a gradual migration — the type aliases make this zero-cost since `kit.ProviderConfig = models.ProviderConfig` (same underlying type).

## Step-by-Step

### Step 1: Expand `pkg/kit/types.go` with all type groups

**File**: `pkg/kit/types.go`

```go
package kit

import (
    "charm.land/fantasy"
    "github.com/mark3labs/kit/internal/agent"
    "github.com/mark3labs/kit/internal/config"
    "github.com/mark3labs/kit/internal/message"
    "github.com/mark3labs/kit/internal/models"
    "github.com/mark3labs/kit/internal/session"
)

// ==== Message Types (internal/message/content.go) ====

type Message = message.Message
type MessageRole = message.MessageRole

const (
    RoleUser      = message.RoleUser
    RoleAssistant = message.RoleAssistant
    RoleTool      = message.RoleTool
    RoleSystem    = message.RoleSystem
)

type ContentPart = message.ContentPart
type TextContent = message.TextContent
type ReasoningContent = message.ReasoningContent
type ToolCall = message.ToolCall
type ToolResult = message.ToolResult
type Finish = message.Finish

// ==== Session Types (internal/session/) ====

type Session = session.Session
type SessionMetadata = session.Metadata
type SessionManager = session.Manager
type SessionInfo = session.SessionInfo
type TreeManager = session.TreeManager
type SessionHeader = session.SessionHeader
type MessageEntry = session.MessageEntry

// ==== Config Types (internal/config/) ====

type Config = config.Config
type MCPServerConfig = config.MCPServerConfig

// ==== Agent Types (internal/agent/) ====

type AgentConfig = agent.AgentConfig
type GenerateResult = agent.GenerateWithLoopResult

type (
    ToolCallHandler          = agent.ToolCallHandler
    ToolExecutionHandler     = agent.ToolExecutionHandler
    ToolResultHandler        = agent.ToolResultHandler
    ResponseHandler          = agent.ResponseHandler
    StreamingResponseHandler = agent.StreamingResponseHandler
    ToolCallContentHandler   = agent.ToolCallContentHandler
)

// ==== Provider & Model Types (internal/models/) ====

type ProviderConfig = models.ProviderConfig
type ProviderResult = models.ProviderResult
type ModelInfo = models.ModelInfo
type ModelCost = models.Cost
type ModelLimit = models.Limit
type ProviderInfo = models.ProviderInfo
type ModelsRegistry = models.ModelsRegistry

// ==== Fantasy Types (re-exported) ====

type FantasyMessage = fantasy.Message
type FantasyUsage = fantasy.Usage
type FantasyResponse = fantasy.Response

// ==== Constructor & Helper Functions ====

var (
    NewSession       = session.NewSession
    NewSessionManager = session.NewManager
    ListSessions     = session.ListSessions
    ListAllSessions  = session.ListAllSessions
    ParseModelString = models.ParseModelString
    CreateProvider   = models.CreateProvider
    GetGlobalRegistry = models.GetGlobalRegistry
    LoadSystemPrompt = config.LoadSystemPrompt
)

// ==== Conversion Helpers ====

func ConvertToFantasyMessages(msg *Message) []fantasy.Message {
    return msg.ToFantasyMessages()
}

func ConvertFromFantasyMessage(msg fantasy.Message) Message {
    return message.FromFantasyMessage(msg)
}
```

### Step 2: App-as-Consumer — Migrate `cmd/` to use SDK types

After this plan, start migrating `cmd/` callers to use `kit.*` types. Since these are aliases, this is purely cosmetic and zero-cost, but it establishes the pattern:

**Example in `cmd/setup.go`**:
```go
// Before:
import "github.com/mark3labs/kit/internal/models"
cfg := &models.ProviderConfig{...}

// After (preferred, gradual migration):
import kit "github.com/mark3labs/kit/pkg/kit"
cfg := &kit.ProviderConfig{...}
```

This is not blocking — both work simultaneously due to Go type aliases.

### Step 3: Write a compilation test

**File**: `pkg/kit/types_test.go`

```go
package kit_test

import (
    "testing"
    kit "github.com/mark3labs/kit/pkg/kit"
)

func TestTypeExports(t *testing.T) {
    if kit.RoleUser != "user" { t.Error("RoleUser") }
    if kit.RoleAssistant != "assistant" { t.Error("RoleAssistant") }

    msg := kit.Message{
        Role: kit.RoleUser,
        Parts: []kit.ContentPart{
            kit.TextContent{Text: "hello"},
        },
    }
    if msg.Content() != "hello" { t.Error("message content") }

    s := kit.NewSession()
    if s == nil { t.Error("NewSession") }
}
```

### Step 4: Verify

```bash
go build -o output/kit ./cmd/kit
go test -race ./...
go vet ./...
```

## Files Changed Summary

| Action | File | Change |
|--------|------|--------|
| EDIT | `pkg/kit/types.go` | Add ~40 type aliases, constants, constructors |
| CREATE | `pkg/kit/types_test.go` | Compilation test |

## Verification Checklist

- [ ] `go build -o output/kit ./cmd/kit` succeeds
- [ ] `go test -race ./...` passes
- [ ] No circular import errors
- [ ] Type aliases are interchangeable with internal types
- [ ] `cmd/` can import and use `kit.*` types alongside internal types
