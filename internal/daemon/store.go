package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Credential stores for the pairing model.
//
// Both stores are plain JSON files with 0600 permissions under
// ~/.config/kit, written atomically (temp file + rename), matching how kit
// keeps its other local secrets.
//
//   - The client keeps a "host book": friendly name -> daemon endpoint id
//     plus the host's fingerprint. The endpoint id is the host's ed25519
//     public key; iroh's QUIC handshake authenticates the peer against it,
//     so dialing a stored id cannot be hijacked by an impostor endpoint.
//   - The host keeps an allowlist of paired clients: fingerprint ->
//     ed25519 public key. Reconnecting clients sign the handshake nonce;
//     the host verifies against this list. Revoking a client deletes an
//     entry — the host stores no secrets.

func sha256Sum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

// fingerprintShort is the human-facing form of a fingerprint: abcd…wxyz.
func fingerprintShort(fp string) string {
	if len(fp) <= 8 {
		return fp
	}
	return fp[:4] + "…" + fp[len(fp)-4:]
}

// ---------------------------------------------------------------------------
// Client side: known hosts
// ---------------------------------------------------------------------------

// HostEntry is one paired daemon, as known on the client.
type HostEntry struct {
	Name       string    `json:"name"`
	EndpointID string    `json:"endpoint_id"` // 64 hex chars (ed25519 public key)
	HostFP     string    `json:"host_fp"`     // Fingerprint(EndpointID)
	AddedAt    time.Time `json:"added_at"`
	LastUsed   time.Time `json:"last_used"`
}

type hostBookFile struct {
	Hosts []HostEntry `json:"hosts"`
}

func hostBookPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("daemon: config dir: %w", err)
	}
	return filepath.Join(base, "kit", "remote", "hosts.json"), nil
}

func readHostBook() ([]HostEntry, error) {
	path, err := hostBookPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("daemon: read host book: %w", err)
	}
	var book hostBookFile
	if err := json.Unmarshal(b, &book); err != nil {
		return nil, fmt.Errorf("daemon: parse host book %s: %w", path, err)
	}
	return book.Hosts, nil
}

func writeHostBook(hosts []HostEntry) error {
	path, err := hostBookPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("daemon: host book dir: %w", err)
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].Name < hosts[j].Name })
	b, err := json.MarshalIndent(hostBookFile{Hosts: hosts}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".hosts-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// SaveHost adds (or replaces) a paired host entry under the given name.
func SaveHost(name string, endpointID string) error {
	if name == "" {
		return fmt.Errorf("daemon: host name must not be empty")
	}
	hosts, err := readHostBook()
	if err != nil {
		return err
	}
	entry := HostEntry{
		Name:       name,
		EndpointID: endpointID,
		HostFP:     Fingerprint(mustHexDecode(endpointID)),
		AddedAt:    time.Now(),
		LastUsed:   time.Now(),
	}
	replaced := false
	for i := range hosts {
		if hosts[i].Name == name {
			entry.AddedAt = hosts[i].AddedAt
			hosts[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		hosts = append(hosts, entry)
	}
	return writeHostBook(hosts)
}

// GetHost returns the stored entry for name.
func GetHost(name string) (HostEntry, error) {
	hosts, err := readHostBook()
	if err != nil {
		return HostEntry{}, err
	}
	for _, h := range hosts {
		if h.Name == name {
			return h, nil
		}
	}
	return HostEntry{}, fmt.Errorf("daemon: no paired host named %q — run 'kit remote --pair <code>' first", name)
}

// ListHosts returns all paired hosts, sorted by name.
func ListHosts() ([]HostEntry, error) {
	hosts, err := readHostBook()
	if err != nil {
		return nil, err
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].Name < hosts[j].Name })
	return hosts, nil
}

// ForgetHost removes a stored host. Returns an error when unknown.
func ForgetHost(name string) error {
	hosts, err := readHostBook()
	if err != nil {
		return err
	}
	out := hosts[:0]
	found := false
	for _, h := range hosts {
		if h.Name == name {
			found = true
			continue
		}
		out = append(out, h)
	}
	if !found {
		return fmt.Errorf("daemon: no paired host named %q", name)
	}
	return writeHostBook(out)
}

// TouchHost updates last_used after a successful connection.
func TouchHost(name string) error {
	hosts, err := readHostBook()
	if err != nil {
		return err
	}
	for i := range hosts {
		if hosts[i].Name == name {
			hosts[i].LastUsed = time.Now()
			return writeHostBook(hosts)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Host side: authorized clients
// ---------------------------------------------------------------------------

// ClientEntry is one paired client, as known on the host.
type ClientEntry struct {
	FP       string    `json:"fp"`     // Fingerprint(client public key)
	PubKey   string    `json:"pubkey"` // 64 hex chars (ed25519 public)
	Label    string    `json:"label"`  // optional note set at pairing time
	AddedAt  time.Time `json:"added_at"`
	LastSeen time.Time `json:"last_seen"`
}

type allowlistFile struct {
	Clients []ClientEntry `json:"clients"`
}

func allowlistPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("daemon: config dir: %w", err)
	}
	return filepath.Join(base, "kit", "daemon", "authorized.json"), nil
}

func readAllowlist() ([]ClientEntry, error) {
	path, err := allowlistPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("daemon: read allowlist: %w", err)
	}
	var list allowlistFile
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("daemon: parse allowlist %s: %w", path, err)
	}
	return list.Clients, nil
}

