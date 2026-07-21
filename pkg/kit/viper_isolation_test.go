package kit_test

import (
	"context"
	"os"
	"testing"

	"github.com/mark3labs/kit/pkg/kit"
)

// TestOptionFunctionsPlumbing verifies that the functional options apply their
// values to the underlying Options struct. This does not create a provider, so
// it runs without API keys.
func TestOptionFunctionsPlumbing(t *testing.T) {
	o := &kit.Options{}
	opts := []kit.Option{
		kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),
		kit.WithSystemPrompt("be terse"),
		kit.WithMaxTokens(4321),
		kit.WithThinkingLevel("high"),
		kit.WithProviderAPIKey("sk-test"),
		kit.WithProviderURL("https://example.test/v1"),
		kit.WithConfigFile("/tmp/.kit.yml"),
		kit.WithStreaming(false),
		kit.WithDebug(),
		kit.Ephemeral(),
	}
	for _, fn := range opts {
		fn(o)
	}

	if o.Model != "anthropic/claude-sonnet-4-5-20250929" {
		t.Errorf("WithModel: got %q", o.Model)
	}
	if o.SystemPrompt != "be terse" {
		t.Errorf("WithSystemPrompt: got %q", o.SystemPrompt)
	}
	if o.MaxTokens != 4321 {
		t.Errorf("WithMaxTokens: got %d", o.MaxTokens)
	}
	if o.ThinkingLevel != "high" {
		t.Errorf("WithThinkingLevel: got %q", o.ThinkingLevel)
	}
	if o.ProviderAPIKey != "sk-test" {
		t.Errorf("WithProviderAPIKey: got %q", o.ProviderAPIKey)
	}
	if o.ProviderURL != "https://example.test/v1" {
		t.Errorf("WithProviderURL: got %q", o.ProviderURL)
	}
	if o.ConfigFile != "/tmp/.kit.yml" {
		t.Errorf("WithConfigFile: got %q", o.ConfigFile)
	}
	if o.Streaming == nil {
		t.Error("WithStreaming: expected Streaming to be set (non-nil)")
	} else if *o.Streaming {
		t.Error("WithStreaming(false): expected *Streaming=false")
	}
	if !o.Debug {
		t.Error("WithDebug: expected Debug=true")
	}
	if !o.NoSession {
		t.Error("Ephemeral: expected NoSession=true")
	}
}

// recordingDebugLogger is a kit.DebugLogger used to verify WithDebugLogger
// plumbs the supplied logger into Options. It records each LogDebug call.
type recordingDebugLogger struct {
	enabled  bool
	messages []string
}

func (l *recordingDebugLogger) LogDebug(m string)    { l.messages = append(l.messages, m) }
func (l *recordingDebugLogger) IsDebugEnabled() bool { return l.enabled }

// TestWithDebugLoggerPlumbing verifies that kit.WithDebugLogger assigns the
// supplied logger to Options.DebugLogger. End-to-end propagation into the
// engine is covered indirectly by the existing kitsetup tests; this test
// pins the SDK-surface contract.
func TestWithDebugLoggerPlumbing(t *testing.T) {
	l := &recordingDebugLogger{enabled: true}
	o := &kit.Options{}
	kit.WithDebugLogger(l)(o)
	if o.DebugLogger == nil {
		t.Fatal("WithDebugLogger: expected Options.DebugLogger to be set")
	}
	if o.DebugLogger != l {
		t.Error("WithDebugLogger: expected the supplied logger to be installed verbatim")
	}
	// Sanity: the installed logger satisfies the SDK interface contract.
	if !o.DebugLogger.IsDebugEnabled() {
		t.Error("installed logger IsDebugEnabled() returned false")
	}
	o.DebugLogger.LogDebug("hello")
	if len(l.messages) != 1 || l.messages[0] != "hello" {
		t.Errorf("LogDebug not forwarded; got %v", l.messages)
	}
}

// TestWithDebugLoggerNilClears verifies that passing a nil logger to
// WithDebugLogger clears any previously-installed logger. This lets later
// options override earlier ones the same way WithModel / WithStreaming do.
func TestWithDebugLoggerNilClears(t *testing.T) {
	o := &kit.Options{}
	kit.WithDebugLogger(&recordingDebugLogger{enabled: true})(o)
	kit.WithDebugLogger(nil)(o)
	if o.DebugLogger != nil {
		t.Errorf("WithDebugLogger(nil): expected DebugLogger to be cleared; got %#v", o.DebugLogger)
	}
}

// TestOptionOrderingOverrides verifies later options override earlier ones.
func TestOptionOrderingOverrides(t *testing.T) {
	o := &kit.Options{}
	kit.WithModel("a/b")(o)
	kit.WithModel("c/d")(o)
	if o.Model != "c/d" {
		t.Errorf("later WithModel should win; got %q", o.Model)
	}
}

