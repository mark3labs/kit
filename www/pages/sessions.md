---
title: Session Management
description: How Kit persists and manages conversation sessions.
---

# Session Management

Kit uses a tree-based session model that supports branching and forking conversations.

## Session storage

Sessions are stored as JSONL (JSON Lines) files:

```
~/.kit/sessions/<cwd-path>/<timestamp>_<id>.jsonl
```

Path separators in the working directory are replaced with `--`. For example, `/home/user/project` becomes `home--user--project`.

Each line in the session file is a JSON entry representing a message, tool call, model change, or extension data. The tree structure allows branching from any message to explore alternate paths.

## Resuming sessions

### Continue most recent

Resume the most recent session for the current directory:

```bash
kit --continue
kit -c
```

### Interactive picker

Choose from previous sessions interactively:

```bash
kit --resume
kit -r
```

The session picker supports search, scope/filter toggles (all sessions vs. current directory), and session deletion. You can also open it during a session with the `/resume` slash command.

### Open a specific session

```bash
kit --session path/to/session.jsonl
kit -s path/to/session.jsonl
```

## Session commands

These slash commands are available during an interactive session:

| Command | Description |
|---------|-------------|
| `/name [name]` | Set or display the session's display name |
| `/session` | Show session info (path, ID, message count) |
| `/resume` | Open the session picker to switch sessions |
| `/export [path]` | Export session as JSONL (auto-generates path if omitted) |
| `/import <path>` | Import and switch to a session from a JSONL file |
| `/tree` | Navigate the session tree |
| `/fork` | Branch from an earlier message |
| `/new` | Start a fresh session |

## Ephemeral mode

Run without creating a session file:

```bash
kit --no-session
```

This is useful for one-off prompts, scripting, and subagent patterns where persistence isn't needed.
