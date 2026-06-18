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

func writeSkill(t *testing.T, path, name, desc, body string) {
	t.Helper()
	content := "---\nname: " + name + "\ndescription: " + desc + "\n---\n" + body
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSkills_ProjectLocal(t *testing.T) {
	dir := t.TempDir()
	// Isolate user-level scopes so the host machine's skills don't leak in.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("HOME", filepath.Join(dir, "home"))
	skillsDir := filepath.Join(dir, ".kit", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(skillsDir, "local.md"), "local", "A local skill", "Local skill")

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

// TestLoadSkills_SkipsMissingDescription verifies that a skill without a
// required description is skipped during auto-discovery (gap #2).
func TestLoadSkills_SkipsMissingDescription(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("HOME", filepath.Join(dir, "home"))
	skillsDir := filepath.Join(dir, ".kit", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// No description — should be skipped.
	if err := os.WriteFile(filepath.Join(skillsDir, "nodesc.md"), []byte("Just a body"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(skillsDir, "good.md"), "good", "Has a description", "Body")

	skills, err := LoadSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (missing-description skipped), got %d", len(skills))
	}
	if skills[0].Name != "good" {
		t.Errorf("Name = %q, want %q", skills[0].Name, "good")
	}
}

// TestLoadSkills_NameCollisionPrecedence verifies project-level skills override
// user-level skills with the same name (gap #5).
func TestLoadSkills_NameCollisionPrecedence(t *testing.T) {
	dir := t.TempDir()

	// Set XDG_CONFIG_HOME so the user-level skill lives under our temp dir.
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", filepath.Join(dir, "home"))

	userDir := filepath.Join(dir, "kit", "skills")
	projectDir := filepath.Join(dir, ".kit", "skills")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeSkill(t, filepath.Join(userDir, "shared.md"), "shared", "User version", "USER")
	writeSkill(t, filepath.Join(projectDir, "shared.md"), "shared", "Project version", "PROJECT")

	skills, err := LoadSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (deduped by name), got %d", len(skills))
	}
	if !strings.Contains(skills[0].Content, "PROJECT") {
		t.Errorf("expected project version to win, got content %q", skills[0].Content)
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
		{Name: "test-skill", Description: "A test", Path: "/tmp/test-skill/SKILL.md", Content: "Do things."},
	}
	result := FormatForPrompt(skills)
	if !strings.Contains(result, "<name>test-skill</name>") {
		t.Errorf("result should contain skill name in XML")
	}
	if !strings.Contains(result, "<description>A test</description>") {
		t.Errorf("result should contain description in XML")
	}
	if !strings.Contains(result, "<location>/tmp/test-skill/SKILL.md</location>") {
		t.Errorf("result should contain bare file location (no file:// prefix)")
	}
	if !strings.Contains(result, "<available_skills>") {
		t.Errorf("result should contain available_skills root element")
	}
	// Content should NOT appear — metadata only.
	if strings.Contains(result, "Do things.") {
		t.Errorf("result should NOT contain skill content (metadata only)")
	}
}

func TestFormatForPrompt_MultipleSkills(t *testing.T) {
	skills := []*Skill{
		{Name: "skill-a", Path: "/tmp/a/SKILL.md", Content: "A content"},
		{Name: "skill-b", Description: "B desc", Path: "/tmp/b/SKILL.md", Content: "B content"},
	}
	result := FormatForPrompt(skills)
	if !strings.Contains(result, "<name>skill-a</name>") {
		t.Error("missing skill-a name")
	}
	if !strings.Contains(result, "<name>skill-b</name>") {
		t.Error("missing skill-b name")
	}
	if !strings.Contains(result, "<available_skills>") {
		t.Error("missing available_skills element")
	}
	if !strings.Contains(result, "Use the read tool") {
		t.Error("missing preamble instructions")
	}
}

// ---------------------------------------------------------------------------
// agentskills.io spec compliance (issue #65)
// ---------------------------------------------------------------------------

// TestFormatForPrompt_XMLEscaping verifies special characters in name and
// description are escaped so the catalog is valid XML (gap #1).
func TestFormatForPrompt_XMLEscaping(t *testing.T) {
	skills := []*Skill{
		{Name: "a&b", Description: "use when <tag> & \"quoted\"", Path: "/tmp/x"},
	}
	result := FormatForPrompt(skills)
	if strings.Contains(result, "<tag>") {
		t.Errorf("raw < should have been escaped, got: %q", result)
	}
	if !strings.Contains(result, "&lt;tag&gt;") {
		t.Errorf("expected escaped <tag>, got: %q", result)
	}
	if !strings.Contains(result, "a&amp;b") {
		t.Errorf("expected escaped ampersand in name, got: %q", result)
	}
}

// TestFormatForPrompt_DisableModelInvocation verifies that a skill flagged
// disable-model-invocation is omitted from the catalog (gap #10).
func TestFormatForPrompt_DisableModelInvocation(t *testing.T) {
	skills := []*Skill{
		{Name: "visible", Description: "shown", Path: "/tmp/a"},
		{Name: "hidden", Description: "not shown", Path: "/tmp/b", DisableModelInvocation: true},
	}
	result := FormatForPrompt(skills)
	if !strings.Contains(result, "<name>visible</name>") {
		t.Error("visible skill should be in catalog")
	}
	if strings.Contains(result, "<name>hidden</name>") {
		t.Error("disable-model-invocation skill should be omitted from catalog")
	}
}

// TestLoadSkill_NewSpecFields verifies the new frontmatter fields parse (gap #6).
func TestLoadSkill_NewSpecFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	content := `---
name: spec-skill
description: A spec-compliant skill
license: MIT
compatibility: claude-code, cursor
allowed-tools: read, bash
disable-model-invocation: true
metadata:
  author: jane
  version: "1.2"
---
Body.`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSkill(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.License != "MIT" {
		t.Errorf("License = %q, want MIT", s.License)
	}
	if s.Compatibility != "claude-code, cursor" {
		t.Errorf("Compatibility = %q", s.Compatibility)
	}
	if s.AllowedTools != "read, bash" {
		t.Errorf("AllowedTools = %q", s.AllowedTools)
	}
	if !s.DisableModelInvocation {
		t.Error("DisableModelInvocation should be true")
	}
	if s.Metadata["author"] != "jane" || s.Metadata["version"] != "1.2" {
		t.Errorf("Metadata = %v", s.Metadata)
	}
}

// TestLoadSkill_UnquotedColonFallback verifies the YAML repair fallback for
// the common `description: Use when: ...` mistake (gap #9).
func TestLoadSkill_UnquotedColonFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "colon.md")
	content := "---\nname: colon-skill\ndescription: Use when: extracting tables from PDFs\n---\nBody."
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSkill(path)
	if err != nil {
		t.Fatalf("expected unquoted-colon fallback to succeed, got error: %v", err)
	}
	if s.Name != "colon-skill" {
		t.Errorf("Name = %q", s.Name)
	}
	if !strings.Contains(s.Description, "extracting tables") {
		t.Errorf("Description = %q", s.Description)
	}
}

