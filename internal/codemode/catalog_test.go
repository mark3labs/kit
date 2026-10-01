package codemode

import (
	"fmt"
	"strings"
	"testing"
)

func TestSignatureRendering(t *testing.T) {
	e, ok := testCatalog().Lookup("github__list_issues")
	if !ok {
		t.Fatal("lookup failed")
	}
	want := `tools.github.list_issues(args: { repo: string; state?: "open" | "closed" }): Promise<string>`
	if got := e.Signature(); got != want {
		t.Fatalf("signature:\n got %s\nwant %s", got, want)
	}
	if got := e.ShortSignature(); got != "tools.github.list_issues({repo, state?})" {
		t.Fatalf("short signature = %s", got)
	}
}

func TestSchemaToTS(t *testing.T) {
	cases := []struct {
		schema map[string]any
		want   string
	}{
		{map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}, "number[]"},
		{map[string]any{"type": "array", "items": map[string]any{"enum": []any{"a", "b"}}}, `("a" | "b")[]`},
		{map[string]any{"type": []any{"string", "null"}}, "string | null"},
		{map[string]any{"type": "object", "properties": map[string]any{"x-y": map[string]any{"type": "boolean"}}, "required": []any{"x-y"}}, `{ "x-y": boolean }`},
		{map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "number"}}}, "string | number"},
		{map[string]any{}, "any"},
	}
	for _, c := range cases {
		if got := schemaToTS(c.schema, 0); got != c.want {
			t.Errorf("schemaToTS(%v) = %s, want %s", c.schema, got, c.want)
		}
	}
}

func TestNonIdentifierNames(t *testing.T) {
	c := NewCatalog([]ToolSpec{{Name: "my-tool"}, {Name: "srv-1__do it"}})
	e, _ := c.Lookup("my-tool")
	if e.Path() != `tools["my-tool"]` {
		t.Fatalf("path = %s", e.Path())
	}
	e, _ = c.Lookup("srv-1__do it")
	if e.Path() != `tools["srv-1"]["do it"]` {
		t.Fatalf("path = %s", e.Path())
	}
}

func TestRootNamespaceCollision(t *testing.T) {
	c := NewCatalog([]ToolSpec{{Name: "github"}, {Name: "github__issues"}})
	e, _ := c.Lookup("github")
	if e.Path() != "tools.github_tool" {
		t.Fatalf("path = %s", e.Path())
	}
}

func TestRenderCatalogRoundRobinAndBudget(t *testing.T) {
	var specs []ToolSpec
	for i := range 50 {
		specs = append(specs, ToolSpec{Name: fmt.Sprintf("big__tool_%02d", i), Description: "A big server tool.", Exposure: ExposureCodeMode})
	}
	specs = append(specs,
		ToolSpec{Name: "small__only", Description: "The only small tool.", Exposure: ExposureCodeMode},
		ToolSpec{Name: "read", Parameters: map[string]any{"path": map[string]any{"type": "string"}}, Required: []string{"path"}},
		ToolSpec{Name: "hidden__x", Exposure: ExposureDeferred},
	)
	out, omitted := NewCatalog(specs).RenderCatalog(600)
	if len(out) > 600 {
		t.Fatalf("catalog exceeds budget: %d bytes", len(out))
	}
	if !strings.Contains(out, "tools.small.only(") {
		t.Fatalf("small namespace starved:\n%s", out)
	}
	if !strings.Contains(out, "tools.read({path})") {
		t.Fatalf("direct tool must use short signature:\n%s", out)
	}
	if strings.Contains(out, "hidden") {
		t.Fatalf("deferred tool listed:\n%s", out)
	}
	if omitted == 0 {
		t.Fatal("expected omitted tools")
	}
}

func TestPolicy(t *testing.T) {
	rules, err := ParseRules(map[string]string{
		"github__*":            "codemode",
		"github__create_issue": "direct",
		"linear__*":            "deferred",
		"subagent":             "model-only",
	})
	if err != nil {
		t.Fatal(err)
	}
	p := Policy{Rules: rules, MCPDefault: ExposureCodeMode}
	cases := map[string]Exposure{
		"github__list_issues":  ExposureCodeMode,
		"github__create_issue": ExposureDirect,
		"linear__search":       ExposureDeferred,
		"subagent":             ExposureModelOnly,
		"other__tool":          ExposureCodeMode, // MCP default
		"read":                 ExposureDirect,
	}
	for name, want := range cases {
		if got := p.Resolve(name); got != want {
			t.Errorf("Resolve(%s) = %s, want %s", name, got, want)
		}
	}
	if _, err := ParseRules(map[string]string{"x": "bogus"}); err == nil {
		t.Fatal("expected error for unknown exposure")
	}
	if _, err := ParseRules(map[string]string{"[": "direct"}); err == nil {
		t.Fatal("expected error for bad pattern")
	}
}

func TestStoreLimit(t *testing.T) {
	s := NewStore()
	s.limit = 10
	if err := s.Set("a", "12345"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("b", "123456"); err == nil {
		t.Fatal("expected store full error")
	}
	if err := s.Set("a", "123456789"); err != nil {
		t.Fatalf("replace within limit failed: %v", err)
	}
	s.Delete("a")
	if s.bytes != 0 {
		t.Fatalf("bytes = %d after delete", s.bytes)
	}
}
