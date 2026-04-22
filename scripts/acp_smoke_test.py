#!/usr/bin/env python3
"""
ACP smoke test — drives `kit acp` over JSON-RPC 2.0 stdio.

Protocol flow:
  1. initialize        → negotiate capabilities
  2. session/new       → get sessionId
  3. session/list      → verify session listing works
  4. session/set_config_option → set model
  5. session/prompt    → "What is 2+2? Answer in one sentence."
  6. Collect session/update notifications until prompt response
  7. session/cancel    → verify cancel is accepted (no-op since prompt is done)
"""

import json
import subprocess
import sys
import threading
import time
import os

KIT_BIN = os.path.join(os.path.dirname(__file__), "..", "output", "kit")
MODEL   = os.environ.get("MODEL", "opencode/kimi-k2.5")
CWD     = os.path.expanduser("~")
TIMEOUT = 60  # seconds to wait for the prompt to complete

# Request ID counter — initialize=1, session/new=2, etc.
_next_id = 0


def next_id():
    global _next_id
    _next_id += 1
    return _next_id


def rpc_request(method, params):
    """Build a JSON-RPC 2.0 request with auto-incrementing ID."""
    return json.dumps({"jsonrpc": "2.0", "id": next_id(), "method": method, "params": params}) + "\n"


def rpc_notification(method, params):
    """Build a JSON-RPC 2.0 notification (no id)."""
    return json.dumps({"jsonrpc": "2.0", "method": method, "params": params}) + "\n"


def send(proc, line):
    print(f"\n→ SEND  {line.strip()}", flush=True)
    proc.stdin.write(line)
    proc.stdin.flush()


def read_responses(proc, collected, done_event, prompt_id):
    """Read newline-delimited JSON from stdout until process exits."""
    for raw in proc.stdout:
        raw = raw.strip()
        if not raw:
            continue
        try:
            msg = json.loads(raw)
        except json.JSONDecodeError:
            print(f"  [non-JSON stdout]: {raw}", flush=True)
            continue

        collected.append(msg)

        # Pretty-print condensed
        if "result" in msg:
            result = msg["result"]
            print(f"← RESP  id={msg.get('id')}  result={json.dumps(result)[:200]}", flush=True)
            # Prompt complete when we get a stopReason on the prompt request ID
            if msg.get("id") == prompt_id and "stopReason" in result:
                done_event.set()
        elif "error" in msg:
            print(f"← ERROR id={msg.get('id')}  {json.dumps(msg['error'])}", flush=True)
            # If it's the prompt call that errored, unblock
            if msg.get("id") == prompt_id:
                done_event.set()
        elif "method" in msg:
            # Notification / session update
            m = msg.get("method", "")
            p = msg.get("params", {})
            if m == "session/update":
                update = p.get("update", {})
                stype = update.get("sessionUpdate", "?")
                content = update.get("content", {})
                text = content.get("text", "")
                if stype == "agent_thought_chunk":
                    print(f"  [thinking] {text}", end="", flush=True)
                elif stype == "agent_message_chunk":
                    print(f"  [response] {text}", end="", flush=True)
                elif stype in ("tool_call", "tool_call_update"):
                    title = update.get("title", update.get("toolCallId", "?"))
                    status = update.get("status", "?")
                    print(f"\n  [{stype}] {title} ({status})", flush=True)
                else:
                    print(f"\n  [update/{stype}] {json.dumps(update)[:200]}", flush=True)
            else:
                print(f"\n← NOTIF {m}  {json.dumps(p)[:200]}", flush=True)