func writeAllowlist(clients []ClientEntry) error {
	path, err := allowlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("daemon: allowlist dir: %w", err)
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i].FP < clients[j].FP })
	b, err := json.MarshalIndent(allowlistFile{Clients: clients}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".authorized-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// AuthorizeClient stores a freshly paired client's public key.
func AuthorizeClient(pubKeyHex string) (string, error) {
	raw, err := hex.DecodeString(pubKeyHex)
	if err != nil || len(raw) != ed25519PubLen {
		return "", fmt.Errorf("daemon: bad client public key")
	}
	fp := Fingerprint(raw)
	clients, err := readAllowlist()
	if err != nil {
		return "", err
	}
	for i := range clients {
		if clients[i].FP == fp {
			clients[i].LastSeen = time.Now()
			return fp, writeAllowlist(clients)
		}
	}
	clients = append(clients, ClientEntry{FP: fp, PubKey: pubKeyHex, AddedAt: time.Now(), LastSeen: time.Now()})
	return fp, writeAllowlist(clients)
}

// LookupClient verifies a fingerprint is authorized and returns its entry.
func LookupClient(fp string) (ClientEntry, bool, error) {
	clients, err := readAllowlist()
	if err != nil {
		return ClientEntry{}, false, err
	}
	for _, c := range clients {
		if c.FP == fp {
			return c, true, nil
		}
	}
	return ClientEntry{}, false, nil
}

// TouchClient updates last_seen after a successful authenticated handshake.
func TouchClient(fp string) error {
	clients, err := readAllowlist()
	if err != nil {
		return err
	}
	for i := range clients {
		if clients[i].FP == fp {
			clients[i].LastSeen = time.Now()
			return writeAllowlist(clients)
		}
	}
	return nil
}

// ListAuthorized returns all paired clients, sorted by fingerprint.
func ListAuthorized() ([]ClientEntry, error) {
	clients, err := readAllowlist()
	if err != nil {
		return nil, err
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i].FP < clients[j].FP })
	return clients, nil
}

// RevokeClient removes an authorized client by fingerprint (or by its
// unique short prefix). Returns the removed entry.
func RevokeClient(fpOrPrefix string) (ClientEntry, error) {
	clients, err := readAllowlist()
	if err != nil {
		return ClientEntry{}, err
	}
	matches := []ClientEntry{}
	for _, c := range clients {
		if len(fpOrPrefix) <= len(c.FP) && c.FP[:len(fpOrPrefix)] == fpOrPrefix {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return ClientEntry{}, fmt.Errorf("daemon: no paired client with fingerprint prefix %q — see 'kit daemon pair --list'", fpOrPrefix)
	}
	if len(matches) > 1 {
		return ClientEntry{}, fmt.Errorf("daemon: fingerprint prefix %q matches %d clients — be more specific", fpOrPrefix, len(matches))
	}
	out := clients[:0]
	for _, c := range clients {
		if c.FP != matches[0].FP {
			out = append(out, c)
		}
	}
	if err := writeAllowlist(out); err != nil {
		return ClientEntry{}, err
	}
	return matches[0], nil
}
