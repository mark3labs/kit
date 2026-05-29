package kit

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/skills"
)

// TestAddSkill_AddsAndDeduplicates verifies that AddSkill registers new skills
// and that re-adding a skill with the same Name replaces the existing entry
// rather than appending a duplicate. agent is nil in these tests; the method
// must still mutate the in-memory state and tolerate the absent agent.
func TestAddSkill_AddsAndDeduplicates(t *testing.T) {
	k := &Kit{basePrompt: "base"}

	if err := k.AddSkill(&skills.Skill{Name: "alpha", Content: "first"}); err != nil {
		t.Fatalf("AddSkill alpha: %v", err)
	}
	if err := k.AddSkill(&skills.Skill{Name: "beta", Content: "second"}); err != nil {
		t.Fatalf("AddSkill beta: %v", err)
	}
	got := k.GetSkills()
	if len(got) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(got))
	}

	// Re-adding alpha with new content must replace, not duplicate.
	if err := k.AddSkill(&skills.Skill{Name: "alpha", Content: "replaced"}); err != nil {
		t.Fatalf("AddSkill alpha replace: %v", err)
	}
	got = k.GetSkills()
	if len(got) != 2 {
		t.Fatalf("expected 2 skills after replace, got %d", len(got))
	}
	for _, s := range got {
		if s.Name == "alpha" && s.Content != "replaced" {
			t.Errorf("alpha content = %q; want %q", s.Content, "replaced")
		}
	}
}

// TestAddSkill_Validation rejects nil skills and unnamed skills with errors
// instead of corrupting state.
func TestAddSkill_Validation(t *testing.T) {
	k := &Kit{}
	if err := k.AddSkill(nil); err == nil {
		t.Error("expected error for nil skill")
	}
	if err := k.AddSkill(&skills.Skill{Content: "x"}); err == nil {
		t.Error("expected error for unnamed skill")
	}
	if got := k.GetSkills(); got != nil {
		t.Errorf("skills list mutated after invalid AddSkill calls: %#v", got)
	}
}

// TestRemoveSkill verifies removal and the false return for misses.
func TestRemoveSkill(t *testing.T) {
	k := &Kit{}
	_ = k.AddSkill(&skills.Skill{Name: "alpha"})
	_ = k.AddSkill(&skills.Skill{Name: "beta"})

	if removed := k.RemoveSkill("missing"); removed {
		t.Error("RemoveSkill(missing) = true; want false")
	}
	if removed := k.RemoveSkill("alpha"); !removed {
		t.Error("RemoveSkill(alpha) = false; want true")
	}
	got := k.GetSkills()
	if len(got) != 1 || got[0].Name != "beta" {
		t.Errorf("remaining skills = %#v; want [beta]", got)
	}
}

// TestSetSkills replaces the entire set and validates input.
func TestSetSkills(t *testing.T) {
	k := &Kit{}
	_ = k.AddSkill(&skills.Skill{Name: "alpha"})

	err := k.SetSkills([]*skills.Skill{
		{Name: "one"},
		{Name: "two"},
		{Name: "three"},
	})
	if err != nil {
		t.Fatalf("SetSkills: %v", err)
	}
	if got := k.GetSkills(); len(got) != 3 {
		t.Errorf("expected 3 skills, got %d", len(got))
	}

	// Invalid entry rejects the whole batch.
	bad := []*skills.Skill{{Name: "ok"}, nil}
	if err := k.SetSkills(bad); err == nil {
		t.Error("expected error when batch contains nil")
	}
	// State unchanged after rejected batch.
	if got := k.GetSkills(); len(got) != 3 {
		t.Errorf("skills mutated by rejected SetSkills batch: len=%d", len(got))
	}

	// Empty slice clears.
	if err := k.SetSkills(nil); err != nil {
		t.Fatalf("SetSkills(nil): %v", err)
	}
	if got := k.GetSkills(); got != nil {
		t.Errorf("expected nil skills after clear; got %#v", got)
	}
}

