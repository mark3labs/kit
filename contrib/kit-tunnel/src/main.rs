//! kit-tunnel — transport sidecar for kit remote sessions.
//!
//! The sidecar owns everything iroh: endpoint binding, discovery, dialing,
//! and the pairing handshake. It exposes a byte-pump interface on its own
//! stdio so the Go side (kit daemon / kit --remote) needs no iroh code.
//!
//!   kit-tunnel serve --seed-hex <64 hex>   (daemon side; long-lived)
//!   kit-tunnel dial  --seed-hex <64 hex>   (client side; one connection)
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
//! Handshake flow (the client speaks first: in QUIC an open_bi stream
//! carries no bytes until the initiator writes, so a server-first hello on
//! a client-opened stream would never reach accept_bi):
//!
//!   client -> CLIENT_HELLO {ver, c_nonce}   (also materializes the stream)
//!   server -> SERVER_HELLO {ver, s_nonce}
//!   client -> CLIENT_AUTH  {HMAC(key, "kit-client" | s_nonce | c_nonce)}
//!   server -> SERVER_OK {HMAC(key, "kit-server" | c_nonce | s_nonce)} | DENIED
//!   server -> SESSION_ASSIGN {id}   (multi-session: this connection's id)
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
//! pairing code (see kit's internal/daemon/pairing.go). The server endpoint
//! identity is derived from the seed: anyone who can compute the endpoint id
//! is holding the code. The HMAC handshake additionally protects the live
//! endpoint from peers that learn its id without the code (e.g. by
//! observing DNS/relay traffic). Failed handshakes back off exponentially
//! (up to 8s) since the code no longer rotates per attempt.

use std::collections::HashMap;
use std::io::{self, Read, Write};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use hkdf::Hkdf;
use hmac::{Hmac, Mac};
use iroh::endpoint::{
    presets, IdleTimeout, Incoming, QuicTransportConfig, ReadExactError, RecvStream, SendStream,
};
use iroh::{Endpoint, SecretKey};
use sha2::Sha256;
use subtle::ConstantTimeEq;
use tokio::sync::mpsc;

const ALPN: &[u8] = b"kit/remote/1";
const PROTOCOL_VERSION: u16 = 2;

const MAX_PAYLOAD: usize = 65535;
/// Cap on concurrent sessions over one endpoint; extra peers are denied.
const MAX_SESSIONS: usize = 8;
const HANDSHAKE_TIMEOUT: u64 = 30;

// Frame types (shared with internal/daemon/protocol.go).
const FRAME_DATA: u8 = 0x01;
const FRAME_RESIZE: u8 = 0x02;
const FRAME_BYE: u8 = 0x03;
const FRAME_PING: u8 = 0x04;
const FRAME_PONG: u8 = 0x05;
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

fn secret_from_seed(seed: &[u8]) -> SecretKey {
    let mut bytes = [0u8; 32];
    bytes.copy_from_slice(seed);
    SecretKey::from_bytes(&bytes)
}

fn parse_seed(hex_seed: &str) -> Vec<u8> {
    hex::decode(hex_seed.trim()).unwrap_or_else(|e| fail(&format!("bad seed hex: {e}")))
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
    key: &[u8; 32],
    session: u32,
) -> anyhow::Result<()> {
    let hello = read_frame(recv).await?;
    if hello.t != FRAME_CLIENT_HELLO || hello.payload.len() < 2 + NONCE_LEN {
        anyhow::bail!("malformed client hello");
    }
    let c_ver = u16::from_be_bytes([hello.payload[0], hello.payload[1]]);
    if c_ver != PROTOCOL_VERSION {
        write_frame(send, FRAME_DENIED, 0, format!("version {c_ver}").as_bytes()).await?;
        anyhow::bail!("version mismatch: {c_ver}");
    }
    let c_nonce = hello.payload[2..2 + NONCE_LEN].to_vec();

    let s_nonce = random_nonce();
    let mut reply = Vec::with_capacity(2 + NONCE_LEN);
    reply.extend_from_slice(&PROTOCOL_VERSION.to_be_bytes());
    reply.extend_from_slice(&s_nonce);
    write_frame(send, FRAME_SERVER_HELLO, 0, &reply).await?;

    let auth = read_frame(recv).await?;
    if auth.t != FRAME_CLIENT_AUTH || auth.payload.len() != TAG_LEN {
        write_frame(send, FRAME_DENIED, 0, b"bad auth frame").await?;
        anyhow::bail!("malformed client auth");
    }
    let expect = hmac_tag(key, &[b"kit-client", &s_nonce, &c_nonce]);
    if auth.payload.as_slice().ct_eq(&expect).unwrap_u8() != 1 {
        write_frame(send, FRAME_DENIED, 0, b"bad tag").await?;
        anyhow::bail!("pairing tag mismatch");
    }

    let s_tag = hmac_tag(key, &[b"kit-server", &c_nonce, &s_nonce]);
    write_frame(send, FRAME_SERVER_OK, 0, &s_tag).await?;

    // Tell the client which session id to use on this connection. It goes
    // after the verdict so the client's handshake loop never sees it.
    write_frame(send, FRAME_SESSION_ASSIGN, 0, &session.to_be_bytes()).await?;
    Ok(())
}

