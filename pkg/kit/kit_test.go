package kit_test

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/viper"

	kit "github.com/mark3labs/kit/pkg/kit"
)

func TestNew(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	// Test default initialization
	opts := &kit.Options{
		Model: "anthropic/claude-sonnet-4-5-20250929",
	}
	host, err := kit.New(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to create Kit with defaults: %v", err)
	}
	defer func() { _ = host.Close() }()

	if host.GetModelString() == "" {
		t.Error("Model string should not be empty")
	}
}

func TestNewWithOptions(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	opts := &kit.Options{
		Model:    "anthropic/claude-sonnet-4-5-20250929",
		MaxSteps: 5,
		Quiet:    true,
	}

	host, err := kit.New(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to create Kit with options: %v", err)
	}
	defer func() { _ = host.Close() }()

	if host.GetModelString() != opts.Model {
		t.Errorf("Expected model %s, got %s", opts.Model, host.GetModelString())
	}
}

// TestNewWithGenerationOptions verifies that the SDK-only generation
// parameter overrides on Options propagate all the way through to the
// agent without requiring any viper.Set workarounds in caller code.
func TestNewWithGenerationOptions(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	// MaxTokens override — keep ThinkingLevel off so Anthropic's thinking
	// budget doesn't auto-bump MaxTokens above what we configured.
	t.Run("MaxTokens", func(t *testing.T) {
		defer resetViper()

		const want = 12345
		host, err := kit.New(ctx, &kit.Options{
			Model:     "anthropic/claude-sonnet-4-5-20250929",
			Quiet:     true,
			MaxTokens: want,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit: %v", err)
		}
		defer func() { _ = host.Close() }()

		if got := host.MaxTokens(); got != want {
			t.Errorf("Options.MaxTokens=%d did not propagate; Kit.MaxTokens()=%d", want, got)
		}
		if !host.ConfigValueIsSetForTest("max-tokens") {
			t.Error("max-tokens should be marked explicitly set on the instance store after MaxTokens override")
		}
	})

	// ThinkingLevel override — verified via the public getter, which
	// reads back the configured (not provider-derived) level.
	t.Run("ThinkingLevel", func(t *testing.T) {
		defer resetViper()

		const want = "high"
		host, err := kit.New(ctx, &kit.Options{
			Model:         "anthropic/claude-sonnet-4-5-20250929",
			Quiet:         true,
			ThinkingLevel: want,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit: %v", err)
		}
		defer func() { _ = host.Close() }()

		if got := host.GetThinkingLevel(); got != want {
			t.Errorf("Options.ThinkingLevel=%q did not propagate; Kit.GetThinkingLevel()=%q", want, got)
		}
	})

	// Temperature override — pointer semantics let callers distinguish
	// "explicitly 0.0" from "unset", which we assert by pushing a distinct
	// value and reading it back off viper's merged state.
	t.Run("Temperature", func(t *testing.T) {
		defer resetViper()

		want := float32(0.12345)
		host, err := kit.New(ctx, &kit.Options{
			Model:       "anthropic/claude-sonnet-4-5-20250929",
			Quiet:       true,
			Temperature: &want,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit: %v", err)
		}
		defer func() { _ = host.Close() }()

		if !host.ConfigValueIsSetForTest("temperature") {
			t.Fatal("temperature should be marked explicitly set on the instance store after Temperature override")
		}
		if got := float32(host.ConfigFloatForTest("temperature")); got != want {
			t.Errorf("Options.Temperature=%v did not propagate; instance store=%v", want, got)
		}
	})
}

// TestNewPreservesIsSetSemantics verifies that creating a Kit WITHOUT
// populating the generation-param Options fields does NOT mark those
// keys as explicitly set in viper. This is the precedence contract
// that per-model defaults (ApplyModelSettings) and right-sizing
// (rightSizeMaxTokens) rely on.
//
// Previously setSDKDefaults() used viper.SetDefault() for every param,
// which caused viper.IsSet() to return true for all of them — silently
// suppressing per-model defaults and pinning max-tokens at 4096 even
// on models with much larger output limits.
func TestNewPreservesIsSetSemantics(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	defer resetViper()

	ctx := context.Background()
	host, err := kit.New(ctx, &kit.Options{
		Model:      "anthropic/claude-sonnet-4-5-20250929",
		Quiet:      true,
		NoSession:  true,
		SkipConfig: true, // isolate from any ~/.kit.yml values
	})
	if err != nil {
		t.Fatalf("Failed to create Kit: %v", err)
	}
	defer func() { _ = host.Close() }()

	// These keys must remain "unset" from viper's perspective so the
	// downstream isExplicitlySet() checks allow per-model defaults to
	// take effect.
	checkKeys := []string{
		"max-tokens",
		"temperature",
		"top-p",
		"top-k",
		"frequency-penalty",
		"presence-penalty",
		"thinking-level",
	}

	// With SkipConfig: true, InitConfig() is not invoked, so viper has
	// no env-var bindings registered. Any IsSet() here would come purely
	// from SDK-side SetDefault/Set calls — which is exactly what this
	// test is guarding against.
	for _, k := range checkKeys {
		if host.ConfigValueIsSetForTest(k) {
			t.Errorf("instance store reports %q explicitly set when no Options field set it "+
				"(SDK defaults must not corrupt IsSet semantics)", k)
		}
	}
}

// TestNewWithProviderOptions verifies that programmatic provider overrides
// (API key, URL) take effect without env vars or config files, and that
// Options.ProviderAPIKey *wins* over any pre-existing viper state.
func TestNewWithProviderOptions(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	t.Run("succeeds with API key from Options", func(t *testing.T) {
		defer resetViper()

		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		host, err := kit.New(ctx, &kit.Options{
			Model:          "anthropic/claude-sonnet-4-5-20250929",
			Quiet:          true,
			NoSession:      true,
			ProviderAPIKey: apiKey,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit with ProviderAPIKey option: %v", err)
		}
		defer func() { _ = host.Close() }()

		if got := host.ConfigStringForTest("provider-api-key"); got != apiKey {
			t.Errorf("Options.ProviderAPIKey did not propagate to the instance store; got %q (len=%d)", got, len(got))
		}
	})

	// Override precedence: even when the process-global store already holds a
	// different provider-api-key value, Options.ProviderAPIKey must win on the
	// Kit's isolated store.
	t.Run("Options override beats pre-existing viper state", func(t *testing.T) {
		defer resetViper()

		viper.Set("provider-api-key", "sk-config-file-placeholder")

		want := "sk-from-options-override"
		// Use an OpenAI-flavored model so the validation path accepts
		// the placeholder without attempting a real Anthropic handshake.
		host, err := kit.New(ctx, &kit.Options{
			Model:            "openai/gpt-4o-mini",
			Quiet:            true,
			NoSession:        true,
			NoExtensions:     true,
			DisableCoreTools: true,
			ProviderAPIKey:   want,
		})
		// Creation may still fail if the model registry is strict, but
		// we only care that the override reached the instance store before
		// any provider handshake happened.
		if host == nil {
			t.Fatalf("expected a Kit instance to inspect; got nil (err=%v)", err)
		}
		defer func() { _ = host.Close() }()
		_ = err

		if got := host.ConfigStringForTest("provider-api-key"); got != want {
			t.Errorf("Options.ProviderAPIKey did not override pre-existing value on the instance store; got %q, want %q", got, want)
		}
	})

	// ProviderURL override must also reach viper.
	t.Run("ProviderURL propagates", func(t *testing.T) {
		defer resetViper()

		const want = "https://custom.example.com/v1"
		host, err := kit.New(ctx, &kit.Options{
			Model:       "anthropic/claude-sonnet-4-5-20250929",
			Quiet:       true,
			NoSession:   true,
			ProviderURL: want,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit with ProviderURL option: %v", err)
		}
		defer func() { _ = host.Close() }()

		if got := host.ConfigStringForTest("provider-url"); got != want {
			t.Errorf("Options.ProviderURL did not propagate; got %q, want %q", got, want)
		}
	})
}

func TestSessionManagement(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	host, err := kit.New(ctx, &kit.Options{Quiet: true, NoSession: true})
	if err != nil {
		t.Fatalf("Failed to create Kit: %v", err)
	}
	defer func() { _ = host.Close() }()

	// Tree session should be configured.
	ts := host.GetTreeSession()
	if ts == nil {
		t.Fatal("Expected tree session to be configured")
	}

	// Test clear session resets leaf.
	host.ClearSession()

	// Verify session info accessors.
	if id := host.GetSessionID(); id == "" {
		t.Error("Expected non-empty session ID")
	}
}

// resetViper wipes viper's global state so a test case doesn't leak
// viper.Set() calls into the next one. Used via defer in subtests.
func resetViper() { viper.Reset() }

// TestNewSystemPromptFilePath is a regression test for issue #25.
//
// When Options.SystemPrompt (or the --system-prompt flag / config entry) is a
// file path, Kit must resolve the path to its file contents *before* the
// PromptBuilder composes the runtime context. Previously the path string
// itself was used verbatim as the base prompt, so the LLM received the path —
// not the prompt — as its system message.
func TestNewSystemPromptFilePath(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}
	defer resetViper()

	const promptContent = "You are a strict regression-test persona. Marker: KIT-25-OK"

	tmpFile, err := os.CreateTemp(t.TempDir(), "kit-system-prompt-*.md")
	if err != nil {
		t.Fatalf("failed to create temp prompt file: %v", err)
	}
	if _, err := tmpFile.WriteString(promptContent); err != nil {
		t.Fatalf("failed to write temp prompt file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp prompt file: %v", err)
	}

	ctx := context.Background()
	host, err := kit.New(ctx, &kit.Options{
		Model:        "anthropic/claude-sonnet-4-5-20250929",
		SystemPrompt: tmpFile.Name(),
		Quiet:        true,
		NoSession:    true,
	})
	if err != nil {
		t.Fatalf("Failed to create Kit with system-prompt file: %v", err)
	}
	defer func() { _ = host.Close() }()

	if !host.HasCustomSystemPrompt() {
		t.Error("HasCustomSystemPrompt() = false; want true when --system-prompt is set")
	}
	if got, want := host.GetSystemPromptSource(), tmpFile.Name(); got != want {
		t.Errorf("GetSystemPromptSource() = %q; want %q", got, want)
	}

	// The composed system prompt is written back to the instance store after
	// PromptBuilder runs. It must contain the file's contents, not the file path.
	composed := host.ConfigStringForTest("system-prompt")
	if !strings.Contains(composed, promptContent) {
		t.Errorf("composed system-prompt does not contain file contents\n  composed = %q\n  want substring = %q", composed, promptContent)
	}
	if strings.TrimSpace(composed) == tmpFile.Name() {
		t.Errorf("composed system-prompt is the file path verbatim (%q); LoadSystemPrompt was not applied before PromptBuilder", composed)
	}
}

// TestNewWithSkillsOptions verifies that the three skills-related Options
// fields (NoSkills, Skills, SkillsDir) are wired correctly into kit.New().
func TestNewWithSkillsOptions(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	t.Run("NoSkills disables skill loading", func(t *testing.T) {
		host, err := kit.New(ctx, &kit.Options{
			Model:     "anthropic/claude-sonnet-4-5-20250929",
			Quiet:     true,
			NoSession: true,
			NoSkills:  true,
		})
		if err != nil {
			t.Fatalf("kit.New failed: %v", err)
		}
		defer func() { _ = host.Close() }()

		if got := host.GetSkills(); len(got) != 0 {
			t.Errorf("NoSkills=true: expected 0 skills, got %d", len(got))
		}
	})

	t.Run("SkillsDir propagates", func(t *testing.T) {
		// Use a non-existent dir — no skills will load but the option must be
		// accepted without error and result in zero skills.
		dir := t.TempDir()
		host, err := kit.New(ctx, &kit.Options{
			Model:     "anthropic/claude-sonnet-4-5-20250929",
			Quiet:     true,
			NoSession: true,
			SkillsDir: dir,
		})
		if err != nil {
			t.Fatalf("kit.New failed: %v", err)
		}
		defer func() { _ = host.Close() }()

		// Empty dir → no skills; the important thing is no error.
		_ = host.GetSkills()
	})

	t.Run("explicit Skills paths load correctly", func(t *testing.T) {
		// Write a minimal skill file to a temp dir.
		dir := t.TempDir()
		skillFile := dir + "/my-skill.md"
		content := "---\nname: test-skill\ndescription: A test skill\n---\nDo the thing.\n"
		if err := os.WriteFile(skillFile, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write skill file: %v", err)
		}

		host, err := kit.New(ctx, &kit.Options{
			Model:     "anthropic/claude-sonnet-4-5-20250929",
			Quiet:     true,
			NoSession: true,
			Skills:    []string{skillFile},
		})
		if err != nil {
			t.Fatalf("kit.New failed: %v", err)
		}
		defer func() { _ = host.Close() }()

		skills := host.GetSkills()
		if len(skills) != 1 {
			t.Fatalf("expected 1 skill, got %d", len(skills))
		}
		if skills[0].Name != "test-skill" {
			t.Errorf("skill name = %q; want %q", skills[0].Name, "test-skill")
		}
	})
}

// TestNewSystemPromptInline confirms that inline system-prompt strings still
// flow through unchanged after the file-path resolution change.
func TestNewSystemPromptInline(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}
	defer resetViper()

	const inline = "You are a concise inline-prompt persona."

	ctx := context.Background()
	host, err := kit.New(ctx, &kit.Options{
		Model:        "anthropic/claude-sonnet-4-5-20250929",
		SystemPrompt: inline,
		Quiet:        true,
		NoSession:    true,
	})
	if err != nil {
		t.Fatalf("Failed to create Kit with inline system-prompt: %v", err)
	}
	defer func() { _ = host.Close() }()

	if !host.HasCustomSystemPrompt() {
		t.Error("HasCustomSystemPrompt() = false; want true for inline prompt")
	}
	if got := host.GetSystemPromptSource(); got != inline {
		t.Errorf("GetSystemPromptSource() = %q; want %q", got, inline)
	}
	if composed := host.ConfigStringForTest("system-prompt"); !strings.Contains(composed, inline) {
		t.Errorf("composed system-prompt missing inline content; got %q", composed)
	}
}

// TestDisableCoreTools verifies that setting Options.DisableCoreTools to true
// does not set any tools even when tools are explicitly listed in the kit.Options.ToolList.
func TestDisableCoreTools(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()
	t.Run("DisableCoreTools true with empty CoreToolList", func(t *testing.T) {
		defer resetViper()
		host, err := kit.New(ctx, &kit.Options{
			Model:            "anthropic/claude-sonnet-4-5-20250929",
			Quiet:            true,
			NoSession:        true,
			NoExtensions:     true,
			DisableCoreTools: true,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit with DisableCoreTools: %v", err)
		}
		defer func() { _ = host.Close() }()

		tools := host.GetToolNames()
		if len(tools) != 0 {
			t.Errorf("Expected 0 tool when DisableCoreTools is true, got %d: %v", len(tools), tools)
		}
	})
	t.Run("DisableCoreTools true with populated CoreToolList", func(t *testing.T) {
		defer resetViper()
		host, err := kit.New(ctx, &kit.Options{
			Model:            "anthropic/claude-sonnet-4-5-20250929",
			Quiet:            true,
			NoSession:        true,
			NoExtensions:     true,
			DisableCoreTools: true,
			CoreToolList:     []string{"subagent", "bash"},
		})
		if err != nil {
			t.Fatalf("Failed to create Kit with DisableCoreTools and Tool list: %v", err)
		}
		defer func() { _ = host.Close() }()

		tools := host.GetToolNames()
		if len(tools) != 0 {
			t.Errorf("Expected 0 tool when DisableCoreTools is true, got %d: %v", len(tools), tools)
		}
	})
}

func TestCoreToolList(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()
	t.Run("Test wether empty CoreToolList adds all tools", func(t *testing.T) {
		defer resetViper()
		host, err := kit.New(ctx, &kit.Options{
			Quiet:        true,
			NoSession:    true,
			NoExtensions: true,
		})
		if err != nil {
			t.Fatalf("Failed to create Kit with DisableCoreTools: %v", err)
		}
		defer func() { _ = host.Close() }()

		tools := host.GetToolNames()
		if len(tools) != len(kit.ListAllCoreToolNames()) {
			t.Errorf("Expected %d tool when DisableCoreTools is true, got %d: %v",
				len(kit.ListAllCoreToolNames()), len(tools), tools)
		}
	})
	t.Run("Test ToolList populated with two valid and one invalid tool name", func(t *testing.T) {
		defer resetViper()
		host, err := kit.New(ctx, &kit.Options{
			Quiet:            true,
			NoSession:        true,
			NoExtensions:     true,
			DisableCoreTools: true,
			CoreToolList:     []string{"subagent", "bash", "deadbeef"},
		})
		if err != nil {
			t.Fatalf("Failed to create Kit with DisableCoreTools and Tool list: %v", err)
		}
		defer func() { _ = host.Close() }()

		tools := host.GetToolNames()
		if len(tools) != 0 {
			t.Errorf("Expected 2 tools with 2 valid and 1 invalid tool name, got %d: %v", len(tools), tools)
		}
		for _, s := range tools {
			if s != "subagent" && s != "bash" {
				t.Errorf("Expected tool to be 'bash' or 'subagent' but got %s", s)
			}
		}
	})
}

func TestCoreToolListOption(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}
	ctx := context.Background()
	t.Run("Test whether exclude-core-tools and include-core-tools combined generates failure", func(t *testing.T) {
		cfgFile := t.TempDir() + "/.kit.yml"
		txt := []byte(`include-core-tools:
   - "subagent"
exclude-core-tools:
   - "bash"`)
		if err := os.WriteFile(cfgFile, txt, 0o644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}
		host, err := kit.New(ctx, &kit.Options{
			Model:      "anthropic/claude-sonnet-4-5-20250929",
			Quiet:      true,
			NoSession:  true,
			ConfigFile: cfgFile,
		})
		if err != nil {
			if err.Error() != "cannot use both include-core-tools and exclude-core-tools options" {
				t.Errorf("kit.New: unexpected error %v", err)
			}
		} else {
			defer func() { _ = host.Close() }()
			t.Errorf("kit.New was expected to fail with both include-core-tools and exclude-core-tools specified")
		}
	})
	t.Run("Test whether include-core-tools works", func(t *testing.T) {
		cfgFile := t.TempDir() + "/.kit.yml"
		txt := []byte(`include-core-tools:
   - "subagent"
   - "deadbeef"
   - "bash"`)
		if err := os.WriteFile(cfgFile, txt, 0o644); err != nil {
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
		tools := host.GetToolNames()
		for _, tool := range tools {
			if tool != "bash" && tool != "subagent" {
				t.Errorf("include-core-tools expected to have only 'bash' and 'subagent' tools but have %s", tool)
			}
		}
	})
	t.Run("Test whether exclude-core-tools works", func(t *testing.T) {
		cfgFile := t.TempDir() + "/.kit.yml"
		txt := []byte(`exclude-core-tools:
   - "subagent"
   - "deadbeef"
   - "bash"`)
		if err := os.WriteFile(cfgFile, txt, 0o644); err != nil {
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
		var registeredTools []string
		for _, tool := range kit.ListAllCoreToolNames() {
			if tool != "bash" && tool != "subagent" {
				registeredTools = append(registeredTools, tool)
			}
		}
		tools := host.GetToolNames()
		for _, tool := range tools {
			if slices.Contains(registeredTools, tool) {
				continue
			}
			t.Errorf("exclude-core-tools added unexpected %s tool", tool)
		}
	})
}

func TestGetToolsForSubagent(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}
	defer resetViper()

	ctx := context.Background()
	type HelloInput struct {
		Query string `json:"query,omitempty" description:"Hello request"`
	}
	helloTool := kit.NewTool("hello", "Say \"Hello!\"",
		func(ctx context.Context, input HelloInput) (kit.ToolOutput, error) {
			return kit.TextResult("Hello!"), nil
		},
	)

	host, err := kit.New(ctx, &kit.Options{
		Model:        "anthropic/claude-sonnet-4-5-20250929",
		Quiet:        true,
		NoSession:    true,
		NoExtensions: true,
		CoreToolList: []string{"subagent", "bash"},
		ExtraTools:   []kit.Tool{helloTool},
	})
	if err != nil {
		t.Fatalf("Failed to create Kit: %v", err)
	}
	defer func() { _ = host.Close() }()
	toolsForSubagent := host.GetToolsForSubagent()
	if len(toolsForSubagent) != 2 {
		t.Fatalf("`GetToolsForSubagent()` should return 2 tools but has: %d", len(toolsForSubagent))
	}
	hasTool := func(name string) bool {
		for _, tool := range toolsForSubagent {
			if tool.Info().Name == name {
				return true
			}
		}
		return false
	}
	if hasTool("subagent") {
		t.Errorf("`GetToolsForSubagent()` unexpectedly includes the `subagent` tool")
	}
	if !hasTool("hello") {
		t.Errorf("`GetToolsForSubagent()` is missing the `hello` tool")
	}
	if !hasTool("bash") {
		t.Errorf("`GetToolsForSubagent()` is missing the `bash` tool")
	}
}
