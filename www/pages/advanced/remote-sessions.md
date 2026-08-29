---
title: Remote Sessions
description: Run Kit on one machine and drive it from another over an end-to-end encrypted iroh connection.
---

# Remote Sessions

Kit can run as a daemon on one machine and be driven from another. All work —
the agent, tools, extensions, sessions — happens on the daemon host; your
local terminal just renders it. The transport is [iroh](https://iroh.computer):
a direct, end-to-end encrypted QUIC connection that holes through NATs and
falls back to relays.

```bash
# On the machine that does the work:
kit daemon
#   Pairing code: A1B2-C3D4

# On the machine you are sitting at:
kit --remote A1B2C3D4
```

On connection the remote peer picks a working directory (the picker starts in
the daemon user's home directory), and the session TUI starts there. The
session behaves exactly like a local one: extensions, widgets, tool
rendering, and session persistence all run on the daemon host.

## Requirements

- Both machines run a recent `kit` build (the transport sidecar is embedded
  in release binaries; source builds need `task tunnel` once).
- Outbound internet access for iroh discovery and, when a direct path cannot
  be punched, the n0 relay fleet.

## Commands

| Command | Purpose |
|---------|---------|
| `kit daemon` | Start the daemon and print the pairing code |
| `kit daemon status` | Show code, endpoint, uptime and active sessions of a running daemon |
| `kit daemon service install` | Install and start a systemd user service |
| `kit daemon service remove` | Stop and uninstall the service |
| `kit --remote CODE` | Attach this terminal to a daemon session |

Useful daemon flags: `--code ABCD2345` pins a specific pairing code
(hidden, mainly for tests).

## Multiple sessions

Each verified client gets its own session with its own working directory
choice. Exiting a session (`/quit`) closes only that client's connection;
detaching with `Ctrl-]` keeps the session running until it is reaped by
its own timeout. One pairing code stays valid for the whole daemon run, so
teammates (or your other machines) can attach while you are working.

## systemd

```bash
kit daemon service install   # writes ~/.config/systemd/user/kit.service, enables + starts it
kit daemon status            # shows the live pairing code
systemctl --user status kit  # manage the service directly
kit daemon service remove    # stop and uninstall
```

`install` captures provider credentials from your current shell
(`*_API_KEY`, `*_TOKEN`, `PROVIDER_*`, and similar) into
`~/.config/kit/daemon.env`, which the unit loads via `EnvironmentFile`. Edit
that file and run `systemctl --user restart kit` when keys change.

## Security model

- The pairing code is 8 characters from a 32-symbol alphabet (~40 bits of
  entropy). It stays valid for the daemon's lifetime — treat it like a
  password.
- The daemon's endpoint identity is derived from the code: without it, a
  peer cannot even find the endpoint. Connections are additionally
  authenticated with a mutual HMAC handshake; failed attempts back off
  exponentially.
- Session slots are capped, and polite rejections of over-cap peers are
  budgeted so connection floods cannot pin daemon resources.
- Only one daemon may run per user (enforced with a `flock`); state lives in
  `~/.cache/kit/daemon/`.

## Troubleshooting

| Symptom | Cause and fix |
|---------|---------------|
| `another instance is already running` | A daemon (or service) already holds the lock — see `kit daemon status` |
| `no daemon is live for this pairing code` | The daemon restarted or stopped; get the current code from `kit daemon status` |
| `could not reach the daemon` | Network or relay issue; check connectivity on both sides |
| Session dies with `API key not provided` | The daemon environment is missing provider keys — for the systemd service, edit `~/.config/kit/daemon.env` and restart |
| Detached by accident | Just reconnect with the same code; the daemon is still running and sessions persist on the daemon host |
