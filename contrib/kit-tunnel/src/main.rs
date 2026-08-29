//! kit-tunnel — transport sidecar for kit remote sessions.
//!
//! The sidecar owns everything iroh: endpoint binding, discovery, dialing,
//! and the pairing handshake. It exposes a byte-pump interface on its own
//! stdio so the Go side (kit daemon / kit --remote) needs no iroh code.
//!
//!   kit-tunnel serve     --secret-hex <64 hex>          (daemon main endpoint)
//!   kit-tunnel serve-pair --pair-seed-hex <64 hex>      (pairing window)
//!   kit-tunnel dial-host --endpoint-id <64 hex> --client-seed-hex <64 hex>
//!   kit-tunnel dial-pair --pair-seed-hex <64 hex> --client-pub-hex <64 hex>
//!
//! `serve` accepts multiple connections over ONE endpoint. Each verified
//! connection becomes a session with its own id; frames on stdio are
//! multiplexed with that id so the Go daemon can run one PTY child per
//! session. `dial` speaks for a single client connection; the daemon-side
//! session id is assigned via a SESSION_ASSIGN frame right after the
//! handshake and normalized away on the client's stdio (always 0 there).
//!
//! Frame format on the iroh bi-directional stream and on stdio:
//!
//!   byte 0       frame type
//!   bytes 1..5   session id (u32, big endian; 0 on the client's stdio)
//!   bytes 5..7   payload length (u16, big endian)
//!   bytes 7..    payload
//!
//! Handshake and session-assignment frames (0x10..0x18) are consumed inside
//! the tunnel and never forwarded. After the handshake the tunnel relays
//! DATA, RESIZE and BYE frames verbatim in both directions; PING/PONG are
//! reserved for a future keepalive.
//!
//! Protocol v3 — pairing model. The daemon owns a STABLE ed25519 identity
//! (--secret-hex); its endpoint id is that public key, and clients store it
//! after pairing, so iroh's QUIC handshake authenticates the host against
//! the pinned id. Clients hold their own ed25519 signing key; the host
//! keeps an allowlist of client public keys (Go side), and reconnects are
//! authenticated by signature — no shared code anywhere in the steady
//! state.
//!
//! Main-endpoint handshake (client speaks first: in QUIC an open_bi stream
//! carries no bytes until the initiator writes):
//!
//!   client -> CLIENT_HELLO {ver, c_nonce, client_pub}
//!   server -> SERVER_HELLO {ver, s_nonce}
//!   server -> daemon  AUTH_REQUEST {c_nonce, s_nonce, client_pub}
//!   client -> CLIENT_AUTH  {ed25519_sig("kit-remote-v3-auth"|c_nonce|s_nonce)}
//!   server -> daemon  AUTH_PAYLOAD {c_nonce, sig}
//!   daemon  -> server AUTH_DECISION {c_nonce, 0|1[, reason]}
//!   server -> SERVER_OK {} | DENIED {reason}
//!   server -> SESSION_ASSIGN {id}
//!
//! The AUTH_* frames travel on the sidecar's stdio: signature verification
//! against the allowlist is policy and stays in Go. Concurrent handshakes
//! are correlated by c_nonce.
//!
//! Pairing window (serve-pair, bootstrap endpoint derived from a one-time
//! code — the ONLY place the code is ever used, and it expires with the
//! window):
//!
//!   client -> PAIR_CLIENT_HELLO {ver, c_nonce, client_pub, tag}
//!             tag = HMAC(pair_key, "kit-pair-client" | c_nonce)
//!   server -> daemon  PAIR_REQUEST {c_nonce, client_pub}
//!             ... human accept/reject on the host terminal ...
//!   daemon  -> server PAIR_DECISION {c_nonce, 0} | {c_nonce, 1, host_endpoint_id}
//!   server -> PAIR_SERVER_OK {s_nonce, tag2, host_endpoint_id} | DENIED
//!             tag2 = HMAC(pair_key, "kit-pair-server" | c_nonce | s_nonce)
//!
//! tag proves the peer knows the code before the human is bothered; the
//! host_endpoint_id (the daemon's stable public key) is what the client
//! stores for future codeless reconnection.
//!
//! Human-facing status goes to stderr as lines of the form:
//!
//!   STATUS READY node_id=<id>
//!   STATUS PAIRING [id=<n>]
//!   STATUS VERIFIED [id=<n>]
//!   STATUS DENIED reason=<text>
//!   STATUS SESSION_OPEN id=<n>
//!   STATUS SESSION_CLOSED id=<n>
//!   STATUS CLOSED
//!   STATUS ERROR msg=<text>
//!
//! The pairing seed is 32 bytes of HKDF-SHA256 output derived from the
//! one-time code (see kit's internal/daemon/pairing.go). It exists only for
//! the pairing window and only proves code knowledge; access itself is
//! granted by the human accept and persisted as a public-key allowlist
//! entry. Failed pairings back off exponentially (up to 8s).

use std::collections::HashMap;
use std::io::{self, Read, Write};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use ed25519_dalek::{Signature, Signer, SigningKey};
use hkdf::Hkdf;
use hmac::{Hmac, Mac};
use iroh::endpoint::{
    presets, IdleTimeout, Incoming, QuicTransportConfig, ReadExactError, RecvStream, SendStream,
};
use iroh::{Endpoint, EndpointId, SecretKey};
use sha2::{Digest, Sha256};
use subtle::ConstantTimeEq;
use tokio::sync::{mpsc, Semaphore};

const ALPN: &[u8] = b"kit/remote/1";
const PROTOCOL_VERSION: u16 = 3;

/// Domain separator for reconnect handshake signatures. Both ends
/// (Rust ed25519-dalek, Go crypto/ed25519) sign/verify this exact prefix.
const SIGN_CONTEXT: &[u8] = b"kit-remote-v3-auth";
const SIGNATURE_LEN: usize = 64;
const ED25519_PUB_LEN: usize = 32;
const PAIR_TAG_ROLE_CLIENT: &[u8] = b"kit-pair-client";
const PAIR_TAG_ROLE_SERVER: &[u8] = b"kit-pair-server";

const MAX_PAYLOAD: usize = 65535;
/// Cap on concurrent sessions over one endpoint; extra peers are denied.
const MAX_SESSIONS: usize = 8;
/// Cap on concurrent polite rejections of over-cap peers. Each rejection
/// task lives at most one handshake timeout; beyond this budget over-cap
/// peers are closed immediately without a denial frame.
const REJECT_BUDGET: usize = 32;
const HANDSHAKE_TIMEOUT: u64 = 30;