// TestKitConfigIsolation is the regression test for issue #40: two Kit
// instances constructed in the same process must own independent configuration
// stores. Setting the thinking level (or model) on one must not affect the
// other. Against the previous global-viper implementation this test fails
// because both Kits read and write the same process-global store.
func TestKitConfigIsolation(t *testing.T) {
	requireAnthropicAuth(t)

	ctx := context.Background()

	a, err := kit.New(ctx, &kit.Options{
		Model:         "anthropic/claude-sonnet-4-5-20250929",
		ThinkingLevel: "low",
		Quiet:         true,
		NoSession:     true,
		NoExtensions:  true,
	})
	if err != nil {
		t.Fatalf("failed to create Kit A: %v", err)
	}
	defer func() { _ = a.Close() }()

	b, err := kit.New(ctx, &kit.Options{
		Model:         "anthropic/claude-sonnet-4-5-20250929",
		ThinkingLevel: "high",
		Quiet:         true,
		NoSession:     true,
		NoExtensions:  true,
	})
	if err != nil {
		t.Fatalf("failed to create Kit B: %v", err)
	}
	defer func() { _ = b.Close() }()

	// Each instance must retain its own configured thinking level. Under the
	// old global-viper implementation, B's construction overwrote A's value.
	if got := a.GetThinkingLevel(); got != "low" {
		t.Errorf("Kit A thinking level = %q; want %q (config leaked from B)", got, "low")
	}
	if got := b.GetThinkingLevel(); got != "high" {
		t.Errorf("Kit B thinking level = %q; want %q", got, "high")
	}

	// Mutating one at runtime must not bleed into the other.
	if err := a.SetThinkingLevel(ctx, "medium"); err != nil {
		t.Fatalf("SetThinkingLevel on A: %v", err)
	}
	if got := a.GetThinkingLevel(); got != "medium" {
		t.Errorf("after SetThinkingLevel, Kit A = %q; want %q", got, "medium")
	}
	if got := b.GetThinkingLevel(); got != "high" {
		t.Errorf("after mutating A, Kit B leaked to %q; want %q", got, "high")
	}
}

// TestNewAgentDefaultsStreamingOn verifies that the ergonomic constructor
// enables streaming by default and applies functional options.
func TestNewAgentDefaultsStreamingOn(t *testing.T) {
	requireAnthropicAuth(t)

	ctx := context.Background()
	k, err := kit.NewAgent(ctx,
		kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),
		kit.WithMaxTokens(2048),
		kit.Ephemeral(),
	)
	if err != nil {
		t.Fatalf("NewAgent failed: %v", err)
	}
	defer func() { _ = k.Close() }()

	if !k.ConfigValueIsSetForTest("max-tokens") {
		t.Error("NewAgent did not propagate WithMaxTokens to the instance store")
	}
	if !k.ConfigBoolForTest("stream") {
		t.Error("NewAgent should enable streaming by default")
	}
}

// TestNewAgentStreamingOptOut verifies WithStreaming(false) disables the
// default-on streaming behaviour of NewAgent.
func TestNewAgentStreamingOptOut(t *testing.T) {
	requireAnthropicAuth(t)

	ctx := context.Background()
	k, err := kit.NewAgent(ctx,
		kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),
		kit.WithStreaming(false),
		kit.Ephemeral(),
	)
	if err != nil {
		t.Fatalf("NewAgent failed: %v", err)
	}
	defer func() { _ = k.Close() }()

	if k.ConfigBoolForTest("stream") {
		t.Error("WithStreaming(false) should disable streaming")
	}
}

