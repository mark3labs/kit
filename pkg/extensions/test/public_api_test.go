package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/kit/pkg/extensions"
)

// TestPublicSignaturesAvoidInternalTypes fails when an exported declaration of
// this package names a type from internal/extensions. Such a signature cannot
// be satisfied from outside Kit's module: Go refuses the import, so extension
// authors cannot call the harness at all (issue #136). Signatures must use the
// aliases in github.com/mark3labs/kit/pkg/extensions instead.
func TestPublicSignaturesAvoidInternalTypes(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("cannot read package directory: %v", err)
	}

	fset := token.NewFileSet()
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++

		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", name, err)
		}

		internalNames := internalImportNames(file)
		if len(internalNames) == 0 {
			continue
		}
		for _, decl := range file.Decls {
			for _, node := range exportedSignatures(decl) {
				ast.Inspect(node, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					ident, ok := sel.X.(*ast.Ident)
					if !ok || !internalNames[ident.Name] {
						return true
					}
					t.Errorf("%s: exported signature names internal type %s.%s — "+
						"use the alias extensions.%s from pkg/extensions instead",
						fset.Position(sel.Pos()), ident.Name, sel.Sel.Name, sel.Sel.Name)
					return true
				})
			}
		}
	}

	if checked == 0 {
		t.Fatal("found no package sources to check — test is broken")
	}
}

// internalImportNames returns the local names a file uses for the
// internal/extensions package.
func internalImportNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path != "github.com/mark3labs/kit/internal/extensions" {
			continue
		}
		if imp.Name != nil {
			names[imp.Name.Name] = true
			continue
		}
		names["extensions"] = true
	}
	return names
}

// exportedSignatures returns the nodes of a declaration that are part of the
// package's public surface: function signatures and exported struct fields.
// Function bodies are excluded — internal types there are an implementation
// detail no consumer has to name.
func exportedSignatures(decl ast.Decl) []ast.Node {
	var nodes []ast.Node

	switch d := decl.(type) {
	case *ast.FuncDecl:
		if !d.Name.IsExported() {
			return nil
		}
		if d.Recv != nil && !exportedReceiver(d.Recv) {
			return nil
		}
		nodes = append(nodes, d.Type)
	case *ast.GenDecl:
		if d.Tok != token.TYPE {
			return nil
		}
		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || !ts.Name.IsExported() {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				nodes = append(nodes, ts.Type)
				continue
			}
			for _, field := range st.Fields.List {
				if len(field.Names) == 0 {
					nodes = append(nodes, field.Type)
					continue
				}
				for _, name := range field.Names {
					if name.IsExported() {
						nodes = append(nodes, field.Type)
						break
					}
				}
			}
		}
	}

	return nodes
}

// exportedReceiver reports whether a method receiver type is exported.
func exportedReceiver(recv *ast.FieldList) bool {
	if len(recv.List) == 0 {
		return false
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	ident, ok := expr.(*ast.Ident)
	return ok && ident.IsExported()
}

// TestHarnessDrivenWithPublicTypesOnly exercises the harness the way a
// consumer outside Kit's module must: every type it names comes from
// pkg/extensions. It fails to compile if a signature regresses to an internal
// type.
func TestHarnessDrivenWithPublicTypesOnly(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
		ctx.Print("session started")
	})

	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if tc.ToolName == "dangerous" {
			return &ext.ToolCallResult{Block: true, Reason: "not allowed"}
		}
		return nil
	})
}
`

	h := New(t)

	// Each helper below takes a parameter typed with a pkg/extensions alias,
	// so the call compiles only while the harness speaks the public types.
	requireExtensionPath(t, h.LoadString(src, "public-types-ext.go"), "public-types-ext.go")
	requireRunner(t, h.Runner())

	startEvent := asEvent(extensions.SessionStartEvent{SessionID: "test"})
	if _, err := h.Emit(startEvent); err != nil {
		t.Fatalf("emit session start: %v", err)
	}
	AssertPrinted(t, h, "session started")

	eventType := asEventType(extensions.ToolCall)
	if !h.HasHandlers(eventType) {
		t.Fatalf("expected handlers for %q", eventType)
	}

	result, err := h.Emit(extensions.ToolCallEvent{ToolName: "dangerous", Input: "{}"})
	if err != nil {
		t.Fatalf("emit tool call: %v", err)
	}
	AssertBlocked(t, asResult(result), "not allowed")

	blocked, err := h.EmitJSON("dangerous", "{}")
	if err != nil {
		t.Fatalf("emit json: %v", err)
	}
	requireBlocked(t, blocked)
	requireNoRegistrations(t, h.RegisteredTools(), h.RegisteredCommands())
}

// The helpers below pin the harness signatures to the public aliases. Each one
// accepts (or returns) a pkg/extensions type, so a signature that regresses to
// an internal-only type breaks the build of this file.

// requireExtensionPath fails the test unless the loaded extension carries the
// expected path. Its parameter pins Harness.LoadFile and Harness.LoadString to
// the public *extensions.LoadedExtension.
func requireExtensionPath(t *testing.T, loaded *extensions.LoadedExtension, want string) {
	t.Helper()
	if loaded == nil {
		t.Fatal("expected a loaded extension")
	}
	if loaded.Path != want {
		t.Fatalf("unexpected extension path %q, want %q", loaded.Path, want)
	}
}

// requireRunner fails the test when the harness exposes no runner. Its
// parameter pins Harness.Runner to the public *extensions.Runner.
func requireRunner(t *testing.T, runner *extensions.Runner) {
	t.Helper()
	if runner == nil {
		t.Fatal("expected a runner after loading")
	}
}

// requireBlocked fails the test unless the tool call was blocked. Its
// parameter pins Harness.EmitJSON to the public *extensions.ToolCallResult.
func requireBlocked(t *testing.T, result *extensions.ToolCallResult) {
	t.Helper()
	if result == nil || !result.Block {
		t.Fatal("expected the tool call to be blocked")
	}
}

// requireNoRegistrations fails the test when the extension registered a tool
// or a command. Its parameters pin Harness.RegisteredTools and
// Harness.RegisteredCommands to the public []extensions.ToolDef and
// []extensions.CommandDef.
func requireNoRegistrations(t *testing.T, tools []extensions.ToolDef, commands []extensions.CommandDef) {
	t.Helper()
	if len(tools) != 0 || len(commands) != 0 {
		t.Fatalf("expected no tools or commands, got %d/%d", len(tools), len(commands))
	}
}

// asEvent returns the event unchanged. It exists to type the value as the
// public extensions.Event before Harness.Emit receives it, which fails to
// compile if Emit stops accepting the public interface.
func asEvent(event extensions.Event) extensions.Event { return event }

// asEventType returns the event type unchanged. It exists to type the value as
// the public extensions.EventType before Harness.HasHandlers receives it.
func asEventType(eventType extensions.EventType) extensions.EventType { return eventType }

// asResult returns the result unchanged. It exists to type the value as the
// public extensions.Result before an assertion helper receives it, which fails
// to compile if Emit stops returning the public interface.
func asResult(result extensions.Result) extensions.Result { return result }
