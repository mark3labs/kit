# Plan 04: Enhanced Session Management

**Priority**: P1
**Effort**: High
**Goal**: Expose session management in the SDK; CLI session flags map to SDK options

## Background

Kit has rich session infrastructure internally (`store.go`, `tree_manager.go`) but none of it is in the SDK. The CLI handles sessions in `cmd/root.go:479-557` with flags like `--continue`, `--resume`, `--session`, `--no-session`. After this plan, both the CLI and external users configure sessions through `kit.Options`.

## Prerequisites

- Plan 00 (Create `pkg/kit/`)
- Plan 02 (Richer type exports)

## Key Principle

The CLI should NOT have its own session setup logic. Instead:
1. CLI parses `--continue`, `--session`, etc. into `kit.Options` fields
2. `kit.New()` handles all session initialization
3. The CLI gets back a `*Kit` with the session already configured

## Step-by-Step

### Step 1: Add session options to Kit Options

**File**: `pkg/kit/kit.go`

```go
type Options struct {
    // ... existing fields (Model, SystemPrompt, ConfigFile, etc.) ...

    // Session configuration
    SessionDir  string // Base directory for session discovery (default: cwd)
    SessionPath string // Open a specific session file
    Continue    bool   // Continue most recent session for SessionDir
    NoSession   bool   // Ephemeral mode — no persistence
}
```

### Step 2: Add tree session to Kit struct

```go
type Kit struct {
    agent       *agent.Agent
    sessionMgr  *session.Manager
    treeSession *session.TreeManager
    modelString string
    events      *eventBus
}
```

### Step 3: Initialize tree session in New()

```go
func New(ctx context.Context, opts *Options) (*Kit, error) {
    // ... existing config + agent setup ...

    cwd, _ := os.Getwd()
    sessionDir := cwd
    if opts != nil && opts.SessionDir != "" {
        sessionDir = opts.SessionDir
    }

    var treeSession *session.TreeManager
    if opts != nil && opts.NoSession {
        treeSession = session.InMemoryTreeSession(sessionDir)
    } else if opts != nil && opts.Continue {
        ts, err := session.ContinueRecent(sessionDir)
        if err != nil {
            ts, err = session.CreateTreeSession(sessionDir)
            if err != nil {
                return nil, fmt.Errorf("failed to create session: %w", err)
            }
        }
        treeSession = ts
    } else if opts != nil && opts.SessionPath != "" {
        ts, err := session.OpenTreeSession(opts.SessionPath)
        if err != nil {
            return nil, fmt.Errorf("failed to open session: %w", err)
        }
        treeSession = ts
    } else {
        ts, err := session.CreateTreeSession(sessionDir)
        if err != nil {
            return nil, fmt.Errorf("failed to create session: %w", err)
        }
        treeSession = ts
    }

    return &Kit{
        agent:       setupResult.Agent,
        sessionMgr:  sessionMgr,
        treeSession: treeSession,
        modelString: modelString,
        events:      newEventBus(),
    }, nil
}
```

### Step 4: Wire Prompt() to use tree session

```go
func (m *Kit) Prompt(ctx context.Context, message string) (string, error) {
    var messages []fantasy.Message
    if m.treeSession != nil {
        msgs, _, _ := m.treeSession.BuildContext()
        messages = msgs
    } else {
        messages = m.sessionMgr.GetMessages()
    }

    // ... generation ...

    // Persist to tree session
    if m.treeSession != nil {
        m.treeSession.AppendFantasyMessage(userMsg)
        for _, msg := range result.Messages {
            m.treeSession.AppendMessage(msg)
        }
    }

    // Keep legacy manager in sync
    _ = m.sessionMgr.ReplaceAllMessages(result.ConversationMessages)
    return response, nil
}
```

### Step 5: Add session management methods

**File**: `pkg/kit/sessions.go` (new)