async fn client_handshake(
    send: &mut SendStream,
    recv: &mut RecvStream,
    key: &[u8; 32],
) -> anyhow::Result<()> {
    let c_nonce = random_nonce();
    let mut hello = Vec::with_capacity(2 + NONCE_LEN);
    hello.extend_from_slice(&PROTOCOL_VERSION.to_be_bytes());
    hello.extend_from_slice(&c_nonce);
    write_frame(send, FRAME_CLIENT_HELLO, 0, &hello).await?;

    let reply = read_frame(recv).await?;
    if reply.t != FRAME_SERVER_HELLO || reply.payload.len() < 2 + NONCE_LEN {
        anyhow::bail!("malformed server hello");
    }
    let s_ver = u16::from_be_bytes([reply.payload[0], reply.payload[1]]);
    if s_ver != PROTOCOL_VERSION {
        anyhow::bail!("daemon version mismatch: {s_ver}");
    }
    let s_nonce = reply.payload[2..2 + NONCE_LEN].to_vec();

    let tag = hmac_tag(key, &[b"kit-client", &s_nonce, &c_nonce]);
    write_frame(send, FRAME_CLIENT_AUTH, 0, &tag).await?;

    let verdict = read_frame(recv).await?;
    match verdict.t {
        FRAME_SERVER_OK => {
            if verdict.payload.len() != TAG_LEN {
                anyhow::bail!("malformed server ok");
            }
            let expect = hmac_tag(key, &[b"kit-server", &c_nonce, &s_nonce]);
            if verdict.payload.as_slice().ct_eq(&expect).unwrap_u8() != 1 {
                anyhow::bail!("daemon failed tag verification");
            }
            Ok(())
        }
        FRAME_DENIED => {
            let reason = String::from_utf8_lossy(&verdict.payload);
            anyhow::bail!("daemon rejected the pairing code: {reason}");
        }
        other => anyhow::bail!("unexpected handshake frame {other:#04x}"),
    }
}

// ---------------------------------------------------------------------------
// Serve mode: one endpoint, many sessions
// ---------------------------------------------------------------------------

type Registry = Arc<Mutex<HashMap<u32, mpsc::UnboundedSender<Frame>>>>;

