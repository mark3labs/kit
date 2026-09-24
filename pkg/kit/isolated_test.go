package kit

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// applyOptions builds an Options value the way NewAgent does, without
// constructing a Kit.
func applyOptions(opts ...Option) *Options {
	o := &Options{}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

func TestIsolated_DisablesAmbientFeatures(t *testing.T) {
	o := applyOptions(Isolated())

	checks := map[string]bool{
		"SkipConfig":       o.SkipConfig,
		"NoContextFiles":   o.NoContextFiles,
		"NoSkills":         o.NoSkills,
		"NoExtensions":     o.NoExtensions,
		"NoAgents":         o.NoAgents,
		"NoSession":        o.NoSession,
		"DisableCoreTools": o.DisableCoreTools,
	}
	for name, set := range checks {
		if !set {
			t.Errorf("Isolated() must set %s", name)
		}
	}
	// Bare is one switch for everything; Isolated deliberately uses the
	// separate fields so each one can be turned back on.
	if o.Bare {
		t.Error("Isolated() must not set Bare")
	}
}

func TestIsolated_OptInsReenableFeatures(t *testing.T) {
	o := applyOptions(
		Isolated(),
		WithConfig(),
		WithContextFiles(),
		WithSkills("/tmp/skill.md"),
		WithExtensions(),
		WithAgents(),
		WithSessions(),
		WithCoreTools("read", "grep"),
	)

	if o.SkipConfig || o.NoContextFiles || o.NoSkills || o.NoExtensions ||
		o.NoAgents || o.NoSession || o.DisableCoreTools {
		t.Errorf("opt-in options did not re-enable every feature: %+v", o)
	}
	if !slices.Equal(o.Skills, []string{"/tmp/skill.md"}) {
		t.Errorf("WithSkills paths = %v", o.Skills)
	}
	if !slices.Equal(o.CoreToolList, []string{"read", "grep"}) {
		t.Errorf("WithCoreTools names = %v", o.CoreToolList)
	}
}

// TestIsolated_OrderMatters documents that options apply in order: an
// opt-in placed before Isolated is overridden by it.
func TestIsolated_OrderMatters(t *testing.T) {
	o := applyOptions(WithCoreTools(), WithSessions(), Isolated())
	if !o.DisableCoreTools || !o.NoSession {
		t.Error("Isolated() placed last must win over earlier opt-ins")
	}
}

// TestWithOptInsWithoutArgs checks that the no-argument forms keep the
// defaults: auto-discovered skills and the full core tool set.
func TestWithOptInsWithoutArgs(t *testing.T) {
	o := applyOptions(Isolated(), WithSkills(), WithCoreTools())
	if o.Skills != nil {
		t.Errorf("WithSkills() must keep auto-discovery, got Skills=%v", o.Skills)
	}
	if o.CoreToolList != nil {
		t.Errorf("WithCoreTools() must keep all tools, got CoreToolList=%v", o.CoreToolList)
	}
}

// TestWithConfigFile_LoadsUnderIsolated: an explicitly named config file is
// a request to load it, so it must cancel Isolated's SkipConfig.
func TestWithConfigFile_LoadsUnderIsolated(t *testing.T) {
	o := applyOptions(Isolated(), WithConfigFile("/tmp/kit.yml"))
	if o.SkipConfig {
		t.Error("WithConfigFile must clear SkipConfig")
	}
	if o.ConfigFile != "/tmp/kit.yml" {
		t.Errorf("ConfigFile = %q", o.ConfigFile)
	}
}

// newIsolatedTestEnv creates a fake home with a ~/.kit.yml and a working
// directory with an AGENTS.md, a project skill and a project .kit.yml, so
// tests can see what Isolated keeps out.
func newIsolatedTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("OPENAI_API_KEY", "sk-test")

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	if err := os.WriteFile(filepath.Join(home, ".kit.yml"), []byte("temperature: 0.123\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte("Always speak in Latin."), 0o644); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(project, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: demo-skill\ndescription: A project skill.\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
}

func newIsolatedTestKit(t *testing.T, opts ...Option) *Kit {
	t.Helper()
	opts = append([]Option{WithModel("openai/gpt-4o-mini")}, opts...)
	k, err := NewIsolatedAgent(context.Background(), opts...)
	if err != nil {
		t.Fatalf("NewIsolatedAgent: %v", err)
	}
	t.Cleanup(func() { _ = k.Close() })
	return k
}

func TestNewIsolatedAgent_LoadsNothingAmbient(t *testing.T) {
	newIsolatedTestEnv(t)
	k := newIsolatedTestKit(t)

	if got := k.v.GetFloat64("temperature"); got == 0.123 {
		t.Error("isolated agent loaded ~/.kit.yml")
	}
	if got := k.GetContextFiles(); len(got) != 0 {
		t.Errorf("isolated agent loaded %d context files", len(got))
	}
	if got := k.GetSkills(); len(got) != 0 {
		t.Errorf("isolated agent loaded %d skills", len(got))
	}
	if got := k.GetAgents(); len(got) != 0 {
		t.Errorf("isolated agent loaded %d named agents", len(got))
	}
	if got := k.GetToolNames(); len(got) != 0 {
		t.Errorf("isolated agent has tools: %v", got)
	}
	if got := k.GetSessionPath(); got != "" {
		t.Errorf("isolated agent persists its session to %s", got)
	}
}

// TestNewIsolatedAgent_OptIns is the control for the test above: the same
// environment with each feature turned back on must load it. Without it the
// test above could pass for the wrong reason.
func TestNewIsolatedAgent_OptIns(t *testing.T) {
	newIsolatedTestEnv(t)
	k := newIsolatedTestKit(t,
		WithConfig(),
		WithContextFiles(),
		WithSkills(),
		WithAgents(),
		WithCoreTools("read"),
	)

	if got := k.v.GetFloat64("temperature"); got != 0.123 {
		t.Errorf("WithConfig: temperature = %v, want 0.123 from ~/.kit.yml", got)
	}
	if got := k.GetContextFiles(); len(got) != 1 {
		t.Errorf("WithContextFiles: want 1 context file, got %d", len(got))
	}
	found := false
	for _, s := range k.GetSkills() {
		if s.Name == "demo-skill" {
			found = true
		}
	}
	if !found {
		t.Error("WithSkills: project skill was not loaded")
	}
	if got := k.GetAgents(); len(got) == 0 {
		t.Error("WithAgents: no named agents loaded")
	}
	if names := k.GetToolNames(); !slices.Contains(names, "read") {
		t.Errorf("WithCoreTools(\"read\"): tools = %v", names)
	}
}

// TestInheritIsolationOptions_Isolated checks that a subagent of an isolated
// Kit does not rediscover what the parent refused.
func TestInheritIsolationOptions_Isolated(t *testing.T) {
	child := &Options{}
	inheritIsolationOptions(child, applyOptions(Isolated()))
	if !child.SkipConfig || !child.NoContextFiles || !child.NoSkills ||
		!child.NoExtensions || !child.NoAgents {
		t.Errorf("child did not inherit isolation: %+v", child)
	}

	// A parent that did not disable a feature must not enable it on a
	// child that disabled it.
	child = applyOptions(Isolated())
	inheritIsolationOptions(child, &Options{})
	if !child.SkipConfig || !child.NoContextFiles || !child.NoSkills ||
		!child.NoExtensions || !child.NoAgents {
		t.Errorf("non-isolated parent re-enabled child features: %+v", child)
	}
}
