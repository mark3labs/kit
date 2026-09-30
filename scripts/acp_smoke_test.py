#!/usr/bin/env python3
"""
ACP smoke test: drives `kit acp` over JSON-RPC 2.0 stdio and checks the
behavior the Agent Client Protocol (protocol version 1) requires.

The script acts as an editor client: it advertises the fs and terminal
capabilities, answers permission requests, serves fs/read_text_file and
fs/write_text_file, and runs terminal/* commands as local processes.

Protocol flow:
   1. initialize                  -> protocol version, capabilities, agentInfo
   2. session/new                 -> sessionId + configOptions (approval, model),
                                     available_commands_update with a project template
   3. session/set_config_option   -> full configOptions list in the response
   4. session/set_config_option   -> unknown option is rejected
   5. session/set_mode            -> rejected (Kit has no modes)
   6. session/prompt (text)       -> streamed chunks, stopReason end_turn
   7. session/prompt (ls tool)    -> read-only tool runs in the session cwd, no permission
   8. session/prompt (shell)      -> permission asked, command runs in a client terminal
   9. session/prompt (write)      -> permission asked, file written through fs/write_text_file
  10. session/prompt (rejected)   -> rejected tool does not run, turn ends normally
  11. session/prompt (plan)       -> update_plan shows a plan (warning only)
  12. session/prompt + cancel     -> stopReason cancelled
  13. session/list                -> the session is listed
  14. session/close               -> session is freed
  15. session/load                -> history replayed before the response
  16. session/prompt (loaded)     -> the loaded session keeps its context
  17. session/new (relative cwd)  -> rejected

It also checks that every line on stdout is a JSON-RPC 2.0 message, and it
deletes the sessions it created when it ends.

Environment variables:
  MODEL       model to use (default: opencode/kimi-k3)
  KIT_BIN     kit binary (default: ../output/kit)
  SKIP_TOOLS  set to 1 to skip the tool steps (7-11)
  TIMEOUT     seconds to wait for each prompt (default: 120)
"""

import glob
import json
import os
import queue
import random
import subprocess
import sys
import tempfile
import threading
import time

HERE = os.path.dirname(os.path.abspath(__file__))
KIT_BIN = os.environ.get("KIT_BIN", os.path.join(HERE, "..", "output", "kit"))
MODEL = os.environ.get("MODEL", "opencode/kimi-k3")
SKIP_TOOLS = os.environ.get("SKIP_TOOLS") == "1"
TIMEOUT = float(os.environ.get("TIMEOUT", "120"))

# The session cwd. A fixed directory keeps test sessions out of real
# projects and contains a marker file for the tool-call step.
CWD = os.path.join(tempfile.gettempdir(), "kit-acp-smoke")
MARKER = "acp_smoke_marker.txt"
TEMPLATE = "smoke-greet"

failures = []
warnings = []


def ok(msg):
    print(f"✓ {msg}", flush=True)


def fail(msg):
    failures.append(msg)
    print(f"✗ FAIL: {msg}", flush=True)


def warn(msg):
    warnings.append(msg)
    print(f"⚠ WARN: {msg}", flush=True)


