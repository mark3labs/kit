package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/kit/pkg/extensions"
	"github.com/mark3labs/kit/pkg/extensions/test"
)

const fileExplorerFile = "file-explorer.go"
const fileExplorerWidget = "file-explorer:tree"

// explorerTree builds:
//
//	root/
//	  .hidden
//	  b.txt
//	  sub/
//	    inner.go
func explorerTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, p := range []string{".hidden", "b.txt", "sub/inner.go"} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runExplorer(t *testing.T, h *test.Harness, args string) {
	t.Helper()
	runExplorerCtx(t, h, args, h.Context().ToContext())
}

// runExplorerCtx runs /explorer with a caller-built context. The extension
// keeps that context for its key handler, so tests can observe async calls.
func runExplorerCtx(t *testing.T, h *test.Harness, args string, ctx extensions.Context) {
	t.Helper()
	for _, cmd := range h.RegisteredCommands() {
		if cmd.Name == "explorer" {
			if _, err := cmd.Execute(args, ctx); err != nil {
				t.Fatalf("/explorer %s: %v", args, err)
			}
			return
		}
	}
	t.Fatal("command /explorer is not registered")
}

func explorerView(t *testing.T, h *test.Harness) string {
	t.Helper()
	w, ok := h.Context().GetWidget(fileExplorerWidget)
	if !ok {
		t.Fatal("explorer widget is not set")
	}
	if !w.Content.FillHeight || w.Content.RenderSized == nil {
		t.Fatal("explorer widget has no fill renderer")
	}
	return w.Content.RenderSized(100, 20)
}

func explorerKey(t *testing.T, h *test.Harness, key, text string) extensions.EditorKeyAction {
	t.Helper()
	ed := h.Context().EditorConfig
	if ed == nil || ed.HandleKey == nil {
		t.Fatal("explorer editor interceptor is not set")
	}
	return ed.HandleKey(key, text)
}

func TestFileExplorer_NavigateAttachAndClose(t *testing.T) {
	root := explorerTree(t)
	h := test.New(t)
	h.LoadFile(fileExplorerFile)

	// SetEditorText is called off the TUI event loop, so capture it here.
	editorText := make(chan string, 1)
	ctx := h.Context().ToContext()
	ctx.SetEditorText = func(s string) { editorText <- s }
	runExplorerCtx(t, h, root, ctx)
	view := explorerView(t, h)
	if !strings.Contains(view, "sub/") || !strings.Contains(view, "b.txt") {
		t.Fatalf("tree should list sub/ and b.txt:\n%s", view)
	}
	if strings.Contains(view, ".hidden") {
		t.Fatalf("dotfiles should be hidden by default:\n%s", view)
	}
	// Directories sort first, so "sub/" is above "b.txt".
	if strings.Index(view, "sub/") > strings.Index(view, "b.txt") {
		t.Fatalf("directories should sort before files:\n%s", view)
	}

	// Cursor starts on sub/; enter expands it.
	if a := explorerKey(t, h, "enter", ""); a.Type != extensions.EditorKeyConsumed {
		t.Fatalf("enter should be consumed, got %q", a.Type)
	}
	if view = explorerView(t, h); !strings.Contains(view, "inner.go") {
		t.Fatalf("enter on a dir should expand it:\n%s", view)
	}

	// Plain typing must not leak into the prompt; modifier combos pass.
	if a := explorerKey(t, h, "x", ""); a.Type != extensions.EditorKeyConsumed {
		t.Fatalf("printable key should be consumed, got %q", a.Type)
	}
	if a := explorerKey(t, h, "ctrl+w", ""); a.Type != extensions.EditorKeyPassthrough {
		t.Fatalf("ctrl combo should pass through, got %q", a.Type)
	}

	// down → inner.go, then "a" attaches it to the prompt.
	explorerKey(t, h, "down", "")
	explorerKey(t, h, "a", "explain")
	select {
	case got := <-editorText:
		if !strings.HasPrefix(got, "explain @") || !strings.Contains(got, "inner.go") {
			t.Fatalf("editor text = %q, want an @ reference to inner.go", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("'a' should set the editor text")
	}

	// "." shows dotfiles.
	explorerKey(t, h, ".", "")
	if view = explorerView(t, h); !strings.Contains(view, ".hidden") {
		t.Fatalf("'.' should show dotfiles:\n%s", view)
	}

	// esc closes the panel and removes the interceptor.
	explorerKey(t, h, "esc", "")
	if !h.Context().EditorReset {
		t.Fatal("esc should reset the editor interceptor")
	}
	test.AssertWidgetNotSet(t, h, fileExplorerWidget)

	// The toggle opens it again, with the tree state kept, then closes it.
	runExplorer(t, h, "")
	if view = explorerView(t, h); !strings.Contains(view, "inner.go") {
		t.Fatalf("reopen should keep expanded dirs:\n%s", view)
	}
	runExplorer(t, h, "")
	test.AssertWidgetNotSet(t, h, fileExplorerWidget)
}

func TestFileExplorer_BackspaceMovesRootUp(t *testing.T) {
	root := explorerTree(t)
	h := test.New(t)
	h.LoadFile(fileExplorerFile)

	runExplorer(t, h, filepath.Join(root, "sub"))
	if view := explorerView(t, h); !strings.Contains(view, "inner.go") {
		t.Fatalf("root sub/ should list inner.go:\n%s", view)
	}
	explorerKey(t, h, "backspace", "")
	if view := explorerView(t, h); !strings.Contains(view, "b.txt") {
		t.Fatalf("backspace should move the root up:\n%s", view)
	}
}

func TestFileExplorer_RejectsNonDirectory(t *testing.T) {
	root := explorerTree(t)
	h := test.New(t)
	h.LoadFile(fileExplorerFile)

	for _, cmd := range h.RegisteredCommands() {
		if cmd.Name == "explorer" {
			if _, err := cmd.Execute(filepath.Join(root, "b.txt"), h.Context().ToContext()); err == nil {
				t.Fatal("/explorer on a file should fail")
			}
			return
		}
	}
	t.Fatal("command /explorer is not registered")
}
