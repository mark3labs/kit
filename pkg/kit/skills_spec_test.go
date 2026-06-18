package kit

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkillFile(t *testing.T, path, name, desc string) {
	t.Helper()
	content := "---\nname: " + name + "\ndescription: " + desc + "\n---\nBody."
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoadSkills_SkillsDirIsDirect verifies --skills-dir scans the directory
// directly rather than appending .agents/.kit beneath it (issue #65, gap #3).
func TestLoadSkills_SkillsDirIsDirect(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, filepath.Join(dir, "direct.md"), "direct", "A direct skill")

	got, err := loadSkills(&Options{SkillsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "direct" {
		t.Fatalf("expected 1 skill named 'direct', got %+v", got)
	}
}

// TestApplySkillDisableList verifies the disable list hides a skill from the
// catalog (issue #65, gap #10).
func TestApplySkillDisableList(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, filepath.Join(dir, "a.md"), "a", "skill a")
	writeSkillFile(t, filepath.Join(dir, "b.md"), "b", "skill b")

	got, err := loadSkills(&Options{SkillsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	applySkillDisableList(got, []string{"a"})

	var aDisabled, bDisabled bool
	for _, s := range got {
		switch s.Name {
		case "a":
			aDisabled = s.DisableModelInvocation
		case "b":
			bDisabled = s.DisableModelInvocation
		}
	}
	if !aDisabled {
		t.Error("skill 'a' should be disabled")
	}
	if bDisabled {
		t.Error("skill 'b' should not be disabled")
	}
}

// TestProjectSkillsTrust verifies the trust gate drops untrusted project
// skills and honours a Trust decision (issue #65, gap #8).
func TestProjectSkillsTrust(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("HOME", filepath.Join(dir, "home"))

	projectDir := filepath.Join(dir, "repo")
	skillsDir := filepath.Join(projectDir, ".agents", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSkillFile(t, filepath.Join(skillsDir, "proj.md"), "proj", "project skill")

	// Skip decision → no project skills loaded.
	skipped, err := loadSkills(&Options{
		SessionDir: projectDir,
		SkillTrustPrompt: func(_ string, _ int) TrustDecision {
			return SkipProjectSkills
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Fatalf("expected 0 skills when trust is skipped, got %d", len(skipped))
	}

	// Trust decision → project skills loaded and directory persisted.
	prompted := 0
	trusted, err := loadSkills(&Options{
		SessionDir: projectDir,
		SkillTrustPrompt: func(_ string, _ int) TrustDecision {
			prompted++
			return TrustProject
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(trusted) != 1 {
		t.Fatalf("expected 1 skill when trusted, got %d", len(trusted))
	}

	// A subsequent load should not prompt again (persisted trust).
	again, err := loadSkills(&Options{
		SessionDir: projectDir,
		SkillTrustPrompt: func(_ string, _ int) TrustDecision {
			t.Error("should not prompt for an already-trusted directory")
			return SkipProjectSkills
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 {
		t.Fatalf("expected 1 skill on trusted reload, got %d", len(again))
	}
}
