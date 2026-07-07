package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeAgent(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestLoad_FullFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := writeAgent(t, dir, "code-reviewer.md", `---
description: Reviews code for quality and best practices
model: anthropic/claude-sonnet-4
tools: [read, grep, find, ls]
temperature: 0.1
timeout: 300
hidden: true
disabled: false
---
You are in code review mode. Focus on correctness.`)

	a, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if a.Name != "code-reviewer" {
		t.Errorf("Name = %q, want code-reviewer", a.Name)
	}
	if a.Description != "Reviews code for quality and best practices" {
		t.Errorf("Description = %q", a.Description)
	}
	if a.Model != "anthropic/claude-sonnet-4" {
		t.Errorf("Model = %q", a.Model)
	}
	if want := []string{"read", "grep", "find", "ls"}; len(a.Tools) != len(want) {
		t.Errorf("Tools = %v, want %v", a.Tools, want)
	}
	if a.Temperature == nil || *a.Temperature != 0.1 {
		t.Errorf("Temperature = %v, want 0.1", a.Temperature)
	}
	if a.Timeout != 300*time.Second {
		t.Errorf("Timeout = %v, want 5m", a.Timeout)
	}
	if !a.Hidden {
		t.Error("Hidden should be true")
	}
	if a.Disabled {
		t.Error("Disabled should be false")
	}
	if a.SystemPrompt != "You are in code review mode. Focus on correctness." {
		t.Errorf("SystemPrompt = %q", a.SystemPrompt)
	}
	if a.FilePath != path {
		t.Errorf("FilePath = %q, want %q", a.FilePath, path)
	}
}

func TestLoad_MinimalFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := writeAgent(t, dir, "helper.md", "---\ndescription: Helps\n---\nBe helpful.")

	a, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if a.Model != "" || a.Tools != nil || a.Temperature != nil || a.Timeout != 0 || a.Hidden || a.Disabled {
		t.Errorf("optional fields should be zero-valued: %+v", a)
	}
}

func TestLoad_MissingDescription(t *testing.T) {
	dir := t.TempDir()
	path := writeAgent(t, dir, "bad.md", "---\nmodel: foo/bar\n---\nBody.")

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing description")
	} else if !strings.Contains(err.Error(), "description is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := writeAgent(t, dir, "plain.md", "Just a body, no frontmatter.")

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing frontmatter/description")
	}
}

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantFM   string
		wantBody string
	}{
		{"basic", "---\na: 1\n---\nbody", "a: 1", "body"},
		{"empty body", "---\na: 1\n---\n", "a: 1", ""},
		{"closing sep at EOF", "---\na: 1\n---", "a: 1", ""},
		{"no frontmatter", "body only", "", "body only"},
		{"crlf", "---\r\na: 1\r\n---\r\nbody", "a: 1", "body"},
		{"unclosed", "---\na: 1\nbody", "", "---\na: 1\nbody"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body := splitFrontmatter(tt.content)
			if fm != tt.wantFM {
				t.Errorf("fm = %q, want %q", fm, tt.wantFM)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestLoadFromDir(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "one.md", "---\ndescription: One\n---\nPrompt one.")
	writeAgent(t, dir, "two.md", "---\ndescription: Two\n---\nPrompt two.")
	writeAgent(t, dir, "broken.md", "---\nmodel: x\n---\nNo description.")
	writeAgent(t, dir, "notes.txt", "not an agent")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFromDir(dir)
	if err == nil {
		t.Error("expected aggregated error for broken.md")
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d agents, want 2", len(loaded))
	}
}

func TestLoadFromDir_Missing(t *testing.T) {
	loaded, err := LoadFromDir(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if loaded != nil {
		t.Errorf("expected nil agents, got %v", loaded)
	}
}

func TestMerge_PrecedenceAndDisabled(t *testing.T) {
	high := []*Agent{
		{Name: "shared", Description: "high"},
		{Name: "off", Description: "disabled high", Disabled: true},
	}
	low := []*Agent{
		{Name: "shared", Description: "low"},
		{Name: "off", Description: "shadowed low"},
		{Name: "solo", Description: "only low"},
	}

	merged := Merge(high, low)

	byName := map[string]*Agent{}
	for _, a := range merged {
		byName[a.Name] = a
	}
	if len(merged) != 2 {
		t.Fatalf("merged %d agents, want 2: %v", len(merged), byName)
	}
	if byName["shared"] == nil || byName["shared"].Description != "high" {
		t.Errorf("shared should resolve to high-precedence definition, got %+v", byName["shared"])
	}
	// A high-precedence disabled agent removes the name entirely, including
	// anything it shadows.
	if byName["off"] != nil {
		t.Error("disabled agent should be dropped")
	}
	if byName["solo"] == nil {
		t.Error("non-colliding low-precedence agent should survive")
	}
}

func TestBuiltins(t *testing.T) {
	builtins := Builtins()
	byName := map[string]*Agent{}
	for _, a := range builtins {
		byName[a.Name] = a
		if a.Description == "" {
			t.Errorf("builtin %s has no description", a.Name)
		}
		if a.Source != SourceBuiltin {
			t.Errorf("builtin %s has source %q", a.Name, a.Source)
		}
	}
	if byName["general"] == nil {
		t.Fatal("missing builtin: general")
	}
	if byName["explore"] == nil {
		t.Fatal("missing builtin: explore")
	}
	if len(byName["general"].Tools) != 0 {
		t.Errorf("general should have no tool restriction, got %v", byName["general"].Tools)
	}
	want := []string{"read", "grep", "find", "ls"}
	got := byName["explore"].Tools
	if len(got) != len(want) {
		t.Fatalf("explore tools = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("explore tools = %v, want %v", got, want)
		}
	}
}

func TestLoadAgents_Precedence(t *testing.T) {
	cwd := t.TempDir()
	// Isolate the user-level dir.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	userDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "kit", "agents")

	// User-level agent, shadowed by a project one.
	writeAgent(t, userDir, "reviewer.md", "---\ndescription: user reviewer\n---\nUser prompt.")
	// Project agents in both project scopes; .agents/agents wins over .kit/agents.
	writeAgent(t, filepath.Join(cwd, ".agents", "agents"), "reviewer.md", "---\ndescription: project reviewer\n---\nProject prompt.")
	writeAgent(t, filepath.Join(cwd, ".kit", "agents"), "reviewer.md", "---\ndescription: kit-dir reviewer\n---\nKit prompt.")
	// User-level only agent.
	writeAgent(t, userDir, "tester.md", "---\ndescription: writes tests\n---\nTest prompt.")
	// Project override disabling a builtin.
	writeAgent(t, filepath.Join(cwd, ".agents", "agents"), "explore.md", "---\ndescription: off\ndisabled: true\n---\n")

	loaded, err := LoadAgents(cwd)
	if err != nil {
		t.Fatalf("LoadAgents failed: %v", err)
	}

	byName := map[string]*Agent{}
	for _, a := range loaded {
		byName[a.Name] = a
	}

	if r := byName["reviewer"]; r == nil || r.Description != "project reviewer" {
		t.Errorf("reviewer should come from .agents/agents, got %+v", r)
	} else if r.Source != SourceProject {
		t.Errorf("reviewer source = %q, want project", r.Source)
	}
	if tst := byName["tester"]; tst == nil || tst.Source != SourceUser {
		t.Errorf("tester should come from user scope, got %+v", tst)
	}
	if byName["explore"] != nil {
		t.Error("disabled project override should remove builtin explore")
	}
	if byName["general"] == nil {
		t.Error("builtin general should still be present")
	}
}