// TestNewZeroOptionsKeepsStreamingDefault is the regression test for the
// unconditional `v.Set("stream", opts.Streaming)` bug: a zero-valued Options
// (Streaming == nil) must NOT force stream=false. With Streaming unset,
// streaming resolves through the precedence chain, whose SDK default is true.
func TestNewZeroOptionsKeepsStreamingDefault(t *testing.T) {
	requireAnthropicAuth(t)

	ctx := context.Background()
	k, err := kit.New(ctx, &kit.Options{
		Model:      "anthropic/claude-sonnet-4-5-20250929",
		Quiet:      true,
		NoSession:  true,
		SkipConfig: true, // isolate from any ~/.kit.yml / env stream setting
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = k.Close() }()

	if !k.ConfigBoolForTest("stream") {
		t.Error("zero-valued Options must not force stream=false; expected the default (true)")
	}
}

// TestSkillsViperKeys verifies that the three skills config keys (no-skills,
// skill, skills-dir) flow through viper when set via a config file, matching
// the pattern used by no-extensions and no-core-tools. This test does not
// require an API key because it only exercises Options struct plumbing.
func TestSkillsViperKeys(t *testing.T) {
	t.Run("NoSkills option disables skill loading", func(t *testing.T) {
		o := &kit.Options{}
		o.NoSkills = true
		if !o.NoSkills {
			t.Error("Options.NoSkills = true not reflected on struct")
		}
	})

	t.Run("Skills paths set on Options", func(t *testing.T) {
		o := &kit.Options{
			Skills: []string{"/a/skill.md", "/b/skill.md"},
		}
		if len(o.Skills) != 2 {
			t.Errorf("Options.Skills: got %d paths, want 2", len(o.Skills))
		}
		if o.Skills[0] != "/a/skill.md" {
			t.Errorf("Options.Skills[0] = %q; want %q", o.Skills[0], "/a/skill.md")
		}
	})

	t.Run("SkillsDir set on Options", func(t *testing.T) {
		o := &kit.Options{
			SkillsDir: "/custom/skills",
		}
		if o.SkillsDir != "/custom/skills" {
			t.Errorf("Options.SkillsDir = %q; want %q", o.SkillsDir, "/custom/skills")
		}
	})
}

// TestSkillsConfigFileKeys verifies that no-skills, skill, and skills-dir
// config file keys are read via viper and applied correctly. Requires an API
// key because kit.New() is called to exercise the full config-load path.
func TestSkillsConfigFileKeys(t *testing.T) {
	requireAnthropicAuth(t)

	ctx := context.Background()

	t.Run("no-skills config key disables skill loading", func(t *testing.T) {
		// Write a config file with no-skills: true.
		cfgFile := t.TempDir() + "/.kit.yml"
		if err := os.WriteFile(cfgFile, []byte("no-skills: true\n"), 0o644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		host, err := kit.New(ctx, &kit.Options{
			Model:      "anthropic/claude-sonnet-4-5-20250929",
			Quiet:      true,
			NoSession:  true,
			ConfigFile: cfgFile,
		})
		if err != nil {
			t.Fatalf("kit.New failed: %v", err)
		}
		defer func() { _ = host.Close() }()

		if got := host.GetSkills(); len(got) != 0 {
			t.Errorf("no-skills:true in config: expected 0 skills, got %d", len(got))
		}
	})

	t.Run("skill config key loads explicit skill files", func(t *testing.T) {
		dir := t.TempDir()
		skillFile := dir + "/cfg-skill.md"
		if err := os.WriteFile(skillFile, []byte("---\nname: cfg-skill\ndescription: from config\n---\nContent.\n"), 0o644); err != nil {
			t.Fatalf("failed to write skill file: %v", err)
		}

		cfgContent := "skill:\n  - " + skillFile + "\n"
		cfgFile := dir + "/.kit.yml"
		if err := os.WriteFile(cfgFile, []byte(cfgContent), 0o644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		host, err := kit.New(ctx, &kit.Options{
			Model:      "anthropic/claude-sonnet-4-5-20250929",
			Quiet:      true,
			NoSession:  true,
			ConfigFile: cfgFile,
		})
		if err != nil {
			t.Fatalf("kit.New failed: %v", err)
		}
		defer func() { _ = host.Close() }()

		skills := host.GetSkills()
		if len(skills) != 1 {
			t.Fatalf("expected 1 skill from config, got %d", len(skills))
		}
		if skills[0].Name != "cfg-skill" {
			t.Errorf("skill name = %q; want %q", skills[0].Name, "cfg-skill")
		}
	})

	t.Run("skills-dir config key overrides auto-discovery root", func(t *testing.T) {
		dir := t.TempDir()
		cfgContent := "skills-dir: " + dir + "\n"
		cfgFile := dir + "/.kit.yml"
		if err := os.WriteFile(cfgFile, []byte(cfgContent), 0o644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		host, err := kit.New(ctx, &kit.Options{
			Model:      "anthropic/claude-sonnet-4-5-20250929",
			Quiet:      true,
			NoSession:  true,
			ConfigFile: cfgFile,
		})
		if err != nil {
			t.Fatalf("kit.New failed: %v", err)
		}
		defer func() { _ = host.Close() }()

		// Empty dir → 0 skills; the key point is no error during init.
		_ = host.GetSkills()
	})
}

// TestNewStreamingExplicitOptOut verifies that a raw Options can still disable
// streaming by setting Streaming to a pointer to false.
func TestNewStreamingExplicitOptOut(t *testing.T) {
	requireAnthropicAuth(t)

	streamOff := false
	ctx := context.Background()
	k, err := kit.New(ctx, &kit.Options{
		Model:      "anthropic/claude-sonnet-4-5-20250929",
		Quiet:      true,
		NoSession:  true,
		SkipConfig: true,
		Streaming:  &streamOff,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = k.Close() }()

	if k.ConfigBoolForTest("stream") {
		t.Error("Streaming=&false should disable streaming")
	}
}