// Frame types (shared with internal/daemon/protocol.go).
const FRAME_DATA: u8 = 0x01;
const FRAME_RESIZE: u8 = 0x02;
const FRAME_BYE: u8 = 0x03;
const FRAME_PING: u8 = 0x04;
const FRAME_PONG: u8 = 0x05;
// Pairing-model control frames on the sidecar's stdio: the daemon<->sidecar
// consultation channel that keeps authentication policy in Go (v3).
const FRAME_AUTH_REQUEST: u8 = 0x30;
const FRAME_AUTH_PAYLOAD: u8 = 0x31;
const FRAME_AUTH_DECISION: u8 = 0x32;
const FRAME_PAIR_REQUEST: u8 = 0x40;
const FRAME_PAIR_DECISION: u8 = 0x41;
// Client<->bootstrap-endpoint pairing handshake frames (iroh stream, v3).
const FRAME_PAIR_CLIENT_HELLO: u8 = 0x20;
const FRAME_PAIR_SERVER_OK: u8 = 0x21;
// Handshake + session control (in-tunnel only; never forwarded).
const FRAME_SERVER_HELLO: u8 = 0x10;
const FRAME_CLIENT_HELLO: u8 = 0x11;
const FRAME_SERVER_OK: u8 = 0x12;
const FRAME_DENIED: u8 = 0x13;
const FRAME_CLIENT_AUTH: u8 = 0x14;
const FRAME_SESSION_OPEN: u8 = 0x16; // serve tunnel -> Go daemon
const FRAME_SESSION_CLOSED: u8 = 0x17; // serve tunnel -> Go daemon
const FRAME_SESSION_ASSIGN: u8 = 0x18; // server -> client, in-tunnel

// HKDF domain separation (must match pairing.go).
const HKDF_SALT: &[u8] = b"kit-remote-v1";
const HKDF_AUTH_INFO: &[u8] = b"kit-remote auth";

const NONCE_LEN: usize = 32;
const TAG_LEN: usize = 32;

fn status(line: &str) {
    eprintln!("STATUS {line}");
}

fn fail(msg: &str) -> ! {
    status(&format!("ERROR msg={msg}"));
    std::process::exit(1);
}

// ---------------------------------------------------------------------------
// Crypto helpers
// ---------------------------------------------------------------------------

fn auth_key(seed: &[u8]) -> [u8; 32] {
    let hk = Hkdf::<Sha256>::new(Some(HKDF_SALT), seed);
    let mut out = [0u8; 32];
    hk.expand(HKDF_AUTH_INFO, &mut out)
        .expect("32 byte expansion is valid");
    out
}

fn hmac_tag(key: &[u8; 32], parts: &[&[u8]]) -> [u8; TAG_LEN] {
    let mut mac = <Hmac<Sha256> as Mac>::new_from_slice(key).expect("hmac accepts any key length");
    for p in parts {
        mac.update(p);
    }
    mac.finalize().into_bytes().into()
}

fn random_nonce() -> [u8; NONCE_LEN] {
    rand::random()
}

/// Write a DENIED verdict and finish the stream. Dropping the send side
/// without finishing would reset the stream and silently discard the
/// verdict — the client would only see a connection loss.
async fn deny_and_finish(send: &mut SendStream, reason: &str) {
    let _ = write_frame(send, FRAME_DENIED, 0, reason.as_bytes()).await;
    let _ = send.finish();
    // Hold the streams open while the peer drains the verdict: dropping
    // them closes the connection and can overtake the in-flight data.
    tokio::time::sleep(Duration::from_secs(2)).await;
}

/// Short public identity of a key: first 16 hex chars of SHA-256 over the
/// raw bytes. Mirrors Go's daemon.Fingerprint.
fn fingerprint(b: &[u8]) -> String {
    let digest = Sha256::digest(b);
    hex::encode(digest)[..16].to_string()
}

fn secret_from_seed(seed: &[u8]) -> SecretKey {
    let mut bytes = [0u8; 32];
    bytes.copy_from_slice(seed);
    SecretKey::from_bytes(&bytes)
}

fn parse_seed(hex_seed: &str) -> Vec<u8> {
    hex::decode(hex_seed.trim()).unwrap_or_else(|e| fail(&format!("bad seed hex: {e}")))
}

/// Key material never travels in argv (world-readable via ps); the Go side
/// passes it in the child's environment and the mode flag selects the
/// variable to read.
fn secret_material(flags: &Flags, env_var: &str) -> String {
    if let Some(flag) = ["secret-hex", "pair-seed-hex", "client-seed-hex"]
        .iter()
        .find_map(|f| {
            let v = flags.get(f);
            (!v.is_empty()).then_some(v)
        })
    {
        // Direct hex flag (used by tests and manual runs).
        return flag;
    }
    std::env::var(env_var).unwrap_or_else(|_| fail(&format!("missing key material: set {env_var}")))
}

/// Transport tuning: a keep-alive plus a hard idle timeout so a silently
/// vanished peer (killed process, dropped network, sleeping laptop) is
/// detected in seconds instead of hanging the other side forever.
fn transport_config() -> QuicTransportConfig {
    QuicTransportConfig::builder()
        .keep_alive_interval(Duration::from_secs(5))
        .max_idle_timeout(Some(
            IdleTimeout::try_from(Duration::from_secs(20)).expect("valid timeout"),
        ))
        .build()
}

// ---------------------------------------------------------------------------
// Frame codec
// ---------------------------------------------------------------------------

#[derive(Clone, Debug)]
struct Frame {
    t: u8,
    session: u32,
    payload: Vec<u8>,
}

impl Frame {
    fn new(t: u8, session: u32, payload: Vec<u8>) -> Self {
        Frame {
            t,
            session,
            payload,
        }
    }
}

const FRAME_HEADER_LEN: usize = 7; // type + session + len

fn encode_frame_into(buf: &mut Vec<u8>, t: u8, session: u32, payload: &[u8]) {
    buf.clear();
    buf.push(t);
    buf.extend_from_slice(&session.to_be_bytes());
    buf.extend_from_slice(&(payload.len() as u16).to_be_bytes());
    buf.extend_from_slice(payload);
}

fn write_frame_sync<W: Write>(w: &mut W, f: &Frame) -> io::Result<()> {
    if f.payload.len() > MAX_PAYLOAD {
        return Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            "frame too large",
        ));
    }
    let mut buf = Vec::with_capacity(FRAME_HEADER_LEN + f.payload.len());
    encode_frame_into(&mut buf, f.t, f.session, &f.payload);
    w.write_all(&buf)?;
    w.flush()
}

fn read_frame_sync<R: Read>(r: &mut R) -> io::Result<Option<Frame>> {
    let mut hdr = [0u8; FRAME_HEADER_LEN];
    match r.read_exact(&mut hdr) {
        Ok(()) => {}
        Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => return Ok(None),
        Err(e) => return Err(e),
    }
    let len = u16::from_be_bytes([hdr[5], hdr[6]]) as usize;
    let mut payload = vec![0u8; len];
    r.read_exact(&mut payload)?;
    Ok(Some(Frame {
        t: hdr[0],
        session: u32::from_be_bytes([hdr[1], hdr[2], hdr[3], hdr[4]]),
        payload,
    }))
}

