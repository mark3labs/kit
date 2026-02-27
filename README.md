<p align="center">
  <img src="logo.jpg" alt="KIT" width="400">
</p>

# KIT (Knowledge Inference Tool)

A lightweight AI agent for coding. Supports Claude, OpenAI, Google Gemini, Ollama, and any OpenAI-compatible endpoint.

Discuss the Project on [Discord](https://discord.gg/RqSS2NQVsY)

## Table of Contents

- [Overview](#overview-)
- [Features](#features-)
- [Requirements](#requirements-)
- [Environment Setup](#environment-setup-)
- [Installation](#installation-)
- [SDK Usage](#sdk-usage-)
- [Configuration](#configuration-)
  - [MCP Servers](#mcp-servers)
  - [Environment Variable Substitution](#environment-variable-substitution)
  - [Simplified Configuration Schema](#simplified-configuration-schema)
  - [Tool Filtering](#tool-filtering)
  - [Legacy Configuration Support](#legacy-configuration-support)
  - [Transport Types](#transport-types)
  - [System Prompt](#system-prompt)
- [Usage](#usage-)
  - [Interactive Mode](#interactive-mode-default)
  - [Script Mode](#script-mode)
  - [Hooks System](#hooks-system)
  - [Non-Interactive Mode](#non-interactive-mode)
  - [Model Generation Parameters](#model-generation-parameters)
  - [Available Models](#available-models)
  - [Examples](#examples)
  - [Flags](#flags)
  - [Authentication Subcommands](#authentication-subcommands)
  - [Configuration File Support](#configuration-file-support)
  - [Interactive Commands](#interactive-commands)
- [Automation & Scripting](#automation--scripting-)
- [MCP Server Compatibility](#mcp-server-compatibility-)
- [Contributing](#contributing-)
- [License](#license-)
- [Acknowledgments](#acknowledgments-)

## Overview 🌟

KIT acts as a host in the MCP client-server architecture, where:
- **Hosts** (like KIT) are LLM applications that manage connections and interactions
- **Clients** maintain 1:1 connections with MCP servers
- **Servers** provide context, tools, and capabilities to the LLMs

This architecture allows language models to:
- Access external tools and data sources 🛠️
- Maintain consistent context across interactions 🔄
- Execute commands and retrieve information safely 🔒

Currently supports:
- Anthropic Claude models (Claude 3.5 Sonnet, Claude 3.5 Haiku, etc.)
- OpenAI models (GPT-4, GPT-4 Turbo, GPT-3.5, etc.)
- Google Gemini models (Gemini 2.0 Flash, Gemini 1.5 Pro, etc.)
- Any Ollama-compatible model with function calling support
- Any OpenAI-compatible API endpoint

## Features ✨

- Interactive conversations with multiple AI models
- **Non-interactive mode** for scripting and automation
- **Script mode** for executable YAML-based automation scripts
- Support for multiple concurrent MCP servers
- **Tool filtering** with `allowedTools` and `excludedTools` per server
- Dynamic tool discovery and integration
- Tool calling capabilities across all supported models
- Configurable MCP server locations and arguments
- Consistent command interface across model types
- Configurable message history window for context management
- **OAuth authentication** support for Anthropic (alternative to API keys)
- **Hooks system** for custom integrations and security policies
- **Environment variable substitution** in configs and scripts
- **Builtin servers** for common functionality (filesystem, bash, todo, http)

## Requirements 📋

- Go 1.23 or later
- For OpenAI/Anthropic: API key for the respective provider
- For Ollama: Local Ollama installation with desired models
- For Google/Gemini: Google API key (see https://aistudio.google.com/app/apikey)
- One or more MCP-compatible tool servers

## Environment Setup 🔧

1. API Keys:
```bash
# For all providers (use --provider-api-key flag or these environment variables)
export OPENAI_API_KEY='your-openai-key'        # For OpenAI
export ANTHROPIC_API_KEY='your-anthropic-key'  # For Anthropic
export GOOGLE_API_KEY='your-google-key'        # For Google/Gemini
```

2. Ollama Setup:
- Install Ollama from https://ollama.ai
- Pull your desired model:
```bash
ollama pull mistral
```
- Ensure Ollama is running:
```bash
ollama serve
```

You can also configure the Ollama client using standard environment variables, such as `OLLAMA_HOST` for the Ollama base URL.

3. Google API Key (for Gemini):
```bash
export GOOGLE_API_KEY='your-api-key'
```

4. OpenAI Compatible Setup:
- Get your API server base URL, API key and model name
- Use `--provider-url` and `--provider-api-key` flags or set environment variables

5. Self-Signed Certificates (TLS):
If your provider uses self-signed certificates (e.g., local Ollama with HTTPS), you can skip certificate verification:
```bash
kit --provider-url https://192.168.1.100:443 --tls-skip-verify
```
⚠️ **WARNING**: Only use `--tls-skip-verify` for development or when connecting to trusted servers with self-signed certificates. This disables TLS certificate verification and is insecure for production use.

## Installation 📦

```bash
go install github.com/mark3labs/kit/cmd/kit@latest
```

## SDK Usage 🛠️

KIT also provides a Go SDK for programmatic access without spawning OS processes. The SDK maintains identical behavior to the CLI, including configuration loading, environment variables, and defaults.

### Quick Example

```go
package main

import (
    "context"
    "fmt"
    kit "github.com/mark3labs/kit/pkg/kit"
)

func main() {
    ctx := context.Background()
    
    // Create Kit instance with default configuration
    host, err := kit.New(ctx, nil)
    if err != nil {
        panic(err)
    }
    defer host.Close()
    
    // Send a prompt and get response
    response, err := host.Prompt(ctx, "What is 2+2?")
    if err != nil {
        panic(err)
    }
    
    fmt.Println(response)
}
```

### SDK Features

- ✅ Programmatic access without spawning processes
- ✅ Identical configuration behavior to CLI
- ✅ Session management (save/load/clear)
- ✅ Tool execution callbacks for monitoring
- ✅ Streaming support
- ✅ Full compatibility with all providers and MCP servers

For detailed SDK documentation, examples, and API reference, see the [SDK README](pkg/kit/README.md).

## Configuration ⚙️

### MCP Servers
KIT will automatically create a configuration file in your home directory if it doesn't exist. It looks for config files in this order:
- `.kit.yml` or `.kit.json`

**Config file locations by OS:**
- **Linux/macOS**: `~/.kit.yml`, `~/.kit.json`
- **Windows**: `%USERPROFILE%\.kit.yml`, `%USERPROFILE%\.kit.json`

You can also specify a custom location using the `--config` flag.

### Environment Variable Substitution

KIT supports environment variable substitution in both config files and script frontmatter using the syntax:
- **`${env://VAR}`** - Required environment variable (fails if not set)
- **`${env://VAR:-default}`** - Optional environment variable with default value

This allows you to keep sensitive information like API keys in environment variables while maintaining flexible configuration.

**Example:**
```yaml
mcpServers:
  github:
    type: local
    command: ["docker", "run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN=${env://GITHUB_TOKEN}", "ghcr.io/github/github-mcp-server"]
    environment:
      DEBUG: "${env://DEBUG:-false}"
      LOG_LEVEL: "${env://LOG_LEVEL:-info}"

model: "${env://MODEL:-anthropic/claude-sonnet-4-5-20250929}"
provider-api-key: "${env://OPENAI_API_KEY}"  # Required - will fail if not set
```

**Usage:**
```bash
# Set required environment variables
export GITHUB_TOKEN="ghp_your_token_here"
export OPENAI_API_KEY="your_openai_key"

# Optionally override defaults
export DEBUG="true"
export MODEL="openai/gpt-4"

# Run kit
kit
```

### Simplified Configuration Schema

KIT now supports a simplified configuration schema with three server types:

#### Local Servers
For local MCP servers that run commands on your machine:
```json
{
  "mcpServers": {
    "filesystem": {
      "type": "local",
      "command": ["npx", "@modelcontextprotocol/server-filesystem", "${env://WORK_DIR:-/tmp}"],
      "environment": {
        "DEBUG": "${env://DEBUG:-false}",
        "LOG_LEVEL": "${env://LOG_LEVEL:-info}",
        "API_TOKEN": "${env://FS_API_TOKEN}"
      },
      "allowedTools": ["read_file", "write_file"],
      "excludedTools": ["delete_file"]
    },
    "github": {
      "type": "local",
      "command": ["docker", "run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN=${env://GITHUB_TOKEN}", "ghcr.io/github/github-mcp-server"],
      "environment": {
        "DEBUG": "${env://DEBUG:-false}"
      }
    },
    "sqlite": {
      "type": "local",
      "command": ["uvx", "mcp-server-sqlite", "--db-path", "${env://DB_PATH:-/tmp/foo.db}"],
      "environment": {
        "SQLITE_DEBUG": "${env://DEBUG:-0}",
        "DATABASE_URL": "${env://DATABASE_URL:-sqlite:///tmp/foo.db}"
      }
    }
  }
}
```

Each local server entry requires:
- `type`: Must be set to `"local"`
- `command`: Array containing the command and all its arguments
- `environment`: (Optional) Object with environment variables as key-value pairs
- `allowedTools`: (Optional) Array of tool names to include (whitelist)
- `excludedTools`: (Optional) Array of tool names to exclude (blacklist)

#### Remote Servers
For remote MCP servers accessible via HTTP:
```json
{
  "mcpServers": {
    "websearch": {
      "type": "remote",
      "url": "${env://WEBSEARCH_URL:-https://api.example.com/mcp}",
      "headers": ["Authorization: Bearer ${env://WEBSEARCH_TOKEN}"]
    },
    "weather": {
      "type": "remote", 
      "url": "${env://WEATHER_URL:-https://weather-mcp.example.com}"
    }
  }
}
```

Each remote server entry requires:
- `type`: Must be set to `"remote"`
- `url`: The URL where the MCP server is accessible
- `headers`: (Optional) Array of HTTP headers for authentication and custom headers

Remote servers automatically use the StreamableHTTP transport for optimal performance.

#### Builtin Servers
For builtin MCP servers that run in-process for optimal performance:
```json
{
  "mcpServers": {
    "filesystem": {
      "type": "builtin",
      "name": "fs",
      "options": {
        "allowed_directories": ["${env://WORK_DIR:-/tmp}", "${env://HOME}/documents"]
      },
      "allowedTools": ["read_file", "write_file", "list_directory"]
    },
    "filesystem-cwd": {
      "type": "builtin",
      "name": "fs"
    }
  }
}
```

Each builtin server entry requires:
- `type`: Must be set to `"builtin"`
- `name`: Internal name of the builtin server (e.g., `"fs"` for filesystem)
- `options`: Configuration options specific to the builtin server

**Available Builtin Servers:**
- `fs` (filesystem): Secure filesystem access with configurable allowed directories
  - `allowed_directories`: Array of directory paths that the server can access (defaults to current working directory if not specified)
- `bash`: Execute bash commands with security restrictions and timeout controls
  - No configuration options required
- `todo`: Manage ephemeral todo lists for task tracking during sessions
  - No configuration options required (todos are stored in memory and reset on restart)
- `http`: Fetch web content and convert to text, markdown, or HTML formats
  - Tools: `fetch` (fetch and convert web content), `fetch_summarize` (fetch and summarize web content using AI), `fetch_extract` (fetch and extract specific data using AI), `fetch_filtered_json` (fetch JSON and filter using gjson path syntax)
  - No configuration options required

#### Builtin Server Examples

```json
{
  "mcpServers": {
    "filesystem": {
      "type": "builtin",
      "name": "fs",
      "options": {
        "allowed_directories": ["/tmp", "/home/user/documents"]
      }
    },
    "bash-commands": {
      "type": "builtin", 
      "name": "bash"
    },
    "task-manager": {
      "type": "builtin",
      "name": "todo"
    },
    "web-fetcher": {
      "type": "builtin",
      "name": "http"
    }
  }
}
```

### Tool Filtering

All MCP server types support tool filtering to restrict which tools are available:

- **`allowedTools`**: Whitelist - only specified tools are available from the server
- **`excludedTools`**: Blacklist - all tools except specified ones are available

```json
{
  "mcpServers": {
    "filesystem-readonly": {
      "type": "builtin",
      "name": "fs",
      "allowedTools": ["read_file", "list_directory"]
    },
    "filesystem-safe": {
      "type": "local", 
      "command": ["npx", "@modelcontextprotocol/server-filesystem", "/tmp"],
      "excludedTools": ["delete_file"]
    }
  }
}
```

**Note**: `allowedTools` and `excludedTools` are mutually exclusive - you can only use one per server.

### Legacy Configuration Support

KIT maintains full backward compatibility with the previous configuration format. **Note**: A recent bug fix improved legacy stdio transport reliability for external MCP servers (Docker, NPX, etc.).

#### Legacy STDIO Format
```json
{
  "mcpServers": {
    "sqlite": {
      "command": "uvx",
      "args": ["mcp-server-sqlite", "--db-path", "/tmp/foo.db"],
      "env": {
        "DEBUG": "true"
      }
    }
  }
}
```

#### Legacy SSE Format
```json
{
  "mcpServers": {
    "server_name": {
      "url": "http://some_host:8000/sse",
      "headers": ["Authorization: Bearer my-token"]
    }
  }
}
```

#### Legacy Docker/Container Format
```json
{
  "mcpServers": {
    "phalcon": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "ghcr.io/mark3labs/phalcon-mcp:latest",
        "serve"
      ]
    }
  }
}
```

#### Legacy Streamable HTTP Format
```json
{
  "mcpServers": {
    "websearch": {
      "transport": "streamable",
      "url": "https://api.example.com/mcp",
      "headers": ["Authorization: Bearer your-api-token"]
    }
  }
}
```

### Transport Types

KIT supports four transport types:
- **`stdio`**: Launches a local process and communicates via stdin/stdout (used by `"local"` servers)
- **`sse`**: Connects to a server using Server-Sent Events (legacy format)
- **`streamable`**: Connects to a server using Streamable HTTP protocol (used by `"remote"` servers)
- **`inprocess`**: Runs builtin servers in-process for optimal performance (used by `"builtin"` servers)

The simplified schema automatically maps:
- `"local"` type → `stdio` transport
- `"remote"` type → `streamable` transport
- `"builtin"` type → `inprocess` transport

### System Prompt

You can specify a custom system prompt using the `--system-prompt` flag. You can either:

1. **Pass the prompt directly as text:**
   ```bash
   kit --system-prompt "You are a helpful assistant that responds in a friendly tone."
   ```

2. **Pass a path to a text file containing the prompt:**
   ```bash
   kit --system-prompt ./prompts/assistant.md
   ```

   Example `assistant.md` file:
   ```markdown
   You are a helpful coding assistant. 
   
   Please:
   - Write clean, readable code
   - Include helpful comments
   - Follow best practices
   - Explain your reasoning
   ```


## Usage 🚀

KIT is a CLI tool that allows you to interact with various AI models through a unified interface. It supports various tools through MCP servers and can run in both interactive and non-interactive modes.

### Interactive Mode (Default)

Start an interactive conversation session:

```bash
kit
```

### Hooks System

KIT supports a powerful hooks system that allows you to execute custom commands at specific points during execution. This enables security policies, logging, custom integrations, and automated workflows.

#### Quick Start

1. Initialize a hooks configuration:
   ```bash
   kit hooks init
   ```

2. View active hooks:
   ```bash
   kit hooks list
   ```

3. Validate your configuration:
   ```bash
   kit hooks validate
   ```

#### Configuration

Hooks are configured in YAML files with the following precedence (highest to lowest):
- `.kit/hooks.yml` (project-specific hooks)
- `$XDG_CONFIG_HOME/kit/hooks.yml` (user global hooks, defaults to `~/.config/kit/hooks.yml`)

Example configuration:
```yaml
hooks:
  PreToolUse:
    - matcher: "bash"
      hooks:
        - type: command
          command: "/usr/local/bin/validate-bash.py"
          timeout: 5
  
  UserPromptSubmit:
    - hooks:
        - type: command
          command: "~/.kit/hooks/log-prompt.sh"
```

#### Available Hook Events

- **PreToolUse**: Before any tool execution (bash, fetch, todo, MCP tools)
- **PostToolUse**: After tool execution completes
- **UserPromptSubmit**: When user submits a prompt
- **Stop**: When the agent finishes responding
- **SubagentStop**: When a subagent (Task tool) finishes
- **Notification**: When KIT sends notifications

#### Security

⚠️ **WARNING**: Hooks execute arbitrary commands on your system. Only use hooks from trusted sources and always review hook commands before enabling them.

To temporarily disable all hooks, use the `--no-hooks` flag:
```bash
kit --no-hooks
```

See the example hook scripts in `examples/hooks/`:
- `bash-validator.py` - Validates and blocks dangerous bash commands
- `prompt-logger.sh` - Logs all user prompts with timestamps
- `mcp-monitor.py` - Monitors and enforces policies on MCP tool usage

### Non-Interactive Mode

Run a single prompt and exit - perfect for scripting and automation:

```bash
# Basic non-interactive usage
kit -p "What is the weather like today?"

# Quiet mode - only output the AI response (no UI elements)
kit -p "What is 2+2?" --quiet

# Use with different models
kit -m ollama/qwen2.5:3b -p "Explain quantum computing" --quiet
```

### Model Generation Parameters

KIT supports fine-tuning model behavior through various parameters:

```bash
# Control response length
kit -p "Explain AI" --max-tokens 1000

# Adjust creativity (0.0 = focused, 1.0 = creative)
kit -p "Write a story" --temperature 0.9

# Control diversity with nucleus sampling
kit -p "Generate ideas" --top-p 0.8

# Limit token choices for more focused responses
kit -p "Answer precisely" --top-k 20

# Set custom stop sequences
kit -p "Generate code" --stop-sequences "```","END"
```

These parameters work with all supported providers (OpenAI, Anthropic, Google, Ollama) where supported by the underlying model.

### Available Models
Models can be specified using the `--model` (`-m`) flag:
- **Anthropic Claude** (default): `anthropic/claude-sonnet-4-5-20250929`, `anthropic/claude-3-5-sonnet-latest`, `anthropic/claude-3-5-haiku-latest`
- **OpenAI**: `openai/gpt-4`, `openai/gpt-4-turbo`, `openai/gpt-3.5-turbo`
- **Google Gemini**: `google/gemini-2.0-flash`, `google/gemini-1.5-pro`
- **Ollama models**: `ollama/llama3.2`, `ollama/qwen2.5:3b`, `ollama/mistral`
- **OpenAI-compatible**: Any model via custom endpoint with `--provider-url`

### Examples

#### Interactive Mode
```bash
# Use Ollama with Qwen model
kit -m ollama/qwen2.5:3b

# Use OpenAI's GPT-4
kit -m openai/gpt-4

# Use OpenAI-compatible model with custom URL and API key
kit --model openai/<your-model-name> \
--provider-url <your-base-url> \
--provider-api-key <your-api-key>
```

#### Non-Interactive Mode
```bash
# Single prompt with full UI
kit -p "List files in the current directory"

# Compact mode for cleaner output without fancy styling
kit -p "List files in the current directory" --compact

# Quiet mode for scripting (only AI response output, no UI elements)
kit -p "What is the capital of France?" --quiet

# Use in shell scripts
RESULT=$(kit -p "Calculate 15 * 23" --quiet)
echo "The answer is: $RESULT"

# Pipe to other commands
kit -p "Generate a random UUID" --quiet | tr '[:lower:]' '[:upper:]'
```

### Flags
- `--provider-url string`: Base URL for the provider API (applies to OpenAI, Anthropic, Ollama, and Google)
- `--provider-api-key string`: API key for the provider (applies to OpenAI, Anthropic, and Google)
- `--tls-skip-verify`: Skip TLS certificate verification (WARNING: insecure, use only for self-signed certificates)
- `--config string`: Config file location (default is $HOME/.kit.yml)
- `--system-prompt string`: system-prompt file location
- `--debug`: Enable debug logging
- `--max-steps int`: Maximum number of agent steps (0 for unlimited, default: 0)
- `-m, --model string`: Model to use (format: provider/model) (default "anthropic/claude-sonnet-4-5-20250929")
- `-p, --prompt string`: **Run in non-interactive mode with the given prompt**
- `--quiet`: **Suppress all output except the AI response (only works with --prompt)**
- `--compact`: **Enable compact output mode without fancy styling (ideal for scripting and automation)**
- `--stream`: Enable streaming responses (default: true, use `--stream=false` to disable)

### Authentication Subcommands
- `kit auth login anthropic`: Authenticate with Anthropic using OAuth (alternative to API keys)
- `kit auth logout anthropic`: Remove stored OAuth credentials
- `kit auth status`: Show authentication status

**Note**: OAuth credentials (when present) take precedence over API keys from environment variables and `--provider-api-key` flags.

#### Model Generation Parameters
- `--max-tokens int`: Maximum number of tokens in the response (default: 4096)
- `--temperature float32`: Controls randomness in responses (0.0-1.0, default: 0.7)
- `--top-p float32`: Controls diversity via nucleus sampling (0.0-1.0, default: 0.95)
- `--top-k int32`: Controls diversity by limiting top K tokens to sample from (default: 40)
- `--stop-sequences strings`: Custom stop sequences (comma-separated)

### Configuration File Support

All command-line flags can be configured via the config file. KIT will look for configuration in this order:
1. `~/.kit.yml` or `~/.kit.json`

Example config file (`~/.kit.yml`):
```yaml
# MCP Servers - New Simplified Format
mcpServers:
  filesystem-local:
    type: "local"
    command: ["npx", "@modelcontextprotocol/server-filesystem", "/path/to/files"]
    environment:
      DEBUG: "true"
  filesystem-builtin:
    type: "builtin"
    name: "fs"
    options:
      allowed_directories: ["/tmp", "/home/user/documents"]
  websearch:
    type: "remote"
    url: "https://api.example.com/mcp"

# Application settings
model: "anthropic/claude-sonnet-4-5-20250929"
max-steps: 20
debug: false
system-prompt: "/path/to/system-prompt.txt"

# Model generation parameters
max-tokens: 4096
temperature: 0.7
top-p: 0.95
top-k: 40
stop-sequences: ["Human:", "Assistant:"]

# Streaming configuration
stream: false  # Disable streaming (default: true)

# API Configuration
provider-api-key: "your-api-key"      # For OpenAI, Anthropic, or Google
provider-url: "https://api.openai.com/v1"  # Custom base URL
tls-skip-verify: false  # Skip TLS certificate verification (default: false)
```

**Note**: Command-line flags take precedence over config file values.


### Interactive Commands

While chatting, you can use:
- `/help`: Show available commands
- `/tools`: List all available tools
- `/servers`: List configured MCP servers
- `/history`: Display conversation history
- `/quit`: Exit the application
- `Ctrl+C`: Exit at any time

### Authentication Commands

Optional OAuth authentication for Anthropic (alternative to API keys):
- `kit auth login anthropic`: Authenticate using OAuth
- `kit auth logout anthropic`: Remove stored OAuth credentials
- `kit auth status`: Show authentication status

### Global Flags
- `--config`: Specify custom config file location

## Automation & Scripting 🤖

KIT's non-interactive mode makes it perfect for automation, scripting, and integration with other tools.

### Use Cases

#### Shell Scripts
```bash
#!/bin/bash
# Get weather and save to file
kit -p "What's the weather in New York?" --quiet > weather.txt

# Process files with AI
for file in *.txt; do
    summary=$(kit -p "Summarize this file: $(cat $file)" --quiet)
    echo "$file: $summary" >> summaries.txt
done
```

#### CI/CD Integration
```bash
# Code review automation
DIFF=$(git diff HEAD~1)
kit -p "Review this code diff and suggest improvements: $DIFF" --quiet

# Generate release notes
COMMITS=$(git log --oneline HEAD~10..HEAD)
kit -p "Generate release notes from these commits: $COMMITS" --quiet
```

#### Data Processing
```bash
# Process CSV data
kit -p "Analyze this CSV data and provide insights: $(cat data.csv)" --quiet

# Generate reports
kit -p "Create a summary report from this JSON: $(cat metrics.json)" --quiet
```

#### API Integration
```bash
# Use as a microservice
curl -X POST http://localhost:8080/process \
  -d "$(kit -p 'Generate a UUID' --quiet)"
```

### Tips
- Use `--quiet` flag to get clean output suitable for parsing (only AI response, no UI)
- Use `--compact` flag for simplified output without fancy styling (when you want to see UI elements)
- Note: `--compact` and `--quiet` are mutually exclusive - `--compact` has no effect with `--quiet`
- **Use environment variables for sensitive data** like API keys instead of hardcoding them
- **Use `${env://VAR}` syntax** in config files for environment variable substitution
- Use environment variables for API keys in production

#### Environment Variable Best Practices
```bash
# Set sensitive variables in environment
export GITHUB_TOKEN="ghp_your_token_here"
export OPENAI_API_KEY="your_openai_key"
export DATABASE_URL="postgresql://user:pass@localhost/db"

# Use in config files
mcpServers:
  github:
    environment:
      GITHUB_TOKEN: "${env://GITHUB_TOKEN}"
      DEBUG: "${env://DEBUG:-false}"
```

## MCP Server Compatibility 🔌

KIT can work with any MCP-compliant server. For examples and reference implementations, see the [MCP Servers Repository](https://github.com/modelcontextprotocol/servers).

## Contributing 🤝

Contributions are welcome! Feel free to:
- Submit bug reports or feature requests through issues
- Create pull requests for improvements
- Share your custom MCP servers
- Improve documentation

Please ensure your contributions follow good coding practices and include appropriate tests.

## License 📄

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments 🙏

- Thanks to the Anthropic team for Claude and the MCP specification
- Thanks to the Ollama team for their local LLM runtime
- Thanks to all contributors who have helped improve this tool
