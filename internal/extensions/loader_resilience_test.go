package extensions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeExt writes an extension source file into a temp dir and returns its path.
func writeExt(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ext.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLoadSurvivesPanicInInit verifies a panicking Init is reported as a load
// error instead of crashing the host process. Before this was handled, a single
// bad extension took down all of Kit with a raw Go stack trace.
func TestLoadSurvivesPanicInInit(t *testing.T) {
	path := writeExt(t, `package main

import ext "kit/ext"

func Init(api ext.API) {
	var m map[string]string
	m["boom"] = "assignment to entry in nil map"
}
`)

	_, err := loadSingleExtension(path)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "panic in Init") {
		t.Errorf("error should identify the panic, got: %v", err)
	}
}

// TestLoadSurvivesPanicAtTopLevel verifies a panic during evaluation of the
// source (not inside Init) is also contained.
func TestLoadSurvivesPanicAtTopLevel(t *testing.T) {
	path := writeExt(t, `package main

import ext "kit/ext"

var boom = []int{1, 2, 3}[99]

func Init(api ext.API) {}
`)

	_, err := loadSingleExtension(path)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// TestLoadGoodExtensionStillWorks is the control: the recovery wrappers must
// not interfere with a well-formed extension.
func TestLoadGoodExtensionStillWorks(t *testing.T) {
	path := writeExt(t, `package main

import ext "kit/ext"

func Init(api ext.API) {
	api.RegisterTool(ext.ToolDef{
		Name:        "noop",
		Description: "does nothing",
		Execute:     func(input string) (string, error) { return "ok", nil },
	})
}
`)

	loaded, err := loadSingleExtension(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded.Tools) != 1 || loaded.Tools[0].Name != "noop" {
		t.Errorf("expected the noop tool to register, got %+v", loaded.Tools)
	}
}

// TestWidgetRenderCallbackCrossesBoundary verifies an extension-supplied
// Render closure survives the Yaegi boundary and returns real content. This
// is the primitive behind arbitrary extension UI, so a silent zero value here
// would be a regression.
func TestWidgetRenderCallbackCrossesBoundary(t *testing.T) {
	path := writeExt(t, `package main

import (
	"fmt"
	ext "kit/ext"
)

func Init(api ext.API) {
	api.RegisterTool(ext.ToolDef{
		Name:        "mk",
		Description: "builds a widget",
		Execute: func(input string) (string, error) { return "", nil },
	})
	api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
		ctx.SetWidget(ext.WidgetConfig{
			ID:        "w",
			Placement: ext.WidgetAbove,
			Content: ext.WidgetContent{
				Render: func(width int) string {
					return fmt.Sprintf("rendered at %d", width)
				},
			},
		})
	})
}
`)

	loaded, err := loadSingleExtension(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handlers := loaded.Handlers[SessionStart]
	if len(handlers) != 1 {
		t.Fatalf("expected 1 session-start handler, got %d", len(handlers))
	}

	var got WidgetConfig
	ctx := Context{SetWidget: func(c WidgetConfig) { got = c }}
	handlers[0](SessionStartEvent{}, ctx)

	if got.Content.Render == nil {
		t.Fatal("Render callback did not cross the interpreter boundary")
	}
	if out := got.Content.Render(42); out != "rendered at 42" {
		t.Errorf("Render returned %q, want %q", out, "rendered at 42")
	}
}
