package trust

import (
	"path/filepath"
	"testing"
)

func TestStore_TrustAndPersist(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "trusted-projects.json")
	project := filepath.Join(dir, "repo")

	s, err := Load(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if s.IsTrusted(project) {
		t.Fatal("project should not be trusted initially")
	}
	if err := s.Trust(project); err != nil {
		t.Fatal(err)
	}
	if !s.IsTrusted(project) {
		t.Fatal("project should be trusted after Trust")
	}

	// Reload from disk to confirm persistence.
	s2, err := Load(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if !s2.IsTrusted(project) {
		t.Fatal("trust decision should persist across reloads")
	}
}

func TestStore_Untrust(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "trusted-projects.json")
	project := filepath.Join(dir, "repo")

	s, _ := Load(storePath)
	_ = s.Trust(project)
	if err := s.Untrust(project); err != nil {
		t.Fatal(err)
	}
	if s.IsTrusted(project) {
		t.Fatal("project should not be trusted after Untrust")
	}
}

func TestStore_MissingFileIsEmpty(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if s.IsTrusted("/anything") {
		t.Fatal("empty store should trust nothing")
	}
}
