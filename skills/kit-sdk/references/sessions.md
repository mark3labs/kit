# Kit SDK: Session Management

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Session Management

Sessions automatically persist as JSONL tree files. No explicit save needed.

### Session modes (via Options)

| Mode | Options | Behavior |
|------|---------|----------|
| Default | `{}` | New session file for cwd |
| Specific file | `{SessionPath: "path.jsonl"}` | Open existing session |
| Continue | `{Continue: true}` | Resume most recent session for cwd |
| Ephemeral | `{NoSession: true}` | In-memory only, no disk persistence |
| Project identity | `{SessionDir: "/path/to/project"}` | Project directory for session discovery, not an output-storage root |

### Instance methods

```go
host.GetSessionPath() // file path of active session
host.GetSessionID()   // UUID of active session
host.ClearSession()   // reset to fresh branch (doesn't delete file)
host.Branch("entry-id") // branch from a specific entry
host.SetSessionName("my session") // set display name

// Get conversation messages
msgs := host.GetSessionMessages()       // []extensions.SessionMessage (flattened text)
msgs := host.GetStructuredMessages()     // []kit.StructuredMessage (typed content parts)
```

### Package-level session operations (no Kit instance needed)

```go
sessions, _ := kit.ListSessions("/path/to/project") // sessions for a directory
sessions, _ := kit.ListAllSessions()                  // all sessions everywhere
kit.DeleteSession("/path/to/session.jsonl")
tm, _ := kit.OpenTreeSession("/path/to/session.jsonl") // open for direct access
```

### Custom Session Manager (Advanced)

You can provide a custom session manager to store conversation history in your own backend (database, cloud storage, etc.) instead of the default JSONL files.

```go
// Implement the SessionManager interface
type MyDatabaseSessionManager struct {
    db *sql.DB
    // ... other fields
}

func (s *MyDatabaseSessionManager) AppendMessage(msg kit.LLMMessage) (string, error) {
    // Store message in your database
}

func (s *MyDatabaseSessionManager) GetMessages() []kit.LLMMessage {
    // Retrieve messages from your database
}

// ... implement all other SessionManager methods

// Use with Kit
host, _ := kit.New(ctx, &kit.Options{
    SessionManager: myCustomSession,  // Your custom implementation
    Model: "anthropic/claude-sonnet-latest",
})
```

**SessionManager Interface:**

```go
type SessionManager interface {
    AppendMessage(msg kit.LLMMessage) (entryID string, err error)
    GetMessages() []kit.LLMMessage
    BuildContext() (messages []kit.LLMMessage, provider string, modelID string)
    Branch(entryID string) error
    GetCurrentBranch() []kit.BranchEntry
    GetChildren(parentID string) []string
    GetEntry(entryID string) *kit.BranchEntry
    GetSessionID() string
    GetSessionName() string
    SetSessionName(name string) error
    GetCreatedAt() time.Time
    IsPersisted() bool
    AppendCompaction(summary string, firstKeptEntryID string,
        tokensBefore, tokensAfter int, messagesRemoved int, readFiles, modifiedFiles []string) (string, error)
    GetLastCompaction() *kit.CompactionEntry
    AppendExtensionData(extType, data string) (string, error)
    GetExtensionData(extType string) []kit.ExtensionDataEntry
    AppendModelChange(provider, modelID string) (string, error)
    GetContextEntryIDs() []string
    Close() error
}
```

**Use Cases:**
- **PocketBase integration**: Store sessions as PocketBase records
- **Cloud storage**: Persist sessions to S3, GCS, or Azure Blob
- **Multi-user apps**: Store sessions per user in a database
- **Custom retention**: Implement your own session cleanup policies

**Note:** When using a custom SessionManager, the following Options are ignored:
- `SessionPath` - your manager handles its own storage
- `Continue` - your manager handles session selection
- `NoSession` - use an in-memory implementation instead


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
