//! kit-tunnel — transport sidecar for kit remote sessions.
//!
//! The sidecar owns everything iroh: endpoint binding, discovery, dialing,
//! and the pairing handshake. It exposes a byte-pump interface on its own
//! stdio so the Go side (kit daemon / kit --remote) needs no iroh code:
//!
//!   kit-tunnel serve --seed-hex <64 hex>   (daemon side; accepts one pairing)
//!   kit-tunnel dial  --seed-hex <64 hex>   (client side; dials the daemon)
//!
//! Wire protocol on the iroh bi-directional stream, and on the sidecar's
//! own stdin/stdout, is the same frame format:
//!
//!   byte 0      frame type
//!   bytes 1..3  payload length (u16, big endian)
//!   bytes 3..   payload
//!
//! Handshake frames (0x10..0x13) are consumed inside the tunnel and never
//! forwarded. After a verified handshake the tunnel relays DATA, RESIZE and
//! BYE frames verbatim in both directions; PING/PONG are reserved for a
//! future keepalive and are currently forwarded like any other frame.
//!
//! Human-facing status goes to stderr as lines of the form:
//!
//!   STATUS READY node_id=<id>
//!   STATUS PAIRING
//!   STATUS VERIFIED
//!   STATUS DENIED reason=<text>
//!   STATUS CLOSED
//!   STATUS ERROR msg=<text>
//!
//! The pairing seed is 32 bytes of HKDF-SHA256 output derived from the
//! 8-character pairing code (see kit's internal/daemon/pairing.go). The
//! server endpoint identity is derived from the seed: anyone who can
//! compute the endpoint id is holding the code. The HMAC handshake below
//! additionally protects the live endpoint from peers that learn its id
//! without the code (e.g. by observing DNS/relay traffic).

use std::io::{self, Read, Write};

use hkdf::Hkdf;
use hmac::{Hmac, Mac};
use iroh::endpoint::{
    presets, IdleTimeout, QuicTransportConfig, ReadExactError, RecvStream, SendStream,
};
use iroh::{Endpoint, SecretKey};
use sha2::Sha256;
use subtle::ConstantTimeEq;

const ALPN: &[u8] = b"kit/remote/1";
const PROTOCOL_VERSION: u16 = 1;

const MAX_PAYLOAD: usize = 65535;

// Frame types (shared with internal/daemon/protocol.go).
const FRAME_DATA: u8 = 0x01;
const FRAME_RESIZE: u8 = 0x02;
const FRAME_BYE: u8 = 0x03;
const FRAME_PING: u8 = 0x04;
const FRAME_PONG: u8 = 0x05;
const FRAME_SERVER_HELLO: u8 = 0x10;
const FRAME_CLIENT_HELLO: u8 = 0x11;
const FRAME_SERVER_OK: u8 = 0x12;
const FRAME_DENIED: u8 = 0x13;
const FRAME_CLIENT_AUTH: u8 = 0x14;

// HKDF domain separation (must match pairing.go).
const HKDF_SALT: &[u8] = b"kit-remote-v1";
const HKDF_AUTH_INFO: &[u8] = b"kit-remote auth";

const NONCE_LEN: usize = 32;
const TAG_LEN: usize = 32;

fn status(line: &str) {
    eprintln!("STATUS {line}");
}

/// Transport tuning: a keep-alive plus a hard idle timeout so a silently
/// vanished peer (killed process, dropped network, sleeping laptop) is
/// detected in seconds instead of hanging the other side forever.
fn transport_config() -> QuicTransportConfig {
    QuicTransportConfig::builder()
        .keep_alive_interval(std::time::Duration::from_secs(5))
        .max_idle_timeout(Some(
            IdleTimeout::try_from(std::time::Duration::from_secs(20)).expect("valid timeout"),
        ))
        .build()
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

// ---------------------------------------------------------------------------
// Frame codec (sync, used for stdio; async wrappers below for iroh streams)
// ---------------------------------------------------------------------------

struct Frame {
    t: u8,
    payload: Vec<u8>,
}

fn write_frame_sync<W: Write>(w: &mut W, t: u8, payload: &[u8]) -> io::Result<()> {
    if payload.len() > MAX_PAYLOAD {
        return Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            "frame too large",
        ));
    }
    let len = (payload.len() as u16).to_be_bytes();
    let mut hdr = [0u8; 3];
    hdr[0] = t;
    hdr[1] = len[0];
    hdr[2] = len[1];
    w.write_all(&hdr)?;
    w.write_all(payload)?;
    w.flush()
}

