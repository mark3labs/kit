package daemon

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// ed25519PubLen is the size of an ed25519 public key (and of an iroh
// endpoint id: the endpoint id IS the endpoint's ed25519 public key).
const ed25519PubLen = 32

// mustHexDecode decodes a hex string that the store already validated.
func mustHexDecode(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}

// Stable identities for the pairing model.
//
// The daemon owns a long-lived ed25519 keypair whose seed is also the iroh
// endpoint secret: the endpoint id clients store after pairing IS this
// public key, and iroh's QUIC handshake proves the peer holds it. The
// client owns its own ed25519 keypair used to sign reconnect handshakes;
// the host stores only the public key (an allowlist entry), so revoking a
// client is deleting a line.
//
// Key files hold the 32-byte seed as hex with 0600 permissions. Losing the
// daemon identity changes the endpoint id, which invalidates every stored
// client entry — both sides simply pair again.

// IdentityPaths resolves the key file locations under ~/.config/kit.
type IdentityPaths struct {
	DaemonSeed string // daemon endpoint seed (~/.config/kit/daemon/identity.key)
	ClientSeed string // client signing seed (~/.config/kit/remote/identity.key)
}

func identityPaths() (IdentityPaths, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return IdentityPaths{}, fmt.Errorf("daemon: config dir: %w", err)
	}
	return IdentityPaths{
		DaemonSeed: filepath.Join(base, "kit", "daemon", "identity.key"),
		ClientSeed: filepath.Join(base, "kit", "remote", "identity.key"),
	}, nil
}

// loadOrCreateSeed returns the 32-byte seed stored at path, generating and
// persisting a fresh one (0600) only when the file does not exist. An
// existing but invalid file is an error, never silently regenerated: for
// the daemon seed that would rotate the endpoint id and orphan every
// paired client.
func loadOrCreateSeed(path string) ([]byte, error) {
	if b, err := os.ReadFile(path); err == nil {
		if len(b) >= 64 {
			seed, derr := hex.DecodeString(string(b)[:64])
			if derr == nil && len(seed) == 32 {
				return seed, nil
			}
		}
		return nil, fmt.Errorf("daemon: corrupt identity file %s — fix or remove it (removing the daemon identity rotates the endpoint id and un-pairs every client)", path)
	}
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("daemon: generate identity: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("daemon: identity dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(seed)+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("daemon: write identity: %w", err)
	}
	return seed, nil
}

// LoadDaemonIdentity returns the daemon's endpoint seed, creating it on
// first use. The seed doubles as the iroh endpoint secret.
func LoadDaemonIdentity() ([]byte, error) {
	paths, err := identityPaths()
	if err != nil {
		return nil, err
	}
	return loadOrCreateSeed(paths.DaemonSeed)
}

// LoadClientIdentity returns the client's signing seed, creating it on
// first use.
func LoadClientIdentity() ([]byte, error) {
	paths, err := identityPaths()
	if err != nil {
		return nil, err
	}
	return loadOrCreateSeed(paths.ClientSeed)
}

// ClientKeyPair is the client's signing identity.
type ClientKeyPair struct {
	Seed   []byte
	Priv   ed25519.PrivateKey
	Pub    ed25519.PublicKey
	PubHex string // 64 hex chars
}

// NewClientKeyPair derives the full ed25519 keypair from the stored seed.
func NewClientKeyPair(seed []byte) ClientKeyPair {
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	return ClientKeyPair{Seed: seed, Priv: priv, Pub: pub, PubHex: hex.EncodeToString(pub)}
}

// Fingerprint is the short public identity used in prompts and stores:
// the first 16 hex chars of SHA-256 over the raw key bytes.
func Fingerprint(raw []byte) string {
	sum := sha256Sum(raw)
	return hex.EncodeToString(sum[:])[:16]
}
