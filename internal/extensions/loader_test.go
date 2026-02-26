package extensions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverExtensionPaths_ExplicitFile(t *testing.T) {
	// Create a temp dir with a .go file.
	dir := t.TempDir()
	f := filepath.Join(dir, "my-ext.go")
	if err := os.WriteFile(f, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	paths := discoverExtensionPaths([]string{f})
	if len(paths) == 0 {
		t.Fatal("expected at least 1 path")
	}

	abs, _ := filepath.Abs(f)
	found := false
	for _, p := range paths {
		if p == abs {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q in discovered paths %v", abs, paths)
	}
}

func TestDiscoverExtensionPaths_ExplicitDir(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ext.go")
	if err := os.WriteFile(f, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	paths := discoverExtensionPaths([]string{dir})
	abs, _ := filepath.Abs(f)
	found := false
	for _, p := range paths {
		if p == abs {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q in discovered paths %v", abs, paths)
	}
}

func TestDiscoverExtensionPaths_SubdirMainGo(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "my-plugin")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(subdir, "main.go")
	if err := os.WriteFile(main, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	paths := discoverExtensionPaths([]string{dir})
	abs, _ := filepath.Abs(main)
	found := false
	for _, p := range paths {
		if p == abs {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q in discovered paths %v", abs, paths)
	}
}

func TestDiscoverExtensionPaths_Dedup(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ext.go")
	if err := os.WriteFile(f, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	// Pass the same file twice.
	paths := discoverExtensionPaths([]string{f, f})
	count := 0
	abs, _ := filepath.Abs(f)
	for _, p := range paths {
		if p == abs {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected dedup to 1, got %d", count)
	}
}

func TestDiscoverExtensionPaths_NonGoFileIgnored(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "readme.txt")
	if err := os.WriteFile(f, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	paths := discoverExtensionPaths([]string{f})
	for _, p := range paths {
		abs, _ := filepath.Abs(f)
		if p == abs {
			t.Error("non-.go file should not be discovered")
		}
	}
}

func TestDiscoverExtensionPaths_NonexistentIgnored(t *testing.T) {
	paths := discoverExtensionPaths([]string{"/nonexistent/path/ext.go"})
	for _, p := range paths {
		if p == "/nonexistent/path/ext.go" {
			t.Error("nonexistent path should not be discovered")
		}
	}
}

func TestFindExtensionsInDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	results := findExtensionsInDir(dir)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFindExtensionsInDir_NonexistentDir(t *testing.T) {
	results := findExtensionsInDir("/nonexistent/dir")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFindExtensionsInDir_MixedContent(t *testing.T) {
	dir := t.TempDir()

	// .go file at top level
	if err := os.WriteFile(filepath.Join(dir, "ext.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	// non-.go file (should be ignored)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	// subdir with main.go
	sub := filepath.Join(dir, "plugin")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	// subdir without main.go (should be ignored)
	empty := filepath.Join(dir, "empty")
	if err := os.MkdirAll(empty, 0755); err != nil {
		t.Fatal(err)
	}

	results := findExtensionsInDir(dir)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
}

func TestLoadSingleExtension_ValidExtension(t *testing.T) {
	dir := t.TempDir()
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		return nil
	})
	api.OnSessionStart(func(se ext.SessionStartEvent, ctx ext.Context) {
	})
}
`
	f := filepath.Join(dir, "valid.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	ext, err := loadSingleExtension(f)
	if err != nil {
		t.Fatalf("failed to load extension: %v", err)
	}
	if ext.Path != f {
		t.Errorf("expected path %q, got %q", f, ext.Path)
	}
	if len(ext.Handlers[ToolCall]) != 1 {
		t.Errorf("expected 1 ToolCall handler, got %d", len(ext.Handlers[ToolCall]))
	}
	if len(ext.Handlers[SessionStart]) != 1 {
		t.Errorf("expected 1 SessionStart handler, got %d", len(ext.Handlers[SessionStart]))
	}
}

func TestLoadSingleExtension_NoInitFunction(t *testing.T) {
	dir := t.TempDir()
	src := `package main

func Hello() string { return "hi" }
`
	f := filepath.Join(dir, "noinit.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadSingleExtension(f)
	if err == nil {
		t.Fatal("expected error for missing Init function")
	}
}

func TestLoadSingleExtension_SyntaxError(t *testing.T) {
	dir := t.TempDir()
	src := `package main
func Init( { broken }
`
	f := filepath.Join(dir, "broken.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadSingleExtension(f)
	if err == nil {
		t.Fatal("expected error for syntax error")
	}
}

func TestLoadSingleExtension_WrongSignature(t *testing.T) {
	dir := t.TempDir()
	src := `package main

func Init(s string) {}
`
	f := filepath.Join(dir, "wrongsig.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadSingleExtension(f)
	if err == nil {
		t.Fatal("expected error for wrong Init signature")
	}
}

func TestLoadSingleExtension_RegistersTool(t *testing.T) {
	dir := t.TempDir()
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.RegisterTool(ext.ToolDef{
		Name:        "my_tool",
		Description: "does stuff",
		Parameters:  "{\"type\":\"object\"}",
		Execute: func(input string) (string, error) {
			return "result: " + input, nil
		},
	})
}
`
	f := filepath.Join(dir, "toolreg.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	ext, err := loadSingleExtension(f)
	if err != nil {
		t.Fatalf("failed to load extension: %v", err)
	}
	if len(ext.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(ext.Tools))
	}
	if ext.Tools[0].Name != "my_tool" {
		t.Errorf("expected tool name 'my_tool', got %q", ext.Tools[0].Name)
	}
}

func TestLoadSingleExtension_RegistersCommand(t *testing.T) {
	dir := t.TempDir()
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.RegisterCommand(ext.CommandDef{
		Name:        "hello",
		Description: "says hello",
		Execute: func(args string) (string, error) {
			return "hello " + args, nil
		},
	})
}
`
	f := filepath.Join(dir, "cmdreg.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	ext, err := loadSingleExtension(f)
	if err != nil {
		t.Fatalf("failed to load extension: %v", err)
	}
	if len(ext.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(ext.Commands))
	}
	if ext.Commands[0].Name != "hello" {
		t.Errorf("expected command name 'hello', got %q", ext.Commands[0].Name)
	}
}

func TestLoadExtensions_SkipsBadFiles(t *testing.T) {
	dir := t.TempDir()

	// Good extension
	good := `package main
import "kit/ext"
func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, _ ext.Context) {})
}
`
	if err := os.WriteFile(filepath.Join(dir, "good.go"), []byte(good), 0644); err != nil {
		t.Fatal(err)
	}

	// Bad extension (syntax error)
	bad := `package main
func Init( { broken }
`
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadExtensions([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have loaded the good one and skipped the bad one.
	if len(loaded) != 1 {
		t.Fatalf("expected 1 loaded extension, got %d", len(loaded))
	}
}

func TestLoadSingleExtension_HandlerExecution(t *testing.T) {
	dir := t.TempDir()
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if tc.ToolName == "banned" {
			return &ext.ToolCallResult{Block: true, Reason: "tool is banned"}
		}
		return nil
	})
}
`
	f := filepath.Join(dir, "blocker.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	ext, err := loadSingleExtension(f)
	if err != nil {
		t.Fatalf("failed to load extension: %v", err)
	}

	// Build a runner and test the handler actually works.
	r := NewRunner([]LoadedExtension{*ext})
	result, err := r.Emit(ToolCallEvent{ToolName: "banned", Input: "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tcr, ok := result.(ToolCallResult)
	if !ok {
		t.Fatalf("expected ToolCallResult, got %T", result)
	}
	if !tcr.Block {
		t.Error("expected Block=true for banned tool")
	}
	if tcr.Reason != "tool is banned" {
		t.Errorf("expected reason 'tool is banned', got %q", tcr.Reason)
	}

	// Non-banned tool should pass through.
	result2, _ := r.Emit(ToolCallEvent{ToolName: "allowed", Input: "{}"})
	if result2 != nil {
		t.Errorf("expected nil result for allowed tool, got %v", result2)
	}
}

func TestGlobalExtensionsDir_XDG(t *testing.T) {
	// Save and restore XDG_CONFIG_HOME.
	orig := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", orig)

	os.Setenv("XDG_CONFIG_HOME", "/custom/config")
	dir := globalExtensionsDir()
	expected := "/custom/config/kit/extensions"
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestGlobalExtensionsDir_Default(t *testing.T) {
	orig := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", orig)

	os.Setenv("XDG_CONFIG_HOME", "")
	dir := globalExtensionsDir()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "kit", "extensions")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestLoadSingleExtension_ContextPrint(t *testing.T) {
	dir := t.TempDir()
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnInput(func(ie ext.InputEvent, ctx ext.Context) *ext.InputResult {
		if ie.Text == "!hello" && ctx.Print != nil {
			ctx.Print("Hello from extension!")
			return &ext.InputResult{Action: "handled"}
		}
		return nil
	})
}
`
	f := filepath.Join(dir, "printer.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	ext, err := loadSingleExtension(f)
	if err != nil {
		t.Fatalf("failed to load extension: %v", err)
	}

	// Wire up a Print function and verify it's called.
	var printed []string
	r := NewRunner([]LoadedExtension{*ext})
	r.SetContext(Context{
		Print: func(text string) {
			printed = append(printed, text)
		},
	})

	result, err := r.Emit(InputEvent{Text: "!hello", Source: "interactive"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ir, ok := result.(InputResult)
	if !ok {
		t.Fatalf("expected InputResult, got %T", result)
	}
	if ir.Action != "handled" {
		t.Errorf("expected Action 'handled', got %q", ir.Action)
	}
	if len(printed) != 1 || printed[0] != "Hello from extension!" {
		t.Errorf("expected Print to capture 'Hello from extension!', got %v", printed)
	}
}

func TestLoadSingleExtension_ContextPrintInfo(t *testing.T) {
	dir := t.TempDir()
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnInput(func(ie ext.InputEvent, ctx ext.Context) *ext.InputResult {
		if ie.Text == "!info" && ctx.PrintInfo != nil {
			ctx.PrintInfo("Styled info from extension")
			return &ext.InputResult{Action: "handled"}
		}
		if ie.Text == "!error" && ctx.PrintError != nil {
			ctx.PrintError("Styled error from extension")
			return &ext.InputResult{Action: "handled"}
		}
		return nil
	})
}
`
	f := filepath.Join(dir, "styled.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	ext, err := loadSingleExtension(f)
	if err != nil {
		t.Fatalf("failed to load extension: %v", err)
	}

	var infos, errors []string
	r := NewRunner([]LoadedExtension{*ext})
	r.SetContext(Context{
		PrintInfo:  func(text string) { infos = append(infos, text) },
		PrintError: func(text string) { errors = append(errors, text) },
	})

	result, _ := r.Emit(InputEvent{Text: "!info"})
	if ir, ok := result.(InputResult); !ok || ir.Action != "handled" {
		t.Fatal("expected handled result for !info")
	}
	if len(infos) != 1 || infos[0] != "Styled info from extension" {
		t.Errorf("expected PrintInfo capture, got %v", infos)
	}

	result, _ = r.Emit(InputEvent{Text: "!error"})
	if ir, ok := result.(InputResult); !ok || ir.Action != "handled" {
		t.Fatal("expected handled result for !error")
	}
	if len(errors) != 1 || errors[0] != "Styled error from extension" {
		t.Errorf("expected PrintError capture, got %v", errors)
	}
}

func TestLoadSingleExtension_ContextPrintBlock(t *testing.T) {
	dir := t.TempDir()
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnInput(func(ie ext.InputEvent, ctx ext.Context) *ext.InputResult {
		if ie.Text == "!status" && ctx.PrintBlock != nil {
			ctx.PrintBlock(ext.PrintBlockOpts{
				Text:        "All systems go\nModel: " + ctx.Model,
				BorderColor: "#a6e3a1",
				Subtitle:    "test-ext",
			})
			return &ext.InputResult{Action: "handled"}
		}
		return nil
	})
}
`
	f := filepath.Join(dir, "block.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	ext, err := loadSingleExtension(f)
	if err != nil {
		t.Fatalf("failed to load extension: %v", err)
	}

	var captured []PrintBlockOpts
	r := NewRunner([]LoadedExtension{*ext})
	r.SetContext(Context{
		Model: "claude-4",
		PrintBlock: func(opts PrintBlockOpts) {
			captured = append(captured, opts)
		},
	})

	result, _ := r.Emit(InputEvent{Text: "!status", Source: "interactive"})
	if ir, ok := result.(InputResult); !ok || ir.Action != "handled" {
		t.Fatal("expected handled result for !status")
	}
	if len(captured) != 1 {
		t.Fatalf("expected 1 PrintBlock call, got %d", len(captured))
	}
	if captured[0].BorderColor != "#a6e3a1" {
		t.Errorf("expected border '#a6e3a1', got %q", captured[0].BorderColor)
	}
	if captured[0].Subtitle != "test-ext" {
		t.Errorf("expected subtitle 'test-ext', got %q", captured[0].Subtitle)
	}
	// Verify the text includes the model from context.
	if captured[0].Text != "All systems go\nModel: claude-4" {
		t.Errorf("unexpected text: %q", captured[0].Text)
	}
}

func TestCountHandlers(t *testing.T) {
	ext := &LoadedExtension{
		Handlers: map[EventType][]HandlerFunc{
			ToolCall:     {func(Event, Context) Result { return nil }, func(Event, Context) Result { return nil }},
			SessionStart: {func(Event, Context) Result { return nil }},
		},
	}
	if n := countHandlers(ext); n != 3 {
		t.Errorf("expected 3 handlers, got %d", n)
	}
}
