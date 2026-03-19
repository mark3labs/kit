---
title: SDK Sessions
description: Session management in the Kit Go SDK.
---

# SDK Sessions

## Automatic persistence

By default, Kit automatically persists sessions to JSONL files. Multi-turn conversations retain context across calls:

```go
host.Prompt(ctx, "My name is Alice")
response, _ := host.Prompt(ctx, "What's my name?")
// response: "Your name is Alice"
```

## Accessing session info

```go
// Get the current session file path
path := host.GetSessionPath()

// Get the session ID
id := host.GetSessionID()

// Get the current model string
model := host.GetModelString()
```

## Configuring sessions via Options

Session behavior is configured at initialization:

```go
// Open a specific session file
host, _ := kit.New(ctx, &kit.Options{
    SessionPath: "./my-session.jsonl",
})

// Resume the most recent session for the current directory
host, _ := kit.New(ctx, &kit.Options{
    Continue: true,
})

// Ephemeral mode (no file persistence)
host, _ := kit.New(ctx, &kit.Options{
    NoSession: true,
})

// Custom session directory
host, _ := kit.New(ctx, &kit.Options{
    SessionDir: "/custom/sessions/",
})
```

## Clearing history

Clear the in-memory conversation history (does not delete the session file):

```go
host.ClearSession()
```

## Tree-based sessions

Kit's session model is tree-based, supporting branching. You can branch from any entry to explore alternate conversation paths:

```go
// Access the tree session manager
ts := host.GetTreeSession()

// Branch from a specific entry
err := host.Branch("entry-id-123")
```

## Listing and managing sessions

Package-level functions for session discovery:

```go
// List sessions for a specific directory
sessions := kit.ListSessions("/home/user/project")

// List all sessions across all directories
all := kit.ListAllSessions()

// Delete a session file
kit.DeleteSession("/path/to/session.jsonl")
```
