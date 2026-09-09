package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCollectSkillsForValidation_PartialResults verifies that a skills
// directory with one malformed entry still yields its valid siblings
// alongside the error, so `kit skill validate` can report on both.
func TestCollectSkillsForValidation_PartialResults(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good")
	bad := filepath.Join(dir, "bad")
	for _, d := range []string{good, bad} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(good, "SKILL.md"), []byte("---\nname: good\ndescription: d\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "SKILL.md"), []byte("---\nname: [unclosed\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}

	list, err := collectSkillsForValidation(dir)
	if err == nil {
		t.Fatal("expected a load error for the malformed sibling")
	}
	if len(list) != 1 || list[0].Name != "good" {
		t.Fatalf("expected the valid sibling to survive, got %+v", list)
	}

	// A single skill directory resolves to its SKILL.md.
	single, err := collectSkillsForValidation(good)
	if err != nil || len(single) != 1 || single[0].Name != "good" {
		t.Fatalf("single skill dir: got %+v, %v", single, err)
	}
}
