---
title: Examples
description: Catalog of example extensions included with Kit.
---

# Extension Examples

Kit ships with a rich set of example extensions in the `examples/extensions/` directory. These serve as both documentation and starting points for your own extensions.

## UI and display

| Extension | Description |
|-----------|-------------|
| [`minimal.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/minimal.go) | Clean UI with custom footer |
| [`branded-output.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/branded-output.go) | Branded output rendering |
| [`header-footer-demo.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/header-footer-demo.go) | Custom headers and footers |
| [`widget-status.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/widget-status.go) | Persistent status widgets |
| [`overlay-demo.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/overlay-demo.go) | Modal dialogs |
| [`tool-renderer-demo.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/tool-renderer-demo.go) | Custom tool call rendering |
| [`custom-editor-demo.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/custom-editor-demo.go) | Vim-like modal editor |
| [`pirate.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/pirate.go) | Pirate-themed personality |

## Workflow and automation

| Extension | Description |
|-----------|-------------|
| [`auto-commit.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/auto-commit.go) | Auto-commit changes on shutdown |
| [`plan-mode.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/plan-mode.go) | Read-only planning mode |
| [`permission-gate.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/permission-gate.go) | Permission gating for destructive tools |
| [`confirm-destructive.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/confirm-destructive.go) | Confirm destructive operations |
| [`protected-paths.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/protected-paths.go) | Path protection for sensitive files |
| [`project-rules.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/project-rules.go) | Project-specific rules injection |
| [`compact-notify.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/compact-notify.go) | Notification on conversation compaction |

## Interactive features

| Extension | Description |
|-----------|-------------|
| [`prompt-demo.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/prompt-demo.go) | Interactive prompts (select/confirm/input) |
| [`bookmark.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/bookmark.go) | Bookmark conversations |
| [`inline-bash.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/inline-bash.go) | Inline bash execution |
| [`interactive-shell.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/interactive-shell.go) | Interactive shell integration |
| [`notify.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/notify.go) | Desktop notifications |

## Agent and context

| Extension | Description |
|-----------|-------------|
| [`tool-logger.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/tool-logger.go) | Log all tool calls |
| [`context-inject.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/context-inject.go) | Inject context into conversations |
| [`summarize.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/summarize.go) | Conversation summarization |
| [`lsp-diagnostics.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/lsp-diagnostics.go) | LSP diagnostic integration |

## Multi-agent

| Extension | Description |
|-----------|-------------|
| [`kit-kit.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/kit-kit.go) | Kit-in-Kit sub-agent spawning |
| [`subagent-widget.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/subagent-widget.go) | Multi-agent orchestration with status widget |
| [`subagent-test.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/subagent-test.go) | Subagent testing utilities |

## Development

| Extension | Description |
|-----------|-------------|
| [`dev-reload.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/dev-reload.go) | Development live-reload |
| [`tool-logger_test.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/tool-logger_test.go) | Example extension tests (see [Testing](/extensions/testing)) |
| [`extension_test_template.go`](https://github.com/mark3labs/kit/blob/master/examples/extensions/extension_test_template.go) | Copy-and-paste test template for your extensions |

## Subdirectory extensions

| Directory | Description |
|-----------|-------------|
| [`kit-kit-agents/`](https://github.com/mark3labs/kit/tree/master/examples/extensions/kit-kit-agents) | Multi-agent orchestration example |
| [`kit-telegram/`](https://github.com/mark3labs/kit/tree/master/examples/extensions/kit-telegram) | Telegram bot integration |
| [`status-tools/`](https://github.com/mark3labs/kit/tree/master/examples/extensions/status-tools) | Status bar tool examples |
