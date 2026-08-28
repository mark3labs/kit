# kit-tunnel

Transport sidecar for kit remote sessions. Owns everything iroh so the Go
side of kit needs no iroh code: endpoint binding, DNS/relay discovery, the
pairing handshake, and frame relay between the network and its own stdio.

## Role in the system

```
kit --remote CODE ──iroh──► kit-tunnel dial   ⇄ stdio ⇄ kit (client)
kit daemon        ──spawns──► kit-tunnel serve ⇄ stdio ⇄ kit daemon ⇄ PTY ⇄ kit --pick-dir (per session)
```

- `serve` binds an endpoint whose keypair is derived from the pairing code's
  seed, so the code itself is what makes the endpoint findable. It accepts
  MULTIPLE connections over that one endpoint (protocol v2): each verified
  peer gets a session id, announced to the Go daemon with `SESSION_OPEN`
  and retired with `SESSION_CLOSED`. One PTY child per session lives on the
  Go side; the code stays valid for the daemon's lifetime.
- Failed handshakes back off exponentially (500ms per consecutive failure,
  capped at 8s) — the old rotate-per-attempt rate limit is gone with the
  persistent endpoint. Concurrent sessions are capped (currently 8); extra
  peers are denied with "too many sessions".
- `dial` derives the daemon's endpoint id from the same seed, connects,
  verifies the mutual HMAC handshake, receives its session id
  (`SESSION_ASSIGN`), then pumps frames.
- After the handshake both sides relay `DATA`/`RESIZE`/`BYE` frames,
  multiplexed by session id on the serve side; the handshake and
  session-assignment frames never leave the tunnel.

The QUIC transport is tuned with a 5s keep-alive and a 20s idle timeout so a
silently vanished peer is detected in seconds.

## Interface

```
kit-tunnel serve --seed-hex <32-byte hex> [--timeout secs]
kit-tunnel dial  --seed-hex <32-byte hex> [--timeout secs]
```

- stdin/stdout carry the framed relay bytes (7-byte header: type u8,
  session id u32 big-endian, payload length u16 big-endian; session 0 on the
  client's stdio — see kit's `internal/daemon/protocol.go`).
- stderr carries `STATUS ...` lifecycle lines: `READY node_id=...`,
  `PAIRING`, `VERIFIED`, `DENIED reason=...`, `CLOSED`, `ERROR msg=...`.
- Set `RUST_LOG=debug` to add iroh traces to stderr.

The seed is `HKDF-SHA256(code, salt="kit-remote-v1", info="kit-remote tunnel seed")`,
derived in kit's `internal/daemon/pairing.go`; the auth key used for the
handshake is expanded from the seed with info `"kit-remote auth"`. Both
sides must agree on these domain separation strings.

## Build

```
cargo build --release
cp target/release/kit-tunnel <dir containing the kit binary>
```

kit locates the sidecar via `KIT_TUNNEL_BIN`, then next to its own binary,
then on `PATH`.
