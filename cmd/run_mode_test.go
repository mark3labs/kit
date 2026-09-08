package cmd

import (
	"log"
	"testing"

	"github.com/spf13/viper"

	"github.com/mark3labs/kit/internal/config"
)

// TestBuildKitOptionsNilAuthHandlerIsTrueNil guards the typed-nil pitfall:
// a nil *CLIMCPAuthHandler must produce a nil MCPAuthHandler interface, not a
// non-nil interface wrapping a nil pointer (which panics on first dispatch).
func TestBuildKitOptionsNilAuthHandlerIsTrueNil(t *testing.T) {
	origQuiet, origJSON, origResume, origPrompt := quietFlag, jsonFlag, resumeFlag, positionalPrompt
	t.Cleanup(func() {
		quietFlag, jsonFlag, resumeFlag, positionalPrompt = origQuiet, origJSON, origResume, origPrompt
	})
	quietFlag, jsonFlag, resumeFlag, positionalPrompt = true, false, false, ""

	mcpConfig := &config.Config{}
	called := false
	opts, err := buildKitOptions(mcpConfig, nil, func(string, int, error) { called = true })
	if err != nil {
		t.Fatalf("buildKitOptions() error = %v", err)
	}
	if opts.MCPAuthHandler != nil {
		t.Errorf("MCPAuthHandler = %#v, want true nil interface", opts.MCPAuthHandler)
	}
	if !opts.Quiet {
		t.Error("Quiet = false with --quiet, want true")
	}
	if opts.CLI == nil {
		t.Fatal("CLI options = nil, want non-nil")
	}
	if opts.CLI.MCPConfig != mcpConfig {
		t.Error("CLI.MCPConfig does not point at the supplied config")
	}
	if opts.CLI.SpinnerFunc != nil {
		t.Error("CLI.SpinnerFunc set while chrome is suppressed, want nil")
	}
	if opts.OnMCPServerLoaded == nil {
		t.Fatal("OnMCPServerLoaded = nil, want callback")
	}
	opts.OnMCPServerLoaded("srv", 1, nil)
	if !called {
		t.Error("OnMCPServerLoaded did not invoke the supplied callback")
	}
}

// TestBuildKitOptionsRejectsConflictingCoreToolFilter verifies that setting
// both include and exclude core tool filters surfaces as an error instead of
// a half-built Options.
func TestBuildKitOptionsRejectsConflictingCoreToolFilter(t *testing.T) {
	origInc := viper.GetStringSlice("include-core-tools")
	origExc := viper.GetStringSlice("exclude-core-tools")
	t.Cleanup(func() {
		viper.Set("include-core-tools", origInc)
		viper.Set("exclude-core-tools", origExc)
	})
	viper.Set("include-core-tools", []string{"bash"})
	viper.Set("exclude-core-tools", []string{"fetch"})

	if _, err := buildKitOptions(&config.Config{}, nil, nil); err == nil {
		t.Error("buildKitOptions() error = nil with both include and exclude filters, want error")
	}
}

// TestStartupSpinnerFuncSuppressed verifies that --quiet / --json disable the
// startup spinner so no chrome reaches stdout.
func TestStartupSpinnerFuncSuppressed(t *testing.T) {
	origQuiet, origJSON := quietFlag, jsonFlag
	t.Cleanup(func() { quietFlag, jsonFlag = origQuiet, origJSON })

	quietFlag, jsonFlag = false, false
	if startupSpinnerFunc() == nil {
		t.Error("startupSpinnerFunc() = nil in interactive mode, want spinner")
	}
	quietFlag, jsonFlag = true, false
	if startupSpinnerFunc() != nil {
		t.Error("startupSpinnerFunc() != nil with --quiet, want nil")
	}
	quietFlag, jsonFlag = false, true
	if startupSpinnerFunc() != nil {
		t.Error("startupSpinnerFunc() != nil with --json, want nil")
	}
}

// TestConfigureDebugLogging verifies that a config-file debug=true promotes
// the package-level debugMode flag and enables file:line log prefixes.
func TestConfigureDebugLogging(t *testing.T) {
	origDebug := debugMode
	origViper := viper.GetBool("debug")
	origFlags := log.Flags()
	t.Cleanup(func() {
		debugMode = origDebug
		viper.Set("debug", origViper)
		log.SetFlags(origFlags)
	})

	debugMode = false
	viper.Set("debug", false)
	log.SetFlags(log.LstdFlags)
	configureDebugLogging()
	if debugMode {
		t.Error("debugMode = true with debug off everywhere, want false")
	}
	if log.Flags()&log.Lshortfile != 0 {
		t.Error("Lshortfile set with debug off, want unset")
	}

	viper.Set("debug", true)
	configureDebugLogging()
	if !debugMode {
		t.Error("debugMode = false after config debug=true, want true")
	}
	if log.Flags()&log.Lshortfile == 0 {
		t.Error("Lshortfile unset after config debug=true, want set")
	}
}

// TestLoadPromptTemplatesDisabled verifies --no-prompt-templates short-circuits
// both the startup load and the hot-reload provider.
func TestLoadPromptTemplatesDisabled(t *testing.T) {
	orig := noPromptTemplates
	t.Cleanup(func() { noPromptTemplates = orig })
	noPromptTemplates = true

	if got := loadPromptTemplates(false); got != nil {
		t.Errorf("loadPromptTemplates(false) = %d templates, want nil", len(got))
	}
	if got := loadPromptTemplates(true); got != nil {
		t.Errorf("loadPromptTemplates(true) = %d templates, want nil", len(got))
	}
}

// TestWatchersNoDirsReturnNoop verifies that bare mode with no explicit paths
// starts no file watcher and returns a safe no-op stop function.
func TestWatchersNoDirsReturnNoop(t *testing.T) {
	origBare, origPrompts, origSkills := bareFlag, promptTemplatePaths, skillsPaths
	origExt := viper.GetStringSlice("extension")
	origPromptCfg := viper.GetStringSlice("prompts")
	t.Cleanup(func() {
		bareFlag, promptTemplatePaths, skillsPaths = origBare, origPrompts, origSkills
		viper.Set("extension", origExt)
		viper.Set("prompts", origPromptCfg)
	})
	bareFlag = true
	promptTemplatePaths, skillsPaths = nil, nil
	viper.Set("extension", []string{})
	viper.Set("prompts", []string{})

	ctx := t.Context()

	stopExt := startExtensionWatcher(ctx, nil, func() error { return nil })
	if stopExt == nil {
		t.Fatal("startExtensionWatcher() = nil stop func, want no-op")
	}
	stopExt()

	stopContent := startContentWatcher(ctx, nil)
	if stopContent == nil {
		t.Fatal("startContentWatcher() = nil stop func, want no-op")
	}
	stopContent()
}

// TestRunModeDepsCopyIsolatesInteractiveHandoff documents the --no-exit
// handoff: clearing interactive-only fields on the copy must not touch the
// original deps.
func TestRunModeDepsCopyIsolatesInteractiveHandoff(t *testing.T) {
	deps := runModeDeps{
		snapshot: startupSnapshot{
			modelName:                "m",
			startupExtensionMessages: []string{"hello"},
		},
	}
	interactive := deps
	interactive.cli = nil
	interactive.snapshot.startupExtensionMessages = nil

	if len(deps.snapshot.startupExtensionMessages) != 1 {
		t.Error("original deps.snapshot.startupExtensionMessages was cleared")
	}
	if interactive.snapshot.modelName != "m" {
		t.Error("copy lost snapshot.modelName")
	}
}
