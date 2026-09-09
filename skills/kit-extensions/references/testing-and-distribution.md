# Kit Extensions: Testing and Distribution

> Part of the `kit-extensions` skill. Read `SKILL.md` first for the overview and critical constraints.

## Testing Extensions

Kit provides a testing package to help you write unit tests for your extensions.

> **Scope:** `Harness.Emit` takes event types from `github.com/mark3labs/kit/internal/extensions`, which Go only lets you import from inside the Kit module. The example below therefore works for tests that live in the Kit repository (for instance `examples/extensions/*_test.go`) or in a fork. From an external extension repository, use `harness.EmitJSON(toolName, input)` for tool-call tests and the `test.Assert*` helpers, which need no internal types.

```go
package main

import (
    "testing"
    "github.com/mark3labs/kit/pkg/extensions/test"
    "github.com/mark3labs/kit/internal/extensions"
)

func TestMyExtension(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    // Test event handlers
    _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    // Verify behavior with assertions
    test.AssertPrinted(t, harness, "session started")
    test.AssertWidgetSet(t, harness, "my-widget")
    
    // Test tool blocking
    result, _ := harness.Emit(extensions.ToolCallEvent{ToolName: "dangerous"})
    test.AssertBlocked(t, result, "not allowed")
}
```

**Key testing patterns:**
- Load extensions with `LoadFile()` or `LoadString()` for inline code
- Emit events with `Emit()` to trigger handlers
- Verify with 25+ assertion helpers: `AssertWidgetSet()`, `AssertToolRegistered()`, `AssertPrintInfo()`, etc.
- Mock prompts by setting results on `harness.Context().SetPromptSelectResult()`
- Test multiple scenarios per extension with isolated harness instances

See `examples/extensions/tool-logger_test.go` for a complete example with 14 test cases.

### CLI Testing Commands

```bash
# Validate syntax of all discovered extensions
kit extensions validate

# List loaded extensions
kit extensions list

# Run with a specific extension
kit -e path/to/extension.go

# Run with multiple extensions
kit -e ext1.go -e ext2.go

# Disable all extensions
kit --no-extensions

# Generate an example extension scaffold
kit extensions init
```

---

## Distributing Extensions via Git Repositories

Extensions can be distributed and installed from git repositories using `kit install`. This enables sharing extensions with others and maintaining versioned collections.

### Repository Structure

Extensions support two organization patterns within a repo:

**Single-file extensions** (simple, standalone):
```
my-extension-repo/
├── weather.go           # Single extension file
├── todo.go              # Another extension
└── README.md            # Installation and usage docs
```

**Multi-file extensions** (with `main.go` entry point):
```
my-extension-repo/
├── git-tools/
│   ├── main.go          # Entry point
│   ├── helpers.go       # Supporting code
│   └── config.go        # Configuration
├── todo/
│   ├── main.go          # Entry point
│   └── storage.go       # Storage logic
└── README.md
```

**Hybrid approach** (single files + subdirectories with main.go):
```
my-extensions/
├── weather.go           # Single file extension
├── calculator.go        # Single file extension
├── git-tools/
│   ├── main.go          # Multi-file extension
│   └── utils.go
└── README.md
```

### Installing from Git

Users install extensions using the `kit install` command:

```bash
# Install from GitHub (latest)
kit install github.com/user/repo

# Pin to a specific version/tag
kit install github.com/user/repo@v1.0.0
kit install github.com/user/repo@main
kit install github.com/user/repo@abc1234

# Install locally in project (./.kit/git/)
kit install github.com/user/repo --local

# Interactive selection for repos with multiple extensions
kit install github.com/user/collection --select
```

Supported URL formats:
- `github.com/user/repo` — Shorthand (defaults to HTTPS)
- `git:github.com/user/repo` — Git prefix format
- `https://github.com/user/repo` — HTTPS URL
- `ssh://git@github.com/user/repo` — SSH URL
- `git@github.com:user/repo` — SSH shorthand

### Managing Installed Extensions

```bash
# Update an installed extension (skips pinned versions)
kit install github.com/user/repo --update

# Remove an installed extension
kit install github.com/user/repo --uninstall

# List all loaded extensions
kit extensions list

# Validate all extensions
kit extensions validate
```

### Extension Selection

For repos containing multiple extensions, users can select which to install:

```bash
# Interactive selection
kit install github.com/user/collection --select
```

This prompts the user to choose which extensions to install. Selected extensions are recorded in the manifest, and only those are loaded at runtime (others in the repo are ignored).

### README Template for Extension Repos

Include this in your extension repo's README.md:

```markdown
# My Kit Extensions

A collection of extensions for [Kit](https://github.com/mark3labs/kit).

## Installation

### Install all extensions
\`\`\`bash
kit install github.com/username/repo
\`\`\`

### Install specific extensions
\`\`\`bash
kit install github.com/username/repo --select
\`\`\`

### Install locally in a project
\`\`\`bash
kit install github.com/username/repo --local
\`\`\`

## Extensions

### Extension Name
Description of what it does.

- **Path**: `./ext-name/main.go` or `./ext-name.go`
- **Commands**: `/command-name`
- **Tools**: `tool_name`

## Requirements

- Kit vX.Y.Z+
- Any other dependencies

## Update

\`\`\`bash
kit install github.com/username/repo --update
\`\`\`
```

### Storage Locations

Installed extensions are stored at:

- **Global**: `~/.local/share/kit/git/<host>/<owner>/<repo>/`
- **Project-local**: `./.kit/git/<host>/<owner>/<repo>/`
- **Manifest**: `packages.json` in respective directories