def wait_for_response(collected, req_id, timeout=5.0, label="response"):
    """Block until we have a response for the given request ID."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        for msg in collected:
            if msg.get("id") == req_id and ("result" in msg or "error" in msg):
                return msg
        time.sleep(0.1)
    print(f"\n✗ FAIL: timed out waiting for {label} (id={req_id})", flush=True)
    return None


def main():
    print(f"Starting: {KIT_BIN} acp -m {MODEL}", flush=True)

    proc = subprocess.Popen(
        [KIT_BIN, "acp", "-m", MODEL],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,
    )

    collected = []
    done_event = threading.Event()

    # We'll set the prompt_id once we know it
    prompt_id_holder = [None]

    # Start reader thread — prompt_id will be set before prompt is sent
    class ReaderThread(threading.Thread):
        def run(self):
            read_responses(proc, collected, done_event, prompt_id_holder[0])

    stderr_lines = []
    def read_stderr():
        for line in proc.stderr:
            line = line.rstrip()
            stderr_lines.append(line)
            if line:
                print(f"  [stderr] {line}", flush=True)
    threading.Thread(target=read_stderr, daemon=True).start()

    time.sleep(0.3)  # let the process initialise

    # ── Step 1: initialize ──────────────────────────────────────────────
    init_id = next_id()
    send(proc, json.dumps({
        "jsonrpc": "2.0",
        "id": init_id,
        "method": "initialize",
        "params": {
            "protocolVersion": 1,
            "clientCapabilities": {
                "fs": {"readTextFile": False, "writeTextFile": False},
            },
            "clientInfo": {"name": "acp-smoke-test", "version": "1.0.0"},
        },
    }) + "\n")

    # Start a simple reader for the initialize response
    reader = threading.Thread(target=read_responses, args=(proc, collected, done_event, None), daemon=True)
    reader.start()

    time.sleep(1.0)

    init_resp = wait_for_response(collected, init_id, timeout=5, label="initialize")
    if not init_resp or "error" in init_resp:
        print(f"\n✗ FAIL: initialize failed: {init_resp}", flush=True)
        proc.terminate()
        sys.exit(1)

    result = init_resp["result"]
    proto_ver = result.get("protocolVersion")
    agent_info = result.get("agentInfo", {})
    print(f"\n✓ Initialized: protocol_version={proto_ver} agent={agent_info.get('name', '?')} v{agent_info.get('version', '?')}", flush=True)

    # ── Step 2: session/new ─────────────────────────────────────────────
    new_session_id = next_id()
    send(proc, json.dumps({
        "jsonrpc": "2.0",
        "id": new_session_id,
        "method": "session/new",
        "params": {"cwd": CWD, "mcpServers": []},
    }) + "\n")
    time.sleep(1.0)

    session_resp = wait_for_response(collected, new_session_id, timeout=10, label="session/new")
    if not session_resp or "error" in session_resp:
        print(f"\n✗ FAIL: session/new failed: {session_resp}", flush=True)
        proc.terminate()
        sys.exit(1)

    session_id = session_resp["result"].get("sessionId")
    if not session_id:
        print("\n✗ FAIL: did not get sessionId from session/new", flush=True)
        proc.terminate()
        sys.exit(1)

    print(f"\n✓ Got sessionId: {session_id}", flush=True)

    # ── Step 3: session/list ────────────────────────────────────────────
    list_id = next_id()
    send(proc, json.dumps({
        "jsonrpc": "2.0",
        "id": list_id,
        "method": "session/list",
        "params": {},
    }) + "\n")
    time.sleep(0.5)

    list_resp = wait_for_response(collected, list_id, timeout=5, label="session/list")
    if not list_resp:
        print("\n⚠ WARN: session/list timed out (non-fatal)", flush=True)
    elif "error" in list_resp:
        print(f"\n⚠ WARN: session/list returned error: {list_resp['error']} (non-fatal)", flush=True)
    else:
        sessions = list_resp["result"].get("sessions", [])
        print(f"\n✓ session/list returned {len(sessions)} session(s)", flush=True)

    # ── Step 4: session/set_config_option (model) ───────────────────────
    # Uses the new session/set_config_option method (replaces the old session/set_model).
    # The model is already set via -m flag, but we exercise the RPC to verify it works.
    config_id = next_id()
    send(proc, json.dumps({
        "jsonrpc": "2.0",
        "id": config_id,
        "method": "session/set_config_option",
        "params": {
            "sessionId": session_id,
            "configId": "model",
            "value": MODEL,
        },
    }) + "\n")
    time.sleep(0.5)

    config_resp = wait_for_response(collected, config_id, timeout=5, label="session/set_config_option")
    if not config_resp:
        print("\n⚠ WARN: session/set_config_option timed out (non-fatal)", flush=True)
    elif "error" in config_resp:
        print(f"\n⚠ WARN: session/set_config_option returned error: {config_resp['error']} (non-fatal)", flush=True)
    else:
        print(f"\n✓ session/set_config_option accepted", flush=True)

    # ── Step 5: session/prompt ──────────────────────────────────────────
    prompt_id = next_id()
    prompt_id_holder[0] = prompt_id

    # Re-wire the reader to know the prompt ID (the existing thread is already running)
    # Since we can't change it mid-flight easily, we check the collected list instead.

    prompt_params = {
        "sessionId": session_id,
        "prompt": [{"type": "text", "text": "What is 2+2? Answer in one sentence."}],
    }
    send(proc, json.dumps({
        "jsonrpc": "2.0",
        "id": prompt_id,
        "method": "session/prompt",
        "params": prompt_params,
    }) + "\n")

    # Wait for finished update or timeout — poll collected list
    deadline = time.time() + TIMEOUT
    prompt_resp = None
    while time.time() < deadline:
        for msg in collected:
            if msg.get("id") == prompt_id and ("result" in msg or "error" in msg):
                prompt_resp = msg
                break
        if prompt_resp:
            break
        time.sleep(0.2)

    if not prompt_resp:
        print(f"\n✗ FAIL: timed out after {TIMEOUT}s waiting for prompt response", flush=True)
        proc.terminate()
        sys.exit(1)

    if "error" in prompt_resp:
        print(f"\n✗ FAIL: prompt returned error: {prompt_resp['error']}", flush=True)
        proc.terminate()
        sys.exit(1)

    stop_reason = prompt_resp["result"].get("stopReason", "?")
    print(f"\n✓ Prompt completed: stopReason={stop_reason}", flush=True)

    # ── Step 6: session/cancel (no-op, prompt already done) ─────────────
    # This is a notification (no id), so no response expected.
    send(proc, rpc_notification("session/cancel", {"sessionId": session_id}))
    time.sleep(0.3)
    print("✓ session/cancel sent (no-op)", flush=True)

    # ── Summary ─────────────────────────────────────────────────────────
    # Count session updates received
    update_count = sum(1 for m in collected if m.get("method") == "session/update")
    print(f"\n✓ SMOKE TEST PASSED  ({update_count} session updates received)", flush=True)
    proc.terminate()
    proc.wait(timeout=5)


if __name__ == "__main__":
    main()
