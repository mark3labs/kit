#!/usr/bin/env python3
"""
ACP smoke test — drives `kit acp` over JSON-RPC 2.0 stdio.

Protocol flow:
  1. session/new  → get sessionId
  2. session/set_model → set opencode/kimi-k2.5
  3. session/prompt → "What is 2+2? Answer in one sentence."
  4. Collect session updates until done
"""

import json
import subprocess
import sys
import threading
import time
import os

KIT_BIN = os.path.join(os.path.dirname(__file__), "..", "output", "kit")
MODEL   = "opencode/kimi-k2.5"
CWD     = os.path.expanduser("~")
TIMEOUT = 60  # seconds to wait for the prompt to complete


def rpc(method, params, req_id):
    return json.dumps({"jsonrpc": "2.0", "id": req_id, "method": method, "params": params}) + "\n"


def send(proc, line):
    print(f"\n→ SEND  {line.strip()}", flush=True)
    proc.stdin.write(line)
    proc.stdin.flush()


def read_responses(proc, collected, done_event):
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
            # Prompt complete when we get a stopReason on id=3
            if msg.get("id") == 3 and "stopReason" in result:
                done_event.set()
        elif "error" in msg:
            print(f"← ERROR id={msg.get('id')}  {json.dumps(msg['error'])}", flush=True)
            # If it's the prompt call that errored, unblock
            if msg.get("id") == 3:
                done_event.set()
        elif "method" in msg:
            # Notification / session update
            m = msg.get("method", "")
            p = msg.get("params", {})
            if m in ("session/update", "session/updated"):
                update = p.get("update", {})
                stype = update.get("sessionUpdate") or update.get("type", "?")
                content = update.get("content", {})
                if stype == "agent_thought_chunk":
                    print(f"  [thinking] {content.get('text','')}", end="", flush=True)
                elif stype == "agent_message_chunk":
                    print(f"  [response] {content.get('text','')}", end="", flush=True)
                else:
                    print(f"\n  [update/{stype}] {json.dumps(update)[:200]}", flush=True)
            else:
                print(f"\n← NOTIF {m}  {json.dumps(p)[:200]}", flush=True)


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

    reader = threading.Thread(target=read_responses, args=(proc, collected, done_event), daemon=True)
    reader.start()

    stderr_lines = []
    def read_stderr():
        for line in proc.stderr:
            line = line.rstrip()
            stderr_lines.append(line)
            if line:
                print(f"  [stderr] {line}", flush=True)
    threading.Thread(target=read_stderr, daemon=True).start()

    time.sleep(0.3)  # let the process initialise

    # 1. session/new
    send(proc, rpc("session/new", {"cwd": CWD, "mcpServers": []}, 1))
    time.sleep(1.0)

    session_id = None
    for msg in collected:
        if msg.get("id") == 1 and "result" in msg:
            session_id = msg["result"].get("sessionId")
            break

    if not session_id:
        print("\n✗ FAIL: did not get sessionId from session/new", flush=True)
        proc.terminate()
        sys.exit(1)

    print(f"\n✓ Got sessionId: {session_id}", flush=True)

    # 2. session/set_model (model already set via -m flag, but exercise the RPC)
    send(proc, rpc("session/set_model", {"sessionId": session_id, "modelId": MODEL}, 2))
    time.sleep(0.5)

    # 3. session/prompt
    prompt_params = {
        "sessionId": session_id,
        "prompt": [{"type": "text", "text": "What is 2+2? Answer in one sentence."}],
    }
    send(proc, rpc("session/prompt", prompt_params, 3))

    # Wait for finished update or timeout
    if not done_event.wait(timeout=TIMEOUT):
        print(f"\n✗ FAIL: timed out after {TIMEOUT}s waiting for finished update", flush=True)
        proc.terminate()
        sys.exit(1)

    # Check we got a successful prompt response
    prompt_resp = next((m for m in collected if m.get("id") == 3), None)
    if prompt_resp and "error" in prompt_resp:
        print(f"\n✗ FAIL: prompt returned error: {prompt_resp['error']}", flush=True)
        proc.terminate()
        sys.exit(1)

    print("\n✓ SMOKE TEST PASSED", flush=True)
    proc.terminate()
    proc.wait(timeout=5)


if __name__ == "__main__":
    main()
