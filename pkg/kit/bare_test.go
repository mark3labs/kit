package kit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newBareTestKit builds a Kit in a temp directory that is deliberately
// "polluted" with an AGENTS.md and a project skill, so tests can assert what
// bare mode does and does not pick up.
func newBareTestKit(t *testing.T, bare bool) *Kit {
	t.Helper()
	t.Setenv("OPENAI_API_KEY", "sk-test")

	project := t.TempDir()
	agents := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("Always speak in Latin."), 0o644); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(project, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: demo-skill\ndescription: A project skill that must not leak.\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}

	// Keep user-level discovery out of the way so the test observes the
	// project directory only.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Chdir(project)

	k, err := New(context.Background(), &Options{
		Model:            "openai/gpt-4o-mini",
		Quiet:            true,
		NoSession:        true,
		NoExtensions:     true,
		DisableCoreTools: true,
		SkipConfig:       true,
		Bare:             bare,
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	t.Cleanup(func() { _ = k.Close() })
	return k
}

// TestBare_ExcludesProjectContextFromSystemPrompt is the load-bearing
// assertion for bare mode: nothing from the working directory reaches the
// model. It checks the composed system prompt rather than the loader
// internals, because that string is what actually ships to the provider.
func TestBare_ExcludesProjectContextFromSystemPrompt(t *testing.T) {
	k := newBareTestKit(t, true)

	prompt := k.v.GetString("system-prompt")

	for _, needle := range []string{
		"Instructions from:", // AGENTS.md section header
		"Always speak in Latin.",
		"demo-skill",
		"A project skill that must not leak.",
	} {
		if strings.Contains(prompt, needle) {
			t.Errorf("bare system prompt leaked project context: contains %q", needle)
		}
	}

	if !strings.Contains(prompt, "bare mode") {
		t.Error("bare system prompt should state that no project context was loaded")
	}

	if got := k.GetContextFiles(); len(got) != 0 {
		t.Errorf("bare mode: want no context files, got %d", len(got))
	}
	if got := k.GetSkills(); len(got) != 0 {
		t.Errorf("bare mode: want no skills, got %d", len(got))
	}
}

// TestBare_Disabled_LoadsProjectContext is the control: the same directory
// without --bare must still load AGENTS.md and the project skill. Without it
// the test above could pass for the wrong reason.
func TestBare_Disabled_LoadsProjectContext(t *testing.T) {
	k := newBareTestKit(t, false)

	prompt := k.v.GetString("system-prompt")

	if !strings.Contains(prompt, "Always speak in Latin.") {
		t.Error("non-bare system prompt should contain AGENTS.md content")
	}
	if !strings.Contains(prompt, "demo-skill") {
		t.Error("non-bare system prompt should contain the project skill")
	}
	if strings.Contains(prompt, "bare mode") {
		t.Error("non-bare system prompt should not mention bare mode")
	}

	if got := k.GetContextFiles(); len(got) != 1 {
		t.Errorf("non-bare mode: want 1 context file, got %d", len(got))
	}
}

// TestBare_KeepsWorkingDirectoryLine confirms bare mode still tells the model
// where it is. The file tools resolve relative paths against the working
// directory, so removing the line would break them; bare suppresses project
// context, not basic orientation.
func TestBare_KeepsWorkingDirectoryLine(t *testing.T) {
	k := newBareTestKit(t, true)

	if prompt := k.v.GetString("system-prompt"); !strings.Contains(prompt, "Current working directory:") {
		t.Error("bare system prompt should still report the working directory")
	}
}

// TestBare_ExplicitSkillsStillLoad verifies that --skill overrides bare mode.
// Bare suppresses discovery, never an explicit instruction from the user.
func TestBare_ExplicitSkillsStillLoad(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	dir := t.TempDir()
	skill := "---\nname: explicit-skill\ndescription: Named on the command line.\n---\n\nBody.\n"
	path := filepath.Join(dir, "explicit.md")
	if err := os.WriteFile(path, []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}

	k, err := New(context.Background(), &Options{
		Model:            "openai/gpt-4o-mini",
		Quiet:            true,
		NoSession:        true,
		NoExtensions:     true,
		DisableCoreTools: true,
		SkipConfig:       true,
		Bare:             true,
		Skills:           []string{path},
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	defer func() { _ = k.Close() }()

	if got := k.GetSkills(); len(got) != 1 {
		t.Fatalf("bare mode with explicit skill: want 1 skill, got %d", len(got))
	}
	if prompt := k.v.GetString("system-prompt"); !strings.Contains(prompt, "explicit-skill") {
		t.Error("explicitly named skill should reach the system prompt in bare mode")
	}
}

// TestBare_SubagentInheritsIsolation is a regression test for a bare parent
// spawning a non-bare child (CodeRabbit review on #109). The child would
// re-run project discovery and load the AGENTS.md, skills and extensions the
// parent deliberately refused — including executing extension code.
//
// It exercises the real helper Kit.Subagent uses, so any future isolation
// field added to inheritIsolationOptions is covered automatically.
func TestBare_SubagentInheritsIsolation(t *testing.T) {
	child := &Options{}
	inheritIsolationOptions(child, &Options{Bare: true})
	if !child.Bare {
		t.Error("bare parent must spawn a bare child")
	}

	// A non-bare parent must not silently make children bare.
	child = &Options{}
	inheritIsolationOptions(child, &Options{Bare: false})
	if child.Bare {
		t.Error("non-bare parent must not produce a bare child")
	}

	// Nil operands are a no-op rather than a panic.
	inheritIsolationOptions(nil, &Options{Bare: true})
	inheritIsolationOptions(&Options{}, nil)
}
