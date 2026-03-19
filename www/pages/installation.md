---
title: Installation
description: Install Kit using npm, Go, or build from source.
---

# Installation

## Using npm (recommended)

```bash
npm install -g @mark3labs/kit
```

## Using Go

```bash
go install github.com/mark3labs/kit/cmd/kit@latest
```

## Building from source

```bash
git clone https://github.com/mark3labs/kit.git
cd kit
go build -o kit ./cmd/kit
```

## Verifying the installation

After installing, verify Kit is available:

```bash
kit --help
```

## Setting up a provider

Kit needs at least one LLM provider configured. Set an API key for your preferred provider:

```bash
# Anthropic (default provider)
export ANTHROPIC_API_KEY="sk-..."

# OpenAI
export OPENAI_API_KEY="sk-..."

# Google Gemini
export GOOGLE_API_KEY="..."
```

For OAuth-enabled providers like Anthropic, you can also authenticate interactively:

```bash
kit auth login anthropic
```

See [Providers](/providers) for the full list of supported providers and their configuration.
