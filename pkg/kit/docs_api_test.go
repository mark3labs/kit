package kit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// sdkDeclarations reads only production declarations owned by the public package.
// Alias members are documented through their owning type's Go reference.
// Include tagged helpers such as ResetForTesting in the index, but label them
// test-only; do not make the inventory depend on the current build tags or OS.
func sdkDeclarations(t *testing.T) (map[string]string, []string, []string) {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	exports := map[string]string{}
	var options, decisions []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		add := func(name string, doc *ast.CommentGroup) {
			if !ast.IsExported(name) {
				return
			}
			if doc == nil || strings.TrimSpace(doc.Text()) == "" {
				t.Errorf("%s: missing public comment for %s", path, name)
			}
			exports[name] = path
			if path == "decide.go" {
				decisions = append(decisions, name)
			}
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				name := d.Name.Name
				if !ast.IsExported(name) {
					continue
				}
				if d.Recv != nil {
					r := d.Recv.List[0].Type
					if p, ok := r.(*ast.StarExpr); ok {
						r = p.X
					}
					id, ok := r.(*ast.Ident)
					if !ok || !ast.IsExported(id.Name) {
						continue
					}
					name = id.Name + "." + name
				} else if d.Type.Results != nil {
					for _, result := range d.Type.Results.List {
						if id, ok := result.Type.(*ast.Ident); ok && id.Name == "Option" {
							options = append(options, name)
						}
					}
				}
				add(name, d.Doc)
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					doc := d.Doc
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Doc != nil {
							doc = s.Doc
						}
						add(s.Name.Name, doc)
						if s.Name.Name == "Options" {
							for _, field := range s.Type.(*ast.StructType).Fields.List {
								for _, n := range field.Names {
									if ast.IsExported(n.Name) {
										options = append(options, n.Name)
									}
								}
							}
						}
					case *ast.ValueSpec:
						if s.Doc != nil {
							doc = s.Doc
						}
						if doc == nil {
							doc = s.Comment
						}
						for _, n := range s.Names {
							add(n.Name, doc)
						}
					}
				}
			}
		}
	}
	return exports, options, decisions
}

// sdkDoc reads a repository documentation file relative to the SDK package.
func sdkDoc(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../..", path))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// sdkTokens collects exact inline-code symbol tokens without substring matching.
func sdkTokens(text string) map[string]bool {
	out := map[string]bool{}
	for _, m := range regexp.MustCompile("`([A-Za-z][A-Za-z0-9_.]*)`").FindAllStringSubmatch(text, -1) {
		out[m[1]] = true
	}
	return out
}

// TestSDKSymbolTokens guards against prefix and receiver-name false positives.
func TestSDKSymbolTokens(t *testing.T) {
	tokens := sdkTokens("`PromptResult` and `Kit.Decide` and `Prompt`")
	if tokens["Prom"] || tokens["Decide"] || !tokens["Prompt"] || !tokens["PromptResult"] || !tokens["Kit.Decide"] {
		t.Fatalf("symbols must match exactly: %v", tokens)
	}
}

// TestSDKDocumentation checks API discovery, guide coverage, examples and local links offline.
func TestSDKDocumentation(t *testing.T) {
	exports, options, decisions := sdkDeclarations(t)
	index := sdkDoc(t, "www/pages/sdk/api-reference.md")
	for name := range exports {
		anchor := "https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#" + name
		if !strings.Contains(index, "[`"+name+"`]("+anchor+")") {
			t.Errorf("API index missing exact entry and Go anchor for %s", name)
		}
	}
	for _, path := range []string{"www/pages/sdk/options.md", "skills/kit-sdk/references/options.md"} {
		tokens := sdkTokens(sdkDoc(t, path))
		for _, name := range options {
			if !tokens[name] {
				t.Errorf("%s: missing option %s", path, name)
			}
		}
	}
	for _, path := range []string{"www/pages/sdk/decisions.md", "skills/kit-sdk/references/decisions.md"} {
		tokens := sdkTokens(sdkDoc(t, path))
		for _, name := range decisions {
			if !tokens[name] {
				t.Errorf("%s: missing decision declaration %s", path, name)
			}
		}
	}
	example := sdkDoc(t, "pkg/kit/decide_example_test.go")
	if strings.Contains(example, "// Output:") {
		t.Error("network-capable decision example must compile without execution")
	}
	for _, path := range []string{"www/pages/sdk/decisions.md", "skills/kit-sdk/references/decisions.md"} {
		if !strings.Contains(sdkDoc(t, path), strings.TrimSpace(example)) {
			t.Errorf("%s: decision example differs from compile-checked source", path)
		}
	}
	nav := sdkDoc(t, "www/tome.config.js")
	for _, page := range []string{"decisions", "authentication", "api-reference"} {
		if !strings.Contains(nav, `"sdk/`+page+`"`) {
			t.Errorf("SDK navigation missing %s", page)
		}
	}
	// Check website-local page targets in SDK guides, not external URLs or
	// section fragments. This stays a focused safeguard, not a Markdown compiler.
	pages, err := filepath.Glob("../../www/pages/sdk/*.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range pages {
		b, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range regexp.MustCompile(`\]\(/([^\s)#]+)(?:#[^\s)]+)?\)`).FindAllStringSubmatch(string(b), -1) {
			target := filepath.Join("../../www/pages", m[1]+".md")
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s: invalid local guide target %s: %v", page, m[1], err)
			}
		}
	}
}
