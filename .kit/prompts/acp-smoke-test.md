---
description: Run the ACP smoke test against opencode/kimi-k3 to check Kit's ACP server against the ACP v1 spec over JSON-RPC stdio
---

Run the ACP smoke test. It checks that `kit acp` follows the Agent Client Protocol (protocol version 1, https://github.com/agentclientprotocol/agent-client-protocol) over JSON-RPC 2.0 stdio.

The script acts as an editor client. It advertises the `fs` and `terminal` capabilities, answers permission requests, serves `fs/read_text_file` and `fs/write_text_file`, and runs `terminal/*` commands as local processes.

## Steps

1. Build the kit binary:
   ```bash
   go build -o output/kit ./cmd/kit
   ```

2. Run the unit and in-process end-to-end tests of the ACP server (no model or API key needed):
   ```bash
   go test -race -count=1 ./internal/acpserver/ ./internal/core/
   go test -race -count=1 ./pkg/kit/ -run 'TestWorkDir|TestBeforeToolCallBlock|TestHookedTool'
   ```

3. Run the smoke test script against opencode/kimi-k3:
   ```bash
   python3 scripts/acp_smoke_test.py
   ```
   The script takes about 2–4 minutes. It uses `$TMPDIR/kit-acp-smoke` as the session cwd. When it ends, it deletes the session files it created.

4. Make sure that the output shows `✓` for each of these checks:
   - `initialize`: `protocolVersion` is `1`; `loadSession` and `sessionCapabilities.list`, `close`, `resume` are advertised; `authMethods` is an array
   - `session/new`: returns a `sessionId` and a `configOptions` array with an `approval` option (category `mode`) and a `model` option
   - `available_commands_update`: lists the project template `/smoke-greet` (the script creates it) with an input hint
   - `session/set_config_option`: returns the full `configOptions` array (never `null`); an unknown option gives InvalidParams (`-32602`)
   - `session/set_mode`: returns an error (Kit has no modes)
   - `session/prompt`: streams `agent_message_chunk` notifications and ends with `stopReason: "end_turn"`
   - Tool prompt (`ls`): `tool_call` and a finished `tool_call_update`, no permission request (read-only), and the output shows `acp_smoke_marker.txt`, which proves the tools run in the session `cwd`
   - Shell prompt: `session/request_permission` with four options, the command runs through `terminal/create` in the session `cwd`, the terminal is embedded in the tool call, and the statuses go `pending` → `in_progress` → `completed`
   - Write prompt: permission is asked, and the file is written through `fs/write_text_file`
   - Reject prompt: the rejected command does not run, the tool call is `failed`, and the turn ends with `end_turn`
   - Plan prompt: `update_plan` produces a `plan` update, not a `tool_call`
   - `session/cancel`: the running prompt ends with `stopReason: "cancelled"`
   - `session/list`: lists the session with its `cwd` and a title
   - `session/close`: accepted; a later prompt on that session is rejected
   - `session/load`: replays the history (`user_message_chunk`, `agent_message_chunk`, tool calls, plans) before the response, and the next prompt keeps the context
   - `session/new` with a relative `cwd` is rejected with InvalidParams
   - stdout carries only JSON-RPC 2.0 messages
   - `✓ SMOKE TEST PASSED` at the end

   `⚠ WARN` lines do not fail the test. Examples: no `agent_thought_chunk` (the model does not stream reasoning), the model did not call `update_plan`, or the prompt finished before the cancel arrived.

5. If the test fails, check:
   - `output/kit` exists and is executable
   - `OPENCODE_API_KEY` or `OPENCODE_ZEN_API_KEY` is set
   - The model is available and serves requests:
     ```bash
     kit models opencode | grep kimi-k3
     kit --quiet --no-session --no-extensions -m opencode/kimi-k3 "Say OK"
     ```
     If the provider says the endpoint is gone, pick another model (see step 6).
   - The `[stderr]`, `[permission]`, `[terminal]` and `[fs write]` lines in the output (run `kit acp --debug` for more logs)

6. To test with a different model, set `MODEL`. Other settings: `KIT_BIN` (binary path), `TIMEOUT` (seconds per prompt, default 120), `SKIP_TOOLS=1` (skip the tool steps for models without tool support):
   ```bash
   MODEL=anthropic/claude-sonnet-4-5 python3 scripts/acp_smoke_test.py
   ```

## Spec updates

The Go SDK (`github.com/coder/acp-go-sdk`) tracks an ACP schema release (see `schema/version` in the module). When you update the SDK, compare its `schema/schema.json` with `schema/v1/schema.json` of the newest spec release. Then add the new stable features to `internal/acpserver/` and to this smoke test.
