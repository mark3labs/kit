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
| Custom dir | `{SessionDir: "/path"}` | Base directory for session discovery |

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

