---
description: Run ACP smoke test against opencode/kimi-k2.5 to verify JSON-RPC stdio works
---

Run the ACP smoke test to verify the Kit ACP server works correctly over JSON-RPC stdio with streaming responses.

## Steps

1. Build the kit binary:
   ```bash
   go build -o output/kit ./cmd/kit
   ```

2. Run the smoke test Python script against opencode/kimi-k2.5:
   ```bash
   python3 scripts/acp_smoke_test.py
   ```

3. Verify the output shows:
   - `session/new` returns a valid `sessionId`
   - `session/prompt` streams `agent_thought_chunk` notifications (reasoning)
   - `session/prompt` streams `agent_message_chunk` notifications (response)
   - Final result has `stopReason: "end_turn"`
   - `✓ SMOKE TEST PASSED` at the end

4. If the test fails, check:
   - `output/kit` binary exists and is executable
   - `OPENCODE_API_KEY` or `OPENCODE_ZEN_API_KEY` environment variable is set
   - `scripts/acp_smoke_test.py` exists
   - The model `opencode/kimi-k2.5` is available (`kit models opencode | grep kimi-k2.5`)

5. For testing with a different model, edit the script or set the `MODEL` variable:
   ```bash
   MODEL=anthropic/claude-sonnet-4-5 python3 scripts/acp_smoke_test.py
   ```

The smoke test exercises the full ACP protocol: session lifecycle, streaming notifications, and tool-free prompt completion.