async fn write_frame(
    send: &mut SendStream,
    t: u8,
    session: u32,
    payload: &[u8],
) -> anyhow::Result<()> {
    if payload.len() > MAX_PAYLOAD {
        anyhow::bail!("frame too large");
    }
    let mut buf = Vec::with_capacity(FRAME_HEADER_LEN + payload.len());
    encode_frame_into(&mut buf, t, session, payload);
    send.write_all(&buf).await?;
    Ok(())
}

async fn read_frame(recv: &mut RecvStream) -> anyhow::Result<Frame> {
    let mut hdr = [0u8; FRAME_HEADER_LEN];
    match recv.read_exact(&mut hdr).await {
        Ok(()) => {}
        Err(ReadExactError::FinishedEarly(_)) => anyhow::bail!("stream closed"),
        Err(e) => anyhow::bail!("read header: {e}"),
    }
    let len = u16::from_be_bytes([hdr[5], hdr[6]]) as usize;
    let mut payload = vec![0u8; len];
    recv.read_exact(&mut payload)
        .await
        .map_err(|e| anyhow::anyhow!("read payload: {e}"))?;
    Ok(Frame {
        t: hdr[0],
        session: u32::from_be_bytes([hdr[1], hdr[2], hdr[3], hdr[4]]),
        payload,
    })
}

// ---------------------------------------------------------------------------
// Handshake
// ---------------------------------------------------------------------------

async fn server_handshake(
    send: &mut SendStream,
    recv: &mut RecvStream,
    pending: Pending,
    session: u32,
) -> anyhow::Result<()> {
    let hello = read_frame(recv).await?;
    if hello.t != FRAME_CLIENT_HELLO || hello.payload.len() != 2 + NONCE_LEN + ED25519_PUB_LEN {
        anyhow::bail!("malformed client hello");
    }
    let c_ver = u16::from_be_bytes([hello.payload[0], hello.payload[1]]);
    if c_ver != PROTOCOL_VERSION {
        deny_and_finish(send, &format!("version {c_ver}")).await;
        anyhow::bail!("version mismatch: {c_ver}");
    }
    let c_nonce = hello.payload[2..2 + NONCE_LEN].to_vec();
    let corr: [u8; 8] = c_nonce[0..8].try_into().expect("nonce is 32 bytes");
    let client_pub = hello.payload[2 + NONCE_LEN..].to_vec();

    let s_nonce = random_nonce();
    let mut reply = Vec::with_capacity(2 + NONCE_LEN);
    reply.extend_from_slice(&PROTOCOL_VERSION.to_be_bytes());
    reply.extend_from_slice(&s_nonce);
    write_frame(send, FRAME_SERVER_HELLO, 0, &reply).await?;

    // Consult the Go daemon: it owns the allowlist and verifies the
    // signature. Register the pending channel BEFORE reading the auth
    // frame so the decision can never race the registration.
    let (tx, mut rx) = mpsc::unbounded_channel::<Frame>();
    pending.lock().unwrap().insert(corr, tx);
    let consult = send_to_go(&Frame::new(
        FRAME_AUTH_REQUEST,
        0,
        [
            c_nonce.as_slice(),
            s_nonce.as_slice(),
            client_pub.as_slice(),
        ]
        .concat(),
    ));
    if !consult {
        anyhow::bail!("daemon gone");
    }

    let auth = read_frame(recv).await?;
    if auth.t != FRAME_CLIENT_AUTH || auth.payload.len() != SIGNATURE_LEN {
        deny_and_finish(send, "bad auth frame").await;
        anyhow::bail!("malformed client auth");
    }
    if !send_to_go(&Frame::new(
        FRAME_AUTH_PAYLOAD,
        0,
        [c_nonce.as_slice(), auth.payload.as_slice()].concat(),
    )) {
        anyhow::bail!("daemon gone");
    }

    let decision = tokio::time::timeout(Duration::from_secs(HANDSHAKE_TIMEOUT), rx.recv()).await;
    match decision {
        // Accept: payload is the 8-byte correlation key + a 0x01 verdict.
        Ok(Some(f)) if f.payload.len() >= 9 && f.payload[8] == 0x01 => {}
        Ok(Some(f)) => {
            let reason = if f.payload.len() > 9 {
                String::from_utf8_lossy(&f.payload[9..]).into_owned()
            } else {
                "not authorized".into()
            };
            deny_and_finish(send, &reason).await;
            anyhow::bail!("unauthorized client: {reason}");
        }
        Ok(None) | Err(_) => {
            anyhow::bail!("auth decision timeout");
        }
    }

    // Tell the client which session id to use on this connection. It goes
    // after the verdict so the client's handshake loop never sees it.
    write_frame(send, FRAME_SERVER_OK, 0, &[]).await?;
    write_frame(send, FRAME_SESSION_ASSIGN, 0, &session.to_be_bytes()).await?;
    Ok(())
}

// ---------------------------------------------------------------------------
// Serve mode: one endpoint, many sessions
// ---------------------------------------------------------------------------

type Registry = Arc<Mutex<HashMap<u32, mpsc::UnboundedSender<Frame>>>>;

/// Consultation replies from the Go daemon, keyed by the handshake's client
/// nonce (8 random bytes — collision-free for practical purposes). The
/// stdin reader routes AUTH_DECISION/PAIR_DECISION frames here; handshake
/// tasks await on the channel.
type Pending = Arc<Mutex<HashMap<[u8; 8], mpsc::UnboundedSender<Frame>>>>;

fn send_to_go(f: &Frame) -> bool {
    tokio::task::block_in_place(|| write_frame_sync(&mut io::stdout().lock(), f)).is_ok()
}

async fn serve(flags: &Flags) {
    let secret_bytes = parse_seed(&flags.get("secret-hex"));
    let secret = secret_from_seed(&secret_bytes);

    let endpoint = Endpoint::builder(presets::N0)
        .secret_key(secret)
        .alpns(vec![ALPN.to_vec()])
        .transport_config(transport_config())
        .bind()
        .await
        .unwrap_or_else(|e| fail(&format!("endpoint bind: {e}")));
    endpoint.online().await;

    status(&format!("READY node_id={}", endpoint.id()));

    let registry: Registry = Arc::new(Mutex::new(HashMap::new()));
    let active = Arc::new(AtomicUsize::new(0));
    let backoff: BackoffState = Arc::new(Mutex::new(Backoff::default()));
    let reject_budget = Arc::new(Semaphore::new(REJECT_BUDGET));
    let pending: Pending = Arc::new(Mutex::new(HashMap::new()));

    // Router: frames arriving on stdin are dispatched to the session named
    // in the frame header; auth decisions are routed to the handshake that
    // requested them (keyed by client nonce); unknown ids are dropped.
    {
        let registry = registry.clone();
        let pending = pending.clone();
        tokio::spawn(async move {
            loop {
                let frame =
                    tokio::task::block_in_place(|| read_frame_sync(&mut io::stdin().lock()));
                match frame {
                    Ok(Some(f)) => {
                        if f.t == FRAME_AUTH_DECISION && f.payload.len() >= 9 {
                            let mut key = [0u8; 8];
                            key.copy_from_slice(&f.payload[0..8]);
                            if let Some(tx) = pending.lock().unwrap().remove(&key) {
                                let _ = tx.send(f);
                            }
                            continue;
                        }
                        let tx = registry.lock().unwrap().get(&f.session).cloned();
                        if let Some(tx) = tx {
                            let _ = tx.send(f);
                        }
                    }
                    Ok(None) => {
                        // Local side (the Go daemon) is gone: end everything.
                        let all: Vec<_> = registry.lock().unwrap().values().cloned().collect();
                        for tx in all {
                            let _ = tx.send(Frame::new(FRAME_BYE, 0, Vec::new()));
                        }
                        break;
                    }
                    Err(e) => {
                        eprintln!("STATUS ERROR msg=local stdin: {e}");
                        break;
                    }
                }
            }
        });
    }

    accept_loop(endpoint, registry, active, backoff, reject_budget, pending).await;
}

