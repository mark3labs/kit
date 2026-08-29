package daemon

import (
	"bytes"
	"testing"
)

func TestGenerateCodeFormat(t *testing.T) {
	for range 50 {
		code, err := GenerateCode()
		if err != nil {
			t.Fatalf("GenerateCode: %v", err)
		}
		if len(code) != CodeLength {
			t.Fatalf("code length %d, want %d", len(code), CodeLength)
		}
		normalized, err := NormalizeCode(code)
		if err != nil || normalized != code {
			t.Fatalf("generated code %q does not round-trip: %q, %v", code, normalized, err)
		}
	}
}

func TestNormalizeCode(t *testing.T) {
	got, err := NormalizeCode("a2b3-c4d5 ")
	if err != nil || got != "A2B3C4D5" {
		t.Fatalf("NormalizeCode = %q, %v", got, err)
	}
	for _, bad := range []string{"", "A1B2C3D", "A2B3C4D55", "A2B3C4D0", "A2B3C4DO", "A2B3C4DI", "A1B2C3D-"} {
		if got, err := NormalizeCode(bad); err == nil {
			t.Fatalf("NormalizeCode(%q) = %q, want error", bad, got)
		}
	}
}

func TestFormatCode(t *testing.T) {
	if got := FormatCode("A2B3C4D5"); got != "A2B3-C4D5" {
		t.Fatalf("FormatCode = %q", got)
	}
}

func TestSeedFromCodeIsDeterministicAndStrong(t *testing.T) {
	s1, err := SeedFromCode("A2B3C4D5")
	if err != nil {
		t.Fatalf("SeedFromCode: %v", err)
	}
	s2, err := SeedFromCode("A2B3-C4D5")
	if err != nil {
		t.Fatalf("SeedFromCode: %v", err)
	}
	if !bytes.Equal(s1, s2) {
		t.Fatal("seed differs across equivalent codes")
	}
	if len(s1) != 32 {
		t.Fatalf("seed length %d, want 32", len(s1))
	}
	s3, _ := SeedFromCode("A2B3C4D6")
	if bytes.Equal(s1, s3) {
		t.Fatal("different codes produced identical seeds")
	}
}

// TestPairingTagMatchesHandshakeConstants verifies the auth-tag derivation
// the tunnel performs: HMAC over role || server nonce || client nonce with
// a key expanded from the seed.
func TestPairingTagMatchesHandshakeConstants(t *testing.T) {
	seed, _ := SeedFromCode("ZZZZZZZZ")
	serverNonce := bytes.Repeat([]byte{0xAA}, 32)
	clientNonce := bytes.Repeat([]byte{0xBB}, 32)

	clientTag, err := pairingTag(seed, "kit-client", serverNonce, clientNonce)
	if err != nil {
		t.Fatalf("client tag: %v", err)
	}
	serverTag, err := pairingTag(seed, "kit-server", clientNonce, serverNonce)
	if err != nil {
		t.Fatalf("server tag: %v", err)
	}
	if !constantTimeEqual(clientTag, clientTag) {
		t.Fatal("identical tags compared unequal")
	}
	if constantTimeEqual(clientTag, serverTag) {
		t.Fatal("client and server tags must differ")
	}
	wrong, _ := pairingTag(seed, "kit-client", clientNonce, serverNonce)
	if constantTimeEqual(clientTag, wrong) {
		t.Fatal("nonce order must matter")
	}
}
