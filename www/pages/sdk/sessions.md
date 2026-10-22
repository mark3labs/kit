---
title: SDK Sessions
description: Session management in the Kit Go SDK.
ogImage: /og-image.png
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

// Project identity for session discovery, not an output-storage root
host, _ := kit.New(ctx, &kit.Options{
    SessionDir: "/home/user/project",
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
ts := host.GetTreeSession() // nil with a custom manager or no session

// Branch from a specific entry
err := host.Branch("entry-id-123")
```

## Listing and managing sessions

Package-level functions for session discovery:

```go
// List sessions for a specific directory
sessions, err := kit.ListSessions("/home/user/project")
if err != nil { return err }

// List all sessions across all directories
all, err := kit.ListAllSessions()
if err != nil { return err }

// Delete a session file
if err := kit.DeleteSession("/path/to/session.jsonl"); err != nil { return err }
```

## Custom session manager

For advanced use cases (databases, cloud storage, multi-user apps), implement the `SessionManager` interface to replace the default JSONL file backend:

```go
host, _ := kit.New(ctx, &kit.Options{
    SessionManager: myCustomSession,
})
```

`SessionManager` implementations must be safe for concurrent reads and writes during generation. Its method set is frozen for v0.x; new capabilities use optional interfaces. `GetSessionManager` accesses custom backends; `GetTreeSession` is only for the default file-backed tree and can return nil. `SessionDir` identifies the project bucket used by discovery and Continue, not a storage-root override.

The interface requires methods for message storage, branching, compaction, branch summaries, extension data, and lifecycle management. See the [`SessionManager` interface definition](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#SessionManager) for the complete method set.

The `AppendBranchSummary(fromID, summary)` method backs `host.CollapseBranch`,
which collapses a branch range into a single summary entry. Custom managers
that don't track branch summaries can return `kit.ErrBranchSummaryNotSupported`
from that method; `host.CollapseBranch` then surfaces the same sentinel so
callers can detect it with `errors.Is`.

When a Kit has no session manager at all, `host.Branch`, `host.NavigateTo`,
`host.SummarizeBranch`, `host.CollapseBranch`, `host.SetSessionName` and the
extension session API return `kit.ErrNoSession`, which is also detectable with
`errors.Is`.

When using a custom `SessionManager`, the `SessionPath`, `Continue`, and `NoSession` options are ignored — your manager handles its own storage and session selection.

## Model request retries

Kit retries transient model request failures, including rate limits and
retryable provider or transport errors. Each model request has its own budget.
The default policy permits four total attempts, with exponential backoff and
jitter. The initial delay is one second, the maximum delay is 30 seconds, and
retry scheduling is limited to two minutes. Use a context deadline to limit the
duration of active requests.

```go
k, err := kit.NewAgent(ctx,
    kit.WithModel("openai/gpt-4.1"),
    kit.WithRetryPolicy(kit.RetryPolicy{
        MaxAttempts:  3, // Includes the initial request.
        InitialDelay: time.Second,
        MaxDelay:     10 * time.Second,
        MaxElapsed:   time.Minute,
    }),
)
```

You can also set `Options.RetryPolicy` when you use `kit.New`. Nil uses the
built-in defaults. Zero policy fields use their default values. Negative
fields also use defaults, except `MaxResumes` (see below).
Set `MaxAttempts: 1` to disable retries. Server `Retry-After` and
`retry-after-ms` hints are respected. A server delay above `MaxDelay` stops
retries instead of sending a request before the server permits it.

`OnRetry` receives a `RetryEvent` before the scheduled wait. `Attempt` starts at
1 for the first retry and resets for each model request. `Delay` is the scheduled
wait. Cancellation can stop the wait, so an event does not guarantee that the
next request will start. Extension retry events also include the delay.

For safety, streaming retries are permitted only before content is delivered.
Warnings and start parts (text, reasoning, tool input) are held until content
arrives, so they do not prevent a retry. Keepalive parts are forwarded
immediately and can reach consumers from an attempt that later fails. A failure after content
was delivered is handled by step resume (below). The TUI resets transient
stream state on a retry event and keeps completed messages. Retries do not
repeat completed steps or tool execution. The underlying agent library's
retries are disabled on these agent calls to prevent a second retry layer.

### Step resume

When a streaming step fails with a transient error after it streamed output,
or after its request retries are used up, Kit restarts that step from the last
completed step. This is like resuming a failed run from its saved conversation.
The default policy permits five resumes for each turn, about 5, 10, 20, 40 and
80 seconds apart (`MaxResumes`, `ResumeInitialDelay`, `ResumeMaxDelay`). Set
`MaxResumes: -1` to disable resumes.

Safety rules:

- Local tools run only after a step stream ends without error, so the failed
  step ran no tools. Completed steps and tool results are never repeated.
- The failed step's partial output is excluded from normal model history and completed-step storage. A terminal failure can still save it as an `incomplete_output` transcript record (see below).
- Only transient errors are resumed. Auth, validation, cancellation, deadline
  and tool errors are not. A failed session write stops resumes.
- The `MaxSteps` limit applies to the whole turn, also across resumes.
- A wait that would pass the context deadline is not started.

`OnStepResume` receives a `StepResumeEvent` before the wait. Remove the output
of the failed step (message, reasoning and tool input deltas since the last
`StepStartEvent`) from your display. `TurnResult.Stream` already excludes it. A
`StepStartEvent` with the same `StepNumber` follows when the step restarts.
Extension handlers can use `OnStepResume` too.

A resume sends a new provider request, which uses tokens again for the
conversation so far. Provider-side tools (for example web search) in the failed
step can run again on the provider.

### Partial results and session write errors

`PromptResult` and the other turn-result methods can return both a non-nil
result and an error. Check both values. `Incomplete` marks a failed turn.
`PartialText` contains text from its unfinished step. `AttemptID` identifies
that generation. `Messages` contains only normal history and completed steps;
it does not contain `PartialText`. `Stream` still contains the observed events.
The string-returning prompt methods keep their existing error behavior.

```go
result, err := host.PromptResult(ctx, "Explain the change.")
if err != nil {
    if result != nil && result.PartialText != "" {
        fmt.Printf("Incomplete output (%s): %s\n", result.AttemptID, result.PartialText)
    }
    return err
}
fmt.Println(result.Response)
```

The default session backend saves unfinished text as an `incomplete_output`
JSONL tree entry with `attempt_id`, `text`, and `incomplete: true`. This entry
is available in the session tree, but `GetMessages` and `BuildContext` exclude
it from model history. The UI keeps the text and marks it as incomplete, also
when the session is loaded again. This records received text after an orderly
error or cancellation; it does not save every token before a process crash.

`SessionManager` has no new required methods. Custom backends can implement
`IncompleteOutputAppender` to save these transcript records. Backends without
this capability still return partial text through `TurnResult`.

Session write errors now return to the caller. A failed prompt write prevents
generation. A failed step write stops the turn and does not count as a confirmed
write. Kit does not retry a failed write in the same turn. With the legacy
per-message fallback, earlier successful writes remain confirmed; implement
`StepAppender` for atomic storage of tool call/result pairs.

Kit gives optional context-aware writes a detached five-second deadline and
keeps the turn's context values. Backends must honor that deadline. Legacy
`AppendMessage` has no context, and local filesystem calls cannot be interrupted
safely; these writes remain synchronous. Kit does not start a background write
that could change the session after the caller starts another turn.

See the [StepAppender contract](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#StepAppender) for atomicity and cancellation requirements.