```go
package kit

import (
    "fmt"
    "os"
    "github.com/mark3labs/kit/internal/session"
)

// Package-level session operations (don't require a Kit instance)

func ListSessions(dir string) ([]SessionInfo, error) {
    if dir == "" {
        var err error
        dir, err = os.Getwd()
        if err != nil { return nil, err }
    }
    return session.ListSessions(dir)
}

func ListAllSessions() ([]SessionInfo, error) {
    return session.ListAllSessions()
}

func DeleteSession(path string) error {
    return session.DeleteSession(path)
}

// Instance methods

func (m *Kit) GetTreeSession() *TreeManager { return m.treeSession }

func (m *Kit) GetSessionPath() string {
    if m.treeSession != nil { return m.treeSession.GetFilePath() }
    return ""
}

func (m *Kit) GetSessionID() string {
    if m.treeSession != nil { return m.treeSession.GetSessionID() }
    return ""
}

func (m *Kit) Branch(entryID string) error {
    if m.treeSession == nil {
        return fmt.Errorf("branching requires tree session")
    }
    m.treeSession.Branch(entryID)
    msgs, _, _ := m.treeSession.BuildContext()
    return m.sessionMgr.ReplaceAllMessages(msgs)
}

func (m *Kit) SetSessionName(name string) error {
    if m.treeSession == nil {
        return fmt.Errorf("session naming requires tree session")
    }
    m.treeSession.AppendSessionInfo(name)
    return nil
}

func (m *Kit) ClearSession() {
    m.sessionMgr = session.NewManager("")
    if m.treeSession != nil {
        m.treeSession.ResetLeaf()
    }
}
```

### Step 6: App-as-Consumer — CLI delegates session setup to SDK

This is the critical step. Currently `cmd/root.go:479-557` has its own session setup logic with if/else chains for each flag. Replace it with `kit.Options`:

**File**: `cmd/root.go` (migration)

```go
// Before (cmd/root.go:479-557):
// Complex if/else chain checking noSessionFlag, continueFlag, resumeFlag, sessionPath

// After:
import kit "github.com/mark3labs/kit/pkg/kit"

func buildKitOptions() *kit.Options {
    opts := &kit.Options{
        Model:       modelFlag,
        ConfigFile:  configFile,
        Quiet:       quietFlag,
    }

    // Map CLI flags to SDK options
    if noSessionFlag {
        opts.NoSession = true
    } else if continueFlag {
        opts.Continue = true
    } else if sessionPath != "" {
        opts.SessionPath = sessionPath
    }
    // resumeFlag: handled by listing sessions then picking one
    // (call kit.ListSessions first, then set opts.SessionPath)

    return opts
}

// The Kit instance handles all session init internally:
k, err := kit.New(ctx, buildKitOptions())
```

**For --resume** (currently half-implemented with a TODO for TUI picker):
```go
if resumeFlag {
    sessions, err := kit.ListSessions("")
    if err != nil || len(sessions) == 0 {
        // Fall back to new session
    } else {
        // TODO: Show TUI picker. For now, pick most recent.
        opts.SessionPath = sessions[0].Path
    }
}
```

### Step 7: App uses Kit's session instead of creating its own TreeManager

Currently `internal/app/app.go` receives a `TreeSession` via its `Options`. After migration, the app receives a `*Kit` instance and uses its tree session:

```go
// Before:
type Options struct {
    TreeSession *session.TreeManager
    // ...
}

// After (gradual):
type Options struct {
    Kit *kit.Kit  // The SDK instance
    // ...
}

// App gets messages:
msgs := a.opts.Kit.GetTreeSession().GetFantasyMessages()
```

### Step 8: Verify

```bash
go build -o output/kit ./cmd/kit
go test -race ./...
go vet ./...
```

## Files Changed Summary

| Action | File | Change |
|--------|------|--------|
| EDIT | `pkg/kit/kit.go` | Add treeSession, session Options fields, wire Prompt |
| CREATE | `pkg/kit/sessions.go` | ListSessions, Branch, SetSessionName, etc. |
| EDIT | `cmd/root.go` | Replace session setup logic with kit.Options mapping |
| EDIT | `internal/app/app.go` | Accept Kit instance for session access (gradual) |

## Verification Checklist

- [ ] `go build -o output/kit ./cmd/kit` succeeds
- [ ] `go test -race ./...` passes
- [ ] `kit.New(ctx, &kit.Options{Continue: true})` resumes recent session
- [ ] `kit.New(ctx, &kit.Options{NoSession: true})` creates ephemeral session
- [ ] `kit.ListSessions("")` returns sessions
- [ ] CLI `--continue` flag maps to `kit.Options{Continue: true}`
- [ ] CLI `--no-session` flag maps to `kit.Options{NoSession: true}`
- [ ] CLI no longer has its own session initialization logic