/// The serve accept loop: reserves a session slot per incoming connection
/// (atomically, so concurrent peers cannot all pass the cap check) and
/// hands each to `handle_connection`.
#[allow(clippy::too_many_arguments)]
async fn accept_loop(
    endpoint: Endpoint,
    registry: Registry,
    active: Arc<AtomicUsize>,
    backoff: BackoffState,
    reject_budget: Arc<Semaphore>,
    pending: Pending,
) {
    let mut next_id: u32 = 1;
    while let Some(incoming) = endpoint.accept().await {
        let slot = active.fetch_add(1, Ordering::Relaxed) + 1;
        if slot > MAX_SESSIONS {
            active.fetch_sub(1, Ordering::Relaxed);
            // Beyond the polite-rejection budget, close the peer
            // immediately: unbounded rejection tasks are their own
            // resource leak under a connection flood.
            match reject_budget.clone().try_acquire_owned() {
                Ok(permit) => {
                    tokio::spawn(reject_session_full(incoming, permit));
                }
                Err(_) => drop(incoming),
            }
            continue;
        }
        let id = next_id;
        next_id = next_id.wrapping_add(1).max(1);
        tokio::spawn(handle_connection(
            incoming,
            id,
            registry.clone(),
            active.clone(),
            backoff.clone(),
            pending.clone(),
        ));
    }
}

/// Handshake-failure backoff shared across connections: each consecutive
/// failure raises the delay a connection waits before authenticating
/// (500 ms steps, capped at 8 s). The count decays after two quiet minutes
/// so a failed-guess burst cannot pin a permanent delay on legitimate
/// peers, and it resets to zero on any successful handshake. The delay is
/// applied inside the per-connection task: a failing peer delays itself
/// and never blocks the accept loop.
#[derive(Default)]
struct Backoff {
    count: usize,
    last_failure: Option<Instant>,
}

type BackoffState = Arc<Mutex<Backoff>>;

impl Backoff {
    fn delay(&mut self) -> Duration {
        let stale = self
            .last_failure
            .map(|t| t.elapsed() > Duration::from_secs(120))
            .unwrap_or(true);
        if self.count > 0 && stale {
            self.count = 0;
        }
        Duration::from_millis((self.count as u64 * 500).min(8000))
    }

    fn record_success(&mut self) {
        self.count = 0;
        self.last_failure = None;
    }

    fn record_failure(&mut self) {
        self.count = self.count.saturating_add(1);
        self.last_failure = Some(Instant::now());
    }
}

/// Releases the reserved session slot on every exit path of
/// `handle_connection`, including handshake failures.
struct SlotGuard<'a> {
    active: &'a AtomicUsize,
}

impl Drop for SlotGuard<'_> {
    fn drop(&mut self) {
        self.active.fetch_sub(1, Ordering::Relaxed);
    }
}

/// Removes the correlation entry from the pending map on every exit path,
/// so peers that connect, say hello, and vanish cannot grow the map.
struct PendingGuard<'a> {
    pending: &'a Pending,
    corr: &'a [u8; 8],
}

impl Drop for PendingGuard<'_> {
    fn drop(&mut self) {
        self.pending.lock().unwrap().remove(self.corr);
    }
}

/// Politely refuse a peer when the session cap is reached. Fully bounded:
/// every wait uses the handshake timeout (an over-cap peer that never opens
/// a stream, or stalls at any point, is reaped instead of pinning a
/// rejection task), and the denial is finished before the connection drops
/// so the peer reliably sees it.
async fn reject_session_full(incoming: Incoming, _permit: tokio::sync::OwnedSemaphorePermit) {
    let opened = tokio::time::timeout(Duration::from_secs(HANDSHAKE_TIMEOUT), async move {
        let accepting = incoming.accept().map_err(|e| anyhow::anyhow!("{e}"))?;
        let conn = accepting.await?;
        let (mut send, _recv) = conn.accept_bi().await?;
        anyhow::Ok((send, conn))
    })
    .await;
    if let Ok(Ok((mut send, _conn))) = opened {
        let _ = write_frame(&mut send, FRAME_DENIED, 0, b"too many sessions").await;
        let _ = send.finish();
        status("DENIED reason=too many sessions");
    }
}

