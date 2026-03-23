# SDK Examples

These examples demonstrate how to use the Kit SDK (`pkg/kit`) to build agents programmatically in Go.

## Examples

### [basic](basic/)

Shows core SDK usage: creating a Kit instance, sending prompts, overriding the model, subscribing to events (tool calls, streaming), and session management.

```bash
go run ./examples/sdk/basic
```

### [scripting](scripting/)

A minimal script-friendly wrapper that takes a prompt from the command line and prints the response — useful for piping and automation.

```bash
go run ./examples/sdk/scripting "Explain what this repo does"
```

## Getting Started

```go
import kit "github.com/mark3labs/kit/pkg/kit"

host, err := kit.New(ctx, nil)        // uses ~/.kit.yml defaults
defer host.Close()

response, err := host.Prompt(ctx, "Hello!")
```

See the [SDK README](../../pkg/kit/README.md) for the full API reference.
