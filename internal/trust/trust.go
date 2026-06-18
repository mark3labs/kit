// Package trust manages a persisted allowlist of project directories that the
// user has marked as trusted for loading project-local skills.
//
// Project-local skills (discovered under <project>/.agents/skills/ and
// <project>/.kit/skills/) are injected into the system prompt. A freshly
// cloned, untrusted repository could therefore smuggle instructions into the
// agent the moment the user runs Kit inside it. To mitigate this prompt-
// injection vector, project-level skill loading can be gated on an explicit
// trust decision recorded here.
//
// The allowlist is stored as JSON at $XDG_CONFIG_HOME/kit/trusted-projects.json
// (default ~/.config/kit/trusted-projects.json).
package trust

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Decision is the outcome of a trust prompt.
type Decision int

const (
	// Skip declines to load project skills this session and records nothing.
	Skip Decision = iota
	// Trust loads project skills this session and persists the directory.
	Trust
	// TrustOnce loads project skills this session without persisting.
	TrustOnce
)

// storeFileName is the basename of the persisted allowlist.
const storeFileName = "trusted-projects.json"

// Store is a persisted set of trusted project directories. The zero value is
// not usable — construct one with Load.
type Store struct {
	mu      sync.Mutex
	path    string
	trusted map[string]bool
}

// store mirrors the on-disk JSON layout.
type store struct {
	Projects []string `json:"projects"`
}

// DefaultPath returns the path the trust allowlist is persisted to, respecting
// $XDG_CONFIG_HOME. Returns the empty string when no home directory can be
// determined.
func DefaultPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "kit", storeFileName)
}

// Load reads the trust allowlist from path. A missing file yields an empty
// store (not an error). Pass an empty path to use DefaultPath.
func Load(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	s := &Store{path: path, trusted: map[string]bool{}}
	if path == "" {
		return s, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}

	var raw store
	if err := json.Unmarshal(data, &raw); err != nil {
		return s, err
	}
	for _, p := range raw.Projects {
		s.trusted[normalize(p)] = true
	}
	return s, nil
}

// IsTrusted reports whether dir has been marked trusted.
func (s *Store) IsTrusted(dir string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trusted[normalize(dir)]
}

// Trust records dir as trusted and persists the allowlist to disk.
func (s *Store) Trust(dir string) error {
	s.mu.Lock()
	s.trusted[normalize(dir)] = true
	s.mu.Unlock()
	return s.save()
}

// Untrust removes dir from the allowlist and persists the change.
func (s *Store) Untrust(dir string) error {
	s.mu.Lock()
	delete(s.trusted, normalize(dir))
	s.mu.Unlock()
	return s.save()
}

// save writes the allowlist to disk, creating parent directories as needed.
func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	s.mu.Lock()
	projects := make([]string, 0, len(s.trusted))
	for p := range s.trusted {
		projects = append(projects, p)
	}
	s.mu.Unlock()

	data, err := json.MarshalIndent(store{Projects: projects}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// normalize resolves dir to an absolute, symlink-evaluated path for stable
// comparison. It falls back to the cleaned input when resolution fails.
func normalize(dir string) string {
	if dir == "" {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Clean(dir)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}