// TestValidate verifies the Validate diagnostics (gaps #2, #15).
func TestValidate(t *testing.T) {
	missing := &Skill{Name: "x"}
	diags := missing.Validate()
	if !hasError(diags) {
		t.Error("expected an error diagnostic for missing description")
	}

	ok := &Skill{Name: "x", Description: "y"}
	if len(ok.Validate()) != 0 {
		t.Error("expected no diagnostics for a complete skill")
	}
}

// TestSkillResources verifies bundled-resource enumeration (gaps #11, #15).
func TestSkillResources(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "run.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "REF.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Skill{Name: "my-skill", Path: filepath.Join(skillDir, "SKILL.md")}
	res := s.Resources()
	if len(res) != 2 {
		t.Fatalf("expected 2 resources, got %d: %v", len(res), res)
	}
	if s.BaseDir() != skillDir {
		t.Errorf("BaseDir = %q, want %q", s.BaseDir(), skillDir)
	}
	formatted := FormatResources(res)
	if !strings.Contains(formatted, "<file>references/REF.md</file>") {
		t.Errorf("FormatResources output missing reference: %q", formatted)
	}
	if !strings.Contains(formatted, "<file>scripts/run.py</file>") {
		t.Errorf("FormatResources output missing script: %q", formatted)
	}
}