async fn handle_connection(
    incoming: Incoming,
    id: u32,
    registry: Registry,
    active: Arc<AtomicUsize>,
    backoff: BackoffState,
    pending: Pending,
) {
    // The slot was reserved by the accept loop; drop releases it.
    let _slot = SlotGuard { active: &active };

    let conn = match incoming.accept() {
        Ok(accepting) => match accepting.await {
            Ok(conn) => conn,
            Err(e) => {
                status(&format!("ERROR msg=connect failed: {e}"));
                return;
            }
        },
        Err(e) => {
            status(&format!("ERROR msg=incoming rejected: {e}"));
            return;
        }
    };
    status(&format!("PAIRING id={id}"));

    // Bound the stream open too: a peer that connects but never opens its
    // bi stream would block here forever, holding its reserved session
    // slot (eight such peers would lock the daemon). The SlotGuard below
    // releases the slot when this timeout fires.
    let (mut send, mut recv) = match tokio::time::timeout(
        Duration::from_secs(HANDSHAKE_TIMEOUT),
        conn.accept_bi(),
    )
    .await
    {
        Ok(Ok(pair)) => pair,
        Ok(Err(e)) => {
            status(&format!("ERROR msg=open stream: {e}"));
            return;
        }
        Err(_) => {
            status("DENIED reason=stream open timeout");
            return;
        }
    };

    // Backoff is applied inside this connection's own task: a failing peer
    // waits here before authenticating instead of stalling the accept loop.
    let delay = { backoff.lock().unwrap().delay() };
    if delay > Duration::ZERO {
        tokio::time::sleep(delay).await;
    }

    let handshake = tokio::time::timeout(
        Duration::from_secs(HANDSHAKE_TIMEOUT),
        server_handshake(&mut send, &mut recv, pending, id),
    )
    .await;
    match handshake {
        Ok(Ok(())) => {
            backoff.lock().unwrap().record_success();
        }
        Ok(Err(e)) => {
            backoff.lock().unwrap().record_failure();
            status(&format!("DENIED reason={e}"));
            return;
        }
        Err(_) => {
            backoff.lock().unwrap().record_failure();
            status("DENIED reason=handshake timeout");
            return;
        }
    }

    // Announce the session to the Go daemon, then register the routing
    // channel. Order matters: SESSION_OPEN must reach stdout before any of
    // this connection's relayed frames do.
    if write_frame_sync(
        &mut io::stdout().lock(),
        &Frame::new(FRAME_SESSION_OPEN, id, Vec::new()),
    )
    .is_err()
    {
        return; // daemon is gone
    }
    status(&format!("SESSION_OPEN id={id}"));

    let (tx, mut rx) = mpsc::unbounded_channel::<Frame>();
    registry.lock().unwrap().insert(id, tx);

    // Connection -> stdout (tagged with our session id).
    let mut out = tokio::spawn(async move {
        loop {
            let frame = match read_frame(&mut recv).await {
                Ok(f) => f,
                Err(_) => break,
            };
            if frame.t == FRAME_BYE {
                break;
            }
            let out = Frame::new(frame.t, id, frame.payload);
            // Blocking stdout write: keep it off the async worker core.
            if tokio::task::block_in_place(|| write_frame_sync(&mut io::stdout().lock(), &out))
                .is_err()
            {
                break; // daemon gone
            }
        }
    });

    // Routed stdin frames -> connection. A routed BYE ends this session.
    let mut inp = tokio::spawn(async move {
        while let Some(f) = rx.recv().await {
            if f.t == FRAME_BYE {
                let _ = write_frame(&mut send, FRAME_BYE, id, &[]).await;
                break;
            }
            if write_frame(&mut send, f.t, id, &f.payload).await.is_err() {
                break;
            }
        }
    });

    // Whichever direction ends first ends the session.
    tokio::select! {
        _ = &mut out => { inp.abort(); }
        _ = &mut inp => { out.abort(); }
    }

    registry.lock().unwrap().remove(&id);
    let _ = write_frame_sync(
        &mut io::stdout().lock(),
        &Frame::new(FRAME_SESSION_CLOSED, id, Vec::new()),
    );
    status(&format!("SESSION_CLOSED id={id}"));
}

// ---------------------------------------------------------------------------
// Dial mode: one client connection
// ---------------------------------------------------------------------------

async fn dial_pair(flags: &Flags) {
    let seed = parse_seed(&secret_material(flags, "KIT_TUNNEL_PAIR_SEED"));
    if seed.len() != 32 {
        fail("pairing seed must be 32 bytes");
    }
    let key = auth_key(&seed);
    // The bootstrap endpoint is derived from the one-time code: knowledge
    // of the code is what makes it findable, exactly like protocol v2 —
    // but this endpoint exists only for the pairing window.
    let server_id = secret_from_seed(&seed).public();
    let client_pub = parse_seed(&flags.get("client-pub-hex"));
    if client_pub.len() != ED25519_PUB_LEN {
        fail("client-pub-hex must be 64 hex chars");
    }
    let timeout_secs = flags.timeout();

    let endpoint = Endpoint::builder(presets::N0)
        .alpns(vec![ALPN.to_vec()])
        .transport_config(transport_config())
        .bind()
        .await
        .unwrap_or_else(|e| fail(&format!("endpoint bind: {e}")));
    endpoint.online().await;

    let conn = match tokio::time::timeout(
        Duration::from_secs(timeout_secs),
        endpoint.connect(server_id, ALPN),
    )
    .await
    {
        Ok(Ok(conn)) => conn,
        Ok(Err(e)) => fail(&format!("connect to daemon: {e}")),
        Err(_) => fail("no daemon is live for this pairing code (wrong code, expired window, or network issue)"),
    };

    let (mut send, mut recv) = match conn.open_bi().await {
        Ok(pair) => pair,
        Err(e) => fail(&format!("open stream: {e}")),
    };

    // Prove code knowledge up front so a wrong code never reaches the
    // human on the host side.
    let c_nonce = random_nonce();
    let tag = hmac_tag(&key, &[PAIR_TAG_ROLE_CLIENT, &c_nonce]);
    let mut hello = Vec::with_capacity(2 + NONCE_LEN + ED25519_PUB_LEN + TAG_LEN);
    hello.extend_from_slice(&PROTOCOL_VERSION.to_be_bytes());
    hello.extend_from_slice(&c_nonce);
    hello.extend_from_slice(&client_pub);
    hello.extend_from_slice(&tag);
    if write_frame(&mut send, FRAME_PAIR_CLIENT_HELLO, 0, &hello)
        .await
        .is_err()
    {
        fail("write pair hello");
    }

    let reply =
        tokio::time::timeout(Duration::from_secs(timeout_secs), read_frame(&mut recv)).await;
    match reply {
        Ok(Ok(f))
            if f.t == FRAME_PAIR_SERVER_OK
                && f.payload.len() == NONCE_LEN + TAG_LEN + ED25519_PUB_LEN =>
        {
            let s_nonce = &f.payload[0..NONCE_LEN];
            let expect = hmac_tag(&key, &[PAIR_TAG_ROLE_SERVER, &c_nonce, s_nonce]);
            if f.payload[NONCE_LEN..NONCE_LEN + TAG_LEN]
                .ct_eq(&expect)
                .unwrap_u8()
                != 1
            {
                fail("daemon failed tag verification");
            }
            // The stable endpoint id the client stores for codeless
            // reconnection. iroh's QUIC handshake authenticates the daemon
            // against it, so dialing it later cannot be hijacked.
            let host_id = hex::encode(&f.payload[NONCE_LEN + TAG_LEN..]);
            status(&format!("PAIRED host_endpoint_id={host_id}"));
            // Hold until the Go side closes the pipe (it saves the host
            // entry first), then end.
            let _ = tokio::task::block_in_place(|| read_frame_sync(&mut io::stdin().lock()));
        }
        Ok(Ok(f)) if f.t == FRAME_DENIED => {
            let reason = String::from_utf8_lossy(&f.payload);
            status(&format!("DENIED reason={reason}"));
            return;
        }
        Ok(Ok(f)) => fail(&format!("unexpected pairing frame {:#04x}", f.t)),
        Ok(Err(e)) => fail(&format!("pairing: {e:#}")),
        Err(_) => fail("pairing timed out (was the request accepted on the host?)"),
    }
}

