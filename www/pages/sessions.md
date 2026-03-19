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

### Open a specific session

```bash
kit --session path/to/session.jsonl
kit -s path/to/session.jsonl
```

## Ephemeral mode

Run without creating a session file:

```bash
kit --no-session
```

This is useful for one-off prompts, scripting, and subagent patterns where persistence isn't needed.
