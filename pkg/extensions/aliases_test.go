package extensions_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// internalFiles are the Kit-internal sources whose exported declarations make
// up the extension-facing contract. Every exported type in them must have an
// alias in this package, otherwise an extension author cannot name it from
// outside Kit's module (issue #136).
var internalFiles = []string{
	"events.go",
	"api.go",
	"runner.go",
	"subagent.go",
}

// notExtensionFacing lists exported internal declarations that deliberately
// stay internal. Add to it only when the symbol has no meaning for code that
// tests an extension.
var notExtensionFacing = map[string]bool{
	// Runner bookkeeping: carries a live handler func, never constructed by
	// a test.
	"ShortcutEntry": true,
}

// TestEveryExtensionFacingTypeIsAliased fails when internal/extensions grows a
// new exported type (a new event, result, or config) that pkg/extensions does
// not re-export. Without the alias, a consumer's test would have to import
// internal/extensions, which Go forbids outside this module.
func TestEveryExtensionFacingTypeIsAliased(t *testing.T) {
	internalTypes := exportedTypeNames(t, filepath.Join("..", "..", "internal", "extensions"), internalFiles)
	if len(internalTypes) == 0 {
		t.Fatal("found no exported types in internal/extensions — test is broken")
	}

	aliased := aliasedTypeNames(t, ".")

	for _, name := range internalTypes {
		if notExtensionFacing[name] {
			continue
		}
		if !aliased[name] {
			t.Errorf("internal/extensions.%s has no alias in pkg/extensions — "+
				"add `type %s = internalext.%s` so extension authors can name it, "+
				"or list it in notExtensionFacing if it must stay internal",
				name, name, name)
		}
	}
}

// TestNoAliasPointsAtAMissingType guards the other direction: an alias left
// behind after an internal type is renamed would not compile, but an alias for
// a type that moved to a non-contract file would silently drift out of the
// checked set.
func TestNoAliasPointsAtAMissingType(t *testing.T) {
	internalTypes := map[string]bool{}
	for _, name := range exportedTypeNames(t, filepath.Join("..", "..", "internal", "extensions"), internalFiles) {
		internalTypes[name] = true
	}

	for name := range aliasedTypeNames(t, ".") {
		if !internalTypes[name] {
			t.Errorf("pkg/extensions aliases %s, which no longer lives in one of %v — "+
				"update internalFiles so the drift check still covers it", name, internalFiles)
		}
	}
}

// exportedTypeNames returns the exported type names declared in the given
// files of a package directory.
func exportedTypeNames(t *testing.T, dir string, files []string) []string {
	t.Helper()

	var names []string
	fset := token.NewFileSet()
	for _, name := range files {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("cannot stat %s: %v", path, err)
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !ts.Name.IsExported() {
					continue
				}
				names = append(names, ts.Name.Name)
			}
		}
	}
	return names
}

// aliasedTypeNames returns the names declared as type aliases in the given
// package directory.
func aliasedTypeNames(t *testing.T, dir string) map[string]bool {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", dir, err)
	}

	fset := token.NewFileSet()
	aliases := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !ts.Assign.IsValid() {
					continue
				}
				aliases[ts.Name.Name] = true
			}
		}
	}
	return aliases
}