// TestLoadAndAddSkill round-trips a skill file from disk.
func TestLoadAndAddSkill(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.md")
	body := "---\nname: demo\ndescription: demo skill\n---\nhello world"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	k := &Kit{}
	s, err := k.LoadAndAddSkill(path)
	if err != nil {
		t.Fatalf("LoadAndAddSkill: %v", err)
	}
	if s.Name != "demo" {
		t.Errorf("loaded skill Name = %q; want demo", s.Name)
	}
	if got := k.GetSkills(); len(got) != 1 {
		t.Errorf("expected 1 skill registered, got %d", len(got))
	}
}

// TestAddContextFile_DeduplicatesByPath confirms identical paths replace
// rather than duplicate.
func TestAddContextFile_DeduplicatesByPath(t *testing.T) {
	k := &Kit{}
	if err := k.AddContextFile(&ContextFile{Path: "/a/AGENTS.md", Content: "v1"}); err != nil {
		t.Fatalf("AddContextFile: %v", err)
	}
	if err := k.AddContextFile(&ContextFile{Path: "/b/AGENTS.md", Content: "vB"}); err != nil {
		t.Fatalf("AddContextFile: %v", err)
	}
	if err := k.AddContextFile(&ContextFile{Path: "/a/AGENTS.md", Content: "v2"}); err != nil {
		t.Fatalf("AddContextFile replace: %v", err)
	}

	got := k.GetContextFiles()
	if len(got) != 2 {
		t.Fatalf("expected 2 context files, got %d", len(got))
	}
	for _, cf := range got {
		if cf.Path == "/a/AGENTS.md" && cf.Content != "v2" {
			t.Errorf("/a/AGENTS.md content = %q; want v2", cf.Content)
		}
	}
}

// TestAddContextFile_Validation rejects nil and unpathed entries.
func TestAddContextFile_Validation(t *testing.T) {
	k := &Kit{}
	if err := k.AddContextFile(nil); err == nil {
		t.Error("expected error for nil context file")
	}
	if err := k.AddContextFile(&ContextFile{Content: "x"}); err == nil {
		t.Error("expected error for empty path")
	}
}

// TestRemoveContextFile_Behavior verifies remove returns true on hit and
// false on miss without mutating state on a miss.
func TestRemoveContextFile_Behavior(t *testing.T) {
	k := &Kit{}
	_ = k.AddContextFile(&ContextFile{Path: "/a", Content: "x"})
	_ = k.AddContextFile(&ContextFile{Path: "/b", Content: "y"})

	if removed := k.RemoveContextFile("/missing"); removed {
		t.Error("RemoveContextFile(missing) = true; want false")
	}
	if removed := k.RemoveContextFile("/a"); !removed {
		t.Error("RemoveContextFile(/a) = false; want true")
	}
	got := k.GetContextFiles()
	if len(got) != 1 || got[0].Path != "/b" {
		t.Errorf("remaining = %#v; want [/b]", got)
	}
}

// TestSetContextFiles replaces and validates batch input.
func TestSetContextFiles(t *testing.T) {
	k := &Kit{}
	_ = k.AddContextFile(&ContextFile{Path: "/seed", Content: "old"})

	err := k.SetContextFiles([]*ContextFile{
		{Path: "/x", Content: "x"},
		{Path: "/y", Content: "y"},
	})
	if err != nil {
		t.Fatalf("SetContextFiles: %v", err)
	}
	if got := k.GetContextFiles(); len(got) != 2 {
		t.Errorf("expected 2 context files, got %d", len(got))
	}

	bad := []*ContextFile{{Path: "/ok"}, {Path: ""}}
	if err := k.SetContextFiles(bad); err == nil {
		t.Error("expected error for empty path in batch")
	}
	if got := k.GetContextFiles(); len(got) != 2 {
		t.Errorf("state mutated by rejected batch: len=%d", len(got))
	}

	if err := k.SetContextFiles(nil); err != nil {
		t.Fatalf("SetContextFiles(nil): %v", err)
	}
	if got := k.GetContextFiles(); got != nil {
		t.Errorf("expected nil after clear; got %#v", got)
	}
}

