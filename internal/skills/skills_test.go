package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// LoadSkill
// ---------------------------------------------------------------------------

func TestLoadSkill_WithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	content := `---
name: my-skill
description: A test skill
tags:
  - testing
  - example
when: always
---
# Hello

This is the body.`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSkill(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", s.Name, "my-skill")
	}
	if s.Description != "A test skill" {
		t.Errorf("Description = %q, want %q", s.Description, "A test skill")
	}
	if len(s.Tags) != 2 || s.Tags[0] != "testing" || s.Tags[1] != "example" {
		t.Errorf("Tags = %v, want [testing example]", s.Tags)
	}
	if s.When != "always" {
		t.Errorf("When = %q, want %q", s.When, "always")
	}
	if !strings.Contains(s.Content, "# Hello") {
		t.Errorf("Content should contain '# Hello', got %q", s.Content)
	}
	if !strings.Contains(s.Content, "This is the body.") {
		t.Errorf("Content should contain body text, got %q", s.Content)
	}
	if s.Path == "" {
		t.Error("Path should be set")
	}
}

func TestLoadSkill_WithoutFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-tool.md")
	content := "# My Tool\n\nSome instructions."

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSkill(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "my-tool" {
		t.Errorf("Name = %q, want %q (derived from filename)", s.Name, "my-tool")
	}
	if s.Content != "# My Tool\n\nSome instructions." {
		t.Errorf("Content = %q, unexpected", s.Content)
	}
}

func TestLoadSkill_SKILLmd_DerivesNameFromDir(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "awesome-plugin")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte("Plugin instructions."), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSkill(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "awesome-plugin" {
		t.Errorf("Name = %q, want %q (derived from parent dir)", s.Name, "awesome-plugin")
	}
}

func TestLoadSkill_FrontmatterNameOverridesFilename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generic.md")
	content := "---\nname: specific-name\n---\nBody."
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSkill(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "specific-name" {
		t.Errorf("Name = %q, want %q", s.Name, "specific-name")
	}
}

func TestLoadSkill_InvalidFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.md")
	content := "---\n: invalid yaml {{{\n---\nBody."
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadSkill(path)
	if err == nil {
		t.Error("expected error for invalid frontmatter")
	}
}

func TestLoadSkill_OpeningSepNoClosing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.md")
	content := "---\nsome text without closing separator"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSkill(path)
	if err != nil {
		t.Fatal(err)
	}
	// Entire file becomes content.
	if !strings.Contains(s.Content, "some text") {
		t.Errorf("Content = %q, expected to contain the text", s.Content)
	}
}

func TestLoadSkill_NonexistentFile(t *testing.T) {
	_, err := LoadSkill("/nonexistent/path.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// ---------------------------------------------------------------------------
// LoadSkillsFromDir
// ---------------------------------------------------------------------------

func TestLoadSkillsFromDir_Mixed(t *testing.T) {
	dir := t.TempDir()

	// Direct .md file.
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("Skill A"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Direct .txt file.
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("Skill B"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-skill file — should be ignored.
	if err := os.WriteFile(filepath.Join(dir, "c.go"), []byte("not a skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Subdirectory with SKILL.md.
	subDir := filepath.Join(dir, "sub-skill")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "SKILL.md"), []byte("Skill Sub"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Subdirectory without SKILL.md — should be ignored.
	emptyDir := filepath.Join(dir, "empty-dir")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	skills, err := LoadSkillsFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(skills))
	}

	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}
	for _, want := range []string{"a", "b", "sub-skill"} {
		if !names[want] {
			t.Errorf("missing skill %q", want)
		}
	}
}

func TestLoadSkillsFromDir_NonexistentDir(t *testing.T) {
	skills, err := LoadSkillsFromDir("/nonexistent/dir")
	if err != nil {
		t.Fatal("should not error for missing directory")
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadSkillsFromDir_CaseInsensitiveSKILLmd(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Lowercase skill.md should also be found.
	if err := os.WriteFile(filepath.Join(subDir, "skill.md"), []byte("lowercase skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := LoadSkillsFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "my-skill" {
		t.Errorf("Name = %q, want %q", skills[0].Name, "my-skill")
	}
}

// ---------------------------------------------------------------------------
// LoadSkills (auto-discovery)
// ---------------------------------------------------------------------------

func TestLoadSkills_ProjectLocal(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, ".kit", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "local.md"), []byte("Local skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := LoadSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "local" {
		t.Errorf("Name = %q, want %q", skills[0].Name, "local")
	}
}

func TestLoadSkills_Deduplication(t *testing.T) {
	dir := t.TempDir()

	// Set XDG_CONFIG_HOME to our temp dir so global and local overlap.
	t.Setenv("XDG_CONFIG_HOME", dir)

	globalDir := filepath.Join(dir, "kit", "skills")
	localDir := filepath.Join(dir, ".kit", "skills")

	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Same content in both directories but different paths — both should load.
	if err := os.WriteFile(filepath.Join(globalDir, "shared.md"), []byte("Global version"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "shared.md"), []byte("Local version"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := LoadSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Different absolute paths = both loaded.
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills (different paths), got %d", len(skills))
	}
}

// ---------------------------------------------------------------------------
// FormatForPrompt
// ---------------------------------------------------------------------------

func TestFormatForPrompt_Empty(t *testing.T) {
	result := FormatForPrompt(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatForPrompt_SingleSkill(t *testing.T) {
	skills := []*Skill{
		{Name: "test-skill", Description: "A test", Content: "Do things."},
	}
	result := FormatForPrompt(skills)
	if !strings.Contains(result, "## test-skill") {
		t.Errorf("result should contain skill name header")
	}
	if !strings.Contains(result, "A test") {
		t.Errorf("result should contain description")
	}
	if !strings.Contains(result, "Do things.") {
		t.Errorf("result should contain content")
	}
}

func TestFormatForPrompt_MultipleSkills(t *testing.T) {
	skills := []*Skill{
		{Name: "skill-a", Content: "A content"},
		{Name: "skill-b", Description: "B desc", Content: "B content"},
	}
	result := FormatForPrompt(skills)
	if !strings.Contains(result, "## skill-a") {
		t.Error("missing skill-a header")
	}
	if !strings.Contains(result, "## skill-b") {
		t.Error("missing skill-b header")
	}
	if !strings.Contains(result, "# Available Skills") {
		t.Error("missing top-level header")
	}
}