class Client:
    """Minimal ACP client over the agent's stdio."""

    def __init__(self, argv):
        self.proc = subprocess.Popen(
            argv,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
        self.next_id = 0
        self.lock = threading.Lock()
        self.responses = {}          # id -> message
        self.cond = threading.Condition(self.lock)
        self.messages = []           # every message, in arrival order
        self.bad_lines = []          # stdout lines that are not JSON-RPC
        self.updates = queue.Queue()  # session/update params
        self.write_lock = threading.Lock()
        # Editor-side state.
        self.permission_answer = "allow_once"  # optionId to pick, or "cancelled"
        self.permission_requests = []          # session/request_permission params
        self.fs_reads = []                     # paths read through fs/read_text_file
        self.fs_writes = {}                    # path -> content written through fs/write_text_file
        self.terminals = {}                    # terminalId -> Terminal
        self.terminal_requests = []            # terminal/create params
        threading.Thread(target=self._read_stdout, daemon=True).start()
        threading.Thread(target=self._read_stderr, daemon=True).start()

    def _read_stdout(self):
        for raw in self.proc.stdout:
            line = raw.rstrip("\n")
            if not line.strip():
                continue
            try:
                msg = json.loads(line)
            except json.JSONDecodeError:
                self.bad_lines.append(line)
                print(f"  [non-JSON stdout] {line[:200]}", flush=True)
                continue
            if msg.get("jsonrpc") != "2.0":
                self.bad_lines.append(line)
            with self.cond:
                self.messages.append(msg)
                if "id" in msg and ("result" in msg or "error" in msg):
                    self.responses[msg["id"]] = msg
                    self.cond.notify_all()
            if msg.get("method") == "session/update":
                self.updates.put(msg["params"])
                print_update(msg["params"].get("update", {}))
            elif "method" in msg and "id" in msg:
                # An agent -> client request. Some block (wait_for_exit), so
                # each runs on its own thread.
                threading.Thread(target=self._serve, args=(msg,), daemon=True).start()

    def _serve(self, msg):
        method, params, rid = msg["method"], msg.get("params", {}), msg["id"]
        try:
            result = self._handle(method, params)
            self.send({"jsonrpc": "2.0", "id": rid, "result": result}, quiet=True)
        except KeyError as e:
            self.send({"jsonrpc": "2.0", "id": rid,
                       "error": {"code": -32002, "message": "Resource not found", "data": str(e)}}, quiet=True)
        except NotImplementedError:
            self.send({"jsonrpc": "2.0", "id": rid,
                       "error": {"code": -32601, "message": "Method not found", "data": {"method": method}}}, quiet=True)
        except Exception as e:  # noqa: BLE001 - report every failure to the agent
            self.send({"jsonrpc": "2.0", "id": rid,
                       "error": {"code": -32603, "message": "Internal error", "data": str(e)}}, quiet=True)

    def _handle(self, method, p):
        if method == "session/request_permission":
            self.permission_requests.append(p)
            answer = self.permission_answer
            title = p.get("toolCall", {}).get("title")
            print(f"\n  [permission] {title!r} -> {answer}", flush=True)
            if answer == "cancelled":
                return {"outcome": {"outcome": "cancelled"}}
            ids = [o["optionId"] for o in p.get("options", [])]
            if answer not in ids:
                raise ValueError(f"option {answer} not offered: {ids}")
            return {"outcome": {"outcome": "selected", "optionId": answer}}
        if method == "fs/read_text_file":
            path = p["path"]
            self.fs_reads.append(path)
            if path in self.fs_writes:
                text = self.fs_writes[path]
            elif os.path.exists(path):
                with open(path) as f:
                    text = f.read()
            else:
                raise KeyError(path)
            if p.get("line") or p.get("limit"):
                lines = text.splitlines(keepends=True)
                start = max((p.get("line") or 1) - 1, 0)
                end = start + p["limit"] if p.get("limit") else None
                text = "".join(lines[start:end])
            return {"content": text}
        if method == "fs/write_text_file":
            # An editor would write its buffer and save it.
            self.fs_writes[p["path"]] = p["content"]
            with open(p["path"], "w") as f:
                f.write(p["content"])
            print(f"\n  [fs write] {p['path']} ({len(p['content'])} bytes)", flush=True)
            return None
        if method == "terminal/create":
            self.terminal_requests.append(p)
            term = Terminal(p)
            self.terminals[term.id] = term
            print(f"\n  [terminal] {term.id}: {[p['command']] + p.get('args', [])} in {p.get('cwd')}", flush=True)
            return {"terminalId": term.id}
        if method == "terminal/output":
            return self.terminals[p["terminalId"]].output_response()
        if method == "terminal/wait_for_exit":
            return self.terminals[p["terminalId"]].wait()
        if method == "terminal/kill":
            self.terminals[p["terminalId"]].kill()
            return {}
        if method == "terminal/release":
            self.terminals[p["terminalId"]].kill()
            return {}
        raise NotImplementedError(method)

    def _read_stderr(self):
        for line in self.proc.stderr:
            line = line.rstrip()
            if line:
                print(f"  [stderr] {line}", flush=True)

    def send(self, obj, quiet=False):
        line = json.dumps(obj)
        if not quiet:
            print(f"\n→ {line[:300]}", flush=True)
        with self.write_lock:
            self.proc.stdin.write(line + "\n")
            self.proc.stdin.flush()

    def request_async(self, method, params):
        with self.lock:
            self.next_id += 1
            rid = self.next_id
        self.send({"jsonrpc": "2.0", "id": rid, "method": method, "params": params})
        return rid

    def wait(self, rid, timeout):
        deadline = time.time() + timeout
        with self.cond:
            while rid not in self.responses:
                left = deadline - time.time()
                if left <= 0:
                    return None
                self.cond.wait(left)
            msg = self.responses[rid]
        if "error" in msg:
            print(f"← id={rid} ERROR {json.dumps(msg['error'])[:300]}", flush=True)
        else:
            print(f"\n← id={rid} {json.dumps(msg['result'])[:300]}", flush=True)
        return msg

    def request(self, method, params, timeout=30):
        return self.wait(self.request_async(method, params), timeout)

    def notify(self, method, params):
        self.send({"jsonrpc": "2.0", "method": method, "params": params})

    def drain_updates(self):
        out = []
        while True:
            try:
                out.append(self.updates.get_nowait())
            except queue.Empty:
                return out

    def close(self):
        try:
            self.proc.stdin.close()
        except OSError:
            pass
        try:
            self.proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.proc.terminate()
            self.proc.wait(timeout=5)


class Terminal:
    """A terminal/create command run as a local process, like an editor would."""

    _seq = 0

    def __init__(self, p):
        Terminal._seq += 1
        self.id = f"term-{Terminal._seq}"
        self.limit = p.get("outputByteLimit")
        env = dict(os.environ)
        for e in p.get("env") or []:
            env[e["name"]] = e["value"]
        self.proc = subprocess.Popen(
            [p["command"]] + (p.get("args") or []),
            cwd=p.get("cwd") or None, env=env,
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
        )
        self.buf = ""
        self.lock = threading.Lock()
        self.reader = threading.Thread(target=self._read, daemon=True)
        self.reader.start()

    def _read(self):
        for chunk in iter(lambda: self.proc.stdout.read(1), ""):
            with self.lock:
                self.buf += chunk

    def _status(self):
        code = self.proc.poll()
        if code is None:
            return None
        if code < 0:
            return {"exitCode": None, "signal": f"signal {-code}"}
        return {"exitCode": code, "signal": None}

    def output_response(self):
        with self.lock:
            out = self.buf
        truncated = False
        if self.limit and len(out.encode()) > self.limit:
            out, truncated = out.encode()[-self.limit:].decode(errors="ignore"), True
        resp = {"output": out, "truncated": truncated}
        status = self._status()
        if status:
            resp["exitStatus"] = status
        return resp

    def wait(self):
        self.proc.wait()
        self.reader.join(timeout=2)
        return self._status()

    def kill(self):
        if self.proc.poll() is None:
            self.proc.kill()


def print_update(update):
    kind = update.get("sessionUpdate", "?")
    text = (update.get("content") or {}).get("text", "") if isinstance(update.get("content"), dict) else ""
    if kind in ("agent_thought_chunk", "agent_message_chunk", "user_message_chunk"):
        tag = {"agent_thought_chunk": "thinking", "agent_message_chunk": "response",
               "user_message_chunk": "user"}[kind]
        print(f"  [{tag}] {text}", end="", flush=True)
    elif kind in ("tool_call", "tool_call_update"):
        terms = [c["terminalId"] for c in update.get("content") or [] if c.get("type") == "terminal"]
        extra = f" terminal={terms[0]}" if terms else ""
        print(f"\n  [{kind}] {update.get('title', update.get('toolCallId'))} "
              f"kind={update.get('kind', '-')} status={update.get('status', '-')}{extra}", flush=True)
    elif kind == "plan":
        steps = [f"{e.get('status')}: {e.get('content')}" for e in update.get("entries", [])]
        print(f"\n  [plan] {steps}", flush=True)
    elif kind == "available_commands_update":
        names = [c.get("name") for c in update.get("availableCommands", [])]
        print(f"\n  [commands] {names}", flush=True)
    else:
        print(f"\n  [update/{kind}] {json.dumps(update)[:200]}", flush=True)


def kinds(updates):
    return [u.get("update", {}).get("sessionUpdate") for u in updates]


def text_of(updates, kind):
    return "".join(
        (u["update"].get("content") or {}).get("text", "")
        for u in updates
        if u.get("update", {}).get("sessionUpdate") == kind and isinstance(u["update"].get("content"), dict)
    )


def find_option(options, option_id):
    for opt in options or []:
        if opt.get("id") == option_id:
            return opt
    return None


def option_values(opt):
    values = []
    for entry in opt.get("options", []):
        if "group" in entry:
            values.extend(o["value"] for o in entry.get("options", []))
        else:
            values.append(entry.get("value"))
    return values


def check_config_options(options, label):
    if not isinstance(options, list):
        fail(f"{label}: configOptions must be an array, got {options!r}")
        return
    model = find_option(options, "model")
    if not model:
        fail(f"{label}: configOptions has no 'model' option")
        return
    if model.get("type") != "select" or model.get("category") != "model":
        fail(f"{label}: model option has type={model.get('type')} category={model.get('category')}")
    if model.get("currentValue") not in option_values(model):
        fail(f"{label}: model currentValue {model.get('currentValue')} is not one of its options")
        return
    approval = find_option(options, "approval")
    if not approval or approval.get("category") != "mode" or approval.get("currentValue") not in option_values(approval):
        fail(f"{label}: missing or invalid 'approval' option: {approval}")
        return
    ok(f"{label}: configOptions valid (approval={approval.get('currentValue')}, "
       f"model={model.get('currentValue')}, {len(options)} option(s))")


def run_prompt(client, session_id, text, label):
    """Send a prompt, wait for the response and return (response, updates)."""
    client.drain_updates()
    resp = client.request("session/prompt", {
        "sessionId": session_id,
        "prompt": [{"type": "text", "text": text}],
    }, timeout=TIMEOUT)
    time.sleep(0.2)
    updates = [u for u in client.drain_updates() if u.get("sessionId") == session_id]
    if resp is None:
        fail(f"{label}: timed out after {TIMEOUT}s")
    elif "error" in resp:
        fail(f"{label}: error {resp['error']}")
    return resp, updates


def cleanup_sessions(client):
    """Delete the session files this run created, so repeated runs do not
    fill ~/.kit/sessions."""
    ids = set()
    for m in client.messages:
        sid = (m.get("result") or {}).get("sessionId") if isinstance(m.get("result"), dict) else None
        if sid:
            ids.add(sid)
    removed = 0
    for path in glob.glob(os.path.expanduser("~/.kit/sessions/*/*.jsonl")):
        try:
            with open(path) as f:
                header = json.loads(f.readline() or "{}")
        except (OSError, ValueError):
            continue
        if header.get("id") in ids:
            os.remove(path)
            removed += 1
            parent = os.path.dirname(path)
            if not os.listdir(parent):
                os.rmdir(parent)
    print(f"\n(cleanup: removed {removed} session file(s))", flush=True)


def main():
    os.makedirs(CWD, exist_ok=True)
    with open(os.path.join(CWD, MARKER), "w") as f:
        f.write("marker for the kit ACP smoke test\n")
    # A project prompt template, which must show up as a slash command.
    os.makedirs(os.path.join(CWD, ".kit", "prompts"), exist_ok=True)
    with open(os.path.join(CWD, ".kit", "prompts", f"{TEMPLATE}.md"), "w") as f:
        f.write("---\ndescription: Smoke test template\n---\nSay hello to $1.\n")

    print(f"Starting: {KIT_BIN} acp -m {MODEL}  (session cwd: {CWD})", flush=True)
    client = Client([KIT_BIN, "acp", "-m", MODEL])
    try:
        run(client)
    finally:
        client.close()
        for term in client.terminals.values():
            term.kill()
        cleanup_sessions(client)

    if client.bad_lines:
        fail(f"{len(client.bad_lines)} stdout line(s) are not JSON-RPC 2.0 messages")
    else:
        ok("stdout carried only JSON-RPC 2.0 messages")

    print()
    for w in warnings:
        print(f"⚠ {w}")
    if failures:
        print(f"\n✗ SMOKE TEST FAILED ({len(failures)} failure(s))")
        for f in failures:
            print(f"  - {f}")
        sys.exit(1)
    updates = sum(1 for m in client.messages if m.get("method") == "session/update")
    print(f"\n✓ SMOKE TEST PASSED  ({updates} session updates received)")


def tool_updates(updates):
    calls = [u["update"] for u in updates if u["update"].get("sessionUpdate") == "tool_call"]
    finals = [u["update"] for u in updates if u["update"].get("sessionUpdate") == "tool_call_update"
              and u["update"].get("status") in ("completed", "failed")]
    return calls, finals


def run_client_tool_steps(client, session_id):
    """Steps that need an editor client: permissions, terminals, fs, plans."""
    # ── shell: permission + client terminal ─────────────────────────────
    marker = f"KIT_SMOKE_{random.randint(10000, 99999)}"
    client.permission_answer = "allow_once"
    asked = len(client.permission_requests)
    terms = len(client.terminal_requests)
    failed_before = len(failures)
    resp, updates = run_prompt(
        client, session_id,
        f"Use the shell tool to run exactly this command: echo {marker}\nThen reply with only its output.",
        "shell prompt")
    if resp and "result" in resp:
        new_perms = client.permission_requests[asked:]
        new_terms = client.terminal_requests[terms:]
        calls, finals = tool_updates(updates)
        shell_perms = [p for p in new_perms if p.get("toolCall", {}).get("kind") == "execute"]
        if not shell_perms:
            fail("shell prompt: no session/request_permission for the shell tool in 'ask' mode")
        elif {o.get("kind") for o in shell_perms[0].get("options", [])} != {"allow_once", "allow_always", "reject_once", "reject_always"}:
            fail(f"shell prompt: permission options = {shell_perms[0].get('options')}")
        if not new_terms:
            fail("shell prompt: the command did not run in a client terminal (no terminal/create)")
        elif new_terms[0].get("cwd") != CWD:
            fail(f"shell prompt: terminal cwd = {new_terms[0].get('cwd')}, want {CWD}")
        embedded = any(c.get("type") == "terminal" for f in finals for c in f.get("content") or [])
        if not embedded:
            fail("shell prompt: the terminal is not embedded in the finished tool call")
        statuses = {}
        for u in updates:
            up = u["update"]
            if up.get("sessionUpdate") in ("tool_call", "tool_call_update") and up.get("status"):
                statuses.setdefault(up["toolCallId"], []).append(up["status"])
        seq = next(iter(statuses.values()), [])
        if seq[:2] != ["pending", "in_progress"]:
            fail(f"shell prompt: tool statuses {seq}, want pending -> in_progress -> ...")
        answer = text_of(updates, "agent_message_chunk")
        if marker not in answer:
            warn(f"shell prompt: answer does not show the command output {marker}: {answer[:200]!r}")
        if len(failures) == failed_before:
            ok(f"shell prompt: permission asked, ran in a client terminal, statuses {seq}")

    # ── write: permission + fs/write_text_file ──────────────────────────
    target = os.path.join(CWD, "smoke_notes.txt")
    if os.path.exists(target):
        os.remove(target)
    asked = len(client.permission_requests)
    resp, updates = run_prompt(
        client, session_id,
        "Use the write tool to create the file smoke_notes.txt with the content: written by the smoke test",
        "write prompt")
    if resp and "result" in resp:
        edit_perms = [p for p in client.permission_requests[asked:] if p.get("toolCall", {}).get("kind") == "edit"]
        if not edit_perms:
            fail("write prompt: no permission request for the write tool in 'ask' mode")
        if target not in client.fs_writes:
            fail(f"write prompt: file not written through fs/write_text_file (writes: {list(client.fs_writes)})")
        else:
            ok("write prompt: permission asked, file written through fs/write_text_file")

    # ── rejected tool call ──────────────────────────────────────────────
    forbidden = os.path.join(CWD, "must_not_exist.txt")
    if os.path.exists(forbidden):
        os.remove(forbidden)
    client.permission_answer = "reject_once"
    asked = len(client.permission_requests)
    resp, updates = run_prompt(
        client, session_id,
        "Use the shell tool to run: touch must_not_exist.txt\nIf the tool call is rejected, do not retry; just say so.",
        "reject prompt")
    client.permission_answer = "allow_once"
    if resp and "result" in resp:
        _, finals = tool_updates(updates)
        if len(client.permission_requests) == asked:
            fail("reject prompt: no permission request")
        if os.path.exists(forbidden):
            fail("reject prompt: the rejected command ran")
        if resp["result"].get("stopReason") != "end_turn":
            fail(f"reject prompt: stopReason={resp['result'].get('stopReason')}, want end_turn")
        if not any(f.get("status") == "failed" for f in finals):
            fail("reject prompt: rejected tool call not marked failed")
        else:
            ok("reject prompt: rejected command did not run; tool failed; turn ended normally")

    # ── plan ─────────────────────────────────────────────────────────────
    resp, updates = run_prompt(
        client, session_id,
        "Call the update_plan tool once with a 3-step plan for writing a haiku about the sea "
        "(first step in_progress, others pending). Do nothing else, then reply 'planned'.",
        "plan prompt")
    if resp and "result" in resp:
        plans = [u["update"] for u in updates if u["update"].get("sessionUpdate") == "plan"]
        shown_as_tool = [u for u in updates if u["update"].get("sessionUpdate") == "tool_call"
                         and "update_plan" in json.dumps(u["update"])]
        if shown_as_tool:
            fail("plan prompt: update_plan was shown as a tool call instead of a plan")
        if not plans:
            warn("plan prompt: the model did not call update_plan (no plan update)")
        elif not plans[-1].get("entries"):
            fail("plan prompt: plan update has no entries")
        else:
            ok(f"plan prompt: plan update with {len(plans[-1]['entries'])} step(s)")


def run(client):
    # ── 1. initialize ────────────────────────────────────────────────────
    resp = client.request("initialize", {
        "protocolVersion": 1,
        "clientCapabilities": {"fs": {"readTextFile": True, "writeTextFile": True}, "terminal": True},
        "clientInfo": {"name": "acp-smoke-test", "title": "ACP smoke test", "version": "2.0.0"},
    }, timeout=15)
    if not resp or "error" in resp:
        fail(f"initialize failed: {resp}")
        return
    init = resp["result"]
    caps = init.get("agentCapabilities", {})
    session_caps = caps.get("sessionCapabilities", {})
    if init.get("protocolVersion") != 1:
        fail(f"initialize: protocolVersion {init.get('protocolVersion')}, want 1")
    if not caps.get("loadSession"):
        fail("initialize: loadSession not advertised")
    for cap in ("list", "close", "resume"):
        if cap not in session_caps:
            fail(f"initialize: sessionCapabilities.{cap} not advertised")
    if not isinstance(init.get("authMethods", []), list):
        fail("initialize: authMethods must be an array")
    info = init.get("agentInfo", {})
    ok(f"initialized: protocol={init.get('protocolVersion')} agent={info.get('name')} v{info.get('version')} "
       f"mcp={caps.get('mcpCapabilities')} prompt={caps.get('promptCapabilities')}")

    # ── 2. session/new ───────────────────────────────────────────────────
    resp = client.request("session/new", {"cwd": CWD, "mcpServers": []}, timeout=60)
    if not resp or "error" in resp or not resp["result"].get("sessionId"):
        fail(f"session/new failed: {resp}")
        return
    session_id = resp["result"]["sessionId"]
    ok(f"session/new: sessionId={session_id}")
    check_config_options(resp["result"].get("configOptions"), "session/new")
    time.sleep(0.5)
    cmds = [m["params"]["update"] for m in list(client.messages) if m.get("method") == "session/update"
            and m["params"].get("sessionId") == session_id
            and m["params"]["update"].get("sessionUpdate") == "available_commands_update"]
    listed = cmds[-1].get("availableCommands", []) if cmds else []
    bad = [c for c in listed if not c.get("name") or not c.get("description")]
    greet = next((c for c in listed if c.get("name") == TEMPLATE), None)
    if not cmds:
        fail("no available_commands_update after session/new")
    elif bad:
        fail(f"available_commands_update: commands without name/description: {bad}")
    elif not greet or not greet.get("input"):
        fail(f"available_commands_update: project template /{TEMPLATE} missing or without input hint")
    else:
        ok(f"available_commands_update: {len(listed)} command(s), including /{TEMPLATE}")

    # ── 3. session/set_config_option (model) ────────────────────────────
    resp = client.request("session/set_config_option", {
        "sessionId": session_id, "configId": "model", "value": MODEL,
    })
    if not resp or "error" in resp:
        fail(f"session/set_config_option failed: {resp}")
    else:
        check_config_options(resp["result"].get("configOptions"), "session/set_config_option")

    # ── 4. unknown config option is rejected ────────────────────────────
    resp = client.request("session/set_config_option", {
        "sessionId": session_id, "configId": "no_such_option", "value": "x",
    })
    if resp and resp.get("error", {}).get("code") == -32602:
        ok("unknown config option rejected with InvalidParams")
    else:
        fail(f"unknown config option: want InvalidParams error, got {resp}")

    # ── 5. session/set_mode is rejected (no modes advertised) ───────────
    resp = client.request("session/set_mode", {"sessionId": session_id, "modeId": "code"})
    if resp and "error" in resp:
        ok("session/set_mode rejected (Kit has no modes)")
    else:
        fail(f"session/set_mode: want an error, got {resp}")

    # ── 6. session/prompt (plain text) ──────────────────────────────────
    resp, updates = run_prompt(client, session_id, "What is 2+2? Answer in one sentence.", "prompt")
    if resp and "result" in resp:
        stop = resp["result"].get("stopReason")
        answer = text_of(updates, "agent_message_chunk")
        if stop != "end_turn":
            fail(f"prompt: stopReason={stop}, want end_turn")
        if not answer.strip():
            fail("prompt: no agent_message_chunk text streamed")
        elif "4" not in answer and "four" not in answer.lower():
            warn(f"prompt: answer does not mention 4: {answer!r}")
        if "agent_thought_chunk" not in kinds(updates):
            warn("prompt: no agent_thought_chunk (fine for non-reasoning models)")
        ok(f"prompt: stopReason={stop}, {kinds(updates).count('agent_message_chunk')} message chunk(s), "
           f"{kinds(updates).count('agent_thought_chunk')} thought chunk(s)")

    # ── 7. session/prompt with a read-only tool in the session cwd ──────
    # ls is read-only, so it must run without a permission request.
    asked_before_ls = len(client.permission_requests)
    if SKIP_TOOLS:
        warn("tool-call step skipped (SKIP_TOOLS=1)")
    else:
        resp, updates = run_prompt(
            client, session_id,
            "Use the ls tool to list the current directory, then reply with only the file names you saw.",
            "tool prompt")
        if resp and "result" in resp:
            calls = [u["update"] for u in updates if u["update"].get("sessionUpdate") == "tool_call"]
            done = [u["update"] for u in updates if u["update"].get("sessionUpdate") == "tool_call_update"
                    and u["update"].get("status") in ("completed", "failed")]
            if not calls:
                fail("tool prompt: no tool_call update")
            else:
                c = calls[0]
                if not c.get("title") or not c.get("toolCallId"):
                    fail(f"tool prompt: tool_call without title/toolCallId: {c}")
                if c.get("kind") not in ("read", "edit", "delete", "move", "search", "execute", "think", "fetch", "other"):
                    fail(f"tool prompt: invalid tool kind {c.get('kind')}")
                ids = {x["toolCallId"] for x in calls}
                if not ids & {x.get("toolCallId") for x in done}:
                    fail("tool prompt: no tool_call_update finished the tool call")
                output = json.dumps(done)
                if len(client.permission_requests) != asked_before_ls:
                    fail("tool prompt: the read-only ls tool asked for permission")
                if MARKER not in output:
                    fail(f"tool prompt: tool output does not show {MARKER}; tools did not run in the session cwd")
                else:
                    ok(f"tool prompt: {len(calls)} tool call(s), ran in session cwd, kind={c.get('kind')}")
            if resp["result"].get("stopReason") != "end_turn":
                fail(f"tool prompt: stopReason={resp['result'].get('stopReason')}, want end_turn")
            msg = text_of(updates, "agent_message_chunk")
            for chunk in {t for t in msg.split("\n") if len(t) > 20}:
                if msg.count(chunk) > 1:
                    warn(f"tool prompt: repeated agent text (possible duplicate chunks): {chunk!r}")
                    break

    if not SKIP_TOOLS:
        run_client_tool_steps(client, session_id)

    # ── 8. cancellation ─────────────────────────────────────────────────
    client.drain_updates()
    rid = client.request_async("session/prompt", {
        "sessionId": session_id,
        "prompt": [{"type": "text", "text": "Write the numbers from 1 to 300, one per line, as words."}],
    })
    deadline = time.time() + TIMEOUT
    while time.time() < deadline:
        pending = client.drain_updates()
        if any(k in ("agent_message_chunk", "agent_thought_chunk") for k in kinds(pending)):
            break
        with client.lock:
            if rid in client.responses:
                break
        time.sleep(0.1)
    client.notify("session/cancel", {"sessionId": session_id})
    resp = client.wait(rid, 30)
    if resp and resp.get("result", {}).get("stopReason") == "cancelled":
        ok("session/cancel: prompt ended with stopReason=cancelled")
    elif resp and "result" in resp:
        warn(f"session/cancel: prompt ended before the cancel arrived (stopReason={resp['result'].get('stopReason')})")
    else:
        fail(f"session/cancel: want stopReason cancelled, got {resp}")

    # ── 9. session/list ─────────────────────────────────────────────────
    resp = client.request("session/list", {"cwd": CWD})
    if not resp or "error" in resp:
        fail(f"session/list failed: {resp}")
    else:
        sessions = resp["result"].get("sessions")
        if not isinstance(sessions, list):
            fail("session/list: sessions must be an array")
        elif session_id not in [s.get("sessionId") for s in sessions]:
            fail(f"session/list: session {session_id} not listed ({len(sessions)} listed)")
        else:
            entry = next(s for s in sessions if s.get("sessionId") == session_id)
            if entry.get("cwd") != CWD:
                fail(f"session/list: cwd={entry.get('cwd')}, want {CWD}")
            ok(f"session/list: {len(sessions)} session(s), ours has title={entry.get('title')!r}")

    # ── 10. session/close ───────────────────────────────────────────────
    resp = client.request("session/close", {"sessionId": session_id})
    if not resp or "error" in resp:
        fail(f"session/close failed: {resp}")
    else:
        ok("session/close accepted")
        resp = client.request("session/prompt", {
            "sessionId": session_id, "prompt": [{"type": "text", "text": "hi"}]})
        if resp and "error" in resp:
            ok("prompt on a closed session is rejected")
        else:
            fail(f"prompt on a closed session: want an error, got {resp}")

    # ── 11. session/load replays the history ────────────────────────────
    client.drain_updates()
    rid = client.request_async("session/load", {"sessionId": session_id, "cwd": CWD, "mcpServers": []})
    resp = client.wait(rid, 60)
    if not resp or "error" in resp:
        fail(f"session/load failed: {resp}")
    else:
        # Updates must arrive before the response: check arrival order.
        with client.lock:
            idx = next(i for i, m in enumerate(client.messages) if m.get("id") == rid and "result" in m)
            before = [m for m in client.messages[:idx] if m.get("method") == "session/update"
                      and m["params"].get("sessionId") == session_id]
        replay = [m["params"] for m in before]
        user_text = text_of(replay, "user_message_chunk")
        if "2+2" not in user_text:
            fail(f"session/load: first user message not replayed (user text: {user_text[:100]!r})")
        elif "agent_message_chunk" not in kinds(replay):
            fail("session/load: agent messages not replayed")
        else:
            ok(f"session/load: replayed {len(replay)} update(s) before the response")
        check_config_options(resp["result"].get("configOptions"), "session/load")

        # ── 12. the loaded session keeps its context ────────────────────
        resp, updates = run_prompt(client, session_id,
                                   "What was my first question in this conversation? Quote it.",
                                   "prompt after load")
        if resp and "result" in resp:
            answer = text_of(updates, "agent_message_chunk")
            if "2" not in answer:
                warn(f"prompt after load: answer does not quote the first question: {answer[:200]!r}")
            else:
                ok("prompt after load: context kept")

    # ── 13. relative cwd is rejected ─────────────────────────────────────
    resp = client.request("session/new", {"cwd": "relative/dir", "mcpServers": []})
    if resp and resp.get("error", {}).get("code") == -32602:
        ok("session/new with a relative cwd rejected with InvalidParams")
    else:
        fail(f"session/new with a relative cwd: want InvalidParams, got {resp}")

    # session/cancel for an idle session is a no-op notification.
    client.notify("session/cancel", {"sessionId": session_id})


if __name__ == "__main__":
    main()