async fn dial_host(flags: &Flags) {
    let server_bytes = parse_seed(&flags.get("endpoint-id"));
    if server_bytes.len() != ED25519_PUB_LEN {
        fail("endpoint-id must be 64 hex chars");
    }
    let server_id = EndpointId::from_bytes(&server_bytes.try_into().expect("checked above"))
        .unwrap_or_else(|e| fail(&format!("bad endpoint id: {e}")));
    let signing_seed = parse_seed(&secret_material(flags, "KIT_TUNNEL_CLIENT_SEED"));
    if signing_seed.len() != 32 {
        fail("client seed must be 32 bytes");
    }
    let signing = SigningKey::from_bytes(&signing_seed.try_into().expect("checked above"));
    let timeout_secs = flags.timeout();

    // Transport identity is ephemeral; the application-level identity is
    // the client signing key. The daemon authenticates the signature, and
    // iroh authenticates the daemon against the endpoint id we dialed —
    // the one pinned at pairing time.
    let endpoint = Endpoint::builder(presets::N0)
        .secret_key(SecretKey::generate())
        .alpns(vec![ALPN.to_vec()])
        .transport_config(transport_config())
        .bind()
        .await
        .unwrap_or_else(|e| fail(&format!("endpoint bind: {e}")));
    endpoint.online().await;

    let conn = match tokio::time::timeout(
        Duration::from_secs(timeout_secs),
        endpoint.connect(server_id, ALPN),
    )
    .await
    {
        Ok(Ok(conn)) => conn,
        Ok(Err(e)) => fail(&format!("connect to daemon: {e}")),
        Err(_) => fail("could not reach the daemon (network or relay issue)"),
    };

    let (mut send, mut recv) = match conn.open_bi().await {
        Ok(pair) => pair,
        Err(e) => fail(&format!("open stream: {e}")),
    };

    let c_nonce = random_nonce();
    let mut hello = Vec::with_capacity(2 + NONCE_LEN + ED25519_PUB_LEN);
    hello.extend_from_slice(&PROTOCOL_VERSION.to_be_bytes());
    hello.extend_from_slice(&c_nonce);
    hello.extend_from_slice(signing.verifying_key().as_bytes());
    if write_frame(&mut send, FRAME_CLIENT_HELLO, 0, &hello)
        .await
        .is_err()
    {
        fail("write hello");
    }

    let reply = match tokio::time::timeout(Duration::from_secs(timeout_secs), read_frame(&mut recv))
        .await
    {
        Ok(Ok(f)) => f,
        Ok(Err(e)) => fail(&format!("handshake: {e}")),
        Err(_) => fail("handshake timed out"),
    };
    if reply.t != FRAME_SERVER_HELLO || reply.payload.len() < 2 + NONCE_LEN {
        fail("malformed server hello");
    }
    let s_ver = u16::from_be_bytes([reply.payload[0], reply.payload[1]]);
    if s_ver != PROTOCOL_VERSION {
        fail(&format!("daemon version mismatch: {s_ver}"));
    }
    let s_nonce = &reply.payload[2..2 + NONCE_LEN];

    let mut msg = Vec::with_capacity(SIGN_CONTEXT.len() + NONCE_LEN * 2);
    msg.extend_from_slice(SIGN_CONTEXT);
    msg.extend_from_slice(&c_nonce);
    msg.extend_from_slice(s_nonce);
    let sig: Signature = signing.sign(&msg);
    if write_frame(&mut send, FRAME_CLIENT_AUTH, 0, sig.to_bytes().as_slice())
        .await
        .is_err()
    {
        fail("write auth");
    }

    let verdict = match tokio::time::timeout(
        Duration::from_secs(timeout_secs),
        read_frame(&mut recv),
    )
    .await
    {
        Ok(Ok(f)) => f,
        Ok(Err(e)) => fail(&format!("handshake: {e}")),
        Err(_) => fail("handshake timed out"),
    };
    match verdict.t {
        FRAME_SERVER_OK => {}
        FRAME_DENIED => {
            let reason = String::from_utf8_lossy(&verdict.payload);
            status(&format!("DENIED reason={reason}"));
            return;
        }
        other => fail(&format!("unexpected handshake frame {other:#04x}")),
    }

    // The server assigns our session id right after the handshake; client
    // stdio always uses 0 and the tunnel rewrites both directions.
    let assign = match read_frame(&mut recv).await {
        Ok(f) => f,
        Err(e) => fail(&format!("session assignment: {e}")),
    };
    if assign.t != FRAME_SESSION_ASSIGN || assign.payload.len() != 4 {
        fail("malformed session assignment");
    }
    let session = u32::from_be_bytes([
        assign.payload[0],
        assign.payload[1],
        assign.payload[2],
        assign.payload[3],
    ]);
    status(&format!("VERIFIED id={session}"));

    relay_client_session(send, recv, session).await;
    status("CLOSED");
}

/// Bidirectional session relay after a verified client handshake (the
/// session-assignment frame has already been consumed).
async fn relay_client_session(mut send: SendStream, mut recv: RecvStream, session: u32) {
    // Local stdin -> connection (rewritten to the assigned session id).
    let up = tokio::spawn(async move {
        loop {
            let frame = tokio::task::block_in_place(|| read_frame_sync(&mut io::stdin().lock()));
            match frame {
                Ok(Some(f)) => {
                    let bye = f.t == FRAME_BYE;
                    if let Err(e) = write_frame(&mut send, f.t, session, &f.payload).await {
                        if !bye {
                            eprintln!("STATUS ERROR msg=write to remote: {e}");
                        }
                        break;
                    }
                    if bye {
                        break;
                    }
                }
                Ok(None) => {
                    // Local side closed its pipe: tell the remote and stop.
                    let _ = write_frame(&mut send, FRAME_BYE, session, &[]).await;
                    break;
                }
                Err(e) => {
                    eprintln!("STATUS ERROR msg=local stdin: {e}");
                    break;
                }
            }
        }
    });

    // Connection -> local stdout (session id normalized to 0).
    loop {
        let frame = match read_frame(&mut recv).await {
            Ok(f) => f,
            Err(_) => break,
        };
        if frame.t == FRAME_BYE {
            break;
        }
        let out = Frame::new(frame.t, 0, frame.payload);
        if tokio::task::block_in_place(|| write_frame_sync(&mut io::stdout().lock(), &out)).is_err()
        {
            break;
        }
    }
    up.abort();
}

// ---------------------------------------------------------------------------
// Pairing window: one bootstrap endpoint, at most one pairing
// ---------------------------------------------------------------------------

