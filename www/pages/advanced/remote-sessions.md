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

Access is **pairing-based**. A client pairs with the host once — with a
one-time code and an explicit accept/reject on the host's terminal — and
from then on reconnects by name with its own signing key. For normal
reconnects no code is needed, and the host can revoke any client at any
time. (If the host's daemon identity file is deleted, its endpoint id
rotates and every client pairs again.)

```bash
# On the host: start the daemon
kit daemon

# On the host: open a pairing window (shows a one-time code)
kit daemon pair

# On the client: pair (the host terminal asks you to accept)
kit remote --pair A1B2C3D4
Save this host as [workstation]: homelab

# On the client: connect — no code needed, ever again
kit remote --host homelab
```

On connection the daemon reports its **live sessions**; when any exist, an
in-client picker lets you attach to one (the session's screen repaints
exactly where it left off) or start a new one — a fresh child shows the
working-directory picker (starting in the daemon user's home directory).
The session behaves exactly like a local one: extensions, widgets, tool
rendering, and session persistence all run on the daemon host.

## Requirements

- Both machines run a recent `kit` build (the transport sidecar is embedded
  in release binaries; source builds need `task tunnel` once).
- Outbound internet access for iroh discovery and, when a direct path cannot
  be punched, the n0 relay fleet.

## Commands

| Command | Side | Purpose |
|---------|------|---------|
| `kit daemon` | host | Host sessions for paired clients |
| `kit daemon pair` | host | Open a 10-minute pairing window; confirm requests on this terminal |
| `kit daemon pair --list` | host | List paired clients with fingerprints |
| `kit daemon pair --revoke <fp>` | host | Revoke a paired client |
| `kit daemon status` | host | Endpoint, paired clients, active sessions |
| `kit remote --pair <code>` | client | Pair with a host and save it under a name |
| `kit remote --host <name>` | client | Connect to a paired host |
| `kit remote --list` | client | List saved hosts |
| `kit remote --forget <name>` | client | Forget a saved host |

`Ctrl+X d` **detaches**: your terminal returns to the local shell and the
session keeps running on the host — type `/quit` inside the session to end
it for good. Detached sessions are listed on the next connect, so you can
pick up exactly where you left off.

Several clients can also **attach to the same session** (tmux-style shared
view): every attached terminal mirrors the same screen, keystrokes from any
client go into the shared session, and the PTY is resized to the smallest
attached window. Pairing is the authorization — any paired client may list
and attach to any session. Attach rights end where pairing ends: revoking a
client cuts off all of its sessions.

## How pairing works

1. `kit daemon pair` generates a fresh one-time code and opens a bootstrap
   endpoint for **10 minutes** (or until one client pairs).
2. `kit remote --pair <code>` proves knowledge of the code and presents the
   client's signing public key.
3. The host terminal shows the request (`client fp=379d…8510`) and asks
   **Accept? [y/N]** — the default is reject. Requests arriving while no
   terminal can confirm (e.g. the service runs headless) are always denied.
4. On accept, the client's public key joins the host's allowlist
   (`~/.config/kit/daemon/authorized.json`), and the client stores the
   host's endpoint id (`~/.config/kit/remote/hosts.json`). The code is
   burned.

The code itself never grants access: it only makes the pairing window
reachable, and a human still has to approve. Pairing requests that fail the
code check never reach the prompt.

## How reconnection works

`kit remote --host <name>` dials the stored endpoint id and signs the
handshake with the client's private signing key; the host verifies the
signature against its allowlist. iroh's QUIC handshake additionally
authenticates the daemon against the stored endpoint id, so a malicious or
poisoned endpoint cannot impersonate the host.

## Clipboard images

`Ctrl-V` in a remote session attaches an image from the **client
machine's** clipboard: the client reads its own clipboard (via xclip /
wl-paste / osascript), streams the image over the tunnel, and the daemon
hands it to the session as a pending attachment — the same inline
thumbnail preview and `[N image(s) attached]` indicator a local paste
gets. Add your text and submit; `Ctrl-U` clears it.

- The clipboard tools must be installed on the **client** machine; the
  daemon host needs none.
- `Ctrl-V` with no image on the clipboard is forwarded to the session
  unchanged, so host-side `Ctrl-V` behavior (e.g. quoted-insert in an
  embedded shell) is preserved.
- The preview's rendering quality follows the same terminal detection as
  local sessions — the daemon probes the real terminal through the
  connection, so a kitty client gets true graphics and anything else gets
  the half-block thumbnail. `KIT_IMAGE_PROTOCOL` works here too.
- Works in kitty (which reports `Ctrl-V` through the kitty keyboard
  protocol) and in legacy terminals alike.

## Security notes

- The client's signing key lives in `~/.config/kit/remote/identity.key`
  (0600). The host stores only public keys — there are no shared secrets.
- Deleting the host's `~/.config/kit/daemon/identity.key` changes its
  endpoint id; every client must pair again.
- Revocation is immediate and one-sided: `kit daemon pair --revoke <fp>`
  (prefix matching works; ambiguous prefixes are refused).
- The daemon holds a per-user lock; a second instance refuses to start. See
  `kit daemon status`.

## systemd

```bash
kit daemon service install   # writes ~/.config/systemd/user/kit.service, enables + starts it
kit daemon status            # endpoint, paired clients, active sessions
systemctl --user status kit  # manage the service directly
kit daemon service remove    # stop and uninstall
```

`install` captures provider credentials from your current shell
(`*_API_KEY`, `*_TOKEN`, `PROVIDER_*`, and similar) into
`~/.config/kit/daemon.env`, which the unit loads via `EnvironmentFile`. Edit
that file and run `systemctl --user restart kit` when keys change. The
service runs without a terminal, so pairing requests are denied while it is
the only thing running — pair interactively with `kit daemon pair` (the
allowlist is shared).

## Troubleshooting

| Symptom | Cause and fix |
|---------|---------------|
| `another instance is already running` | A daemon (or service) already holds the lock — see `kit daemon status` |
| `no daemon is live for this pairing code` | The pairing window expired or a client already paired; open a new one with `kit daemon pair` |
| `the pairing request was rejected on the host` | The request reached the host and was declined; ask the host user to rerun `kit daemon pair` |
| `the host no longer knows this machine` | The client was revoked — pair again |
| `could not reach the daemon` | Network or relay issue; check connectivity on both sides |
| Session dies with `API key not provided` | The daemon environment is missing provider keys — for the systemd service, edit `~/.config/kit/daemon.env` and restart |
| Detached by accident | Just reconnect with `kit remote --host <name>`; the daemon keeps running |
