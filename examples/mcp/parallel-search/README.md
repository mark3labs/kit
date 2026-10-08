# Parallel Search MCP

Use this project-local configuration to add web search and page fetch to Kit.
It uses Kit's remote MCP transport (Streamable HTTP) and the anonymous
[Parallel Search MCP](https://docs.parallel.ai/integrations/mcp/search-mcp)
endpoint. You do not need a Parallel API key or an OAuth login.

## Run

From the repository root, build Kit with the Go version in `go.mod`:

```bash
go build -o output/kit ./cmd/kit
cd examples/mcp/parallel-search
../../../output/kit --no-session --no-core-tools \
  "Use parallel web_search to find the official Go release notes. Then use parallel web_fetch to read one result and summarize it with its URL."
```

Use your configured model and its credentials. Model inference is separate from
Parallel's free search service. To select another model, add `--model provider/model`.
Approve the MCP tool calls if Kit asks for permission.

Kit loads the `.kit.yml` in this directory and exposes `parallel__web_search`
and `parallel__web_fetch`. The `User-Agent` header identifies this Kit example
on discovery and tool requests. `noOAuth: true` disables OAuth for this public
endpoint. No authorization header is set.

## Use in another project

Copy the `parallel` entry from `.kit.yml` into that project's `mcpServers` map.
Keep any existing server entries. The example does not select a model or change
Kit's default tools. The run command disables core tools only for that invocation.

Anonymous access has lower rate limits and uses the server's fast search mode.
It is suitable for exploration and light use. Check the linked Parallel
documentation for current limits and tool parameters. When you supply a
`session_id`, reuse it across related search and fetch calls in one conversation.