/// The host side of the pairing window. Binds the bootstrap endpoint
/// derived from the one-time code, verifies the caller knows the code
/// (so a wrong guess never reaches the human), asks Go to prompt the
/// user, and — on accept — hands the client the daemon's stable endpoint
/// id. The Go side enforces the window timeout; every wait here is also
/// bounded so a stalled peer cannot pin the task.
async fn serve_pair(flags: &Flags) {
    let seed = parse_seed(&flags.get("pair-seed-hex"));
    let key = Arc::new(auth_key(&seed));
    let secret = secret_from_seed(&seed);

    let endpoint = Endpoint::builder(presets::N0)
        .secret_key(secret)
        .alpns(vec![ALPN.to_vec()])
        .transport_config(transport_config())
        .bind()
        .await
        .unwrap_or_else(|e| fail(&format!("endpoint bind: {e}")));
    endpoint.online().await;

    status(&format!("READY_PAIR node_id={}", endpoint.id()));

    let pending: Pending = Arc::new(Mutex::new(HashMap::new()));
    {
        let pending = pending.clone();
        tokio::spawn(async move {
            loop {
                let frame =
                    tokio::task::block_in_place(|| read_frame_sync(&mut io::stdin().lock()));
                match frame {
                    Ok(Some(f)) => {
                        if f.t == FRAME_PAIR_DECISION && f.payload.len() >= 9 {
                            let mut key = [0u8; 8];
                            key.copy_from_slice(&f.payload[0..8]);
                            if let Some(tx) = pending.lock().unwrap().remove(&key) {
                                let _ = tx.send(f);
                            }
                            continue;
                        }
                    }
                    Ok(None) => break, // Go closed the window
                    Err(e) => {
                        eprintln!("STATUS ERROR msg=local stdin: {e}");
                        break;
                    }
                }
            }
        });
    }

    let Some(incoming) = endpoint.accept().await else {
        return;
    };
    handle_pair_connection(incoming, key, pending).await;
    // Linger briefly so the confirmation frame is delivered and read
    // before the process exit tears the QUIC connection down.
    tokio::time::sleep(Duration::from_millis(1500)).await;
    status("CLOSED");
}

async fn handle_pair_connection(incoming: Incoming, key: Arc<[u8; 32]>, pending: Pending) {
    let backoff: BackoffState = Arc::new(Mutex::new(Backoff::default()));
    let delay = { backoff.lock().unwrap().delay() };
    if delay > Duration::ZERO {
        tokio::time::sleep(delay).await;
    }

    let conn = match incoming.accept() {
        Ok(accepting) => match accepting.await {
            Ok(conn) => conn,
            Err(e) => {
                status(&format!("ERROR msg=connect failed: {e}"));
                return;
            }
        },
        Err(e) => {
            status(&format!("ERROR msg=incoming rejected: {e}"));
            return;
        }
    };

    let opened =
        tokio::time::timeout(Duration::from_secs(HANDSHAKE_TIMEOUT), conn.accept_bi()).await;
    let (mut send, mut recv) = match opened {
        Ok(Ok(pair)) => pair,
        Ok(Err(e)) => {
            status(&format!("ERROR msg=open stream: {e}"));
            return;
        }
        Err(_) => {
            status("DENIED reason=stream open timeout");
            return;
        }
    };

    let hello = match tokio::time::timeout(
        Duration::from_secs(HANDSHAKE_TIMEOUT),
        read_frame(&mut recv),
    )
    .await
    {
        Ok(Ok(f)) => f,
        Ok(Err(e)) => {
            status(&format!("DENIED reason=read hello: {e}"));
            return;
        }
        Err(_) => {
            status("DENIED reason=pairing timeout");
            return;
        }
    };
    // ver(u16) | c_nonce(32) | client_pub(32) | tag(32)
    if hello.t != FRAME_PAIR_CLIENT_HELLO
        || hello.payload.len() != 2 + NONCE_LEN + ED25519_PUB_LEN + TAG_LEN
    {
        backoff.lock().unwrap().record_failure();
        status("DENIED reason=malformed pair hello");
        return;
    }
    let c_ver = u16::from_be_bytes([hello.payload[0], hello.payload[1]]);
    if c_ver != PROTOCOL_VERSION {
        backoff.lock().unwrap().record_failure();
        status(&format!("DENIED reason=version {c_ver}"));
        return;
    }
    let c_nonce = hello.payload[2..2 + NONCE_LEN].to_vec();
    let corr: [u8; 8] = c_nonce[0..8].try_into().expect("nonce is 32 bytes");
    let client_pub = hello.payload[2 + NONCE_LEN..2 + NONCE_LEN + ED25519_PUB_LEN].to_vec();
    let tag = &hello.payload[2 + NONCE_LEN + ED25519_PUB_LEN..];
    let expect = hmac_tag(&key, &[PAIR_TAG_ROLE_CLIENT, &c_nonce]);
    if tag.ct_eq(&expect).unwrap_u8() != 1 {
        backoff.lock().unwrap().record_failure();
        status("DENIED reason=bad pairing tag");
        return;
    }

    // The peer knows the code. Ask the human.
    let fp = fingerprint(&client_pub);
    status(&format!("PAIR_REQUEST fp={fp}"));
    let (tx, mut rx) = mpsc::unbounded_channel::<Frame>();
    pending.lock().unwrap().insert(corr, tx);
    let _guard = PendingGuard {
        pending: &pending,
        corr: &corr,
    };
    if !send_to_go(&Frame::new(
        FRAME_PAIR_REQUEST,
        0,
        [c_nonce.as_slice(), client_pub.as_slice()].concat(),
    )) {
        return;
    }

    // The Go side prompts for up to its own deadline; allow margin.
    let decision = tokio::time::timeout(Duration::from_secs(120), rx.recv()).await;
    match decision {
        // Accept: c_nonce | 0x01 | host_endpoint_id(32)
        Ok(Some(f)) if f.payload.len() == 9 + ED25519_PUB_LEN && f.payload[8] == 0x01 => {
            let host_id = &f.payload[9..];
            backoff.lock().unwrap().record_success();
            let s_nonce = random_nonce();
            let tag2 = hmac_tag(&key, &[PAIR_TAG_ROLE_SERVER, &c_nonce, &s_nonce]);
            let mut ok = Vec::with_capacity(NONCE_LEN + TAG_LEN + ED25519_PUB_LEN);
            ok.extend_from_slice(&s_nonce);
            ok.extend_from_slice(&tag2);
            ok.extend_from_slice(host_id);
            if write_frame(&mut send, FRAME_PAIR_SERVER_OK, 0, &ok)
                .await
                .is_err()
            {
                return;
            }
            // Deliver the confirmation as finished data: dropping the send
            // side unfinished would reset the stream and the client would
            // lose the frame. Hold while the peer drains it.
            let _ = send.finish();
            tokio::time::sleep(Duration::from_secs(2)).await;
            status("PAIRED");
        }
        Ok(Some(f)) => {
            let reason = if f.payload.len() > 9 {
                String::from_utf8_lossy(&f.payload[9..]).into_owned()
            } else {
                "rejected on the host".into()
            };
            backoff.lock().unwrap().record_failure();
            deny_and_finish(&mut send, &reason).await;
            status(&format!("PAIR_DENIED reason={reason}"));
        }
        Ok(None) | Err(_) => {
            backoff.lock().unwrap().record_failure();
            deny_and_finish(&mut send, "pairing window closed").await;
            status("PAIR_DENIED reason=window closed");
        }
    }
}

// ---------------------------------------------------------------------------
// Args / entry
// ---------------------------------------------------------------------------