async fn serve(seed_hex: &str) {
    let seed = parse_seed(seed_hex);
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

    status(&format!("READY node_id={}", endpoint.id()));

    let registry: Registry = Arc::new(Mutex::new(HashMap::new()));
    let active = Arc::new(AtomicUsize::new(0));
    let backoff: BackoffState = Arc::new(Mutex::new(Backoff::default()));

    // Router: frames arriving on stdin are dispatched to the session named
    // in the frame header; unknown ids are dropped.
    {
        let registry = registry.clone();
        tokio::spawn(async move {
            loop {
                let frame =
                    tokio::task::block_in_place(|| read_frame_sync(&mut io::stdin().lock()));
                match frame {
                    Ok(Some(f)) => {
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

    accept_loop(endpoint, key, registry, active, backoff).await;
}

/// The serve accept loop: reserves a session slot per incoming connection
/// (atomically, so concurrent peers cannot all pass the cap check) and
/// hands each to `handle_connection`.
async fn accept_loop(
    endpoint: Endpoint,
    key: Arc<[u8; 32]>,
    registry: Registry,
    active: Arc<AtomicUsize>,
    backoff: BackoffState,
) {
    let mut next_id: u32 = 1;
    while let Some(incoming) = endpoint.accept().await {
        let slot = active.fetch_add(1, Ordering::Relaxed) + 1;
        if slot > MAX_SESSIONS {
            active.fetch_sub(1, Ordering::Relaxed);
            tokio::spawn(reject_session_full(incoming));
            continue;
        }
        let id = next_id;
        next_id = next_id.wrapping_add(1).max(1);
        tokio::spawn(handle_connection(
            incoming,
            id,
            key.clone(),
            registry.clone(),
            active.clone(),
            backoff.clone(),
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

/// Politely refuse a peer when the session cap is reached.
async fn reject_session_full(incoming: Incoming) {
    if let Ok(accepting) = incoming.accept() {
        if let Ok(conn) = accepting.await {
            if let Ok((mut send, mut recv)) = conn.accept_bi().await {
                let _ = read_frame(&mut recv).await; // consume client hello
                let _ = write_frame(&mut send, FRAME_DENIED, 0, b"too many sessions").await;
                status("DENIED reason=too many sessions");
            }
        }
    }
}

async fn handle_connection(
    incoming: Incoming,
    id: u32,
    key: Arc<[u8; 32]>,
    registry: Registry,
    active: Arc<AtomicUsize>,
    backoff: BackoffState,
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
        server_handshake(&mut send, &mut recv, &key, id),
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

async fn dial(seed_hex: &str, timeout_secs: u64) {
    let seed = parse_seed(seed_hex);
    let key = auth_key(&seed);
    // The daemon's endpoint id is the public half of the seed-derived key:
    // knowledge of the code is what makes the endpoint findable.
    let server_id = secret_from_seed(&seed).public();

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
        Err(_) => fail("connect to daemon: timed out"),
    };

    let (mut send, mut recv) = match conn.open_bi().await {
        Ok(pair) => pair,
        Err(e) => fail(&format!("open stream: {e}")),
    };

    let outcome = tokio::time::timeout(
        Duration::from_secs(timeout_secs),
        client_handshake(&mut send, &mut recv, &key),
    )
    .await;
    match outcome {
        Ok(Ok(())) => {}
        Ok(Err(e)) => {
            status(&format!("DENIED reason={e}"));
            return;
        }
        Err(_) => {
            status("DENIED reason=handshake timeout");
            return;
        }
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
    status("CLOSED");
}

// ---------------------------------------------------------------------------
// Args / entry
// ---------------------------------------------------------------------------

fn main() {
    // Tracing is opt-in via RUST_LOG; keep stderr clean for the Go side,
    // which only parses "STATUS ..." lines and buffers the rest.
    if std::env::var_os("RUST_LOG").is_some() {
        let _ = tracing_subscriber::fmt::try_init();
    }

    let args: Vec<String> = std::env::args().collect();
    let mut mode = String::new();
    let mut seed_hex = String::new();
    let mut timeout: u64 = 30;

    let mut i = 1;
    while i < args.len() {
        match args[i].as_str() {
            "serve" | "dial" => mode = args[i].clone(),
            "--seed-hex" => {
                i += 1;
                seed_hex = args.get(i).cloned().unwrap_or_default();
            }
            "--timeout" => {
                i += 1;
                timeout = args.get(i).and_then(|v| v.parse().ok()).unwrap_or(30);
            }
            other => fail(&format!("unknown argument: {other}")),
        }
        i += 1;
    }

    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .unwrap_or_else(|e| fail(&format!("tokio runtime: {e}")));

    runtime.block_on(async move {
        match mode.as_str() {
            "serve" => serve(&seed_hex).await,
            "dial" => dial(&seed_hex, timeout).await,
            other => fail(&format!(
                "usage: kit-tunnel serve|dial --seed-hex <hex> (got: {other:?})"
            )),
        }
    });
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Regression test: peers that complete the QUIC handshake but stall
    /// before authenticating hold session slots only for the pre-auth
    /// timeout — and until then, the session cap must keep rejecting new
    /// peers instead of letting the slot count grow unbounded.
    #[tokio::test(flavor = "multi_thread")]
    async fn stalled_pre_auth_peers_are_capped_and_expire() {
        let secret = SecretKey::generate();
        let key = Arc::new(auth_key(b"test-code-0001"));
        let endpoint = Endpoint::builder(presets::N0)
            .secret_key(secret.clone())
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
        let accept_task = tokio::spawn(accept_loop(
            endpoint,
            key.clone(),
            registry.clone(),
            active.clone(),
            backoff.clone(),
        ));

        // One client endpoint opens MAX_SESSIONS connections that stall
        // before authenticating — half without ever opening a stream (the
        // accept_bi path) and half with a bare CLIENT_HELLO (the handshake
        // read path). Both variants must hold a slot only until the
        // pre-auth timeout.
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
                // Stall before any stream: exercises the accept_bi timeout.
                held_conns.push(conn);
                continue;
            }
            let (mut send, recv) = conn.open_bi().await.expect("open bi");
            let mut hello = Vec::with_capacity(2 + NONCE_LEN);
            hello.extend_from_slice(&PROTOCOL_VERSION.to_be_bytes());
            hello.extend_from_slice(&random_nonce());
            write_frame(&mut send, FRAME_CLIENT_HELLO, 0, &hello)
                .await
                .expect("client hello");
            // Hold the streams open until long after the pre-auth timeout.
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
        let mut hello = Vec::with_capacity(2 + NONCE_LEN);
        hello.extend_from_slice(&PROTOCOL_VERSION.to_be_bytes());
        hello.extend_from_slice(&random_nonce());
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

        // After the pre-auth timeout the stalled slots expire and the
        // cap opens again.
        tokio::time::sleep(Duration::from_secs(HANDSHAKE_TIMEOUT + 2)).await;
        assert!(
            active.load(Ordering::Relaxed) < MAX_SESSIONS,
            "slots must expire after the pre-auth timeout"
        );

        accept_task.abort();
    }
}
