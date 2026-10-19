package daemon

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	fp1, created, err := AuthorizeClient(pub1)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("first authorize must report a created entry")
	}
	if _, again, err := AuthorizeClient(pub1); err != nil || again {
		t.Fatalf("re-authorize must refresh, not create: created=%v err=%v", again, err)
	}
	if _, _, err := AuthorizeClient(pub2); err != nil {
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

func TestWriteJSONFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	for _, data := range []string{`{"value":"long initial value"}`, `{"value":1}`} {
		if err := writeJSONFileAtomic(path, ".store-*", []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		assertStoreBytes(t, path, data+"\n")
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatalf("file mode = %o, want 600", info.Mode().Perm())
		}
		assertNoStoreTemps(t, dir, ".store-*")
	}
}

func TestWriteJSONFileAtomicRenameFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(path, "sentinel")
	if err := os.WriteFile(sentinel, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFileAtomic(path, ".store-*", []byte(`{}`), 0o600); err == nil {
		t.Fatal("expected rename error replacing a non-empty directory")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("destination is no longer a directory")
	}
	assertStoreBytes(t, sentinel, "unchanged")
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "sentinel" {
		t.Fatalf("destination contents changed: %v", entries)
	}
	assertNoStoreTemps(t, dir, ".store-*")
}

func assertStoreBytes(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("file bytes = %q, want %q", got, want)
	}
}

func assertNoStoreTemps(t *testing.T, dir, pattern string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}

func TestWriteHostBookOutput(t *testing.T) {
	isolateConfig(t)
	stamp := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	hosts := []HostEntry{
		{Name: "beta", EndpointID: "endpoint-b", HostFP: "fp-b", AddedAt: stamp, LastUsed: stamp},
		{Name: "alpha", EndpointID: "endpoint-a", HostFP: "fp-a", AddedAt: stamp, LastUsed: stamp},
	}
	if err := writeHostBook(hosts); err != nil {
		t.Fatal(err)
	}
	path, err := hostBookPath()
	if err != nil {
		t.Fatal(err)
	}
	assertStoreBytes(t, path, `{
  "hosts": [
    {
      "name": "alpha",
      "endpoint_id": "endpoint-a",
      "host_fp": "fp-a",
      "added_at": "2026-01-02T03:04:05Z",
      "last_used": "2026-01-02T03:04:05Z"
    },
    {
      "name": "beta",
      "endpoint_id": "endpoint-b",
      "host_fp": "fp-b",
      "added_at": "2026-01-02T03:04:05Z",
      "last_used": "2026-01-02T03:04:05Z"
    }
  ]
}
`)
	assertNoStoreTemps(t, filepath.Dir(path), ".hosts-*")
}

func TestWriteAllowlistOutput(t *testing.T) {
	isolateConfig(t)
	stamp := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	clients := []ClientEntry{
		{FP: "fp-b", PubKey: "key-b", Label: "beta", AddedAt: stamp, LastSeen: stamp},
		{FP: "fp-a", PubKey: "key-a", Label: "alpha", AddedAt: stamp, LastSeen: stamp},
	}
	if err := writeAllowlist(clients); err != nil {
		t.Fatal(err)
	}
	path, err := allowlistPath()
	if err != nil {
		t.Fatal(err)
	}
	assertStoreBytes(t, path, `{
  "clients": [
    {
      "fp": "fp-a",
      "pubkey": "key-a",
      "label": "alpha",
      "added_at": "2026-01-02T03:04:05Z",
      "last_seen": "2026-01-02T03:04:05Z"
    },
    {
      "fp": "fp-b",
      "pubkey": "key-b",
      "label": "beta",
      "added_at": "2026-01-02T03:04:05Z",
      "last_seen": "2026-01-02T03:04:05Z"
    }
  ]
}
`)
	assertNoStoreTemps(t, filepath.Dir(path), ".authorized-*")
}

func TestStoreDirectoryErrorContext(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   func() (string, error)
		write  func() error
		prefix string
	}{
		{"host book", hostBookPath, func() error { return writeHostBook(nil) }, "daemon: host book dir:"},
		{"allowlist", allowlistPath, func() error { return writeAllowlist(nil) }, "daemon: allowlist dir:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateConfig(t)
			path, err := tc.path()
			if err != nil {
				t.Fatal(err)
			}
			parent := filepath.Dir(path)
			if err := os.MkdirAll(filepath.Dir(parent), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(parent, []byte("not a directory"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := tc.write(); err == nil || !strings.HasPrefix(err.Error(), tc.prefix) {
				t.Fatalf("write error = %v, want prefix %q", err, tc.prefix)
			}
		})
	}
}

func TestSaveHostEmptyNameRejected(t *testing.T) {
	isolateConfig(t)
	if err := SaveHost("", "aabb"); err == nil {
		t.Fatal("expected empty-name error")
	}
}