/// Flat --key value flag store parsed from the argv tail.
struct Flags {
    map: HashMap<String, String>,
}

impl Flags {
    fn get(&self, name: &str) -> String {
        self.map.get(name).cloned().unwrap_or_default()
    }

    fn timeout(&self) -> u64 {
        self.get("timeout").parse().unwrap_or(30)
    }
}

fn parse_flags(args: &[String]) -> Flags {
    let mut map = HashMap::new();
    let mut i = 0;
    while i < args.len() {
        if let Some(name) = args[i].strip_prefix("--") {
            if i + 1 < args.len() {
                map.insert(name.to_string(), args[i + 1].clone());
                i += 2;
                continue;
            }
        }
        i += 1;
    }
    Flags { map }
}

fn main() {
    // Tracing is opt-in via RUST_LOG; keep stderr clean for the Go side,
    // which only parses "STATUS ..." lines and buffers the rest.
    if std::env::var_os("RUST_LOG").is_some() {
        let _ = tracing_subscriber::fmt::try_init();
    }

    let args: Vec<String> = std::env::args().skip(1).collect();
    let Some(mode) = args.first().cloned() else {
        fail("usage: kit-tunnel <serve|serve-pair|dial-pair|dial-host> [--flags]");
    };
    let flags = parse_flags(&args[1..]);
    let rt = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("tokio runtime");
    match mode.as_str() {
        "serve" => rt.block_on(serve(&flags)),
        "serve-pair" => rt.block_on(serve_pair(&flags)),
        "dial-pair" => rt.block_on(dial_pair(&flags)),
        "dial-host" => rt.block_on(dial_host(&flags)),
        other => fail(&format!("unknown mode {other}")),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Regression test: peers that complete the QUIC handshake but stall
    /// before authenticating hold session slots only for the pre-auth
    /// timeout — and until then, the session cap must keep rejecting new
    /// peers instead of letting the slot count grow unbounded. Pairs that
    /// get as far as the AUTH consultation hang waiting for a decision
    /// that the test never sends, which is exactly the stall being tested.
    #[tokio::test(flavor = "multi_thread")]
    async fn stalled_pre_auth_peers_are_capped_and_expire() {
        let secret = SecretKey::generate();
        let endpoint = Endpoint::builder(presets::N0)
            .secret_key(secret)
            .alpns(vec![ALPN.to_vec()])
            .transport_config(transport_config())
            .bind()
            .await
            .expect("server bind");
        endpoint.online().await;
        let server_id = endpoint.id();

        let registry: Registry = Arc::new(Mutex::new(HashMap::new()));
        let active = Arc::new(AtomicUsize::new(0));
        let backoff: BackoffState = Arc::new(Mutex::new(Backoff::default()));
        let pending: Pending = Arc::new(Mutex::new(HashMap::new()));

        let accept_task = tokio::spawn(accept_loop(
            endpoint,
            registry.clone(),
            active.clone(),
            backoff.clone(),
            Arc::new(Semaphore::new(REJECT_BUDGET)),
            pending.clone(),
        ));

        // One client endpoint opens MAX_SESSIONS connections that stall:
        // half never open a stream (the accept_bi path), half send a
        // CLIENT_HELLO and then go quiet (the auth-consultation path).
        let client = Endpoint::builder(presets::N0)
            .alpns(vec![ALPN.to_vec()])
            .transport_config(transport_config())
            .bind()
            .await
            .expect("client bind");
        let mut held_conns = Vec::new();
        let mut held_streams = Vec::new();
        for i in 0..MAX_SESSIONS {
            let conn = client.connect(server_id, ALPN).await.expect("connect");
            if i % 2 == 0 {
                held_conns.push(conn);
                continue;
            }
            let (mut send, recv) = conn.open_bi().await.expect("open bi");
            let mut hello = Vec::with_capacity(2 + NONCE_LEN + ED25519_PUB_LEN);
            hello.extend_from_slice(&PROTOCOL_VERSION.to_be_bytes());
            hello.extend_from_slice(&random_nonce());
            hello.extend_from_slice(&[0u8; ED25519_PUB_LEN]);
            write_frame(&mut send, FRAME_CLIENT_HELLO, 0, &hello)
                .await
                .expect("client hello");
            held_streams.push((send, recv));
        }

        // Wait until every slot is reserved by a stalled peer.
        for _ in 0..100 {
            if active.load(Ordering::Relaxed) >= MAX_SESSIONS {
                break;
            }
            tokio::time::sleep(Duration::from_millis(100)).await;
        }
        assert_eq!(
            active.load(Ordering::Relaxed),
            MAX_SESSIONS,
            "stalled peers should hold exactly the cap"
        );

        // The next peer must be refused, not admitted past the cap.
        let conn = client.connect(server_id, ALPN).await.expect("connect 9th");
        let (mut send, mut recv) = conn.open_bi().await.expect("open bi 9th");
        let mut hello = Vec::with_capacity(2 + NONCE_LEN + ED25519_PUB_LEN);
        hello.extend_from_slice(&PROTOCOL_VERSION.to_be_bytes());
        hello.extend_from_slice(&random_nonce());
        hello.extend_from_slice(&[0u8; ED25519_PUB_LEN]);
        write_frame(&mut send, FRAME_CLIENT_HELLO, 0, &hello)
            .await
            .expect("client hello 9th");
        // reject_session_full closes the connection right after writing
        // DENIED, so the client may legitimately see either the frame or
        // the connection drop — both mean "refused".
        let verdict = tokio::time::timeout(Duration::from_secs(10), async {
            match read_frame(&mut recv).await {
                Ok(f) => f.t,
                Err(_) => FRAME_DENIED,
            }
        })
        .await
        .expect("verdict in time");
        assert_eq!(verdict, FRAME_DENIED, "expected refusal for peer 9");

        // Over-cap peers that NEVER open a stream must also be reaped: the
        // rejection task is bounded, so their connections close within the
        // pre-auth timeout instead of leaking forever.
        let mut over_cap = Vec::new();
        let over_cap_count = REJECT_BUDGET + 8;
        for _ in 0..over_cap_count {
            match client.connect(server_id, ALPN).await {
                Ok(conn) => over_cap.push(conn),
                // Beyond-budget peers are dropped as unaccepted Incomings,
                // which the client observes as a connect refusal.
                Err(_) => {}
            }
        }

        // After the pre-auth timeout the stalled slots expire and the
        // cap opens again.
        tokio::time::sleep(Duration::from_secs(HANDSHAKE_TIMEOUT + 2)).await;
        assert!(
            active.load(Ordering::Relaxed) < MAX_SESSIONS,
            "slots must expire after the pre-auth timeout"
        );

        for conn in over_cap {
            let closed = tokio::time::timeout(Duration::from_secs(10), conn.closed()).await;
            assert!(
                closed.is_ok(),
                "over-cap silent peer connection must be reaped"
            );
        }

        accept_task.abort();
        drop(held_conns);
        drop(held_streams);
    }
}
