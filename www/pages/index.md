---
title: Kit
description: Kit is a powerful, extensible AI coding agent CLI with multi-provider support, built-in tools, and a rich extension system.
toc: false
---

<div style="text-align: center; margin: 2rem 0;">
  <img src="/logo.jpg" alt="KIT" style="max-width: 400px; width: 100%; margin: 0 auto; display: block;" />
</div>

A powerful, extensible AI coding agent CLI with multi-provider support, built-in tools, and a rich extension system.

## Features

- **Multi-Provider LLM Support** — Anthropic, OpenAI, Google Gemini, Ollama, Azure OpenAI, AWS Bedrock, OpenRouter, and more
- **Built-in Core Tools** — bash (with interactive sudo password prompt), read, write, edit, grep, find, ls, subagent with no MCP overhead
- **Named Agents** — reusable subagent presets defined in markdown, with per-agent tool allowlists, advertised to the LLM for delegation
- **Smart @ Attachments** — Binary files auto-detected via MIME type, MCP resources via `@mcp:server:uri`
- **MCP Integration** — Connect external MCP servers for expanded capabilities (tools, prompts, and resources)
- **Extension System** — Write custom tools, commands, widgets, and UI modifications in Go
- **Interactive TUI** — Rich terminal interface powered by Bubble Tea with streaming, syntax highlighting, and custom rendering
- **Session Management** — Tree-based conversation history with branching support
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