// TestLoadAndAddContextFile reads from disk and registers the context file.
func TestLoadAndAddContextFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	const content = "# Agent rules\nuse the new lint config"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	k := &Kit{}
	cf, err := k.LoadAndAddContextFile(path)
	if err != nil {
		t.Fatalf("LoadAndAddContextFile: %v", err)
	}
	if !strings.HasSuffix(cf.Path, "AGENTS.md") {
		t.Errorf("Path = %q; want suffix AGENTS.md", cf.Path)
	}
	if !strings.Contains(cf.Content, "use the new lint config") {
		t.Errorf("Content missing expected body: %q", cf.Content)
	}
	got := k.GetContextFiles()
	if len(got) != 1 {
		t.Fatalf("expected 1 context file, got %d", len(got))
	}
}

// TestAddContextFileContent registers an in-memory context blob.
func TestAddContextFileContent(t *testing.T) {
	k := &Kit{}
	cf, err := k.AddContextFileContent("session://user-123/AGENTS.md", "always greet in French")
	if err != nil {
		t.Fatalf("AddContextFileContent: %v", err)
	}
	if cf.Path != "session://user-123/AGENTS.md" {
		t.Errorf("Path = %q", cf.Path)
	}
	if cf.Content != "always greet in French" {
		t.Errorf("Content = %q", cf.Content)
	}
}

// TestComposeSystemPrompt_IncludesSkillsAndContext verifies that runtime
// mutations actually flow into the composed system prompt that the agent
// would receive.
func TestComposeSystemPrompt_IncludesSkillsAndContext(t *testing.T) {
	k := &Kit{basePrompt: "BASE-PROMPT-MARKER"}

	if err := k.AddContextFile(&ContextFile{
		Path:    "/proj/AGENTS.md",
		Content: "CTX-MARKER-OK",
	}); err != nil {
		t.Fatalf("AddContextFile: %v", err)
	}
	if err := k.AddSkill(&skills.Skill{
		Name:        "greeter",
		Description: "SKILL-DESC-MARKER",
		Content:     "do greetings",
		Path:        "/skills/greeter.md",
	}); err != nil {
		t.Fatalf("AddSkill: %v", err)
	}

	composed := k.composeSystemPrompt(k.basePrompt)
	for _, want := range []string{
		"BASE-PROMPT-MARKER",
		"CTX-MARKER-OK",
		"/proj/AGENTS.md",
		"greeter",
		"SKILL-DESC-MARKER",
	} {
		if !strings.Contains(composed, want) {
			t.Errorf("composed prompt missing %q\n--- composed ---\n%s", want, composed)
		}
	}

	// Removing the skill should remove its marker from the next composition.
	k.RemoveSkill("greeter")
	composed = k.composeSystemPrompt(k.basePrompt)
	if strings.Contains(composed, "SKILL-DESC-MARKER") {
		t.Errorf("composed prompt still contains removed skill description:\n%s", composed)
	}
}

// TestRuntimeMutations_AreThreadSafe stresses the mutation API from multiple
// goroutines to surface data races under `go test -race`.
func TestRuntimeMutations_AreThreadSafe(t *testing.T) {
	// Use a non-nil agent so applyComposedSystemPrompt actually invokes
	// agent.SetSystemPrompt (a no-op agent is fine — we only need the
	// systemPrompt mutation + fantasy rebuild path to run concurrently so
	// -race can observe any unsynchronized writes).
	k := &Kit{basePrompt: "base", agent: &agent.Agent{}}
	var wg sync.WaitGroup
	const goroutines = 8
	const iterations = 50

	for g := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range iterations {
				_ = k.AddSkill(&skills.Skill{
					Name:    "skill",
					Content: "content",
				})
				_ = k.AddContextFile(&ContextFile{
					Path:    "/shared/AGENTS.md",
					Content: "shared",
				})
				_ = k.GetSkills()
				_ = k.GetContextFiles()
				_ = k.composeSystemPrompt("base")
				k.RemoveSkill("skill")
				k.RemoveContextFile("/shared/AGENTS.md")
			}
		}(g)
	}
	wg.Wait()
}
