// Package daemon implements kit's remote-session transport: a daemon mode
// (`kit daemon`) that hosts kit sessions for paired clients, and a client
// mode (`kit remote --host <name>`) that attaches a local terminal to that
// remote session. New clients pair via `kit daemon pair` + `kit remote
// --pair <code>`.
//
// The transport is iroh, in-process (see iroh.go): endpoint binding, n0
// relay and DNS discovery, and the QUIC streams all run inside the kit
// binary. The wire protocol is v1 — the same protocol the retired Rust
// kit-tunnel sidecar spoke — so kits from before and after the switch
// interoperate. See docs in the package files.
package daemon

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"
)

// CodeAlphabet excludes 0/O/1/I to keep codes readable and unambiguous when
// read aloud or copied. 8 characters from a 32-symbol alphabet is ~40 bits
// of entropy. The code stays valid for the daemon's lifetime and allows
// multiple sessions; guessing is throttled by the transport's handshake
// backoff (which also persists across failures), so treat the code like a
// password for the duration of the daemon run.
const (
	CodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	CodeLength   = 8
)

// HKDF domain separation. These values are part of the wire contract
// (protocol v1) and must never change: they are what old and new kits
// agree on.
var (
	// hkdfSalt/hkdfInfo/hkdfAuthMsg feed the code→seed and seed→pair-key
	// derivations; the pairing-tag roles are pairRoleClient and
	// pairRoleServer in iroh.go.
	hkdfSalt       = []byte("kit-remote-v1")
	hkdfInfo       = []byte("kit-remote tunnel seed")
	hkdfAuthMsg    = []byte("kit-remote auth")
	signContext    = []byte("kit-remote-v1-auth")
	pairWindowTime = 10 * time.Minute
)

// GenerateCode returns a fresh random pairing code of CodeLength characters.
func GenerateCode() (string, error) {
	buf := make([]byte, CodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("daemon: generate code: %w", err)
	}
	out := make([]byte, CodeLength)
	for i, b := range buf {
		out[i] = CodeAlphabet[int(b)%len(CodeAlphabet)]
	}
	return string(out), nil
}

// NormalizeCode uppercases a code and strips separators/spaces. It rejects
// anything that is not exactly CodeLength alphabet characters.
func NormalizeCode(raw string) (string, error) {
	code := strings.ToUpper(strings.Map(func(r rune) rune {
		switch r {
		case '-', ' ', '_':
			return -1
		}
		return r
	}, strings.TrimSpace(raw)))
	if len(code) != CodeLength {
		return "", fmt.Errorf("daemon: pairing code must be %d characters", CodeLength)
	}
	for _, r := range code {
		if !strings.ContainsRune(CodeAlphabet, r) {
			return "", fmt.Errorf("daemon: pairing code contains invalid character %q", r)
		}
	}
	return code, nil
}

// FormatCode renders a code for display as XXXX-XXXX.
func FormatCode(code string) string {
	if len(code) != CodeLength {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// SeedFromCode derives the 32-byte endpoint seed from a pairing code using
// HKDF-SHA256 with fixed salt/info. The code is normalized first so both
// sides agree on every input regardless of how the user typed it.
func SeedFromCode(rawCode string) ([]byte, error) {
	code, err := NormalizeCode(rawCode)
	if err != nil {
		return nil, err
	}
	return hkdfSha256([]byte(code), hkdfSalt, hkdfInfo, 32)
}

// hkdfSha256 is a minimal RFC 5869 HKDF (extract + expand). Implemented
// locally so the derivation inputs stay in one auditable place; the
// semantics are RFC-exact, matching every other implementation of the
// protocol.
func hkdfSha256(ikm, salt, info []byte, length int) ([]byte, error) {
	if length > 255*sha256.Size {
		return nil, fmt.Errorf("daemon: hkdf length too large")
	}
	// Extract: PRK = HMAC-SHA256(salt, IKM). An empty salt is treated as
	// HashLen zeros, per RFC 5869.
	if len(salt) == 0 {
		salt = make([]byte, sha256.Size)
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write(ikm)
	prk := mac.Sum(nil)

	// Expand: T(i) = HMAC-SHA256(PRK, T(i-1) | info | i).
	var out, t []byte
	for i := 1; len(out) < length; i++ {
		mac := hmac.New(sha256.New, prk)
		mac.Write(t)
		mac.Write(info)
		mac.Write([]byte{byte(i)})
		t = mac.Sum(nil)
		out = append(out, t...)
	}
	return out[:length], nil
}

// pairingTag computes the client/server HMAC proof used by the pairing
// handshake: HMAC-SHA256 over role || first || second with a key expanded
// from the seed. The client tag passes (cNonce, nil); the server tag
// passes (cNonce, sNonce).
func pairingTag(seed []byte, role string, first, second []byte) ([]byte, error) {
	key, err := hkdfSha256(seed, hkdfSalt, hkdfAuthMsg, 32)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(role))
	mac.Write(first)
	mac.Write(second)
	return mac.Sum(nil), nil
}

// constantTimeEqual is a small wrapper so call sites read clearly.
func constantTimeEqual(a, b []byte) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare(a, b) == 1
}
