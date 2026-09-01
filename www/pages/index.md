---
title: Kit
description: Kit is a powerful, extensible AI coding agent CLI with multi-provider support, built-in tools, and a rich extension system.
toc: false
---

<div style="text-align: center; margin: 2rem 0;">
  <img src="/logo.jpg" alt="KIT" style="max-width: 400px; width: 100%; margin: 0 auto; display: block;" />
</div>

A powerful, extensible AI coding agent CLI with multi-provider support, built-in tools, and a rich extension system.

<!-- GIF rather than <video>: Tome's HTML sanitizer strips `playsinline`
     (see SANITIZE_CONFIG in @tomehq/core), and without it iOS Safari refuses
     to autoplay inline. The <img> inside a <video> is only a fallback for
     browsers with no <video> support at all, so iPhones would get a static
     element rather than the GIF. cyberpunk.webm is smaller and is still built
     by www/tapes/record.sh if the sanitizer ever allows the attribute. -->
<img src="/cyberpunk.gif" alt="Kit running in the terminal" style="width: 100%; margin: 2rem 0; border-radius: 8px;" />

## Features

- **Multi-Provider LLM Support** — Anthropic, OpenAI, Google Gemini, Ollama, Azure OpenAI, AWS Bedrock, OpenRouter, and more
- **Built-in Core Tools** — bash (with interactive sudo password prompt), read, write, edit, grep, find, ls, subagent with no MCP overhead
- **Named Agents** — reusable subagent presets defined in markdown, with per-agent tool allowlists, advertised to the LLM for delegation
- **Smart @ Attachments** — Binary files auto-detected via MIME type, MCP resources via `@mcp:server:uri`
- **MCP Integration** — Connect external MCP servers for expanded capabilities (tools, prompts, and resources)
- **Extension System** — Write custom tools, commands, widgets, and UI modifications in Go
- **Interactive TUI** — Rich terminal interface powered by Bubble Tea with streaming, syntax highlighting, and custom rendering
- **Session Management** — Tree-based conversation history with branching support
- **Detachable Sessions** — `kit attach` runs Kit in a session that survives closing the terminal; switch between them like tmux, or attach to a paired machine's sessions over an encrypted connection
- **Non-Interactive Mode** — Script-friendly positional args with JSON output
- **GitHub Integration** — Scaffold a GitHub Actions workflow with `kit github install` to run Kit as a collaborator/reviewer on `/kit` comments
- **ACP Server** — Run Kit as an [Agent Client Protocol](https://agentclientprotocol.com) agent over stdio
- **Go SDK** — Embed Kit in your own applications

## Quick links

| Resource | Description |
|----------|-------------|
| [Installation](/installation) | Get Kit up and running |
| [Quick Start](/quick-start) | Your first Kit session |
| [Configuration](/configuration) | Customize Kit for your workflow |
| [Extensions](/extensions/overview) | Build custom tools and UI components |
| [Go SDK](/sdk/overview) | Embed Kit in your applications |
