# kit-tunnel

Transport sidecar for kit remote sessions. Owns everything iroh so the Go
side of kit needs no iroh code: endpoint binding, DNS/relay discovery, the
pairing and reconnect handshakes, and frame relay between the network and
its own stdio.

The sidecar is only needed for **remote** sessions. `kit daemon` runs
without it and still hosts local sessions over its Unix socket, logging a
warning instead of failing — so a machine with no Rust toolchain can still
run detachable sessions with `kit attach`. `kit daemon pair` and
`kit remote` do require it, and report the same missing-binary error.

## Role in the system

```text
# pairing (one-time, human-approved on the host)
kit remote --pair CODE ──iroh──► kit-tunnel dial-pair    ⇄ stdio ⇄ kit remote (client)
                                 ◄── bootstrap endpoint ── kit-tunnel serve-pair ⇄ kit daemon pair (host)

# sessions (after pairing)
kit remote --host NAME ─► kit-tunnel dial-host ──iroh──► kit-tunnel serve ⇄ stdio ⇄ kit daemon ⇄ PTY ⇄ kit --pick-dir (per session)
```

- The daemon owns a **stable ed25519 identity** (`--secret-hex`); its
  public half is the iroh endpoint id that clients store at pairing time,
  and the QUIC handshake authenticates the daemon against it. Client
  reconnects authenticate by ed25519 signature over the handshake
  transcript; the signature is checked by the Go daemon against its
  allowlist via the `AUTH_REQUEST`/`AUTH_DECISION` stdio consultation —
  authentication policy never lives in the sidecar.
- `serve-pair` runs the **pairing window**: a bootstrap endpoint derived
  from a one-time code (HKDF-SHA256 over the code, salt
  `"kit-remote-v1"`, info `"kit-remote tunnel seed"`). The caller proves
  code knowledge with an HMAC tag, the Go daemon prompts the human, and an
  accepted pairing hands the client the daemon's stable endpoint id. The
  window serves attempts concurrently (a rejection, an abandoned client or
  a failed handshake leaves it open, under a shared guess backoff) and
  closes on the first successful pairing, which burns the code.
- `serve` hosts sessions on the stable endpoint: each authenticated peer
  gets a session id, announced to the Go daemon with `SESSION_OPEN` and
  retired with `SESSION_CLOSED`. One PTY child per session lives on the Go
  side. Concurrent sessions are capped (currently 8) and polite rejections
  of over-cap peers are budgeted (32), so connection floods cannot pin
  resources.
- After the handshake both sides relay `DATA`/`RESIZE`/`BYE`/`CLIPBOARD`
  and the session-lifecycle frames (`SESSION_DETACH`, `SESSION_LIST`,
  `SESSION_LIST_REPLY`, `SESSION_ATTACH`, `SESSION_ATTACH_ACK`) verbatim,
  multiplexed by session id on the serve side. Sessions are logical on the
  Go side: they outlive client connections, so clients can detach and
  reattach, and several clients can share one session. `SESSION_OPEN`
  announces a new client connection to the daemon (registration only —
  spawning happens when the client attaches). Handshake, session-
  assignment, auth-consultation and pairing frames never leave the
  tunnel.

Failed handshakes and failed pairings back off exponentially (500ms per
consecutive failure, capped at 8s).

The QUIC transport is tuned with a 5s keep-alive and a 20s idle timeout so
a silently vanished peer is detected in seconds.

## Interface

```
kit-tunnel serve      --secret-hex <64 hex> [--timeout secs]
                      (env: KIT_TUNNEL_SECRET)
kit-tunnel serve-pair --pair-seed-hex <64 hex> [--timeout secs]
                      (env: KIT_TUNNEL_PAIR_SEED)
kit-tunnel dial-pair  --pair-seed-hex <64 hex> --client-pub-hex <64 hex> [--timeout secs]
                      (env: KIT_TUNNEL_PAIR_SEED)
kit-tunnel dial-host  --endpoint-id <64 hex> --client-seed-hex <64 hex> [--timeout secs]
                      (env: KIT_TUNNEL_CLIENT_SEED)
```

- Key material travels in the child's **environment**, never in argv
  (argv is world-readable via ps); the direct hex flags exist for manual
  runs and tests, and each mode honors only its own flag.
- stdin/stdout carry the framed relay bytes (7-byte header: type u8,
  session id u32 big-endian, payload length u16 big-endian; session 0 on
  the client's stdio — see kit's `internal/daemon/protocol.go`).
- stderr carries `STATUS ...` lifecycle lines: `READY node_id=...`,
  `READY_PAIR node_id=...`, `PAIR_REQUEST fp=...`, `PAIRED
  host_endpoint_id=...`, `PAIR_DENIED reason=...`, `PAIRING id=...`,
  `VERIFIED id=...`, `DENIED reason=...`, `SESSION_OPEN id=...`,
  `SESSION_CLOSED id=...`, `CLOSED`, `ERROR msg=...`.
- Set `RUST_LOG=debug` to add iroh traces to stdout (stderr stays reserved
  for `STATUS` lines).

Signature and tag domain strings (`kit-remote-v3-auth`,
`kit-pair-client`, `kit-pair-server`) and the HKDF parameters above are
shared with kit's `internal/daemon/pairing.go`; both sides must agree on
them exactly.

## Build

```
cargo build --release
cp target/release/kit-tunnel <dir containing the kit binary>
```

kit locates the sidecar in this order: `KIT_TUNNEL_BIN`, next to the kit
executable, a repository build
(`contrib/kit-tunnel/target/release/kit-tunnel`), the embedded copy staged
into the kit build and extracted to the user cache dir, then `PATH`.
