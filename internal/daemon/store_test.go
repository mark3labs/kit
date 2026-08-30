package daemon

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// isolateConfig points the XDG config dir at a temp dir so tests never
// touch the user's real stores.
func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // Windows fallbacks via UserConfigDir
}

func TestLoadOrCreateSeedPersists(t *testing.T) {
	isolateConfig(t)
	paths, err := identityPaths()
	if err != nil {
		t.Fatal(err)
	}
	seed1, err := LoadDaemonIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if len(seed1) != 32 {
		t.Fatalf("seed length = %d, want 32", len(seed1))
	}
	info, err := os.Stat(paths.DaemonSeed)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("identity file mode = %o, want 600", perm)
	}
	seed2, err := LoadDaemonIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(seed1) != hex.EncodeToString(seed2) {
		t.Fatal("identity seed changed between loads")
	}
}

func TestClientKeyPairSignVerify(t *testing.T) {
	isolateConfig(t)
	seed, err := LoadClientIdentity()
	if err != nil {
		t.Fatal(err)
	}
	kp := NewClientKeyPair(seed)
	msg := []byte("kit-remote-v3-auth" + "nonce-c" + "nonce-s")
	sig := ed25519.Sign(kp.Priv, msg)
	if !ed25519.Verify(kp.Pub, msg, sig) {
		t.Fatal("signature does not verify")
	}
	if len(kp.PubHex) != 64 {
		t.Fatalf("pub hex length = %d, want 64", len(kp.PubHex))
	}
	// A tampered message must not verify.
	if ed25519.Verify(kp.Pub, []byte("tampered"), sig) {
		t.Fatal("tampered message verified")
	}
}

func TestFingerprintMatchesSHA256Prefix(t *testing.T) {
	raw := []byte("some client public key bytes")
	sum := sha256.Sum256(raw)
	want := hex.EncodeToString(sum[:])[:16]
	if got := Fingerprint(raw); got != want {
		t.Fatalf("Fingerprint = %s, want %s", got, want)
	}
}

func TestHostBookRoundTrip(t *testing.T) {
	isolateConfig(t)
	if err := SaveHost("beta", "aabbccdd00112233445566778899aabbccddeeff00112233445566778899aabb"); err != nil {
		t.Fatal(err)
	}
	if err := SaveHost("alpha", "1122334455667788aabbccddeeff0011223344556677889900aabbccddeeff11"); err != nil {
		t.Fatal(err)
	}
	hosts, err := ListHosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 || hosts[0].Name != "alpha" || hosts[1].Name != "beta" {
		t.Fatalf("unexpected host book: %+v", hosts)
	}
	got, err := GetHost("beta")
	if err != nil {
		t.Fatal(err)
	}
	if got.EndpointID != "aabbccdd00112233445566778899aabbccddeeff00112233445566778899aabb" {
		t.Fatalf("endpoint id mismatch: %s", got.EndpointID)
	}
	wantFP := Fingerprint(mustHexDecode(got.EndpointID))
	if got.HostFP != wantFP {
		t.Fatalf("host fp = %s, want %s", got.HostFP, wantFP)
	}
	// Saving the same name replaces the entry, keeping added_at.
	first := got.AddedAt
	time.Sleep(2 * time.Millisecond)
	if err := SaveHost("beta", "1111ccdd00112233445566778899aabbccddeeff00112233445566778899aabb"); err != nil {
		t.Fatal(err)
	}
	again, _ := GetHost("beta")
	if again.EndpointID[:4] != "1111" {
		t.Fatal("entry was not replaced")
	}
	if !again.AddedAt.Equal(first) {
		t.Fatal("added_at should be preserved on replace")
	}
	if err := ForgetHost("beta"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetHost("beta"); err == nil {
		t.Fatal("expected unknown-host error after forget")
	}
	if err := ForgetHost("nope"); err == nil {
		t.Fatal("expected error forgetting unknown host")
	}
}

func TestHostBookCorruptFileFails(t *testing.T) {
	isolateConfig(t)
	path, err := hostBookPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := GetHost("beta"); err == nil {
		t.Fatal("expected error on corrupt host book")
	}
}

func TestAllowlistAuthorizeLookupRevoke(t *testing.T) {
	isolateConfig(t)
	pub1 := "aabb" + "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"[4:]
	pub2 := "ccdd" + "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"[4:]
	fp1, err := AuthorizeClient(pub1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AuthorizeClient(pub2); err != nil {
		t.Fatal(err)
	}
	entry, ok, err := LookupClient(fp1)
	if err != nil || !ok {
		t.Fatalf("client not found after authorize: ok=%v err=%v", ok, err)
	}
	if entry.PubKey != pub1 {
		t.Fatalf("stored pubkey mismatch: %s", entry.PubKey)
	}
	// Short-prefix matching for revoke; ambiguous prefixes are refused.
	if _, err := RevokeClient("aa"); err == nil {
		t.Fatal("expected ambiguous-prefix error")
	}
	removed, err := RevokeClient(fp1)
	if err != nil {
		t.Fatal(err)
	}
	if removed.FP != fp1 {
		t.Fatalf("revoked wrong client: %s", removed.FP)
	}
	if _, ok, _ := LookupClient(fp1); ok {
		t.Fatal("client still authorized after revoke")
	}
	if _, err := RevokeClient(fp1); err == nil {
		t.Fatal("expected error revoking unknown client")
	}
}

func TestSaveHostEmptyNameRejected(t *testing.T) {
	isolateConfig(t)
	if err := SaveHost("", "aabb"); err == nil {
		t.Fatal("expected empty-name error")
	}
}
