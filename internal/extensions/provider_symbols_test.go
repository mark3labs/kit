package extensions

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestEveryProviderDeclarationHasInterpreterSymbol(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "providers.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	symbols := Symbols()["kit/ext/ext"]
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range group.Specs {
			switch value := spec.(type) {
			case *ast.TypeSpec:
				if _, ok := symbols[value.Name.Name]; !ok {
					t.Errorf("missing type %s", value.Name.Name)
				}
			case *ast.ValueSpec:
				for _, name := range value.Names {
					if _, ok := symbols[name.Name]; !ok {
						t.Errorf("missing constant %s", name.Name)
					}
				}
			}
		}
	}
}
