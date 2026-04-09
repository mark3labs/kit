#!/usr/bin/env python3
"""Minimal MCP server over stdio for testing. Exposes one tool: echo."""
import json
import sys


def read_message():
    """Read a JSON-RPC message from stdin."""
    line = sys.stdin.readline()
    if not line:
        return None
    return json.loads(line.strip())


def write_message(msg):
    """Write a JSON-RPC message to stdout."""
    sys.stdout.write(json.dumps(msg) + "\n")
    sys.stdout.flush()


def handle(msg):
    method = msg.get("method", "")
    mid = msg.get("id")

    if method == "initialize":
        write_message({
            "jsonrpc": "2.0",
            "id": mid,
            "result": {
                "protocolVersion": "2024-11-05",
                "capabilities": {"tools": {}},
                "serverInfo": {"name": "test-echo", "version": "1.0.0"},
            },
        })
    elif method == "notifications/initialized":
        pass  # no response needed
    elif method == "tools/list":
        write_message({
            "jsonrpc": "2.0",
            "id": mid,
            "result": {
                "tools": [
                    {
                        "name": "echo",
                        "description": "Echoes the input text back.",
                        "inputSchema": {
                            "type": "object",
                            "properties": {
                                "text": {"type": "string", "description": "Text to echo"}
                            },
                            "required": ["text"],
                        },
                    },
                    {
                        "name": "greet",
                        "description": "Returns a greeting.",
                        "inputSchema": {
                            "type": "object",
                            "properties": {
                                "name": {"type": "string", "description": "Name to greet"}
                            },
                            "required": ["name"],
                        },
                    },
                ]
            },
        })
    elif method == "tools/call":
        tool_name = msg["params"]["name"]
        args = msg["params"].get("arguments", {})
        if tool_name == "echo":
            text = args.get("text", "")
            write_message({
                "jsonrpc": "2.0",
                "id": mid,
                "result": {
                    "content": [{"type": "text", "text": text}]
                },
            })
        elif tool_name == "greet":
            name = args.get("name", "World")
            write_message({
                "jsonrpc": "2.0",
                "id": mid,
                "result": {
                    "content": [{"type": "text", "text": f"Hello, {name}!"}]
                },
            })
        else:
            write_message({
                "jsonrpc": "2.0",
                "id": mid,
                "error": {"code": -32601, "message": f"Unknown tool: {tool_name}"},
            })
    elif method == "ping":
        write_message({"jsonrpc": "2.0", "id": mid, "result": {}})
    else:
        if mid is not None:
            write_message({
                "jsonrpc": "2.0",
                "id": mid,
                "error": {"code": -32601, "message": f"Unknown method: {method}"},
            })


if __name__ == "__main__":
    while True:
        msg = read_message()
        if msg is None:
            break
        handle(msg)