fn read_frame_sync<R: Read>(r: &mut R) -> io::Result<Option<Frame>> {
    let mut hdr = [0u8; 3];
    match r.read_exact(&mut hdr) {
        Ok(()) => {}
        Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => return Ok(None),
        Err(e) => return Err(e),
    }
    let len = u16::from_be_bytes([hdr[1], hdr[2]]) as usize;
    let mut payload = vec![0u8; len];
    r.read_exact(&mut payload)?;
    Ok(Some(Frame { t: hdr[0], payload }))
}

// ---------------------------------------------------------------------------
// Handshake
//
// The client speaks first: in QUIC an open_bi stream carries no bytes until
// the initiator writes, so a server-first hello on a client-opened stream
// would never reach accept_bi. Flow:
//
//   client -> CLIENT_HELLO {ver, c_nonce}   (also materializes the stream)
//   server -> SERVER_HELLO {ver, s_nonce}
//   client -> CLIENT_AUTH  {HMAC(key, "kit-client" | s_nonce | c_nonce)}
//   server -> SERVER_OK {HMAC(key, "kit-server" | c_nonce | s_nonce)} | DENIED
// ---------------------------------------------------------------------------

async fn server_handshake(
    send: &mut SendStream,
    recv: &mut RecvStream,
    key: &[u8; 32],
) -> anyhow::Result<()> {
    let hello = read_frame(recv).await?;
    if hello.t != FRAME_CLIENT_HELLO || hello.payload.len() < 2 + NONCE_LEN {
        anyhow::bail!("malformed client hello");
    }
    let c_ver = u16::from_be_bytes([hello.payload[0], hello.payload[1]]);
    if c_ver != PROTOCOL_VERSION {
        write_frame(send, FRAME_DENIED, format!("version {c_ver}").as_bytes()).await?;
        anyhow::bail!("version mismatch: {c_ver}");
    }
    let c_nonce = hello.payload[2..2 + NONCE_LEN].to_vec();

    let s_nonce = random_nonce();
    let mut reply = Vec::with_capacity(2 + NONCE_LEN);
    reply.extend_from_slice(&PROTOCOL_VERSION.to_be_bytes());
    reply.extend_from_slice(&s_nonce);
    write_frame(send, FRAME_SERVER_HELLO, &reply).await?;

    let auth = read_frame(recv).await?;
    if auth.t != FRAME_CLIENT_AUTH || auth.payload.len() != TAG_LEN {
        write_frame(send, FRAME_DENIED, b"bad auth frame").await?;
        anyhow::bail!("malformed client auth");
    }
    let expect = hmac_tag(key, &[b"kit-client", &s_nonce, &c_nonce]);
    if auth.payload.as_slice().ct_eq(&expect).unwrap_u8() != 1 {
        write_frame(send, FRAME_DENIED, b"bad tag").await?;
        anyhow::bail!("pairing tag mismatch");
    }

    let s_tag = hmac_tag(key, &[b"kit-server", &c_nonce, &s_nonce]);
    write_frame(send, FRAME_SERVER_OK, &s_tag).await?;
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
    write_frame(send, FRAME_CLIENT_HELLO, &hello).await?;

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
    write_frame(send, FRAME_CLIENT_AUTH, &tag).await?;

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
// Frame I/O on iroh streams
// ---------------------------------------------------------------------------

async fn write_frame(send: &mut SendStream, t: u8, payload: &[u8]) -> anyhow::Result<()> {
    if payload.len() > MAX_PAYLOAD {
        anyhow::bail!("frame too large");
    }
    let mut buf = Vec::with_capacity(3 + payload.len());
    buf.push(t);
    buf.extend_from_slice(&(payload.len() as u16).to_be_bytes());
    buf.extend_from_slice(payload);
    send.write_all(&buf).await?;
    Ok(())
}

async fn read_frame(recv: &mut RecvStream) -> anyhow::Result<Frame> {
    let mut hdr = [0u8; 3];
    match recv.read_exact(&mut hdr).await {
        Ok(()) => {}
        Err(ReadExactError::FinishedEarly(_)) => anyhow::bail!("stream closed"),
        Err(e) => anyhow::bail!("read header: {e}"),
    }
    let len = u16::from_be_bytes([hdr[1], hdr[2]]) as usize;
    let mut payload = vec![0u8; len];
    recv.read_exact(&mut payload)
        .await
        .map_err(|e| anyhow::anyhow!("read payload: {e}"))?;
    Ok(Frame { t: hdr[0], payload })
}

// ---------------------------------------------------------------------------
// Relay pumps
// ---------------------------------------------------------------------------

/// iroh stream -> stdout. Returns when the stream closes or a BYE arrives.
async fn pump_remote_to_stdout(mut recv: RecvStream) {
    loop {
        let frame = match read_frame(&mut recv).await {
            Ok(f) => f,
            Err(_) => break,
        };
        if frame.t == FRAME_BYE {
            break;
        }
        let mut out = io::stdout().lock();
        if write_frame_sync(&mut out, frame.t, &frame.payload).is_err() {
            break; // our own stdio died; daemon or client is gone
        }
    }
    status("CLOSED");
}

/// stdin -> iroh stream. Returns when stdin hits EOF or a BYE is seen.
/// Blocking reads run via block_in_place so the guard never spans an await.
async fn pump_stdin_to_remote(mut send: SendStream) {
    loop {
        let frame = tokio::task::block_in_place(|| read_frame_sync(&mut io::stdin().lock()));
        match frame {
            Ok(Some(frame)) => {
                let bye = frame.t == FRAME_BYE;
                if let Err(e) = write_frame(&mut send, frame.t, &frame.payload).await {
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
                let _ = write_frame(&mut send, FRAME_BYE, &[]).await;
                break;
            }
            Err(e) => {
                eprintln!("STATUS ERROR msg=local stdin: {e}");
                break;
            }
        }
    }
}

async fn relay(send: iroh::endpoint::SendStream, recv: iroh::endpoint::RecvStream) {
    let up = tokio::spawn(pump_stdin_to_remote(send));
    pump_remote_to_stdout(recv).await;
    up.abort();
}

// ---------------------------------------------------------------------------
// Modes
// ---------------------------------------------------------------------------

async fn serve(seed_hex: &str) {
    let seed = parse_seed(seed_hex);
    let key = auth_key(&seed);
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

    // One connection per process: the Go daemon restarts the tunnel (and
    // rotates the code) after each pairing attempt, which doubles as a
    // rate limit against code guessing.
    let incoming = match endpoint.accept().await {
        Some(i) => i,
        None => fail("accept stream ended"),
    };
    let conn = match incoming.accept() {
        Ok(a) => match a.await {
            Ok(c) => c,
            Err(e) => fail(&format!("connect failed: {e}")),
        },
        Err(e) => fail(&format!("incoming rejected: {e}")),
    };
    status("PAIRING");

    let (mut send, mut recv) = match conn.accept_bi().await {
        Ok(pair) => pair,
        Err(e) => fail(&format!("open stream: {e}")),
    };

    let outcome = tokio::time::timeout(
        std::time::Duration::from_secs(30),
        server_handshake(&mut send, &mut recv, &key),
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
    status("VERIFIED");
    relay(send, recv).await;
}

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
        std::time::Duration::from_secs(timeout_secs),
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
        std::time::Duration::from_secs(timeout_secs),
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
    status("VERIFIED");
    relay(send, recv).await;
}

// ---------------------------------------------------------------------------
// Args / entry
// ---------------------------------------------------------------------------

fn secret_from_seed(seed: &[u8]) -> SecretKey {
    let mut bytes = [0u8; 32];
    bytes.copy_from_slice(seed);
    SecretKey::from_bytes(&bytes)
}

fn parse_seed(hex_seed: &str) -> Vec<u8> {
    hex::decode(hex_seed.trim()).unwrap_or_else(|e| fail(&format!("bad seed hex: {e}")))
}

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

// Frame types used only in comments above; silence unused warnings where a
// type is currently forwarded verbatim rather than interpreted.
#[allow(dead_code)]
fn _type_assertions(t: u8) -> bool {
    matches!(t, FRAME_DATA | FRAME_RESIZE | FRAME_PING | FRAME_PONG)
}
